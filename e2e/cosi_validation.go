package e2e

import (
	"archive/tar"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// cosiDownloadTimeout is the maximum time allowed for downloading and
// streaming through the entire COSI file. ACL COSIs can be multi-GB.
const cosiDownloadTimeout = 30 * time.Minute

// COSI metadata structs mirroring the COSI v1.2 specification.
// See: https://github.com/microsoft/trident/docs/Reference/Composable-OS-Image.md

type cosiMetadata struct {
	Version    string              `json:"version"`
	OsArch     string              `json:"osArch"`
	OsRelease  string              `json:"osRelease"`
	Images     []cosiFilesystem    `json:"images"`
	Disk       *cosiDisk           `json:"disk,omitempty"`
	Bootloader *cosiBootloader     `json:"bootloader,omitempty"`
	OsPackages []cosiOsPackage     `json:"osPackages,omitempty"`
	ID         string              `json:"id,omitempty"`
	Compression *cosiCompression   `json:"compression,omitempty"`
}

type cosiFilesystem struct {
	Image      cosiImageFile    `json:"image"`
	MountPoint string           `json:"mountPoint"`
	FsType     string           `json:"fsType"`
	FsUUID     string           `json:"fsUuid"`
	PartType   string           `json:"partType"`
	Verity     *cosiVerityConfig `json:"verity,omitempty"`
}

type cosiImageFile struct {
	Path             string `json:"path"`
	CompressedSize   int64  `json:"compressedSize"`
	UncompressedSize int64  `json:"uncompressedSize"`
	SHA384           string `json:"sha384"`
}

type cosiVerityConfig struct {
	Image    cosiImageFile `json:"image"`
	RootHash string        `json:"roothash"`
}

type cosiDisk struct {
	Size       int64             `json:"size"`
	LBASize    int               `json:"lbaSize"`
	Type       string            `json:"type"`
	GptRegions []cosiGptRegion   `json:"gptRegions,omitempty"`
}

type cosiGptRegion struct {
	Image  cosiImageFile `json:"image"`
	Type   string        `json:"type"`
	Number int           `json:"number,omitempty"`
}

type cosiBootloader struct {
	Type        string           `json:"type"`
	SystemdBoot *cosiSystemdBoot `json:"systemdBoot,omitempty"`
}

type cosiSystemdBoot struct {
	Entries []cosiBootEntry `json:"entries"`
}

type cosiBootEntry struct {
	Type    string `json:"type"`
	Kernel  string `json:"kernel"`
	Path    string `json:"path"`
	Cmdline string `json:"cmdline"`
}

type cosiOsPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Release string `json:"release,omitempty"`
	Arch    string `json:"arch,omitempty"`
}

type cosiCompression struct {
	Type string `json:"type,omitempty"`
}

// newACLCosiValidator returns a validation function tailored to the ACL
// disk layout version. The VERSION_ID from os-release in the COSI metadata
// determines which validator is returned:
//   - 3.0.* → original layout (5 partitions, inline verity hash)
//   - 3.1.* → A/B hash layout (7 partitions, dedicated verity hash partitions)
//
// Each validator runs common checks (COSI version, arch, disk, bootloader,
// compression) then adds version-specific assertions (expected filesystems,
// verity configuration, GPT region counts, etc.).
func newACLCosiValidator(t *testing.T, osRelease string) func(*testing.T, cosiMetadata) {
	t.Helper()
	major, minor := parseACLVersionID(t, osRelease)
	switch {
	case major == 3 && minor == 0:
		t.Log("ACL version 3.0.x — using original disk layout validator")
		return validateACLCosi30
	case major == 3 && minor >= 1:
		t.Logf("ACL version 3.%d.x — using A/B hash disk layout validator", minor)
		return validateACLCosi31
	default:
		t.Fatalf("unsupported ACL major.minor version: %d.%d", major, minor)
		return nil
	}
}

// parseACLVersionID extracts the major and minor version from the VERSION_ID
// field in the os-release content embedded in COSI metadata. VERSION_ID has
// the format "MAJOR.MINOR.YYYYMMDD" (e.g., "3.0.20250601").
func parseACLVersionID(t *testing.T, osRelease string) (major, minor int) {
	t.Helper()
	for _, line := range strings.Split(osRelease, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "VERSION_ID=") {
			versionID := strings.TrimPrefix(line, "VERSION_ID=")
			versionID = strings.Trim(versionID, "\"")
			parts := strings.SplitN(versionID, ".", 3)
			require.True(t, len(parts) >= 2,
				"VERSION_ID must have at least MAJOR.MINOR, got %q", versionID)
			_, err := fmt.Sscanf(parts[0]+"."+parts[1], "%d.%d", &major, &minor)
			require.NoError(t, err, "parsing VERSION_ID major.minor from %q", versionID)
			t.Logf("parsed ACL VERSION_ID=%s (major=%d, minor=%d)", versionID, major, minor)
			return major, minor
		}
	}
	t.Fatal("VERSION_ID not found in osRelease metadata")
	return 0, 0
}

// expectedFilesystem describes an expected filesystem entry in the COSI metadata.
type expectedFilesystem struct {
	MountPoint    string
	FsType        string
	RequireVerity bool
}

// validateACLCosiCommon runs validation checks shared across all ACL disk
// layout versions: COSI version, architecture, disk basics, bootloader,
// and compression.
func validateACLCosiCommon(t *testing.T, m cosiMetadata) {
	t.Helper()
	validateCosiMetadataVersion(t, m)
	validateCosiOsArch(t, m)
	validateCosiDisk(t, m)
	validateCosiBootloader(t, m)
	validateCosiCompression(t, m)
}

// validateACLCosi30 validates the 3.0.x disk layout:
// 5 partitions (ESP, USR-A, USR-B, OEM, ROOT). USR-B is an empty A/B slot
// and is not in the COSI images array. Verity hash for USR-A is inline
// (referenced via the filesystem's verity.image field, no dedicated partition).
func validateACLCosi30(t *testing.T, m cosiMetadata) {
	t.Helper()
	validateACLCosiCommon(t, m)

	expected := []expectedFilesystem{
		{MountPoint: "/boot", FsType: "vfat", RequireVerity: false},
		{MountPoint: "/usr", FsType: "btrfs", RequireVerity: true},
		{MountPoint: "/oem", FsType: "btrfs", RequireVerity: false},
		{MountPoint: "/", FsType: "ext4", RequireVerity: false},
	}
	validateExpectedFilesystems(t, m, expected)
}

// validateACLCosi31 validates the 3.1.x disk layout:
// 7 partitions (ESP, USR-A, HASH-A, USR-B, HASH-B, OEM, ROOT).
// USR-B and HASH-B are empty A/B slots — not in the COSI images array.
// HASH-A is a dedicated verity hash partition for USR-A.
//
// TODO: update expected filesystems and verity checks once 3.1.x COSI
// output is known (HASH-A may appear as a separate images[] entry or
// may remain referenced only via verity.image on /usr).
func validateACLCosi31(t *testing.T, m cosiMetadata) {
	t.Helper()
	validateACLCosiCommon(t, m)

	expected := []expectedFilesystem{
		{MountPoint: "/boot", FsType: "vfat", RequireVerity: false},
		{MountPoint: "/usr", FsType: "btrfs", RequireVerity: true},
		{MountPoint: "/oem", FsType: "btrfs", RequireVerity: false},
		{MountPoint: "/", FsType: "ext4", RequireVerity: false},
	}
	validateExpectedFilesystems(t, m, expected)

	// TODO: add 3.1.x-specific checks:
	// - verify HASH-A partition exists in GPT regions with type dps-usr-verity
	// - verify verity hash image for /usr references the HASH-A partition image
	// - verify GPT region count reflects 7 partitions
}

// ESP partition type GUID per Discoverable Partition Specification
const espPartTypeGUID = "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"

// cosiPublishingInfo is the JSON structure written by convert-vhd-to-cosi.sh.
type cosiPublishingInfo struct {
	CosiURL string `json:"cosi_url"`
}

// loadCOSIURL reads the cosi_url from a COSI publishing info artifact directory.
// artifactName is the pipeline artifact name (e.g., "cosi-publishing-info-acl-tl-gen2").
// Returns the URL, or empty string if COSI_ARTIFACTS_DIR is not set or the
// artifact was not downloaded (variant not built).
func loadCOSIURL(t *testing.T, artifactName string) string {
	t.Helper()
	dir := os.Getenv("COSI_ARTIFACTS_DIR")
	if dir == "" {
		return ""
	}
	infoPath := filepath.Join(dir, artifactName, "cosi-publishing-info.json")
	data, err := os.ReadFile(infoPath)
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err, "reading %s", infoPath)
	var info cosiPublishingInfo
	require.NoError(t, json.Unmarshal(data, &info), "parsing %s", infoPath)
	require.NotEmpty(t, info.CosiURL, "cosi_url is empty in %s", infoPath)
	return info.CosiURL
}

// ValidateACLCOSI downloads a COSI file from the given URL and validates its
// structure and metadata against the expected ACL UKI disk layout.
func ValidateACLCOSI(t *testing.T, cosiURL string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), cosiDownloadTimeout)
	defer cancel()

	t.Logf("downloading COSI from %s", sanitizeURL(cosiURL))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cosiURL, nil)
	require.NoError(t, err, "creating HTTP request for COSI download")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "downloading COSI file")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "COSI download returned non-200 status: %d", resp.StatusCode)

	tr := tar.NewReader(resp.Body)

	// Track which image paths we find in the tar
	tarImagePaths := make(map[string]string) // path -> sha384 hex

	// --- 1. Validate cosi-marker (must be first entry) ---
	header, err := tr.Next()
	require.NoError(t, err, "reading first tar entry (cosi-marker)")
	require.Equal(t, "cosi-marker", header.Name, "first tar entry must be 'cosi-marker'")
	require.Equal(t, int64(0), header.Size, "cosi-marker must be empty")
	t.Log("✓ cosi-marker is first entry and empty")

	// --- 2. Validate metadata.json (must be second entry) ---
	header, err = tr.Next()
	require.NoError(t, err, "reading second tar entry (metadata.json)")
	require.Equal(t, "metadata.json", header.Name, "second tar entry must be 'metadata.json'")
	require.True(t, header.Size > 0, "metadata.json must not be empty")

	// Read and parse metadata
	metadataBytes := make([]byte, header.Size)
	_, err = io.ReadFull(tr, metadataBytes)
	require.NoError(t, err, "reading metadata.json content")

	var metadata cosiMetadata
	err = json.Unmarshal(metadataBytes, &metadata)
	require.NoError(t, err, "parsing metadata.json")
	t.Log("✓ metadata.json parsed successfully")

	// --- 3. Version-aware validation (common + version-specific checks) ---
	validateACLLayout := newACLCosiValidator(t, metadata.OsRelease)
	validateACLLayout(t, metadata)

	// Collect all image paths referenced in metadata
	metadataImagePaths := collectMetadataImagePaths(t, metadata)

	// --- 4. Stream remaining tar entries and validate images ---
	for {
		header, err = tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "reading tar entry")

		// Skip directory entries
		if header.Typeflag == tar.TypeDir {
			continue
		}

		require.True(t, strings.HasPrefix(header.Name, "images/"),
			"unexpected tar entry outside images/: %s", header.Name)
		require.Equal(t, byte(tar.TypeReg), header.Typeflag,
			"image entry %s must be a regular file", header.Name)

		// Compute SHA-384 while streaming
		hasher := sha512.New384()
		n, err := io.Copy(hasher, tr)
		require.NoError(t, err, "reading image entry %s", header.Name)
		require.Equal(t, header.Size, n, "image entry %s: read size mismatch", header.Name)

		sha384Hex := hex.EncodeToString(hasher.Sum(nil))
		require.NotContains(t, tarImagePaths, header.Name,
			"duplicate tar entry: %s", header.Name)
		tarImagePaths[header.Name] = sha384Hex
	}

	t.Logf("✓ found %d image entries in tar", len(tarImagePaths))

	// --- 5. Cross-reference metadata image paths with tar entries ---
	for path, expectedHash := range metadataImagePaths {
		actualHash, found := tarImagePaths[path]
		require.True(t, found,
			"metadata references image %q but it was not found in tar", path)
		require.Equal(t, strings.ToLower(expectedHash), strings.ToLower(actualHash),
			"SHA-384 mismatch for image %s", path)
		t.Logf("✓ image %s: SHA-384 verified", path)
	}

	// Verify no extra images in tar that aren't in metadata
	for path := range tarImagePaths {
		_, found := metadataImagePaths[path]
		require.True(t, found,
			"tar contains image %q not referenced in metadata", path)
	}

	t.Logf("✓ all %d image entries cross-referenced with metadata", len(tarImagePaths))
}

// validateCosiMetadataVersion checks the COSI version is >= 1.2.
func validateCosiMetadataVersion(t *testing.T, m cosiMetadata) {
	t.Helper()
	parts := strings.SplitN(m.Version, ".", 2)
	require.Len(t, parts, 2, "version must be MAJOR.MINOR format, got %q", m.Version)

	var major, minor int
	_, err := fmt.Sscanf(m.Version, "%d.%d", &major, &minor)
	require.NoError(t, err, "parsing version %q", m.Version)
	require.True(t, major > 1 || (major == 1 && minor >= 2),
		"COSI version must be >= 1.2, got %d.%d", major, minor)
	t.Logf("✓ COSI version: %s", m.Version)
}

// validACLArchitectures lists the architectures expected in ACL COSI images.
var validACLArchitectures = map[string]bool{
	"x86_64":  true,
	"aarch64": true,
}

// validateCosiOsArch checks the architecture field is a known ACL architecture.
func validateCosiOsArch(t *testing.T, m cosiMetadata) {
	t.Helper()
	require.True(t, validACLArchitectures[m.OsArch],
		"unexpected osArch %q, expected one of: x86_64, aarch64", m.OsArch)
	t.Logf("✓ osArch: %s", m.OsArch)
}

// validateCosiDisk validates the disk metadata.
func validateCosiDisk(t *testing.T, m cosiMetadata) {
	t.Helper()
	require.NotNil(t, m.Disk, "disk metadata must be present for COSI >= 1.2")
	require.Equal(t, "gpt", m.Disk.Type, "disk type must be gpt")
	require.True(t, m.Disk.Size > 0, "disk size must be > 0")
	require.True(t, m.Disk.LBASize > 0, "LBA size must be > 0")
	require.NotEmpty(t, m.Disk.GptRegions, "gptRegions must not be empty")

	// First GPT region must be primary-gpt
	require.Equal(t, "primary-gpt", m.Disk.GptRegions[0].Type,
		"first GPT region must be primary-gpt")

	// Count partition regions
	partitionCount := 0
	for _, region := range m.Disk.GptRegions {
		if region.Type == "partition" {
			partitionCount++
			require.True(t, region.Number > 0,
				"partition region must have a positive number")
		}
		// Validate image file for each region
		validateImageFile(t, region.Image, fmt.Sprintf("gptRegion[%s]", region.Type))
	}
	require.True(t, partitionCount > 0, "must have at least one partition region")

	t.Logf("✓ disk: %s, %d GPT regions (%d partitions), size=%d bytes",
		m.Disk.Type, len(m.Disk.GptRegions), partitionCount, m.Disk.Size)
}

// validateCosiBootloader checks the bootloader metadata.
func validateCosiBootloader(t *testing.T, m cosiMetadata) {
	t.Helper()
	require.NotNil(t, m.Bootloader, "bootloader metadata must be present")
	require.Equal(t, "systemd-boot", m.Bootloader.Type,
		"ACL uses systemd-boot bootloader")

	require.NotNil(t, m.Bootloader.SystemdBoot,
		"systemdBoot config must be present when type is systemd-boot")
	require.NotEmpty(t, m.Bootloader.SystemdBoot.Entries,
		"systemdBoot must have at least one boot entry")

	t.Logf("✓ bootloader: %s with %d entries",
		m.Bootloader.Type, len(m.Bootloader.SystemdBoot.Entries))
}

// validateExpectedFilesystems checks that the COSI images array exactly matches
// the given expected filesystem list — no missing, no extras.
func validateExpectedFilesystems(t *testing.T, m cosiMetadata, expected []expectedFilesystem) {
	t.Helper()

	// Exact count — no unexpected filesystems
	require.Equal(t, len(expected), len(m.Images),
		"expected %d filesystem images but COSI contains %d", len(expected), len(m.Images))

	// Build a lookup by mount point
	fsByMount := make(map[string]*cosiFilesystem)
	for i := range m.Images {
		fs := &m.Images[i]
		require.NotContains(t, fsByMount, fs.MountPoint,
			"duplicate mount point: %s", fs.MountPoint)
		fsByMount[fs.MountPoint] = fs
	}

	// Validate each expected filesystem is present with correct properties
	for _, exp := range expected {
		fs, found := fsByMount[exp.MountPoint]
		require.True(t, found,
			"expected filesystem with mount point %q not found in COSI metadata", exp.MountPoint)
		require.Equal(t, exp.FsType, fs.FsType,
			"mount point %s: expected fsType %q, got %q", exp.MountPoint, exp.FsType, fs.FsType)

		// Validate image file
		validateImageFile(t, fs.Image, fmt.Sprintf("filesystem[%s]", exp.MountPoint))

		// Validate fsUuid is present and non-empty
		require.NotEmpty(t, fs.FsUUID,
			"mount point %s: fsUuid must not be empty", exp.MountPoint)

		// Validate partition type GUID
		require.NotEmpty(t, fs.PartType,
			"mount point %s: partType must not be empty", exp.MountPoint)

		// ESP must use the standard ESP partition type GUID
		if exp.MountPoint == "/boot" {
			require.Equal(t, espPartTypeGUID, strings.ToLower(fs.PartType),
				"mount point /boot: partType must be ESP GUID")
		}

		// Verity validation
		if exp.RequireVerity {
			require.NotNil(t, fs.Verity,
				"mount point %s: verity must be present", exp.MountPoint)
			require.NotEmpty(t, fs.Verity.RootHash,
				"mount point %s: verity roothash must not be empty", exp.MountPoint)
			validateImageFile(t, fs.Verity.Image,
				fmt.Sprintf("filesystem[%s].verity", exp.MountPoint))
		}

		t.Logf("✓ filesystem %s: %s (verity=%v)", exp.MountPoint, exp.FsType, exp.RequireVerity)
	}

	t.Logf("✓ all %d expected filesystems validated (exact match)", len(expected))
}

// validateCosiCompression checks the compression metadata.
func validateCosiCompression(t *testing.T, m cosiMetadata) {
	t.Helper()
	require.NotNil(t, m.Compression, "compression metadata must be present for COSI >= 1.2")
	t.Log("✓ compression metadata present")
}

// validateImageFile checks that an ImageFile has valid fields.
func validateImageFile(t *testing.T, img cosiImageFile, context string) {
	t.Helper()
	require.True(t, strings.HasPrefix(img.Path, "images/"),
		"%s: image path must start with 'images/', got %q", context, img.Path)
	require.True(t, img.CompressedSize > 0,
		"%s: compressedSize must be > 0", context)
	require.True(t, img.UncompressedSize > 0,
		"%s: uncompressedSize must be > 0", context)
	require.NotEmpty(t, img.SHA384,
		"%s: sha384 must not be empty", context)
	require.Len(t, mustDecodeHex(t, img.SHA384), 48,
		"%s: sha384 must be 48 bytes (384 bits)", context)
}

// collectMetadataImagePaths returns a map of all image paths referenced in
// metadata (from both images[] and disk.gptRegions[]) to their expected SHA-384.
func collectMetadataImagePaths(t *testing.T, m cosiMetadata) map[string]string {
	t.Helper()
	paths := make(map[string]string)

	// From filesystem images
	for _, fs := range m.Images {
		paths[fs.Image.Path] = fs.Image.SHA384
		if fs.Verity != nil {
			paths[fs.Verity.Image.Path] = fs.Verity.Image.SHA384
		}
	}

	// From GPT regions
	if m.Disk != nil {
		for _, region := range m.Disk.GptRegions {
			if existing, ok := paths[region.Image.Path]; ok {
				// Spec says ImageFile objects must be identical when they correspond
				require.Equal(t, strings.ToLower(existing), strings.ToLower(region.Image.SHA384),
					"GPT region image %s has different SHA-384 than filesystem image", region.Image.Path)
			}
			paths[region.Image.Path] = region.Image.SHA384
		}
	}

	return paths
}

// mustDecodeHex decodes a hex string and fails the test on error.
func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err, "decoding hex string %q", s)
	return b
}

// sanitizeURL removes query parameters (which may contain SAS tokens) from a
// URL for safe logging.
func sanitizeURL(u string) string {
	if idx := strings.IndexByte(u, '?'); idx >= 0 {
		return u[:idx] + "?<redacted>"
	}
	return u
}
