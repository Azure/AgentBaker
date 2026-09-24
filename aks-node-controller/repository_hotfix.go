package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"strings"
	"sync"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/common"
	"golang.org/x/net/http/httpproxy"
)

const (
	defaultYumReposDir           = "/etc/yum.repos.d"
	defaultAptTrustedKeyringsDir = "/etc/apt/trusted.gpg.d"
	repositoryRequestTimeout     = 30 * time.Second
	repositoryMetadataMaxBytes   = 128 << 20
	repositoryPackageMaxBytes    = 512 << 20
	repositoryBinaryMaxBytes     = 128 << 20
	repositoryCommandTimeout     = 60 * time.Second
	ancPackageName               = "aks-node-controller"
	ancPackageBinaryRelativePath = "usr/bin/aks-node-controller"

	archAMD64 = "amd64"
	archARM64 = "arm64"
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
	format                 string
	packageURL             string
	trustedOrigin          *url.URL
	verifyPackageSignature func(context.Context, string) error
	resolveMetadata        func(context.Context) (repositoryPackageMetadata, error)
}

// fetchPackageAndMetadata downloads the package and resolves its authenticated metadata
// concurrently, cancelling the peer as soon as either fails. Without that cancellation a
// fast failure (e.g. a 404 on the package) still waits out the other branch -- gpgv's 60s
// command timeout plus a 30s metadata request -- before the package-manager fallback can
// start, directly extending node provisioning.
//
// Cancellation makes error classification load-bearing. downloadBinaryHotfixIfNeeded
// treats integrity errors as terminal: it disarms the staged hotfix and skips the
// fallback. A cancelled peer must therefore never be reported as integrity, or a benign
// 404 would masquerade as tampering. Conversely, a real integrity error from either
// branch must outrank operational failures, even if the operational failure triggered
// cancellation first.
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
		packageErr  error
		metadataErr error
		cancelOnce  sync.Once
		wg          sync.WaitGroup
	)
	failBranch := func(err error) {
		if err == nil {
			return
		}
		cancelOnce.Do(cancelBranches)
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		packageFile, packageErr = a.downloadRepositoryFile(
			branchCtx, plan.packageURL, plan.trustedOrigin, repositoryPackageMaxBytes)
		failBranch(packageErr)
	}()
	go func() {
		defer wg.Done()
		metadata, metadataErr = plan.resolveMetadata(branchCtx)
		failBranch(metadataErr)
	}()
	wg.Wait()

	// A dead caller context outranks whichever branch happened to notice it first.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return packageFile, metadata, fmt.Errorf("repository fast path cancelled: %w", ctxErr)
	}
	if err := preferredRepositoryDownloadError(packageErr, metadataErr); err != nil {
		return packageFile, metadata, err
	}
	return packageFile, metadata, nil
}

func preferredRepositoryDownloadError(packageErr, metadataErr error) error {
	branches := []struct {
		label string
		err   error
	}{
		{label: "download repository package", err: packageErr},
		{label: "resolve authenticated repository metadata", err: metadataErr},
	}
	for _, branch := range branches {
		if branch.err != nil && !isRepositoryCancellationError(branch.err) && isIntegrityError(branch.err) {
			return fmt.Errorf("%s: %w", branch.label, branch.err)
		}
	}
	for _, branch := range branches {
		if branch.err != nil && !isRepositoryCancellationError(branch.err) {
			return fmt.Errorf("%s: %w", branch.label, branch.err)
		}
	}
	for _, branch := range branches {
		if branch.err != nil {
			return fmt.Errorf("%s: %w", branch.label, branch.err)
		}
	}
	return nil
}

func isRepositoryCancellationError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (a *App) tryRepositoryDownload(ctx context.Context, hotfixVersion string) error {
	start := time.Now()
	info, err := a.parseLinuxPlatformInfo()
	if err != nil {
		return newUnsupportedRepositoryError("determine platform: %v", err)
	}

	var plan repositoryDownloadPlan
	switch info.ID {
	case osReleaseIDUbuntu:
		plan, err = a.ubuntuRepositoryPlan(info, hotfixVersion)
	case osReleaseIDAzureLinux:
		if isImageBasedOSVariant(info.VariantID) {
			return newUnsupportedRepositoryError(
				"repository fast path is not supported on image-based OS %q variant %q",
				info.ID, info.VariantID)
		}
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
	if plan.verifyPackageSignature != nil {
		if verifyErr := plan.verifyPackageSignature(ctx, packageFile.path); verifyErr != nil {
			return verifyErr
		}
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

	// durationMs makes the fast path measurable in the field against the package-manager
	// path, which is the whole reason this code exists.
	slog.Info("downloaded ANC hotfix through authenticated repository fast path",
		"target", hotfixVersion, "format", plan.format, "path", a.hotfixPath(),
		"durationMs", time.Since(start).Milliseconds())
	return nil
}

func (a *App) repositoryStagingDir() string {
	if a.repositoryTempDir != "" {
		return a.repositoryTempDir
	}
	return filepath.Dir(a.hotfixPath())
}

func createExtractedBinaryTemp(destination string) (string, *os.File, func(bool), error) {
	outputPath := filepath.Join(destination, filepath.FromSlash(ancPackageBinaryRelativePath))
	if mkdirErr := os.MkdirAll(filepath.Dir(outputPath), 0o755); mkdirErr != nil {
		return "", nil, nil, fmt.Errorf("create extraction directory: %w", mkdirErr)
	}
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), ".aks-node-controller-extract-*")
	if err != nil {
		return "", nil, nil, fmt.Errorf("create extracted binary temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func(success bool) {
		_ = tmp.Close()
		if !success {
			_ = os.Remove(tmpPath)
		}
	}
	return outputPath, tmp, cleanup, nil
}

func finishExtractedBinary(tmp *os.File, outputPath string) error {
	if err := tmp.Chmod(0o755); err != nil {
		return fmt.Errorf("chmod extracted binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close extracted binary: %w", err)
	}
	if err := os.Rename(tmp.Name(), outputPath); err != nil {
		return fmt.Errorf("rename extracted binary: %w", err)
	}
	return nil
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
	if !sameRepositoryOrigin(base, candidate) {
		return false
	}
	basePath := pathpkg.Clean(base.Path)
	candidatePath := pathpkg.Clean(candidate.Path)
	return basePath == "/" || candidatePath == basePath ||
		strings.HasPrefix(candidatePath, strings.TrimSuffix(basePath, "/")+"/")
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
	// NewBaseTransport defaults to Proxy: nil, which is right for the IMDS/LPS callers that
	// must not be proxied. Repository traffic goes to PMC over the internet, so on
	// proxy-only clusters a direct dial just burns the request timeout before falling back
	// to apt/dnf. cse_main.sh exports HTTP(S)_PROXY/NO_PROXY into ANC's environment, and the
	// redirect and origin checks below still apply to whatever the proxy returns.
	//
	// httpproxy rather than http.ProxyFromEnvironment: the latter snapshots the environment
	// once per process, which silently ignores any proxy configured after the first use.
	transport.Proxy = environmentProxy
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
		if isRepositoryCancellationError(err) {
			return err
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
		return a.extractDeb(ctx, packagePath, destination)
	case "rpm":
		return a.extractRPM(ctx, packagePath, destination)
	default:
		return newUnsupportedRepositoryError("unsupported package format %q", format)
	}
}

// environmentProxy resolves the proxy for req from HTTP_PROXY/HTTPS_PROXY/NO_PROXY, reading
// the environment on every call so a proxy configured after process start is still honored.
func environmentProxy(req *http.Request) (*url.URL, error) {
	return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
}
