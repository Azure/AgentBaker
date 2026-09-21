package scenario

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func branchCSETestDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func branchCSETestFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func TestBranchCSEBlobNamesAreUnique(t *testing.T) {
	t.Parallel()
	names := make(map[string]bool)
	for range 1000 {
		name := branchCSEBlobName("build-9275")
		require.Regexp(t, `^cse-packages/aks-windows-cse-scripts-branch-build-9275-[A-Z2-7]+\.zip$`, name)
		require.False(t, names[name], "duplicate blob name %q", name)
		names[name] = true
	}
	assert.Contains(t, branchCSEBlobName("local"), "-branch-local-")
}

func TestBranchCSEZipContents(t *testing.T) {
	t.Parallel()
	dir := branchCSETestDir(t)
	want := map[string]string{
		"cse.ps1":                    "bootstrap",
		"credentialproviderfunc.ps1": "branch credential provider",
		"debug/collect.ps1":          "diagnostics",
		"nested/script.ps1":          "nested",
	}
	for name, contents := range want {
		branchCSETestFile(t, dir, name, contents)
	}
	for _, name := range []string{
		"README", "debug/update-scripts.ps1", "cse.tests.ps1",
		"credentialProvider.tests.suites/test.ps1", "nested/script.tests.ps1",
	} {
		branchCSETestFile(t, dir, name, "excluded")
	}
	var output bytes.Buffer
	require.NoError(t, writeBranchCSEZip(context.Background(), &output, dir))
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	require.NoError(t, err)
	got := make(map[string]string)
	for _, entry := range archive.File {
		assert.NotContains(t, entry.Name, `\`)
		reader, err := entry.Open()
		require.NoError(t, err)
		contents, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
		got[entry.Name] = string(contents)
	}
	assert.Equal(t, want, got)
}

type branchCSEFailWriter struct{ err error }

func (w branchCSEFailWriter) Write([]byte) (int, error) { return 0, w.err }

func TestBranchCSEZipErrors(t *testing.T) {
	t.Parallel()
	dir := branchCSETestDir(t)
	branchCSETestFile(t, dir, "cse.ps1", "contents")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, writeBranchCSEZip(ctx, io.Discard, dir), context.Canceled)
	require.ErrorIs(t, writeBranchCSEZip(context.Background(), io.Discard, filepath.Join(dir, "missing")), os.ErrNotExist)
	writeErr := errors.New("disk full")
	require.ErrorIs(t, writeBranchCSEZip(context.Background(), branchCSEFailWriter{writeErr}, dir), writeErr)
	reader := branchCSEContextReader{ctx: ctx, reader: strings.NewReader("contents")}
	_, err := reader.Read(make([]byte, 1))
	require.ErrorIs(t, err, context.Canceled)
}

func TestBranchCSEBuildAndUploadCleanup(t *testing.T) {
	t.Parallel()
	for _, failUpload := range []bool{false, true} {
		t.Run(fmt.Sprintf("upload-error-%t", failUpload), func(t *testing.T) {
			dir := branchCSETestDir(t)
			branchCSETestFile(t, dir, "cse.ps1", "branch contents")
			uploadErr := errors.New("upload failed")
			var uploaded *os.File
			ctx := t.Context()
			got, err := buildAndUploadBranchCSEZip(ctx, dir, "test-build",
				func(uploadCtx context.Context, name string, file *os.File) (string, error) {
					assert.Equal(t, ctx, uploadCtx)
					assert.Contains(t, name, "-branch-test-build-")
					uploaded = file
					data, err := io.ReadAll(file)
					require.NoError(t, err)
					archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
					require.NoError(t, err, "upload must receive a finalized zip positioned at offset zero")
					require.Len(t, archive.File, 1)
					assert.Equal(t, "cse.ps1", archive.File[0].Name)
					if failUpload {
						return "", uploadErr
					}
					return "https://example.invalid/package.zip?sas", nil
				})
			require.NotNil(t, uploaded)
			_, statErr := os.Stat(uploaded.Name())
			assert.ErrorIs(t, statErr, os.ErrNotExist)
			_, readErr := uploaded.Read(make([]byte, 1))
			assert.ErrorIs(t, readErr, os.ErrClosed)
			if failUpload {
				require.ErrorIs(t, err, uploadErr)
				assert.Empty(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "https://example.invalid/package.zip?sas", got)
			}
		})
	}
}

func TestBranchCSEBuildFailureDoesNotUpload(t *testing.T) {
	t.Parallel()
	dir := branchCSETestDir(t)
	upload := func(context.Context, string, *os.File) (string, error) {
		t.Fatal("upload called after build failure")
		return "", nil
	}
	_, err := buildAndUploadBranchCSEZip(context.Background(), filepath.Join(dir, "missing"), "test", upload)
	require.ErrorIs(t, err, os.ErrNotExist)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = buildAndUploadBranchCSEZip(ctx, dir, "test", upload)
	require.ErrorIs(t, err, context.Canceled)
	// No Azure clients need to be configured for a canceled initialization.
	_, err = buildWindowsDalecCSEPackageURL(ctx, windowsDalecCSEZipRequest{Location: "unused"})
	require.ErrorIs(t, err, context.Canceled)
}

func TestBranchCSECleanupPreservesErrors(t *testing.T) {
	t.Parallel()
	dir := branchCSETestDir(t)
	branchCSETestFile(t, dir, "cse.ps1", "contents")
	uploadErr := errors.New("upload failed")
	var zipPath string
	link, err := buildAndUploadBranchCSEZip(context.Background(), dir, "test",
		func(_ context.Context, _ string, file *os.File) (string, error) {
			zipPath = file.Name()
			require.NoError(t, file.Close())
			return "", uploadErr
		})
	require.ErrorIs(t, err, uploadErr)
	require.ErrorIs(t, err, os.ErrClosed)
	assert.Empty(t, link)
	_, statErr := os.Stat(zipPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "removal must still run when close fails")
}

func TestBranchCSELocationCache(t *testing.T) {
	t.Parallel()
	calls := make(map[string]int)
	buildErr := errors.New("package failed")
	build := cachedFunc(func(ctx context.Context, request windowsDalecCSEZipRequest) (string, error) {
		calls[request.Location]++
		if request.Location == "failed" {
			return "", buildErr
		}
		return "https://example.invalid/" + request.Location, ctx.Err()
	})
	for range 2 {
		for _, location := range []string{"westus", "eastus"} {
			value, err := build(context.Background(), windowsDalecCSEZipRequest{Location: location})
			require.NoError(t, err)
			assert.Equal(t, "https://example.invalid/"+location, value)
		}
		_, err := build(context.Background(), windowsDalecCSEZipRequest{Location: "failed"})
		require.ErrorIs(t, err, buildErr)
	}
	assert.Equal(t, map[string]int{"westus": 1, "eastus": 1, "failed": 1}, calls)
}

type branchCSETransport func(*http.Request) (*http.Response, error)

func (f branchCSETransport) Do(req *http.Request) (*http.Response, error) { return f(req) }

func branchCSETestClient(t *testing.T, transport branchCSETransport) *azblob.Client {
	t.Helper()
	client, err := azblob.NewClientWithNoCredential("https://account.blob.core.windows.net", &azblob.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: transport,
			Retry:     policy.RetryOptions{MaxRetries: -1},
		},
	})
	require.NoError(t, err)
	return client
}

func branchCSEResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		Request: req, StatusCode: status, Status: http.StatusText(status),
		Header: http.Header{"Content-Type": {"application/xml"}},
		Body:   io.NopCloser(strings.NewReader(body)),
	}
}

func TestBranchCSEUploadCreateOnlyAndSAS(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"", "conflict-409", "conflict-412", "delegation"} {
		t.Run(failure, func(t *testing.T) {
			path := branchCSETestFile(t, branchCSETestDir(t), "cse.zip", "zip bytes")
			file, err := os.Open(path)
			require.NoError(t, err)
			defer file.Close()
			requests := 0
			before := time.Now().UTC()
			client := branchCSETestClient(t, func(req *http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					assert.Equal(t, http.MethodPut, req.Method)
					assert.Equal(t, "/packages/cse-packages/build name.zip", req.URL.Path)
					assert.Equal(t, "*", req.Header.Get("If-None-Match"))
					if strings.HasPrefix(failure, "conflict") {
						status := http.StatusPreconditionFailed
						if failure == "conflict-409" {
							status = http.StatusConflict
						}
						return branchCSEResponse(req, status, "<Error><Code>ConditionNotMet</Code></Error>"), nil
					}
					return branchCSEResponse(req, http.StatusCreated, ""), nil
				}
				require.Equal(t, 2, requests, "unexpected network call")
				assert.Equal(t, http.MethodPost, req.Method)
				assert.Equal(t, "userdelegationkey", req.URL.Query().Get("comp"))
				var keyInfo struct {
					Start  time.Time `xml:"Start"`
					Expiry time.Time `xml:"Expiry"`
				}
				require.NoError(t, xml.NewDecoder(req.Body).Decode(&keyInfo))
				assert.WithinDuration(t, before.Add(-15*time.Minute), keyInfo.Start, 5*time.Second)
				assert.WithinDuration(t, before.Add(6*time.Hour), keyInfo.Expiry, 5*time.Second)
				if failure == "delegation" {
					return branchCSEResponse(req, http.StatusForbidden, "<Error><Code>AuthorizationFailure</Code></Error>"), nil
				}
				return branchCSEResponse(req, http.StatusOK, `<UserDelegationKey>
					<SignedOid>oid</SignedOid><SignedTid>tid</SignedTid>
					<SignedStart>2026-09-18T00:00:00Z</SignedStart><SignedExpiry>2099-01-01T00:00:00Z</SignedExpiry>
					<SignedService>b</SignedService><SignedVersion>2025-11-05</SignedVersion>
					<Value>YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXphYmNkZWY=</Value>
				</UserDelegationKey>`), nil
			})
			link, err := uploadWindowsCSEZipNoOverwrite(context.Background(), client, "packages", "cse-packages/build name.zip", file)
			if failure != "" {
				require.Error(t, err)
				assert.Empty(t, link)
				var responseErr *azcore.ResponseError
				require.ErrorAs(t, err, &responseErr)
				if failure == "delegation" {
					assert.Equal(t, 2, requests)
					assert.ErrorContains(t, err, "get user delegation credential")
					assert.Equal(t, http.StatusForbidden, responseErr.StatusCode)
				} else {
					assert.Equal(t, 1, requests, "must not sign a URL after an upload conflict")
					assert.ErrorContains(t, err, "without overwrite")
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, 2, requests)
			parsed, err := url.Parse(link)
			require.NoError(t, err)
			assert.Equal(t, "/packages/cse-packages/build name.zip", parsed.Path)
			assert.Equal(t, "https", parsed.Scheme)
			assert.Equal(t, "r", parsed.Query().Get("sp"))
			assert.Equal(t, "https", parsed.Query().Get("spr"))
			assert.NotEmpty(t, parsed.Query().Get("sig"))
			expiry, err := time.Parse(time.RFC3339, parsed.Query().Get("se"))
			require.NoError(t, err)
			assert.WithinDuration(t, before.Add(6*time.Hour), expiry, 5*time.Second)
		})
	}
}

func TestBranchCSEUploadCancellation(t *testing.T) {
	t.Parallel()
	file, err := os.Open(branchCSETestFile(t, branchCSETestDir(t), "cse.zip", "contents"))
	require.NoError(t, err)
	defer file.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := branchCSETestClient(t, func(req *http.Request) (*http.Response, error) {
		cancel()
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	link, err := uploadWindowsCSEZipNoOverwrite(ctx, client, "packages", "cse.zip", file)
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, link)
}
