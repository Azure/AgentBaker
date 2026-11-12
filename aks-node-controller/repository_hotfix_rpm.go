package main

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (a *App) extractRPM(ctx context.Context, packagePath, destination string) error {
	commandCtx, cancel := context.WithTimeout(ctx, repositoryCommandTimeout)
	defer cancel()

	pipes, err := newRPMExtractionPipes()
	if err != nil {
		return err
	}
	defer pipes.close()

	outputPath, tmp, cleanup, err := createExtractedBinaryTemp(destination)
	if err != nil {
		return err
	}
	success := false
	defer func() { cleanup(success) }()

	rpmErrCh, cpioErrCh := a.startRPMExtractionCommands(commandCtx, packagePath, pipes)
	copied, copyErr := io.Copy(tmp, io.LimitReader(pipes.outputReader, repositoryBinaryMaxBytes+1))
	if copyErr != nil || copied > repositoryBinaryMaxBytes {
		cancel()
		_ = pipes.outputReader.Close()
		_ = pipes.inputReader.Close()
	}
	rpmErr := <-rpmErrCh
	cpioErr := <-cpioErrCh
	if copied > repositoryBinaryMaxBytes {
		return newIntegrityError(
			"rpm package member %s exceeds %d bytes", ancPackageBinaryRelativePath, repositoryBinaryMaxBytes)
	}
	if copyErr != nil {
		return fmt.Errorf("copy extracted rpm package member: %w", copyErr)
	}
	if err := preferredRPMExtractionError(commandCtx.Err(), rpmErr, cpioErr); err != nil {
		return err
	}
	if copied == 0 {
		return fmt.Errorf("rpm package does not contain %s", ancPackageBinaryRelativePath)
	}
	if err := finishExtractedBinary(tmp, outputPath); err != nil {
		return err
	}
	success = true
	return nil
}

type rpmExtractionPipes struct {
	inputReader  *os.File
	inputWriter  *os.File
	outputReader *os.File
	outputWriter *os.File
}

func newRPMExtractionPipes() (rpmExtractionPipes, error) {
	inputReader, inputWriter, err := os.Pipe()
	if err != nil {
		return rpmExtractionPipes{}, fmt.Errorf("create rpm extraction pipe: %w", err)
	}
	outputReader, outputWriter, err := os.Pipe()
	if err != nil {
		_ = inputReader.Close()
		_ = inputWriter.Close()
		return rpmExtractionPipes{}, fmt.Errorf("create rpm member output pipe: %w", err)
	}
	return rpmExtractionPipes{
		inputReader:  inputReader,
		inputWriter:  inputWriter,
		outputReader: outputReader,
		outputWriter: outputWriter,
	}, nil
}

func (p rpmExtractionPipes) close() {
	_ = p.inputReader.Close()
	_ = p.inputWriter.Close()
	_ = p.outputReader.Close()
	_ = p.outputWriter.Close()
}

func (a *App) startRPMExtractionCommands(
	ctx context.Context,
	packagePath string,
	pipes rpmExtractionPipes,
) (<-chan error, <-chan error) {
	rpm2cpio := exec.CommandContext(ctx, "rpm2cpio", packagePath)
	rpm2cpio.Stdout = pipes.inputWriter
	rpm2cpio.Stderr = os.Stderr
	cpio := exec.CommandContext(ctx, "cpio", "-i", "--to-stdout", "--quiet", "./"+ancPackageBinaryRelativePath)
	cpio.Stdin = pipes.inputReader
	cpio.Stdout = pipes.outputWriter
	cpio.Stderr = os.Stderr

	rpmErrCh := make(chan error, 1)
	cpioErrCh := make(chan error, 1)
	go func() {
		err := a.cmdRun(rpm2cpio)
		_ = pipes.inputWriter.Close()
		rpmErrCh <- err
	}()
	go func() {
		err := a.cmdRun(cpio)
		_ = pipes.outputWriter.Close()
		_ = pipes.inputReader.Close()
		cpioErrCh <- err
	}()
	return rpmErrCh, cpioErrCh
}

func (a *App) verifyRPMPackage(ctx context.Context, packagePath string) error {
	if a.verifyRPMPackageSignature != nil {
		return a.verifyRPMPackageSignature(ctx, packagePath)
	}
	commandCtx, cancel := context.WithTimeout(ctx, repositoryCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(
		commandCtx,
		"rpmkeys",
		"--define", "_pkgverify_level signature",
		"--checksig",
		"--verbose",
		packagePath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := a.cmdRun(cmd); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return newUnsupportedRepositoryError("rpmkeys is not installed: %v", err)
		}
		if isRepositoryCancellationError(commandCtx.Err()) {
			return commandCtx.Err()
		}
		return newIntegrityError("RPM package signature verification failed: %v", err)
	}
	return nil
}

func preferredRPMExtractionError(ctxErr, rpmErr, cpioErr error) error {
	var errs []error
	if ctxErr != nil {
		errs = append(errs, fmt.Errorf("rpm extraction cancelled: %w", ctxErr))
	}
	if rpmErr != nil && !isRepositoryCancellationError(rpmErr) {
		errs = append(errs, fmt.Errorf("rpm2cpio: %w", rpmErr))
	}
	if cpioErr != nil && !isRepositoryCancellationError(cpioErr) {
		errs = append(errs, fmt.Errorf("cpio: %w", cpioErr))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if rpmErr != nil {
		errs = append(errs, fmt.Errorf("rpm2cpio: %w", rpmErr))
	}
	if cpioErr != nil {
		errs = append(errs, fmt.Errorf("cpio: %w", cpioErr))
	}
	return errors.Join(errs...)
}

type rpmRepository struct {
	BaseURL  string
	GPGKeys  []string
	FilePath string
	Section  string
}

func (a *App) rpmRepositoryPlan(info platformInfo, hotfixVersion string) (repositoryDownloadPlan, error) {
	rpmArch, err := rpmArchitecture(info.Arch)
	if err != nil {
		return repositoryDownloadPlan{}, err
	}
	releaseSuffix, err := rpmReleaseSuffix(info)
	if err != nil {
		return repositoryDownloadPlan{}, err
	}
	reposDir := a.yumReposDir
	if reposDir == "" {
		reposDir = defaultYumReposDir
	}
	repository, err := parseMSOSSRepository(reposDir)
	if err != nil {
		return repositoryDownloadPlan{}, err
	}
	releaseVersion := rpmReleaseVersion(info.VersionID)
	baseURL := strings.ReplaceAll(repository.BaseURL, "$releasever", releaseVersion)
	baseURL = strings.ReplaceAll(baseURL, "${releasever}", releaseVersion)
	baseURL = strings.ReplaceAll(baseURL, "$basearch", rpmArch)
	baseURL = strings.ReplaceAll(baseURL, "${basearch}", rpmArch)
	if strings.Contains(baseURL, "$") {
		return repositoryDownloadPlan{}, newUnsupportedRepositoryError(
			"unsupported variable in Microsoft repository baseurl %q", repository.BaseURL)
	}
	origin, err := validateRepositoryURL(baseURL)
	if err != nil {
		return repositoryDownloadPlan{}, err
	}
	origin = asRepositoryBase(origin)

	expectedRelease := "1." + releaseSuffix
	relativePackagePath := fmt.Sprintf(
		"Packages/a/%s-%s-%s.%s.rpm", ancPackageName, hotfixVersion, expectedRelease, rpmArch)
	packageURL, err := resolveRepositoryURL(origin, relativePackagePath)
	if err != nil {
		return repositoryDownloadPlan{}, err
	}
	return repositoryDownloadPlan{
		format:                 "rpm",
		packageURL:             packageURL,
		trustedOrigin:          origin,
		verifyPackageSignature: a.verifyRPMPackage,
		resolveMetadata: func(ctx context.Context) (repositoryPackageMetadata, error) {
			return a.resolveRPMPackageMetadata(
				ctx, origin, repository.GPGKeys, hotfixVersion, expectedRelease, rpmArch, relativePackagePath)
		},
	}, nil
}

func rpmArchitecture(goarch string) (string, error) {
	switch goarch {
	case archAMD64:
		return "x86_64", nil
	case archARM64:
		return "aarch64", nil
	default:
		return "", newUnsupportedRepositoryError("unsupported RPM architecture %q", goarch)
	}
}

// rpmReleaseVersion reduces an os-release VERSION_ID to the major.minor form that PMC
// publishes repositories under. Azure Linux nodes can report a three-part
// VERSION_ID that includes a build date (e.g. "3.0.20260304"), while the repository lives
// at .../azurelinux/3.0/prod/... -- substituting the raw value into $releasever builds a
// URL that 404s, silently costing every such node the fast path. Values already in
// major.minor form, or with no dot at all, are returned unchanged.
func rpmReleaseVersion(versionID string) string {
	parts := strings.Split(versionID, ".")
	if len(parts) < 2 {
		return versionID
	}
	return parts[0] + "." + parts[1]
}

func rpmReleaseSuffix(info platformInfo) (string, error) {
	major := strings.SplitN(info.VersionID, ".", 2)[0]
	switch {
	case info.ID == osReleaseIDAzureLinux && major == "3":
		return "azl3", nil
	default:
		return "", newUnsupportedRepositoryError(
			"cannot establish ANC RPM release suffix for %s %s", info.ID, info.VersionID)
	}
}

// matchesMicrosoftRPMRepo reports whether an INI section names the repository that carries
// the ANC package. Azure Linux publishes it under "ms-oss" (section
// [azurelinux-official-ms-oss], baseurl .../prod/ms-oss/$basearch). The "Microsoft" spelling
// is also accepted because PMC uses it for some repository layouts; comparisons are
// lowercased so a capitalised "/Microsoft/" baseurl still matches.
func matchesMicrosoftRPMRepo(name, baseURL string) bool {
	matchers := []struct {
		section string
		urlPath string
	}{
		{section: "ms-oss", urlPath: "/ms-oss/"},
		{section: "microsoft", urlPath: "/microsoft/"},
	}
	lowerName := strings.ToLower(name)
	lowerURL := strings.ToLower(baseURL)
	for _, matcher := range matchers {
		if strings.Contains(lowerName, matcher.section) || strings.Contains(lowerURL, matcher.urlPath) {
			return true
		}
	}
	return false
}

func parseMSOSSRepository(reposDir string) (rpmRepository, error) {
	paths, err := filepath.Glob(filepath.Join(reposDir, "*.repo"))
	if err != nil {
		return rpmRepository{}, newUnsupportedRepositoryError("scan RPM repos: %v", err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sections := parseINISections(string(data))
		for name, values := range sections {
			if !matchesMicrosoftRPMRepo(name, values["baseurl"]) {
				continue
			}
			if strings.TrimSpace(values["enabled"]) == "0" {
				continue
			}
			keys, err := localGPGKeyPaths(values["gpgkey"])
			if err != nil {
				return rpmRepository{}, err
			}
			baseURL := strings.Fields(values["baseurl"])
			if len(baseURL) == 0 {
				continue
			}
			return rpmRepository{
				BaseURL:  baseURL[0],
				GPGKeys:  keys,
				FilePath: path,
				Section:  name,
			}, nil
		}
	}
	return rpmRepository{}, newUnsupportedRepositoryError(
		"no enabled Microsoft-published RPM repository in %s", reposDir)
}

func parseINISections(contents string) map[string]map[string]string {
	sections := map[string]map[string]string{}
	var current map[string]string
	for _, rawLine := range strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			current = map[string]string{}
			sections[name] = current
			continue
		}
		if current == nil {
			continue
		}
		keyValue := strings.SplitN(line, "=", 2)
		if len(keyValue) == 2 {
			current[strings.ToLower(strings.TrimSpace(keyValue[0]))] = strings.TrimSpace(keyValue[1])
		}
	}
	return sections
}

func localGPGKeyPaths(value string) ([]string, error) {
	var paths []string
	for _, field := range strings.Fields(value) {
		u, err := url.Parse(field)
		if err != nil {
			return nil, newUnsupportedRepositoryError("parse RPM gpgkey %q: %v", field, err)
		}
		switch {
		case u.Scheme == "file" && u.Path != "":
			paths = append(paths, u.Path)
		case u.Scheme == "" && strings.HasPrefix(field, "/"):
			paths = append(paths, field)
		default:
			return nil, newUnsupportedRepositoryError(
				"RPM gpgkey %q is not an installed local key", field)
		}
	}
	if len(paths) == 0 {
		return nil, newUnsupportedRepositoryError("Microsoft repository has no local gpgkey")
	}
	return paths, nil
}

type rpmRepoMD struct {
	Data []struct {
		Type         string      `xml:"type,attr"`
		Checksum     rpmChecksum `xml:"checksum"`
		OpenChecksum rpmChecksum `xml:"open-checksum"`
		Location     struct {
			Href string `xml:"href,attr"`
		} `xml:"location"`
		Size     int64 `xml:"size"`
		OpenSize int64 `xml:"open-size"`
	} `xml:"data"`
}

type rpmChecksum struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type rpmPrimaryPackage struct {
	Name    string `xml:"name"`
	Arch    string `xml:"arch"`
	Version struct {
		Ver string `xml:"ver,attr"`
		Rel string `xml:"rel,attr"`
	} `xml:"version"`
	Checksum rpmChecksum `xml:"checksum"`
	Location struct {
		Href string `xml:"href,attr"`
	} `xml:"location"`
}

// verifiedPrimaryReference downloads repomd.xml and its detached signature, verifies the
// signature against keyrings, and returns the authenticated reference to primary metadata.
// Both downloaded files are removed before returning; only the parsed reference escapes.
func (a *App) verifiedPrimaryReference(
	ctx context.Context,
	origin *url.URL,
	keyrings []string,
) (primaryMetadataReference, error) {
	repomdURL, err := resolveRepositoryURL(origin, "repodata/repomd.xml")
	if err != nil {
		return primaryMetadataReference{}, err
	}
	signatureURL, err := resolveRepositoryURL(origin, "repodata/repomd.xml.asc")
	if err != nil {
		return primaryMetadataReference{}, err
	}
	repomd, err := a.downloadRepositoryFile(ctx, repomdURL, origin, repositoryMetadataMaxBytes)
	if err != nil {
		return primaryMetadataReference{}, fmt.Errorf("download repomd.xml: %w", err)
	}
	defer os.Remove(repomd.path)
	signature, err := a.downloadRepositoryFile(ctx, signatureURL, origin, 1<<20)
	if err != nil {
		return primaryMetadataReference{}, fmt.Errorf("download repomd.xml.asc: %w", err)
	}
	defer os.Remove(signature.path)
	if err = a.verifyRepoSignature(ctx, repomd.path, signature.path, keyrings); err != nil {
		return primaryMetadataReference{}, err
	}
	return parsePrimaryReference(repomd.path)
}

// verifyPrimaryPayload checks the downloaded primary metadata against the authenticated
// reference and returns the path to its uncompressed XML. The returned cleanup func must
// be called by the caller; it removes any file this function created or downloaded.
func (a *App) verifyPrimaryPayload(
	primary primaryMetadataReference,
	primaryFile downloadedRepositoryFile,
) (string, func(), error) {
	cleanup := func() { os.Remove(primaryFile.path) }
	if primary.size > 0 && primary.size != primaryFile.size {
		return "", cleanup, newIntegrityError(
			"primary metadata size mismatch: expected %d, got %d", primary.size, primaryFile.size)
	}
	if !strings.EqualFold(primary.checksum, primaryFile.sha256) {
		return "", cleanup, newIntegrityError(
			"primary metadata SHA-256 mismatch: expected %s, got %s",
			primary.checksum, primaryFile.sha256)
	}

	switch {
	case strings.HasSuffix(primary.location, ".gz"):
		decompressed, decErr := a.decompressPrimaryMetadata(primaryFile.path, primary)
		if decErr != nil {
			return "", cleanup, decErr
		}
		return decompressed, func() {
			os.Remove(decompressed)
			os.Remove(primaryFile.path)
		}, nil
	case strings.HasSuffix(primary.location, ".xml"):
		if primary.openSize > 0 && primary.openSize != primaryFile.size {
			return "", cleanup, newIntegrityError(
				"primary metadata open-size mismatch: expected %d, got %d",
				primary.openSize, primaryFile.size)
		}
		if primary.openChecksum != "" &&
			!strings.EqualFold(primary.openChecksum, primaryFile.sha256) {
			return "", cleanup, newIntegrityError(
				"primary metadata open-checksum mismatch: expected %s, got %s",
				primary.openChecksum, primaryFile.sha256)
		}
		return primaryFile.path, cleanup, nil
	default:
		return "", cleanup, newUnsupportedRepositoryError(
			"unsupported primary metadata compression for %q", primary.location)
	}
}

func (a *App) resolveRPMPackageMetadata(
	ctx context.Context,
	origin *url.URL,
	keyrings []string,
	version, release, arch, expectedLocation string,
) (repositoryPackageMetadata, error) {
	primary, err := a.verifiedPrimaryReference(ctx, origin, keyrings)
	if err != nil {
		return repositoryPackageMetadata{}, err
	}
	primaryURL, err := resolveRepositoryURL(origin, primary.location)
	if err != nil {
		return repositoryPackageMetadata{}, err
	}
	primaryFile, err := a.downloadRepositoryFile(ctx, primaryURL, origin, repositoryMetadataMaxBytes)
	if err != nil {
		return repositoryPackageMetadata{}, fmt.Errorf("download primary metadata: %w", err)
	}
	xmlPath, cleanup, err := a.verifyPrimaryPayload(primary, primaryFile)
	defer cleanup()
	if err != nil {
		return repositoryPackageMetadata{}, err
	}
	packageSHA, err := parseRPMPrimaryMetadata(
		xmlPath, version, release, arch, expectedLocation)
	if err != nil {
		return repositoryPackageMetadata{}, err
	}
	return repositoryPackageMetadata{sha256: packageSHA}, nil
}

type primaryMetadataReference struct {
	location     string
	checksum     string
	size         int64
	openChecksum string
	openSize     int64
}

func parsePrimaryReference(path string) (primaryMetadataReference, error) {
	file, err := os.Open(path)
	if err != nil {
		return primaryMetadataReference{}, fmt.Errorf("open repomd.xml: %w", err)
	}
	defer file.Close()
	var metadata rpmRepoMD
	if err := xml.NewDecoder(file).Decode(&metadata); err != nil {
		return primaryMetadataReference{}, newIntegrityError("parse authenticated repomd.xml: %v", err)
	}
	for _, data := range metadata.Data {
		if data.Type != "primary" {
			continue
		}
		if !strings.EqualFold(data.Checksum.Type, "sha256") ||
			!isSHA256Hex(strings.TrimSpace(data.Checksum.Value)) {
			return primaryMetadataReference{}, newUnsupportedRepositoryError(
				"primary metadata does not provide a SHA-256 checksum")
		}
		ref := primaryMetadataReference{
			location: strings.TrimSpace(data.Location.Href),
			checksum: strings.ToLower(strings.TrimSpace(data.Checksum.Value)),
			size:     data.Size,
			openSize: data.OpenSize,
		}
		if strings.TrimSpace(data.OpenChecksum.Value) != "" {
			if !strings.EqualFold(data.OpenChecksum.Type, "sha256") ||
				!isSHA256Hex(strings.TrimSpace(data.OpenChecksum.Value)) {
				return primaryMetadataReference{}, newUnsupportedRepositoryError(
					"primary metadata open-checksum is not SHA-256")
			}
			ref.openChecksum = strings.ToLower(strings.TrimSpace(data.OpenChecksum.Value))
		}
		if ref.location == "" {
			return primaryMetadataReference{}, newIntegrityError("primary metadata location is empty")
		}
		return ref, nil
	}
	return primaryMetadataReference{}, newUnsupportedRepositoryError("repomd.xml has no primary metadata")
}

func (a *App) decompressPrimaryMetadata(
	compressedPath string,
	primary primaryMetadataReference,
) (string, error) {
	compressed, err := os.Open(compressedPath)
	if err != nil {
		return "", fmt.Errorf("open compressed primary metadata: %w", err)
	}
	defer compressed.Close()
	gzipReader, err := gzip.NewReader(compressed)
	if err != nil {
		return "", newIntegrityError("open primary metadata gzip: %v", err)
	}
	defer gzipReader.Close()
	output, err := os.CreateTemp(a.repositoryStagingDir(), ".aks-node-controller-primary-*")
	if err != nil {
		return "", fmt.Errorf("create primary metadata temp file: %w", err)
	}
	outputPath := output.Name()
	success := false
	defer func() {
		_ = output.Close()
		if !success {
			_ = os.Remove(outputPath)
		}
	}()
	hasher := sha256.New()
	size, err := io.Copy(
		io.MultiWriter(output, hasher),
		io.LimitReader(gzipReader, repositoryMetadataMaxBytes+1))
	if err != nil {
		return "", newIntegrityError("decompress primary metadata: %v", err)
	}
	if size > repositoryMetadataMaxBytes {
		return "", newIntegrityError("decompressed primary metadata exceeds size limit")
	}
	if err := output.Close(); err != nil {
		return "", fmt.Errorf("close decompressed primary metadata: %w", err)
	}
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if primary.openSize > 0 && primary.openSize != size {
		return "", newIntegrityError(
			"primary metadata open-size mismatch: expected %d, got %d", primary.openSize, size)
	}
	if primary.openChecksum != "" && !strings.EqualFold(primary.openChecksum, actualChecksum) {
		return "", newIntegrityError(
			"primary metadata open-checksum mismatch: expected %s, got %s",
			primary.openChecksum, actualChecksum)
	}
	success = true
	return outputPath, nil
}

func parseRPMPrimaryMetadata(path, version, release, arch, expectedLocation string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open primary metadata: %w", err)
	}
	defer file.Close()
	decoder := xml.NewDecoder(file)
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", newIntegrityError("parse primary metadata: %v", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "package" {
			continue
		}
		var pkg rpmPrimaryPackage
		if err := decoder.DecodeElement(&pkg, &start); err != nil {
			return "", newIntegrityError("parse primary package: %v", err)
		}
		if pkg.Name != ancPackageName || pkg.Arch != arch ||
			pkg.Version.Ver != version || pkg.Version.Rel != release {
			continue
		}
		if pkg.Location.Href != expectedLocation {
			return "", newIntegrityError(
				"authenticated RPM location %q does not match deterministic path %q",
				pkg.Location.Href, expectedLocation)
		}
		sum := strings.ToLower(strings.TrimSpace(pkg.Checksum.Value))
		if !strings.EqualFold(pkg.Checksum.Type, "sha256") || !isSHA256Hex(sum) {
			return "", newIntegrityError("RPM package metadata has no valid SHA-256")
		}
		return sum, nil
	}
	return "", newUnsupportedRepositoryError(
		"primary metadata has no exact %s %s-%s.%s package",
		ancPackageName, version, release, arch)
}
