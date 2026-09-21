package scenario

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
)

// windowsDalecCSEZipRequest keys the cache by the location used to initialize shared resources.
// Callers should use config.Config.DefaultLocation, where the storage account lives.
type windowsDalecCSEZipRequest struct {
	Location string
}

// Keep Dalec's create-only packaging separate from the existing RCV1P helper.
// Results, including errors, are cached per location.
var cachedWindowsDalecCSEPackageURL = cachedFunc(buildWindowsDalecCSEPackageURL)

func buildWindowsDalecCSEPackageURL(ctx context.Context, request windowsDalecCSEZipRequest) (string, error) {
	// Allow for cold-start storage creation as well as packaging and uploading.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// The identity helper lazily creates shared storage, but needs the RG first.
	// The scenario may be running outside DefaultLocation, so ensure both here.
	if _, err := CachedEnsureResourceGroup(ctx, request.Location); err != nil {
		return "", fmt.Errorf("ensure shared resource group: %w", err)
	}
	if _, err := CachedCreateVMManagedIdentity(ctx, request.Location); err != nil {
		return "", fmt.Errorf("ensure shared storage account: %w", err)
	}
	return buildAndUploadWindowsDalecCSEZip(ctx)
}

func buildAndUploadWindowsDalecCSEZip(ctx context.Context) (string, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return "", fmt.Errorf("find repo root: %w", err)
	}
	return buildAndUploadBranchCSEZip(ctx, filepath.Join(repoRoot, "staging", "cse", "windows"),
		config.Config.BuildID, func(ctx context.Context, name string, file *os.File) (string, error) {
			return uploadWindowsCSEZipNoOverwrite(ctx, config.Azure.Blob, config.Config.BlobContainer, name, file)
		})
}

func branchCSEBlobName(buildID string) string {
	return fmt.Sprintf("cse-packages/aks-windows-cse-scripts-branch-%s-%s.zip", buildID, rand.Text())
}

func buildAndUploadBranchCSEZip(ctx context.Context, cseDir, buildID string,
	upload func(context.Context, string, *os.File) (string, error),
) (url string, err error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	file, err := os.CreateTemp("", "aks-windows-cse-scripts-branch-*.zip")
	if err != nil {
		return "", fmt.Errorf("create CSE zip file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close CSE zip file: %w", closeErr))
		}
		if removeErr := os.Remove(file.Name()); removeErr != nil {
			err = errors.Join(err, fmt.Errorf("remove CSE zip file: %w", removeErr))
		}
		if err != nil {
			url = ""
		}
	}()

	if err := writeBranchCSEZip(ctx, file, cseDir); err != nil {
		return "", fmt.Errorf("build CSE zip: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek CSE zip file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	url, err = upload(ctx, branchCSEBlobName(buildID), file)
	if err != nil {
		return "", fmt.Errorf("upload CSE zip: %w", err)
	}
	return url, nil
}

func writeBranchCSEZip(ctx context.Context, dst io.Writer, cseDir string) (err error) {
	zw := zip.NewWriter(dst)
	defer func() {
		if closeErr := zw.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close zip writer: %w", closeErr))
		}
	}()
	return filepath.Walk(cseDir, func(path string, info os.FileInfo, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(cseDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if strings.HasSuffix(rel, ".tests.ps1") || strings.Contains(rel, ".tests.suites") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == "README" || rel == "debug/update-scripts.ps1" || info.IsDir() {
			return nil
		}
		w, err := zw.Create(rel)
		if err != nil {
			return fmt.Errorf("create zip entry %s: %w", rel, err)
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		_, copyErr := io.Copy(w, branchCSEContextReader{ctx: ctx, reader: f})
		closeErr := f.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return fmt.Errorf("copy or close %s: %w", path, err)
		}
		return nil
	})
}

type branchCSEContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r branchCSEContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// Keep create-only semantics local to branch packages, not the shared upload helper.
func uploadWindowsCSEZipNoOverwrite(ctx context.Context, client *azblob.Client, container, blobName string, file *os.File) (string, error) {
	_, err := client.UploadFile(ctx, container, blobName, file, &azblob.UploadFileOptions{
		AccessConditions: &blob.AccessConditions{
			ModifiedAccessConditions: &blob.ModifiedAccessConditions{
				IfNoneMatch: to.Ptr(azcore.ETagAny),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("upload blob %q without overwrite: %w", blobName, err)
	}

	now := time.Now().UTC()
	start := now.Add(-15 * time.Minute)
	expiry := now.Add(6 * time.Hour)
	udc, err := client.ServiceClient().GetUserDelegationCredential(ctx, service.KeyInfo{
		Expiry: to.Ptr(expiry.Format(sas.TimeFormat)),
		Start:  to.Ptr(start.Format(sas.TimeFormat)),
	}, nil)
	if err != nil {
		return "", fmt.Errorf("get user delegation credential: %w", err)
	}
	sig, err := (sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		ExpiryTime:    expiry,
		Permissions:   to.Ptr(sas.BlobPermissions{Read: true}).String(),
		ContainerName: container,
		BlobName:      blobName,
	}).SignWithUserDelegation(udc)
	if err != nil {
		return "", fmt.Errorf("sign blob: %w", err)
	}
	blobURL := client.ServiceClient().NewContainerClient(container).NewBlockBlobClient(blobName).URL()
	return fmt.Sprintf("%s?%s", blobURL, sig.Encode()), nil
}
