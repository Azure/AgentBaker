package main

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/stretchr/testify/require"
)

type testCredential struct{}

func (testCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "test-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func newUploadTestClient(test *testing.T, handler http.HandlerFunc) *azblob.Client {
	test.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer test-token" {
			test.Error("upload request is missing its bearer token")
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler(response, request)
	}))
	test.Cleanup(server.Close)
	client, err := azblob.NewClient(server.URL, testCredential{}, &azblob.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: server.Client(),
			Retry:     policy.RetryOptions{MaxRetries: -1},
		},
	})
	require.NoError(test, err)
	return client
}

func TestUploadFileMultipart(test *testing.T) {
	file, err := os.CreateTemp(test.TempDir(), "*.cosi")
	require.NoError(test, err)
	fileSize := int64(blockblob.MaxUploadBlobBytes + 1)
	require.NoError(test, file.Truncate(fileSize))
	require.NoError(test, file.Close())

	var mutex sync.Mutex
	blocks := make(map[string]int64)
	var committed struct {
		Latest []string `xml:"Latest"`
	}
	client := newUploadTestClient(test, func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/cosi/acl-arm64.cosi" {
			test.Errorf("unexpected upload request: %s %s", request.Method, request.URL.Path)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.URL.Query().Get("comp") {
		case "block":
			size, copyErr := io.Copy(io.Discard, request.Body)
			if copyErr != nil {
				test.Error(copyErr)
				response.WriteHeader(http.StatusInternalServerError)
				return
			}
			mutex.Lock()
			blocks[request.URL.Query().Get("blockid")] = size
			mutex.Unlock()
		case "blocklist":
			mutex.Lock()
			decodeErr := xml.NewDecoder(request.Body).Decode(&committed)
			mutex.Unlock()
			if decodeErr != nil {
				test.Error(decodeErr)
				response.WriteHeader(http.StatusBadRequest)
				return
			}
		default:
			test.Error("large file did not use multipart upload")
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		response.WriteHeader(http.StatusCreated)
	})

	require.NoError(test, uploadFile(context.Background(), client, "cosi", "acl-arm64.cosi", file.Name()))
	mutex.Lock()
	defer mutex.Unlock()
	require.Len(test, blocks, 17)
	require.Len(test, committed.Latest, len(blocks))
	var totalBytes int64
	for _, blockID := range committed.Latest {
		size, exists := blocks[blockID]
		require.True(test, exists, "committed an unstaged block")
		require.Positive(test, size)
		require.LessOrEqual(test, size, int64(16*1024*1024))
		totalBytes += size
		delete(blocks, blockID)
	}
	require.Equal(test, fileSize, totalBytes)
	require.Empty(test, blocks)
}

func TestUploadFileMissingFile(test *testing.T) {
	client := newUploadTestClient(test, func(http.ResponseWriter, *http.Request) {
		test.Error("missing file must not send an upload request")
	})
	err := uploadFile(context.Background(), client, "cosi", "missing.cosi", filepath.Join(test.TempDir(), "missing.cosi"))
	require.ErrorIs(test, err, os.ErrNotExist)
}

func TestUploadFileHTTPError(test *testing.T) {
	filePath := filepath.Join(test.TempDir(), "test.cosi")
	require.NoError(test, os.WriteFile(filePath, []byte("test COSI"), 0600))
	client := newUploadTestClient(test, func(response http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		response.Header().Set("x-ms-error-code", "AuthorizationPermissionMismatch")
		response.WriteHeader(http.StatusForbidden)
	})
	err := uploadFile(context.Background(), client, "cosi", "test.cosi", filePath)
	var responseError *azcore.ResponseError
	require.ErrorAs(test, err, &responseError)
	require.Equal(test, http.StatusForbidden, responseError.StatusCode)
	require.Equal(test, "AuthorizationPermissionMismatch", responseError.ErrorCode)
}
