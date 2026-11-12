package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func (a *App) extractDeb(ctx context.Context, packagePath, destination string) error {
	commandCtx, cancel := context.WithTimeout(ctx, repositoryCommandTimeout)
	defer cancel()

	dpkgDeb := exec.CommandContext(commandCtx, "dpkg-deb", "--fsys-tarfile", packagePath)
	dpkgDeb.Stderr = os.Stderr
	stdout, err := dpkgDeb.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create dpkg-deb stdout pipe: %w", err)
	}
	if err := dpkgDeb.Start(); err != nil {
		return fmt.Errorf("start dpkg-deb: %w", err)
	}

	found, extractErr := extractRepositoryTarMember(stdout, destination)
	if found {
		cancel()
	}
	waitErr := dpkgDeb.Wait()
	if extractErr != nil {
		return fmt.Errorf("extract deb package member: %w", extractErr)
	}
	if !found {
		if commandCtx.Err() != nil {
			return commandCtx.Err()
		}
		if waitErr != nil {
			return fmt.Errorf("dpkg-deb: %w", waitErr)
		}
		return fmt.Errorf("deb package does not contain %s", ancPackageBinaryRelativePath)
	}
	return nil
}

func extractRepositoryTarMember(tarStream io.Reader, destination string) (bool, error) {
	reader := tar.NewReader(tarStream)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !isANCBinaryTarMember(header.Name) {
			continue
		}
		return true, extractRepositoryANCBinary(reader, header, destination)
	}
}

func isANCBinaryTarMember(name string) bool {
	return strings.TrimPrefix(name, "./") == ancPackageBinaryRelativePath
}

func extractRepositoryANCBinary(reader io.Reader, header *tar.Header, destination string) error {
	if header.Typeflag != tar.TypeReg && header.Typeflag != 0 {
		return newIntegrityError("deb package member %s is not a regular file", header.Name)
	}
	if header.Size > repositoryBinaryMaxBytes {
		return newIntegrityError("deb package member %s exceeds %d bytes", header.Name, repositoryBinaryMaxBytes)
	}
	outputPath, tmp, cleanup, err := createExtractedBinaryTemp(destination)
	if err != nil {
		return err
	}
	success := false
	defer func() { cleanup(success) }()
	copied, copyErr := io.Copy(tmp, io.LimitReader(reader, repositoryBinaryMaxBytes+1))
	if copyErr != nil {
		return fmt.Errorf("copy extracted binary: %w", copyErr)
	}
	if copied > repositoryBinaryMaxBytes {
		return newIntegrityError("deb package member %s exceeds %d bytes", header.Name, repositoryBinaryMaxBytes)
	}
	if header.Size >= 0 && copied != header.Size {
		return fmt.Errorf("short read extracting %s: copied %d of %d bytes", header.Name, copied, header.Size)
	}
	if err := finishExtractedBinary(tmp, outputPath); err != nil {
		return err
	}
	success = true
	return nil
}

type aptRepository struct {
	URI        string
	Suite      string
	Component  string
	SignedBy   []string
	SourcePath string
}

func (a *App) ubuntuRepositoryPlan(info platformInfo, hotfixVersion string) (repositoryDownloadPlan, error) {
	debArch, err := debArchitecture(info.Arch)
	if err != nil {
		return repositoryDownloadPlan{}, err
	}
	sourcePath, err := a.microsoftProdSourceListPath()
	if err != nil {
		return repositoryDownloadPlan{}, newUnsupportedRepositoryError("%v", err)
	}
	repository, err := parseAptRepositoryFile(sourcePath, debArch)
	if err != nil {
		return repositoryDownloadPlan{}, err
	}
	if len(repository.SignedBy) == 0 {
		keyringsDir := a.aptTrustedKeyringsDir
		if keyringsDir == "" {
			keyringsDir = defaultAptTrustedKeyringsDir
		}
		repository.SignedBy, err = microsoftAptTrustedKeyrings(keyringsDir)
		if err != nil {
			return repositoryDownloadPlan{}, err
		}
	}
	origin, err := validateRepositoryURL(repository.URI)
	if err != nil {
		return repositoryDownloadPlan{}, err
	}
	origin = asRepositoryBase(origin)
	if info.VersionID == "" {
		return repositoryDownloadPlan{}, newUnsupportedRepositoryError("Ubuntu VERSION_ID is empty")
	}

	fullVersion := hotfixVersion + "-ubuntu" + info.VersionID + "u1"
	relativePackagePath := fmt.Sprintf(
		"pool/main/a/%s/%s_%s_%s.deb", ancPackageName, ancPackageName, fullVersion, debArch)
	packageURL, err := resolveRepositoryURL(origin, relativePackagePath)
	if err != nil {
		return repositoryDownloadPlan{}, err
	}

	return repositoryDownloadPlan{
		format:        "deb",
		packageURL:    packageURL,
		trustedOrigin: origin,
		resolveMetadata: func(ctx context.Context) (repositoryPackageMetadata, error) {
			return a.resolveUbuntuPackageMetadata(
				ctx, origin, repository, debArch, fullVersion, relativePackagePath)
		},
	}, nil
}

func debArchitecture(goarch string) (string, error) {
	switch goarch {
	case archAMD64:
		return archAMD64, nil
	case archARM64:
		return archARM64, nil
	default:
		return "", newUnsupportedRepositoryError("unsupported Debian architecture %q", goarch)
	}
}

func parseAptRepositoryFile(path, arch string) (aptRepository, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return aptRepository{}, newUnsupportedRepositoryError("read apt source %s: %v", path, err)
	}
	if strings.HasSuffix(path, ".sources") {
		return parseDeb822Repository(string(data), path, arch)
	}
	return parseOneLineAptRepository(string(data), path, arch)
}

// parseAptLineOptions parses the optional bracketed option group of a one-line apt
// entry (e.g. `[arch=amd64 signed-by=/path/key.gpg]`). It returns the parsed options
// and the index of the first field after the group. The bool is false when the group is
// opened but never closed, in which case the caller must skip the line.
func parseAptLineOptions(fields []string, index int) (map[string]string, int, bool, error) {
	options := map[string]string{}
	if !strings.HasPrefix(fields[index], "[") {
		return options, index, true, nil
	}
	end := index
	for end < len(fields) && !strings.HasSuffix(fields[end], "]") {
		end++
	}
	if end >= len(fields) {
		return nil, 0, false, nil
	}
	optionText := strings.Trim(strings.Join(fields[index:end+1], " "), "[]")
	for _, option := range strings.Fields(optionText) {
		keyValue := strings.SplitN(option, "=", 2)
		if len(keyValue) != 2 {
			return nil, 0, false, newUnsupportedRepositoryError(
				"APT source option contains unsupported non-key-value constraint %q", option)
		}
		options[strings.ToLower(keyValue[0])] = keyValue[1]
	}
	return options, end + 1, true, nil
}

// parseOneLineAptEntry parses a single non-comment one-line apt entry. The bool is
// false when the line is not a usable deb entry for arch and must be skipped.
func parseOneLineAptEntry(line, path, arch string) (aptRepository, bool, error) {
	if line == "" || !strings.HasPrefix(line, "deb ") {
		return aptRepository{}, false, nil
	}
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return aptRepository{}, false, nil
	}
	options, index, ok, err := parseAptLineOptions(fields, 1)
	if err != nil {
		return aptRepository{}, false, err
	}
	if !ok || len(fields) < index+3 {
		return aptRepository{}, false, nil
	}
	if configuredArch := options["arch"]; configuredArch != "" &&
		!containsString(strings.Split(configuredArch, ","), arch) {
		return aptRepository{}, false, nil
	}
	signedBy, err := splitConfiguredPaths(options["signed-by"])
	if err != nil {
		return aptRepository{}, false, err
	}
	return aptRepository{
		URI:        fields[index],
		Suite:      fields[index+1],
		Component:  fields[index+2],
		SignedBy:   signedBy,
		SourcePath: path,
	}, true, nil
}

func parseOneLineAptRepository(contents, path, arch string) (aptRepository, error) {
	for _, rawLine := range strings.Split(contents, "\n") {
		line := strings.TrimSpace(strings.SplitN(rawLine, "#", 2)[0])
		repo, ok, err := parseOneLineAptEntry(line, path, arch)
		if err != nil {
			return aptRepository{}, err
		}
		if ok {
			return repo, nil
		}
	}
	return aptRepository{}, newUnsupportedRepositoryError("no usable deb entry in %s", path)
}

func parseDeb822Repository(contents, path, arch string) (aptRepository, error) {
	for _, paragraph := range splitParagraphs(contents) {
		fields := parseDeb822Fields(paragraph)
		if !containsString(strings.Fields(fields["types"]), "deb") ||
			strings.EqualFold(strings.TrimSpace(fields["enabled"]), "no") {
			continue
		}
		architectures := strings.Fields(fields["architectures"])
		if len(architectures) > 0 && !containsString(architectures, arch) {
			continue
		}
		uris := strings.Fields(fields["uris"])
		suites := strings.Fields(fields["suites"])
		components := strings.Fields(fields["components"])
		signedBy, err := splitConfiguredPaths(fields["signed-by"])
		if err != nil {
			return aptRepository{}, err
		}
		if len(uris) == 0 || len(suites) == 0 || len(components) == 0 {
			continue
		}
		return aptRepository{
			URI:        uris[0],
			Suite:      suites[0],
			Component:  components[0],
			SignedBy:   signedBy,
			SourcePath: path,
		}, nil
	}
	return aptRepository{}, newUnsupportedRepositoryError("no usable deb822 entry in %s", path)
}

func microsoftAptTrustedKeyrings(dir string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "microsoft*.gpg"))
	if err != nil {
		return nil, newUnsupportedRepositoryError("scan Microsoft APT keyrings: %v", err)
	}
	if len(paths) == 0 {
		return nil, newUnsupportedRepositoryError(
			"APT source has no Signed-By and %s has no Microsoft keyring", dir)
	}
	return paths, nil
}

func splitParagraphs(contents string) []string {
	contents = strings.ReplaceAll(contents, "\r\n", "\n")
	var paragraphs []string
	var current []string
	for _, line := range strings.Split(contents, "\n") {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				paragraphs = append(paragraphs, strings.Join(current, "\n"))
				current = nil
			}
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		paragraphs = append(paragraphs, strings.Join(current, "\n"))
	}
	return paragraphs
}

func parseDeb822Fields(paragraph string) map[string]string {
	fields := map[string]string{}
	var currentKey string
	for _, line := range strings.Split(paragraph, "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if currentKey != "" {
				fields[currentKey] += " " + strings.TrimSpace(line)
			}
			continue
		}
		keyValue := strings.SplitN(line, ":", 2)
		if len(keyValue) != 2 {
			currentKey = ""
			continue
		}
		currentKey = strings.ToLower(strings.TrimSpace(keyValue[0]))
		fields[currentKey] = strings.TrimSpace(keyValue[1])
	}
	return fields
}

func splitConfiguredPaths(value string) ([]string, error) {
	var paths []string
	for _, field := range strings.Fields(strings.ReplaceAll(value, ",", " ")) {
		if !strings.HasPrefix(field, "/") {
			return nil, newUnsupportedRepositoryError("APT Signed-By contains unsupported non-path constraint %q", field)
		}
		paths = append(paths, field)
	}
	return paths, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
}

// decompressGzipToTemp expands a gzip file into a new temp file under the staging dir,
// bounded by repositoryMetadataMaxBytes so a decompression bomb cannot exhaust the disk.
// The caller owns the returned path.
func (a *App) decompressGzipToTemp(compressedPath, namePattern string) (string, error) {
	compressed, err := os.Open(compressedPath)
	if err != nil {
		return "", fmt.Errorf("open compressed metadata: %w", err)
	}
	defer compressed.Close()
	gzipReader, err := gzip.NewReader(compressed)
	if err != nil {
		return "", newIntegrityError("open metadata gzip: %v", err)
	}
	defer gzipReader.Close()

	output, err := os.CreateTemp(a.repositoryStagingDir(), namePattern)
	if err != nil {
		return "", fmt.Errorf("create decompressed metadata temp file: %w", err)
	}
	outputPath := output.Name()
	success := false
	defer func() {
		_ = output.Close()
		if !success {
			_ = os.Remove(outputPath)
		}
	}()
	size, err := io.Copy(output, io.LimitReader(gzipReader, repositoryMetadataMaxBytes+1))
	if err != nil {
		return "", newIntegrityError("decompress metadata: %v", err)
	}
	if size > repositoryMetadataMaxBytes {
		return "", newIntegrityError("decompressed metadata exceeds size limit")
	}
	if err := output.Close(); err != nil {
		return "", fmt.Errorf("close decompressed metadata: %w", err)
	}
	success = true
	return outputPath, nil
}

// packagesVariant is a candidate encoding of the Packages index, named as the InRelease
// indexes it (suite-relative).
type packagesVariant struct {
	suiteRelativePath string
	gzipped           bool
}

// selectPackagesVariant picks the Packages encoding to fetch. The gzipped index is ~6x
// smaller (720 KB vs 4.2 MB for jammy/main/binary-amd64), and apt itself fetches the
// compressed form, so preferring it keeps the fast path from being heavier on the wire than
// the package-manager path it is meant to beat. Integrity is unaffected: the InRelease
// signs a SHA-256 for each encoding, and we verify the bytes we actually downloaded.
// Repositories that publish only the plain index still work.
func selectPackagesVariant(releasePayload []byte, base string) packagesVariant {
	gzipped := base + ".gz"
	if _, _, err := parseReleaseSHA256(releasePayload, gzipped); err == nil {
		return packagesVariant{suiteRelativePath: gzipped, gzipped: true}
	}
	return packagesVariant{suiteRelativePath: base}
}

func (a *App) resolveUbuntuPackageMetadata(
	ctx context.Context,
	origin *url.URL,
	repository aptRepository,
	arch string,
	fullVersion string,
	expectedPackagePath string,
) (repositoryPackageMetadata, error) {
	inReleaseURL, err := resolveRepositoryURL(origin,
		filepath.ToSlash(filepath.Join("dists", repository.Suite, "InRelease")))
	if err != nil {
		return repositoryPackageMetadata{}, err
	}
	// An InRelease indexes its checksum entries relative to the suite directory that
	// contains it (e.g. "main/binary-amd64/Packages"), while the download URL needs the
	// full repository-root-relative path. Keep the two separate: the suite-relative form
	// is what parseReleaseSHA256 must match against.
	packagesBaseSuiteRelative := filepath.ToSlash(filepath.Join(
		repository.Component, "binary-"+arch, "Packages"))

	inRelease, err := a.downloadRepositoryFile(ctx, inReleaseURL, origin, repositoryMetadataMaxBytes)
	if err != nil {
		return repositoryPackageMetadata{}, fmt.Errorf("download InRelease: %w", err)
	}
	defer os.Remove(inRelease.path)
	if err = a.verifyRepoSignature(ctx, inRelease.path, "", repository.SignedBy); err != nil {
		return repositoryPackageMetadata{}, err
	}

	releasePayload, err := readClearSignedPayload(inRelease.path)
	if err != nil {
		return repositoryPackageMetadata{}, newIntegrityError("parse authenticated InRelease: %v", err)
	}

	variant := selectPackagesVariant(releasePayload, packagesBaseSuiteRelative)
	expectedPackagesSHA, expectedPackagesSize, err := parseReleaseSHA256(
		releasePayload, variant.suiteRelativePath)
	if err != nil {
		return repositoryPackageMetadata{}, newIntegrityError("%v", err)
	}
	packagesURL, err := resolveRepositoryURL(origin, filepath.ToSlash(filepath.Join(
		"dists", repository.Suite, variant.suiteRelativePath)))
	if err != nil {
		return repositoryPackageMetadata{}, err
	}

	packages, err := a.downloadRepositoryFile(ctx, packagesURL, origin, repositoryMetadataMaxBytes)
	if err != nil {
		return repositoryPackageMetadata{}, fmt.Errorf("download Packages: %w", err)
	}
	defer os.Remove(packages.path)
	// Verify the downloaded encoding against its own signed entry, before decompressing.
	if packages.size != expectedPackagesSize || !strings.EqualFold(packages.sha256, expectedPackagesSHA) {
		return repositoryPackageMetadata{}, newIntegrityError(
			"Packages metadata mismatch for %s: expected size/SHA256 %d/%s, got %d/%s",
			variant.suiteRelativePath, expectedPackagesSize, expectedPackagesSHA,
			packages.size, packages.sha256)
	}

	packagesPath := packages.path
	if variant.gzipped {
		decompressed, decErr := a.decompressGzipToTemp(
			packages.path, ".aks-node-controller-packages-*")
		if decErr != nil {
			return repositoryPackageMetadata{}, decErr
		}
		defer os.Remove(decompressed)
		packagesPath = decompressed
	}

	packageSHA, err := parseDebPackageMetadata(
		packagesPath, fullVersion, arch, expectedPackagePath)
	if err != nil {
		return repositoryPackageMetadata{}, err
	}
	return repositoryPackageMetadata{sha256: packageSHA}, nil
}

func readClearSignedPayload(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	const begin = "-----BEGIN PGP SIGNED MESSAGE-----"
	const signature = "-----BEGIN PGP SIGNATURE-----"
	if !strings.HasPrefix(text, begin) {
		return nil, fmt.Errorf("missing clear-signed message header")
	}
	headerEnd := strings.Index(text, "\n\n")
	signatureStart := strings.Index(text, "\n"+signature)
	if headerEnd < 0 || signatureStart < 0 || signatureStart <= headerEnd {
		return nil, fmt.Errorf("malformed clear-signed message")
	}
	payload := text[headerEnd+2 : signatureStart]
	var unescaped []string
	for _, line := range strings.Split(payload, "\n") {
		unescaped = append(unescaped, strings.TrimPrefix(line, "- "))
	}
	return []byte(strings.Join(unescaped, "\n")), nil
}

func parseReleaseSHA256(payload []byte, expectedPath string) (string, int64, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(payload)))
	inSHA256 := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "SHA256:" {
			inSHA256 = true
			continue
		}
		if inSHA256 && line != "" && line[0] != ' ' && line[0] != '\t' {
			break
		}
		if !inSHA256 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 || filepath.ToSlash(fields[2]) != expectedPath {
			continue
		}
		size, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || size < 0 {
			return "", 0, fmt.Errorf("invalid size for %s in InRelease", expectedPath)
		}
		if !isSHA256Hex(fields[0]) {
			return "", 0, fmt.Errorf("invalid SHA256 for %s in InRelease", expectedPath)
		}
		return strings.ToLower(fields[0]), size, nil
	}
	if err := scanner.Err(); err != nil {
		return "", 0, err
	}
	return "", 0, fmt.Errorf("InRelease has no SHA256 entry for %s", expectedPath)
}

// matchDebPackageStanza reports whether a parsed Packages stanza is the exact ANC
// package being sought. The bool is true once the identifying fields line up, at which
// point the stanza is authoritative: a location or checksum problem is an integrity
// failure rather than a reason to keep scanning.
func matchDebPackageStanza(stanza map[string]string, fullVersion, arch, expectedLocation string) (string, bool, error) {
	if stanza["Package"] != ancPackageName ||
		stanza["Version"] != fullVersion ||
		stanza["Architecture"] != arch {
		return "", false, nil
	}
	if stanza["Filename"] != expectedLocation {
		return "", true, newIntegrityError(
			"authenticated Packages location %q does not match deterministic path %q",
			stanza["Filename"], expectedLocation)
	}
	sum := strings.ToLower(strings.TrimSpace(stanza["SHA256"]))
	if !isSHA256Hex(sum) {
		return "", true, newIntegrityError("package stanza has no valid SHA256")
	}
	return sum, true, nil
}

// accumulateDebStanzaLine folds one Packages line into stanza. It returns true when a
// blank line ends the current stanza; continuation lines (leading space/tab) are ignored.
func accumulateDebStanzaLine(stanza map[string]string, line string) bool {
	if strings.TrimSpace(line) == "" {
		return true
	}
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	if keyValue := strings.SplitN(line, ":", 2); len(keyValue) == 2 {
		stanza[keyValue[0]] = strings.TrimSpace(keyValue[1])
	}
	return false
}

func parseDebPackageMetadata(path, fullVersion, arch, expectedLocation string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open Packages metadata: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	stanza := map[string]string{}
	for scanner.Scan() {
		if !accumulateDebStanzaLine(stanza, scanner.Text()) {
			continue
		}
		if sum, matched, matchErr := matchDebPackageStanza(stanza, fullVersion, arch, expectedLocation); matched || matchErr != nil {
			return sum, matchErr
		}
		stanza = map[string]string{}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan Packages metadata: %w", err)
	}
	// The final stanza may not be terminated by a trailing blank line.
	if sum, matched, matchErr := matchDebPackageStanza(stanza, fullVersion, arch, expectedLocation); matched || matchErr != nil {
		return sum, matchErr
	}
	return "", newUnsupportedRepositoryError(
		"authenticated Packages metadata has no exact %s %s %s stanza",
		ancPackageName, fullVersion, arch)
}
