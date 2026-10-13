package scenario

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/stretchr/testify/require"
)

const aclCOSIAMD64ImageID = "/SharedGalleries/035db282-f1c8-4ce7-b78f-2a7265d5398c-ACLDEVEL/Images/acldevel/Versions/0.20260827.1192019"

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

func TestValidateACLCOSIContent(t *testing.T) {
	t.Setenv("AGENT_TEMPDIRECTORY", t.TempDir())
	for _, test := range []struct {
		name                string
		version             string
		architecture        string
		corruptImage        bool
		corruptMetadataHash bool
		wantError           string
	}{
		{name: "amd64 3.0", version: "3.0.20260912", architecture: "x86_64"},
		{name: "arm64 3.1", version: "3.1.20260912", architecture: "aarch64"},
		{name: "corrupt image", version: "3.0.20260912", architecture: "x86_64", corruptImage: true, wantError: "SHA-384 mismatch"},
		{name: "corrupt metadata", version: "3.0.20260912", architecture: "x86_64", corruptMetadataHash: true, wantError: "does not match the publishing artifact"},
		{name: "unknown layout", version: "4.0.20260912", architecture: "x86_64", wantError: "unsupported ACL"},
		{name: "unknown architecture", version: "3.0.20260912", architecture: "riscv64", wantError: "unexpected osArch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			archive, metadataHash := cosiTestArchive(t, test.version, test.architecture, test.corruptImage)
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				_, _ = response.Write(archive)
			}))
			defer server.Close()
			if test.corruptMetadataHash {
				metadataHash = base64.StdEncoding.EncodeToString(make([]byte, sha512.Size384))
			}
			err := validateACLCOSIContent(t.Context(), server.URL, metadataHash)
			if test.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantError)
			}
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

func TestCOSIContentMetadataLimit(t *testing.T) {
	t.Setenv("AGENT_TEMPDIRECTORY", t.TempDir())
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	require.NoError(t, writer.WriteHeader(&tar.Header{Name: "cosi-marker", Mode: 0600}))
	require.NoError(t, writer.WriteHeader(&tar.Header{Name: "metadata.json", Mode: 0600, Size: 64*1024*1024 + 1}))
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		_, _ = response.Write(archive.Bytes())
	}))
	defer server.Close()
	err := validateACLCOSIContent(t.Context(), server.URL, "unused")
	require.ErrorContains(t, err, "64 MiB limit")
}

func cosiTestArchive(t *testing.T, version, architecture string, corruptImage bool) ([]byte, string) {
	t.Helper()
	payload := []byte("test image bytes")
	payloadHash := sha512.Sum384(payload)
	files := map[string][]byte{}
	image := func(name string) cosiImageFile {
		path := "images/" + name
		files[path] = payload
		return cosiImageFile{Path: path, CompressedSize: int64(len(payload)), UncompressedSize: int64(len(payload)), SHA384: hex.EncodeToString(payloadHash[:])}
	}
	metadata := cosiMetadata{
		Version:     "1.2",
		OsArch:      architecture,
		OsRelease:   "VERSION_ID=\"" + version + "\"\n",
		Bootloader:  &cosiBootloader{Type: "systemd-boot", SystemdBoot: &cosiSystemdBoot{Entries: []cosiBootEntry{{Type: "uki"}}}},
		Compression: &cosiCompression{Type: "zstd"},
		Disk: &cosiDisk{Size: 1024 * 1024, LBASize: 512, Type: "gpt", GptRegions: []cosiGptRegion{
			{Type: "primary-gpt", Image: image("gpt.raw")},
			{Type: "partition", Number: 1, Image: image("boot.raw")},
		}},
		Images: []cosiFilesystem{
			{MountPoint: "/boot", FsType: "vfat", FsUUID: "boot", PartType: espPartTypeGUID, Image: image("boot.raw")},
			{MountPoint: "/usr", FsType: "btrfs", FsUUID: "usr", PartType: "usr-guid", Image: image("usr.raw"), Verity: &cosiVerityConfig{Image: image("usr.hash"), RootHash: "root-hash"}},
			{MountPoint: "/oem", FsType: "btrfs", FsUUID: "oem", PartType: "oem-guid", Image: image("oem.raw")},
			{MountPoint: "/", FsType: "ext4", FsUUID: "root", PartType: "root-guid", Image: image("root.raw")},
		},
	}
	metadataBytes, err := json.Marshal(metadata)
	require.NoError(t, err)
	metadataHash := sha512.Sum384(metadataBytes)
	if corruptImage {
		files["images/usr.raw"] = []byte("corrupted image")
	}
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	writeEntry := func(name string, contents []byte) {
		require.NoError(t, writer.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(contents))}))
		_, err := writer.Write(contents)
		require.NoError(t, err)
	}
	writeEntry("cosi-marker", nil)
	writeEntry("metadata.json", metadataBytes)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		writeEntry(name, files[name])
	}
	require.NoError(t, writer.Close())
	return archive.Bytes(), base64.StdEncoding.EncodeToString(metadataHash[:])
}
