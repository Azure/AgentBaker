package scenario

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/stretchr/testify/require"
)

const aclCOSIAMD64ImageID = "/SharedGalleries/035db282-f1c8-4ce7-b78f-2a7265d5398c-ACLDEVEL/Images/acldevel/Versions/0.20260827.1192019"

func TestCOSIUpdateScenariosDisabled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COSI_ARTIFACTS_DIR", dir)
	data, err := json.Marshal(cosiPublishingInfo{
		CosiURL:        "https://download.invalid/image.cosi",
		MetadataSHA384: base64.StdEncoding.EncodeToString(make([]byte, sha512.Size384)),
	})
	require.NoError(t, err)

	for _, test := range []struct {
		artifact string
		build    func() *Scenario
	}{
		{cosiAMD64PublishingArtifact, cosiUpdateAMD64Scenario},
		{cosiARM64PublishingArtifact, cosiUpdateARM64Scenario},
		{cosiAMD64FIPSPublishingArtifact, cosiUpdateAMD64FIPSScenario},
		{cosiARM64FIPSPublishingArtifact, cosiUpdateARM64FIPSScenario},
	} {
		t.Run(test.artifact, func(t *testing.T) {
			artifactDir := filepath.Join(dir, test.artifact)
			require.NoError(t, os.Mkdir(artifactDir, 0700))
			require.NoError(t, os.WriteFile(filepath.Join(artifactDir, "cosi-publishing-info.json"), data, 0600))

			s := test.build()
			require.True(t, s.Tags.COSIUpdate)
			require.NotNil(t, s.Validator)
			outcome := runExecution(t.Context(), s.Name, s.Name, s,
				func(context.Context, string, *Scenario) error {
					t.Fatal("disabled COSI update scenario ran")
					return nil
				})
			require.NoError(t, outcome.Error)
			require.NotEmpty(t, outcome.SkipReason)
		})
	}
}

func TestValidateCOSIUpdateInput(t *testing.T) {
	validHash := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xab}, sha512.Size384))
	require.NoError(t, validateCOSIUpdateInput("https://download.example.com/acl.cosi", validHash))
	require.ErrorContains(t, validateCOSIUpdateInput("http://download.example.com/acl.cosi", validHash), "HTTPS")
	require.ErrorContains(t, validateCOSIUpdateInput("https://download.example.com/acl.cosi", "abcd"), "48 bytes")
}

func TestResolveCOSISharedGalleryImageReference(t *testing.T) {
	imageReference, err := resolveImageReference(context.Background(), &config.Image{
		SharedGalleryImageID: aclCOSIAMD64ImageID,
		Version:              "would-trigger-gallery-lookup-without-shared-id",
	}, "westus2")

	require.NoError(t, err)
	require.Nil(t, imageReference.ID)
	require.Equal(t, aclCOSIAMD64ImageID, *imageReference.SharedGalleryImageID)
}

func TestStageCOSIUpdateImage(t *testing.T) {
	t.Setenv("AGENT_TEMPDIRECTORY", t.TempDir())
	payload := []byte("test COSI bytes")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write(payload)
	}))
	defer server.Close()
	names := map[string]bool{}
	for _, uploadErr := range []error{nil, errors.New("upload failed")} {
		var localPath string
		blobName, signedURL, err := stageCOSIUpdateImage(t.Context(), server.URL, func(ctx context.Context, name string, file *os.File) (string, error) {
			require.NoError(t, ctx.Err())
			require.True(t, strings.HasPrefix(name, "cosi-updates/acl-cosi-"))
			require.False(t, names[name])
			names[name] = true
			contents, readErr := io.ReadAll(file)
			require.NoError(t, readErr)
			require.Equal(t, payload, contents)
			localPath = file.Name()
			return "https://storage.invalid/test.cosi?sig=private", uploadErr
		})
		require.ErrorIs(t, err, uploadErr)
		require.True(t, names[blobName])
		require.Equal(t, "https://storage.invalid/test.cosi?sig=private", signedURL)
		require.NoFileExists(t, localPath)
	}
}

func TestCheckTridentUpdateResult(t *testing.T) {
	for _, test := range []struct {
		name      string
		result    *podExecResult
		err       error
		wantError string
	}{
		{name: "success", result: &podExecResult{exitCode: "0", stdout: "https://storage.invalid/image?sig=private"}},
		{name: "exit failure", result: &podExecResult{exitCode: "1", stderr: "https://storage.invalid/image?sig=private"}, wantError: "exited with code 1"},
		{name: "transport failure", err: errors.New("https://storage.invalid/image?sig=private"), wantError: "SSH command error"},
		{name: "missing result", wantError: "returned no result"},
	} {
		t.Run(test.name, func(t *testing.T) {
			logger := &executionLogger{}
			err := checkTridentUpdateResult(logging.WithLogger(t.Context(), logger), "stage", test.result, test.err)
			if test.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantError)
				require.NotContains(t, err.Error(), "sig=private")
			}
			require.NotContains(t, strings.Join(logger.logs, "\n"), "sig=private")
		})
	}
}

func TestLoadCOSIPublishingInfo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("COSI_ARTIFACTS_DIR", dir)
	_, exists := loadCOSIPublishingInfo("not-built")
	require.False(t, exists)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "built"), 0700))
	info := cosiPublishingInfo{
		CosiURL:        "https://download.invalid/image.cosi",
		MetadataSHA384: base64.StdEncoding.EncodeToString(make([]byte, sha512.Size384)),
	}
	writeInfo := func() {
		data, err := json.Marshal(info)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "built", "cosi-publishing-info.json"), data, 0600))
	}
	writeInfo()
	actual, exists := loadCOSIPublishingInfo("built")
	require.True(t, exists)
	require.Equal(t, info, actual)
	info.MetadataSHA384 = "bad-hash"
	writeInfo()
	require.Panics(t, func() { loadCOSIPublishingInfo("built") })
}
