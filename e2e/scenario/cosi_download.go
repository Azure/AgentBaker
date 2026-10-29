package scenario

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
)

// cosiDownloadTimeout is the maximum time allowed for downloading and
// streaming through the entire COSI file. ACL COSIs can be multi-GB.
const cosiDownloadTimeout = 30 * time.Minute

// cosiDownloadRetries is the number of attempts made to download a COSI file
// before giving up. Multi-GB COSI downloads are occasionally interrupted
// mid-stream by transient network errors, so the whole download is retried
// from scratch on failure.
const cosiDownloadRetries = 3

// cosiDownloadRetryBackoff is the delay between COSI download retry attempts.
const cosiDownloadRetryBackoff = 15 * time.Second

// downloadCOSIFileOnce performs a single attempt at downloading the COSI file
// at cosiURL to destPath on local disk.
func downloadCOSIFileOnce(ctx context.Context, cosiURL, destPath string) (err error) {
	ctx, cancel := context.WithTimeout(ctx, cosiDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cosiURL, nil)
	if err != nil {
		return fmt.Errorf("creating HTTP request for COSI download: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading COSI file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("COSI download returned non-200 status: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating local COSI file %s: %w", destPath, err)
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
	}()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("writing COSI file to %s: %w", destPath, err)
	}
	return nil
}

// downloadCOSIFileWithRetry downloads the COSI file at cosiURL to destPath on
// local disk, retrying the whole download on failure. Multi-GB COSI
// downloads are occasionally interrupted mid-stream, and since a partially
// read tar stream can't be resumed, the simplest reliable fix is to retry
// the entire download from scratch.
func downloadCOSIFileWithRetry(ctx context.Context, cosiURL, destPath string) error {
	var lastErr error
	for attempt := 1; attempt <= cosiDownloadRetries; attempt++ {
		if attempt > 1 {
			logging.Logf(ctx, "retrying COSI download (attempt %d/%d) after error: %v", attempt, cosiDownloadRetries, lastErr)
			select {
			case <-time.After(cosiDownloadRetryBackoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if err := downloadCOSIFileOnce(ctx, cosiURL, destPath); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return fmt.Errorf("download COSI file after %d attempts: %w", cosiDownloadRetries, lastErr)
}

// cosiDownloadDir returns a scratch directory for downloading COSI files,
// along with a cleanup function the caller must defer. COSIs can be
// multi-GB, and the OS temp dir is too small on some ADO runners. When the
// ADO agent's own temp directory is available (AGENT_TEMPDIRECTORY, set via
// Agent.TempDirectory), which lives on the same, larger volume as the
// checked-out source (Agent.WorkFolder/_work), prefer that instead.
func cosiDownloadDir() (string, func(), error) {
	base := os.Getenv("AGENT_TEMPDIRECTORY")
	downloadDir, err := os.MkdirTemp(base, "acl-cosi-*")
	if err != nil {
		if base != "" {
			return "", nil, fmt.Errorf("creating COSI download directory under AGENT_TEMPDIRECTORY: %w", err)
		}
		return "", nil, fmt.Errorf("creating COSI download directory: %w", err)
	}
	return downloadDir, func() { os.RemoveAll(downloadDir) }, nil
}

// sanitizeURL removes query parameters (which may contain SAS tokens) from a
// URL for safe logging.
func sanitizeURL(u string) string {
	if idx := strings.IndexByte(u, '?'); idx >= 0 {
		return u[:idx] + "?<redacted>"
	}
	return u
}
