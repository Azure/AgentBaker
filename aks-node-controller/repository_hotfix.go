package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/common"
)

const (
	defaultYumReposDir           = "/etc/yum.repos.d"
	defaultAptTrustedKeyringsDir = "/etc/apt/trusted.gpg.d"
	repositoryRequestTimeout     = 30 * time.Second
	repositoryMetadataMaxBytes   = 128 << 20
	repositoryPackageMaxBytes    = 512 << 20
	repositoryCommandTimeout     = 60 * time.Second
	ancPackageName               = "aks-node-controller"
	ancPackageBinaryRelativePath = "usr/bin/aks-node-controller"

	archAMD64 = "amd64"
	archARM64 = "arm64"

	osIDAzureLinux = "azurelinux"
	osIDMariner    = "mariner"
)

type integrityError struct {
	msg string
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (e *integrityError) Error() string { return e.msg }

func newIntegrityError(format string, args ...any) error {
	return &integrityError{msg: fmt.Sprintf(format, args...)}
}

func isIntegrityError(err error) bool {
	var target *integrityError
	return errors.As(err, &target)
}

type unsupportedRepositoryError struct {
	msg string
}

func (e *unsupportedRepositoryError) Error() string { return e.msg }

func newUnsupportedRepositoryError(format string, args ...any) error {
	return &unsupportedRepositoryError{msg: fmt.Sprintf(format, args...)}
}

type downloadedRepositoryFile struct {
	path   string
	sha256 string
	size   int64
}

type repositoryPackageMetadata struct {
	sha256 string
}

type repositoryDownloadPlan struct {
	format          string
	packageURL      string
	trustedOrigin   *url.URL
	resolveMetadata func(context.Context) (repositoryPackageMetadata, error)
}

// fetchPackageAndMetadata downloads the package and resolves its authenticated metadata
// concurrently, cancelling the peer as soon as either fails. Without that cancellation a
// fast failure (e.g. a 404 on the package) still waits out the other branch -- gpgv's 60s
// command timeout plus a 30s metadata request -- before the package-manager fallback can
// start, directly extending node provisioning.
//
// Cancellation makes error classification load-bearing. A killed gpgv surfaces from
// verifyRepoSignature as an integrityError, and downloadBinaryHotfixIfNeeded treats
// integrity errors as terminal: it disarms the staged hotfix and skips the fallback. A
// cancelled peer must therefore never be reported, or a benign 404 would masquerade as
// tampering. Only the branch that failed first is returned; the peer's error is induced
// noise and is dropped.
//
// The returned file is the caller's to remove, including on the error paths.
func (a *App) fetchPackageAndMetadata(
	ctx context.Context,
	plan repositoryDownloadPlan,
) (downloadedRepositoryFile, repositoryPackageMetadata, error) {
	branchCtx, cancelBranches := context.WithCancel(ctx)
	defer cancelBranches()

	var (
		packageFile downloadedRepositoryFile
		metadata    repositoryPackageMetadata
		firstErr    error
		firstIsMeta bool
		firstErrOne sync.Once
		wg          sync.WaitGroup
	)
	failBranch := func(err error, isMetadata bool) {
		if err == nil {
			return
		}
		firstErrOne.Do(func() {
			firstErr = err
			firstIsMeta = isMetadata
			cancelBranches()
		})
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		var err error
		packageFile, err = a.downloadRepositoryFile(
			branchCtx, plan.packageURL, plan.trustedOrigin, repositoryPackageMaxBytes)
		failBranch(err, false)
	}()
	go func() {
		defer wg.Done()
		var err error
		metadata, err = plan.resolveMetadata(branchCtx)
		failBranch(err, true)
	}()
	wg.Wait()

	// A dead caller context outranks whichever branch happened to notice it first.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return packageFile, metadata, fmt.Errorf("repository fast path cancelled: %w", ctxErr)
	}
	if firstErr != nil {
		if firstIsMeta {
			return packageFile, metadata, fmt.Errorf(
				"resolve authenticated repository metadata: %w", firstErr)
		}
		return packageFile, metadata, fmt.Errorf("download repository package: %w", firstErr)
	}
	return packageFile, metadata, nil
}

func (a *App) tryRepositoryDownload(ctx context.Context, hotfixVersion string) error {
	info, err := a.parseLinuxPlatformInfo()
	if err != nil {
		return newUnsupportedRepositoryError("determine platform: %v", err)
	}

	var plan repositoryDownloadPlan
	switch info.ID {
	case "ubuntu":
		plan, err = a.ubuntuRepositoryPlan(info, hotfixVersion)
	case osIDAzureLinux, osIDMariner:
		plan, err = a.rpmRepositoryPlan(info, hotfixVersion)
	default:
		err = newUnsupportedRepositoryError("unsupported repository platform %q", info.ID)
	}
	if err != nil {
		return err
	}

	packageFile, metadata, err := a.fetchPackageAndMetadata(ctx, plan)
	if packageFile.path != "" {
		defer os.Remove(packageFile.path)
	}
	if err != nil {
		return err
	}
	if !strings.EqualFold(packageFile.sha256, metadata.sha256) {
		return newIntegrityError("package SHA-256 mismatch: expected %s, got %s",
			metadata.sha256, packageFile.sha256)
	}

	extractDir, err := os.MkdirTemp(a.repositoryStagingDir(), ".aks-node-controller-extract-*")
	if err != nil {
		return fmt.Errorf("create package extraction directory: %w", err)
	}
	defer os.RemoveAll(extractDir)

	if err := a.extractPackage(ctx, plan.format, packageFile.path, extractDir); err != nil {
		return fmt.Errorf("extract authenticated %s package: %w", plan.format, err)
	}
	extractedBinary := filepath.Join(extractDir, filepath.FromSlash(ancPackageBinaryRelativePath))
	if err := copyBinaryAlongside(extractedBinary, a.hotfixPath(), a.vhdPath()); err != nil {
		return fmt.Errorf("stage extracted ANC binary: %w", err)
	}

	slog.Info("downloaded ANC hotfix through authenticated repository fast path",
		"target", hotfixVersion, "format", plan.format, "path", a.hotfixPath())
	return nil
}

func (a *App) repositoryStagingDir() string {
	if a.repositoryTempDir != "" {
		return a.repositoryTempDir
	}
	return filepath.Dir(a.hotfixPath())
}

func validateRepositoryURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, newUnsupportedRepositoryError("parse repository URL %q: %v", rawURL, err)
	}
	if u.User != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, newUnsupportedRepositoryError("repository URL %q is not an HTTP(S) origin", rawURL)
	}
	if u.Scheme == "http" && !isTrustedLocalRepositoryHost(u.Hostname()) {
		return nil, newUnsupportedRepositoryError(
			"plain HTTP repository %q is not a trusted local source", u.Hostname())
	}
	u.Fragment = ""
	return u, nil
}

func asRepositoryBase(u *url.URL) *url.URL {
	base := *u
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	return &base
}

func isTrustedLocalRepositoryHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

func sameRepositoryOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func isWithinRepositoryBase(base, candidate *url.URL) bool {
	return sameRepositoryOrigin(base, candidate) &&
		strings.HasPrefix(candidate.EscapedPath(), base.EscapedPath())
}

func resolveRepositoryURL(base *url.URL, relative string) (string, error) {
	ref, err := url.Parse(relative)
	if err != nil {
		return "", newUnsupportedRepositoryError("parse repository-relative URL %q: %v", relative, err)
	}
	if ref.IsAbs() || ref.Host != "" || strings.HasPrefix(ref.Path, "/") ||
		ref.RawQuery != "" || ref.Fragment != "" {
		return "", newIntegrityError("repository path is not relative: %q", relative)
	}
	cleaned := pathpkg.Clean(ref.Path)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", newIntegrityError("repository path escapes configured base: %q", relative)
	}
	ref.Path = cleaned
	resolved := base.ResolveReference(ref)
	if !isWithinRepositoryBase(base, resolved) {
		return "", newIntegrityError("repository metadata escaped configured base path: %q", relative)
	}
	return resolved.String(), nil
}

func (a *App) downloadRepositoryFile(
	ctx context.Context,
	rawURL string,
	trustedOrigin *url.URL,
	maxBytes int64,
) (downloadedRepositoryFile, error) {
	u, err := validateRepositoryURL(rawURL)
	if err != nil {
		return downloadedRepositoryFile{}, err
	}
	if trustedOrigin == nil || !sameRepositoryOrigin(trustedOrigin, u) {
		return downloadedRepositoryFile{}, newIntegrityError("download URL is outside configured repository origin: %s", rawURL)
	}

	if err = os.MkdirAll(a.repositoryStagingDir(), 0o755); err != nil {
		return downloadedRepositoryFile{}, fmt.Errorf("create repository staging directory: %w", err)
	}
	tmp, err := os.CreateTemp(a.repositoryStagingDir(), ".aks-node-controller-repository-*")
	if err != nil {
		return downloadedRepositoryFile{}, fmt.Errorf("create repository temp file: %w", err)
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		_ = tmp.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	requestCtx, cancel := context.WithTimeout(ctx, repositoryRequestTimeout)
	defer cancel()
	transport := common.NewBaseTransport(common.HTTPTransportOptions{
		DialTimeout:           10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	})
	transport.DisableCompression = true
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			redirectURL, redirectErr := validateRepositoryURL(req.URL.String())
			if redirectErr != nil {
				return redirectErr
			}
			if !isWithinRepositoryBase(trustedOrigin, redirectURL) {
				return fmt.Errorf("redirect outside configured repository base")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return downloadedRepositoryFile{}, fmt.Errorf("create GET request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return downloadedRepositoryFile{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return downloadedRepositoryFile{}, fmt.Errorf("HTTP %d from %s", resp.StatusCode, u.Redacted())
	}

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return downloadedRepositoryFile{}, fmt.Errorf("stream %s: %w", u.Redacted(), err)
	}
	if written > maxBytes {
		return downloadedRepositoryFile{}, fmt.Errorf("repository response exceeds %d bytes", maxBytes)
	}
	if err := tmp.Close(); err != nil {
		return downloadedRepositoryFile{}, fmt.Errorf("close repository temp file: %w", err)
	}
	success = true
	return downloadedRepositoryFile{
		path:   tmpPath,
		sha256: hex.EncodeToString(hasher.Sum(nil)),
		size:   written,
	}, nil
}

func (a *App) verifyRepoSignature(
	ctx context.Context,
	signedPath string,
	signaturePath string,
	keyrings []string,
) error {
	if len(keyrings) == 0 {
		return newUnsupportedRepositoryError("repository has no configured signing key")
	}
	if a.verifyRepositorySignature != nil {
		if err := a.verifyRepositorySignature(ctx, signedPath, signaturePath, keyrings); err != nil {
			return newIntegrityError("repository signature verification failed: %v", err)
		}
		return nil
	}

	preparedKeyrings, cleanup, err := a.prepareGPGVKeyrings(ctx, keyrings)
	if err != nil {
		return err
	}
	defer cleanup()

	args := make([]string, 0, 2*len(preparedKeyrings)+2)
	for _, keyring := range preparedKeyrings {
		args = append(args, "--keyring", keyring)
	}
	if signaturePath != "" {
		args = append(args, signaturePath)
	}
	args = append(args, signedPath)
	if err := a.runRepositoryCommand(ctx, "gpgv", args...); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return newUnsupportedRepositoryError("gpgv is not installed: %v", err)
		}
		return newIntegrityError("gpgv verification failed: %v", err)
	}
	return nil
}

func (a *App) prepareGPGVKeyrings(
	ctx context.Context,
	keyrings []string,
) ([]string, func(), error) {
	var prepared []string
	var temporary []string
	cleanup := func() {
		for _, path := range temporary {
			_ = os.Remove(path)
		}
	}
	for _, keyring := range keyrings {
		data, err := os.ReadFile(keyring)
		if err != nil {
			cleanup()
			return nil, func() {}, newUnsupportedRepositoryError(
				"read repository keyring %s: %v", keyring, err)
		}
		if !bytes.Contains(data, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----")) {
			prepared = append(prepared, keyring)
			continue
		}
		output, err := os.CreateTemp(a.repositoryStagingDir(), ".aks-node-controller-keyring-*.gpg")
		if err != nil {
			cleanup()
			return nil, func() {}, fmt.Errorf("create repository keyring temp file: %w", err)
		}
		outputPath := output.Name()
		if err := output.Close(); err != nil {
			_ = os.Remove(outputPath)
			cleanup()
			return nil, func() {}, fmt.Errorf("close repository keyring temp file: %w", err)
		}
		temporary = append(temporary, outputPath)
		if err := a.runRepositoryCommand(
			ctx, "gpg", "--batch", "--yes", "--dearmor", "--output", outputPath, keyring,
		); err != nil {
			cleanup()
			return nil, func() {}, newUnsupportedRepositoryError(
				"dearmor repository key %s: %v", keyring, err)
		}
		prepared = append(prepared, outputPath)
	}
	return prepared, cleanup, nil
}

func (a *App) runRepositoryCommand(ctx context.Context, name string, args ...string) error {
	commandCtx, cancel := context.WithTimeout(ctx, repositoryCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := a.cmdRun(cmd); err != nil {
		if commandCtx.Err() != nil {
			return commandCtx.Err()
		}
		return err
	}
	return nil
}

func (a *App) extractPackage(ctx context.Context, format, packagePath, destination string) error {
	if a.extractRepositoryPackage != nil {
		return a.extractRepositoryPackage(ctx, format, packagePath, destination)
	}
	switch format {
	case "deb":
		return a.runRepositoryCommand(ctx, "dpkg-deb", "-x", packagePath, destination)
	case "rpm":
		return a.extractRPM(ctx, packagePath, destination)
	default:
		return newUnsupportedRepositoryError("unsupported package format %q", format)
	}
}

func (a *App) extractRPM(ctx context.Context, packagePath, destination string) error {
	commandCtx, cancel := context.WithTimeout(ctx, repositoryCommandTimeout)
	defer cancel()

	reader, writer, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create rpm extraction pipe: %w", err)
	}
	defer reader.Close()
	defer writer.Close()

	rpm2cpio := exec.CommandContext(commandCtx, "rpm2cpio", packagePath)
	rpm2cpio.Stdout = writer
	rpm2cpio.Stderr = os.Stderr
	cpio := exec.CommandContext(commandCtx, "cpio", "-idmu", "--quiet", "./usr/bin/aks-node-controller")
	cpio.Dir = destination
	cpio.Stdin = reader
	cpio.Stdout = os.Stdout
	cpio.Stderr = os.Stderr

	cpioErrCh := make(chan error, 1)
	go func() {
		cpioErrCh <- a.cmdRun(cpio)
	}()
	rpmErr := a.cmdRun(rpm2cpio)
	_ = writer.Close()
	cpioErr := <-cpioErrCh
	if rpmErr != nil {
		return fmt.Errorf("rpm2cpio: %w", rpmErr)
	}
	if cpioErr != nil {
		return fmt.Errorf("cpio: %w", cpioErr)
	}
	if commandCtx.Err() != nil {
		return commandCtx.Err()
	}
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
	sourcesDir := a.aptSourcesDir
	if sourcesDir == "" {
		sourcesDir = defaultAptSourcesDir
	}
	sourcePath, err := resolveMicrosoftProdSourceListPath(sourcesDir)
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
func parseAptLineOptions(fields []string, index int) (map[string]string, int, bool) {
	options := map[string]string{}
	if !strings.HasPrefix(fields[index], "[") {
		return options, index, true
	}
	end := index
	for end < len(fields) && !strings.HasSuffix(fields[end], "]") {
		end++
	}
	if end >= len(fields) {
		return nil, 0, false
	}
	optionText := strings.Trim(strings.Join(fields[index:end+1], " "), "[]")
	for _, option := range strings.Fields(optionText) {
		keyValue := strings.SplitN(option, "=", 2)
		if len(keyValue) == 2 {
			options[strings.ToLower(keyValue[0])] = keyValue[1]
		}
	}
	return options, end + 1, true
}

// parseOneLineAptEntry parses a single non-comment one-line apt entry. The bool is
// false when the line is not a usable deb entry for arch and must be skipped.
func parseOneLineAptEntry(line, path, arch string) (aptRepository, bool) {
	if line == "" || !strings.HasPrefix(line, "deb ") {
		return aptRepository{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return aptRepository{}, false
	}
	options, index, ok := parseAptLineOptions(fields, 1)
	if !ok || len(fields) < index+3 {
		return aptRepository{}, false
	}
	if configuredArch := options["arch"]; configuredArch != "" &&
		!containsString(strings.Split(configuredArch, ","), arch) {
		return aptRepository{}, false
	}
	return aptRepository{
		URI:        fields[index],
		Suite:      fields[index+1],
		Component:  fields[index+2],
		SignedBy:   splitConfiguredPaths(options["signed-by"]),
		SourcePath: path,
	}, true
}

func parseOneLineAptRepository(contents, path, arch string) (aptRepository, error) {
	for _, rawLine := range strings.Split(contents, "\n") {
		line := strings.TrimSpace(strings.SplitN(rawLine, "#", 2)[0])
		if repo, ok := parseOneLineAptEntry(line, path, arch); ok {
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
		signedBy := splitConfiguredPaths(fields["signed-by"])
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

func splitConfiguredPaths(value string) []string {
	var paths []string
	for _, field := range strings.Fields(strings.ReplaceAll(value, ",", " ")) {
		if strings.HasPrefix(field, "/") {
			paths = append(paths, field)
		}
	}
	return paths
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	return false
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
	packagesSuiteRelativePath := filepath.ToSlash(filepath.Join(
		repository.Component, "binary-"+arch, "Packages"))
	packagesRelativePath := filepath.ToSlash(filepath.Join(
		"dists", repository.Suite, packagesSuiteRelativePath))
	packagesURL, err := resolveRepositoryURL(origin, packagesRelativePath)
	if err != nil {
		return repositoryPackageMetadata{}, err
	}

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
	expectedPackagesSHA, expectedPackagesSize, err := parseReleaseSHA256(releasePayload, packagesSuiteRelativePath)
	if err != nil {
		return repositoryPackageMetadata{}, newIntegrityError("%v", err)
	}

	packages, err := a.downloadRepositoryFile(ctx, packagesURL, origin, repositoryMetadataMaxBytes)
	if err != nil {
		return repositoryPackageMetadata{}, fmt.Errorf("download Packages: %w", err)
	}
	defer os.Remove(packages.path)
	if packages.size != expectedPackagesSize || !strings.EqualFold(packages.sha256, expectedPackagesSHA) {
		return repositoryPackageMetadata{}, newIntegrityError(
			"Packages metadata mismatch: expected size/SHA256 %d/%s, got %d/%s",
			expectedPackagesSize, expectedPackagesSHA, packages.size, packages.sha256)
	}

	packageSHA, err := parseDebPackageMetadata(
		packages.path, fullVersion, arch, expectedPackagePath)
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
	baseURL := strings.ReplaceAll(repository.BaseURL, "$releasever", info.VersionID)
	baseURL = strings.ReplaceAll(baseURL, "${releasever}", info.VersionID)
	baseURL = strings.ReplaceAll(baseURL, "$basearch", rpmArch)
	baseURL = strings.ReplaceAll(baseURL, "${basearch}", rpmArch)
	if strings.Contains(baseURL, "$") {
		return repositoryDownloadPlan{}, newUnsupportedRepositoryError(
			"unsupported variable in ms-oss baseurl %q", repository.BaseURL)
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
		format:        "rpm",
		packageURL:    packageURL,
		trustedOrigin: origin,
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

func rpmReleaseSuffix(info platformInfo) (string, error) {
	major := strings.SplitN(info.VersionID, ".", 2)[0]
	switch {
	case info.ID == osIDAzureLinux && major == "3":
		return "azl3", nil
	case info.ID == osIDMariner && major == "2":
		return "cm2", nil
	default:
		return "", newUnsupportedRepositoryError(
			"cannot establish ANC RPM release suffix for %s %s", info.ID, info.VersionID)
	}
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
			if !strings.Contains(strings.ToLower(name), "ms-oss") &&
				!strings.Contains(strings.ToLower(values["baseurl"]), "/ms-oss/") {
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
	return rpmRepository{}, newUnsupportedRepositoryError("no enabled ms-oss repository in %s", reposDir)
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
		return nil, newUnsupportedRepositoryError("ms-oss repository has no local gpgkey")
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
