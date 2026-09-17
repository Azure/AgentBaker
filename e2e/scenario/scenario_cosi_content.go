package scenario

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

// COSI metadata structs mirroring the COSI v1.2 specification.
// See: https://github.com/microsoft/trident/docs/Reference/Composable-OS-Image.md

type cosiMetadata struct {
	Version     string           `json:"version"`
	OsArch      string           `json:"osArch"`
	OsRelease   string           `json:"osRelease"`
	Images      []cosiFilesystem `json:"images"`
	Disk        *cosiDisk        `json:"disk,omitempty"`
	Bootloader  *cosiBootloader  `json:"bootloader,omitempty"`
	OsPackages  []cosiOsPackage  `json:"osPackages,omitempty"`
	ID          string           `json:"id,omitempty"`
	Compression *cosiCompression `json:"compression,omitempty"`
}

type cosiFilesystem struct {
	Image      cosiImageFile     `json:"image"`
	MountPoint string            `json:"mountPoint"`
	FsType     string            `json:"fsType"`
	FsUUID     string            `json:"fsUuid"`
	PartType   string            `json:"partType"`
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
	Size       int64           `json:"size"`
	LBASize    int             `json:"lbaSize"`
	Type       string          `json:"type"`
	GptRegions []cosiGptRegion `json:"gptRegions,omitempty"`
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
func newACLCosiValidator(osRelease string) (func(cosiMetadata) error, error) {
	major, minor, err := parseACLVersionID(osRelease)
	if err != nil {
		return nil, err
	}
	switch {
	case major == 3 && minor == 0:
		return validateACLCosi30, nil
	case major == 3 && minor >= 1:
		return validateACLCosi31, nil
	default:
		return nil, fmt.Errorf("unsupported ACL major.minor version: %d.%d", major, minor)
	}
}

// parseACLVersionID extracts the major and minor version from the VERSION_ID
// field in the os-release content embedded in COSI metadata. VERSION_ID has
// the format "MAJOR.MINOR.YYYYMMDD" (e.g., "3.0.20250601").
func parseACLVersionID(osRelease string) (major, minor int, err error) {
	for _, line := range strings.Split(osRelease, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "VERSION_ID=") {
			versionID := strings.TrimPrefix(line, "VERSION_ID=")
			versionID = strings.Trim(versionID, "\"")
			parts := strings.SplitN(versionID, ".", 3)
			if len(parts) < 2 {
				return 0, 0, fmt.Errorf("VERSION_ID must have at least MAJOR.MINOR, got %q", versionID)
			}
			if _, err := fmt.Sscanf(parts[0]+"."+parts[1], "%d.%d", &major, &minor); err != nil {
				return 0, 0, fmt.Errorf("parsing VERSION_ID major.minor from %q: %w", versionID, err)
			}
			return major, minor, nil
		}
	}
	return 0, 0, fmt.Errorf("VERSION_ID not found in osRelease metadata")
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
func validateACLCosiCommon(m cosiMetadata) error {
	if err := validateCosiMetadataVersion(m); err != nil {
		return err
	}
	if err := validateCosiOsArch(m); err != nil {
		return err
	}
	if err := validateCosiDisk(m); err != nil {
		return err
	}
	if err := validateCosiBootloader(m); err != nil {
		return err
	}
	return validateCosiCompression(m)
}

// validateACLCosi30 validates the 3.0.x disk layout:
// 5 partitions (ESP, USR-A, USR-B, OEM, ROOT). USR-B is an empty A/B slot
// and is not in the COSI images array. Verity hash for USR-A is inline
// (referenced via the filesystem's verity.image field, no dedicated partition).
func validateACLCosi30(m cosiMetadata) error {
	if err := validateACLCosiCommon(m); err != nil {
		return err
	}

	expected := []expectedFilesystem{
		{MountPoint: "/boot", FsType: "vfat", RequireVerity: false},
		{MountPoint: "/usr", FsType: "btrfs", RequireVerity: true},
		{MountPoint: "/oem", FsType: "btrfs", RequireVerity: false},
		{MountPoint: "/", FsType: "ext4", RequireVerity: false},
	}
	return validateExpectedFilesystems(m, expected)
}

// validateACLCosi31 validates the 3.1.x disk layout:
// 7 partitions (ESP, USR-A, HASH-A, USR-B, HASH-B, OEM, ROOT).
// USR-B and HASH-B are empty A/B slots — not in the COSI images array.
// HASH-A is a dedicated verity hash partition for USR-A.
//
// TODO: update expected filesystems and verity checks once 3.1.x COSI
// output is known (HASH-A may appear as a separate images[] entry or
// may remain referenced only via verity.image on /usr).
func validateACLCosi31(m cosiMetadata) error {
	if err := validateACLCosiCommon(m); err != nil {
		return err
	}

	expected := []expectedFilesystem{
		{MountPoint: "/boot", FsType: "vfat", RequireVerity: false},
		{MountPoint: "/usr", FsType: "btrfs", RequireVerity: true},
		{MountPoint: "/oem", FsType: "btrfs", RequireVerity: false},
		{MountPoint: "/", FsType: "ext4", RequireVerity: false},
	}
	if err := validateExpectedFilesystems(m, expected); err != nil {
		return err
	}

	// TODO: add 3.1.x-specific checks:
	// - verify HASH-A partition exists in GPT regions with type dps-usr-verity
	// - verify verity hash image for /usr references the HASH-A partition image
	// - verify GPT region count reflects 7 partitions
	return nil
}

// ESP partition type GUID per Discoverable Partition Specification
const espPartTypeGUID = "c12a7328-f81f-11d2-ba4b-00a0c93ec93b"

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

// validateACLCOSIContent downloads a COSI file from the given URL and
// validates its structure and metadata against the expected ACL disk
// layout. It runs entirely on the test runner and does not provision a VM.
func validateACLCOSIContent(ctx context.Context, cosiURL string) error {
	dir, cleanup, err := cosiDownloadDir()
	if err != nil {
		return err
	}
	defer cleanup()

	localPath := filepath.Join(dir, "acl.cosi")
	logging.Logf(ctx, "downloading COSI from %s to %s", sanitizeURL(cosiURL), localPath)
	if err := downloadCOSIFileWithRetry(ctx, cosiURL, localPath); err != nil {
		return fmt.Errorf("downloading COSI file: %w", err)
	}

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("opening downloaded COSI file: %w", err)
	}
	defer f.Close()

	tr := tar.NewReader(f)

	// Track which image paths we find in the tar
	tarImagePaths := make(map[string]string) // path -> sha384 hex

	// --- 1. Validate cosi-marker (must be first entry) ---
	header, err := tr.Next()
	if err != nil {
		return fmt.Errorf("reading first tar entry (cosi-marker): %w", err)
	}
	if header.Name != "cosi-marker" {
		return fmt.Errorf("first tar entry must be 'cosi-marker', got %q", header.Name)
	}
	if header.Size != 0 {
		return fmt.Errorf("cosi-marker must be empty, got size %d", header.Size)
	}
	logging.Logf(ctx, "cosi-marker is first entry and empty")

	// --- 2. Validate metadata.json (must be second entry) ---
	header, err = tr.Next()
	if err != nil {
		return fmt.Errorf("reading second tar entry (metadata.json): %w", err)
	}
	if header.Name != "metadata.json" {
		return fmt.Errorf("second tar entry must be 'metadata.json', got %q", header.Name)
	}
	if header.Size <= 0 {
		return fmt.Errorf("metadata.json must not be empty")
	}

	// Read and parse metadata
	metadataBytes := make([]byte, header.Size)
	if _, err := io.ReadFull(tr, metadataBytes); err != nil {
		return fmt.Errorf("reading metadata.json content: %w", err)
	}

	var metadata cosiMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return fmt.Errorf("parsing metadata.json: %w", err)
	}
	logging.Logf(ctx, "metadata.json parsed successfully")

	// --- 3. Version-aware validation (common + version-specific checks) ---
	validateACLLayout, err := newACLCosiValidator(metadata.OsRelease)
	if err != nil {
		return err
	}
	if err := validateACLLayout(metadata); err != nil {
		return err
	}

	// Collect all image paths referenced in metadata
	metadataImagePaths, err := collectMetadataImagePaths(metadata)
	if err != nil {
		return err
	}

	// --- 4. Stream remaining tar entries and validate images ---
	for {
		header, err = tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar entry: %w", err)
		}

		// Skip directory entries
		if header.Typeflag == tar.TypeDir {
			continue
		}

		if !strings.HasPrefix(header.Name, "images/") {
			return fmt.Errorf("unexpected tar entry outside images/: %s", header.Name)
		}
		if header.Typeflag != tar.TypeReg {
			return fmt.Errorf("image entry %s must be a regular file", header.Name)
		}

		// Compute SHA-384 while streaming
		hasher := sha512.New384()
		n, err := io.Copy(hasher, tr)
		if err != nil {
			return fmt.Errorf("reading image entry %s: %w", header.Name, err)
		}
		if n != header.Size {
			return fmt.Errorf("image entry %s: read size mismatch, expected %d got %d", header.Name, header.Size, n)
		}

		sha384Hex := hex.EncodeToString(hasher.Sum(nil))
		if _, exists := tarImagePaths[header.Name]; exists {
			return fmt.Errorf("duplicate tar entry: %s", header.Name)
		}
		tarImagePaths[header.Name] = sha384Hex
	}

	logging.Logf(ctx, "found %d image entries in tar", len(tarImagePaths))

	// --- 5. Cross-reference metadata image paths with tar entries ---
	for path, expectedHash := range metadataImagePaths {
		actualHash, found := tarImagePaths[path]
		if !found {
			return fmt.Errorf("metadata references image %q but it was not found in tar", path)
		}
		if !strings.EqualFold(expectedHash, actualHash) {
			return fmt.Errorf("SHA-384 mismatch for image %s", path)
		}
	}

	// Verify no extra images in tar that aren't in metadata
	for path := range tarImagePaths {
		if _, found := metadataImagePaths[path]; !found {
			return fmt.Errorf("tar contains image %q not referenced in metadata", path)
		}
	}

	logging.Logf(ctx, "all %d image entries cross-referenced with metadata", len(tarImagePaths))
	return nil
}

// validateCosiMetadataVersion checks the COSI version is >= 1.2.
func validateCosiMetadataVersion(m cosiMetadata) error {
	parts := strings.SplitN(m.Version, ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("version must be MAJOR.MINOR format, got %q", m.Version)
	}

	var major, minor int
	if _, err := fmt.Sscanf(m.Version, "%d.%d", &major, &minor); err != nil {
		return fmt.Errorf("parsing version %q: %w", m.Version, err)
	}
	if !(major > 1 || (major == 1 && minor >= 2)) {
		return fmt.Errorf("COSI version must be >= 1.2, got %d.%d", major, minor)
	}
	return nil
}

// validACLArchitectures lists the architectures expected in ACL COSI images.
var validACLArchitectures = map[string]bool{
	"x86_64":  true,
	"aarch64": true,
}

// validateCosiOsArch checks the architecture field is a known ACL architecture.
func validateCosiOsArch(m cosiMetadata) error {
	if !validACLArchitectures[m.OsArch] {
		return fmt.Errorf("unexpected osArch %q, expected one of: x86_64, aarch64", m.OsArch)
	}
	return nil
}

// validateCosiDisk validates the disk metadata.
func validateCosiDisk(m cosiMetadata) error {
	if m.Disk == nil {
		return fmt.Errorf("disk metadata must be present for COSI >= 1.2")
	}
	if m.Disk.Type != "gpt" {
		return fmt.Errorf("disk type must be gpt, got %q", m.Disk.Type)
	}
	if m.Disk.Size <= 0 {
		return fmt.Errorf("disk size must be > 0")
	}
	if m.Disk.LBASize <= 0 {
		return fmt.Errorf("LBA size must be > 0")
	}
	if len(m.Disk.GptRegions) == 0 {
		return fmt.Errorf("gptRegions must not be empty")
	}

	// First GPT region must be primary-gpt
	if m.Disk.GptRegions[0].Type != "primary-gpt" {
		return fmt.Errorf("first GPT region must be primary-gpt, got %q", m.Disk.GptRegions[0].Type)
	}

	// Count partition regions
	partitionCount := 0
	for _, region := range m.Disk.GptRegions {
		if region.Type == "partition" {
			partitionCount++
			if region.Number <= 0 {
				return fmt.Errorf("partition region must have a positive number")
			}
		}
		// Validate image file for each region
		if err := validateImageFile(region.Image, fmt.Sprintf("gptRegion[%s]", region.Type)); err != nil {
			return err
		}
	}
	if partitionCount == 0 {
		return fmt.Errorf("must have at least one partition region")
	}

	return nil
}

// validateCosiBootloader checks the bootloader metadata.
func validateCosiBootloader(m cosiMetadata) error {
	if m.Bootloader == nil {
		return fmt.Errorf("bootloader metadata must be present")
	}
	if m.Bootloader.Type != "systemd-boot" {
		return fmt.Errorf("ACL uses systemd-boot bootloader, got %q", m.Bootloader.Type)
	}

	if m.Bootloader.SystemdBoot == nil {
		return fmt.Errorf("systemdBoot config must be present when type is systemd-boot")
	}
	if len(m.Bootloader.SystemdBoot.Entries) == 0 {
		return fmt.Errorf("systemdBoot must have at least one boot entry")
	}

	return nil
}

// validateExpectedFilesystems checks that the COSI images array exactly matches
// the given expected filesystem list — no missing, no extras.
func validateExpectedFilesystems(m cosiMetadata, expected []expectedFilesystem) error {
	// Exact count — no unexpected filesystems
	if len(expected) != len(m.Images) {
		return fmt.Errorf("expected %d filesystem images but COSI contains %d", len(expected), len(m.Images))
	}

	// Build a lookup by mount point
	fsByMount := make(map[string]*cosiFilesystem)
	for i := range m.Images {
		fs := &m.Images[i]
		if _, exists := fsByMount[fs.MountPoint]; exists {
			return fmt.Errorf("duplicate mount point: %s", fs.MountPoint)
		}
		fsByMount[fs.MountPoint] = fs
	}

	// Validate each expected filesystem is present with correct properties
	for _, exp := range expected {
		fs, found := fsByMount[exp.MountPoint]
		if !found {
			return fmt.Errorf("expected filesystem with mount point %q not found in COSI metadata", exp.MountPoint)
		}
		if fs.FsType != exp.FsType {
			return fmt.Errorf("mount point %s: expected fsType %q, got %q", exp.MountPoint, exp.FsType, fs.FsType)
		}

		// Validate image file
		if err := validateImageFile(fs.Image, fmt.Sprintf("filesystem[%s]", exp.MountPoint)); err != nil {
			return err
		}

		// Validate fsUuid is present and non-empty
		if fs.FsUUID == "" {
			return fmt.Errorf("mount point %s: fsUuid must not be empty", exp.MountPoint)
		}

		// Validate partition type GUID
		if fs.PartType == "" {
			return fmt.Errorf("mount point %s: partType must not be empty", exp.MountPoint)
		}

		// ESP must use the standard ESP partition type GUID
		if exp.MountPoint == "/boot" && !strings.EqualFold(fs.PartType, espPartTypeGUID) {
			return fmt.Errorf("mount point /boot: partType must be ESP GUID")
		}

		// Verity validation
		if exp.RequireVerity {
			if fs.Verity == nil {
				return fmt.Errorf("mount point %s: verity must be present", exp.MountPoint)
			}
			if fs.Verity.RootHash == "" {
				return fmt.Errorf("mount point %s: verity roothash must not be empty", exp.MountPoint)
			}
			if err := validateImageFile(fs.Verity.Image, fmt.Sprintf("filesystem[%s].verity", exp.MountPoint)); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateCosiCompression checks the compression metadata.
func validateCosiCompression(m cosiMetadata) error {
	if m.Compression == nil {
		return fmt.Errorf("compression metadata must be present for COSI >= 1.2")
	}
	return nil
}

// validateImageFile checks that an ImageFile has valid fields.
func validateImageFile(img cosiImageFile, context string) error {
	if !strings.HasPrefix(img.Path, "images/") {
		return fmt.Errorf("%s: image path must start with 'images/', got %q", context, img.Path)
	}
	if img.CompressedSize <= 0 {
		return fmt.Errorf("%s: compressedSize must be > 0", context)
	}
	if img.UncompressedSize <= 0 {
		return fmt.Errorf("%s: uncompressedSize must be > 0", context)
	}
	if img.SHA384 == "" {
		return fmt.Errorf("%s: sha384 must not be empty", context)
	}
	decoded, err := decodeHex(img.SHA384)
	if err != nil {
		return fmt.Errorf("%s: %w", context, err)
	}
	if len(decoded) != 48 {
		return fmt.Errorf("%s: sha384 must be 48 bytes (384 bits)", context)
	}
	return nil
}

// collectMetadataImagePaths returns a map of all image paths referenced in
// metadata (from both images[] and disk.gptRegions[]) to their expected SHA-384.
func collectMetadataImagePaths(m cosiMetadata) (map[string]string, error) {
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
				if !strings.EqualFold(existing, region.Image.SHA384) {
					return nil, fmt.Errorf("GPT region image %s has different SHA-384 than filesystem image", region.Image.Path)
				}
			}
			paths[region.Image.Path] = region.Image.SHA384
		}
	}

	return paths, nil
}

// decodeHex decodes a hex string, wrapping any error with context.
func decodeHex(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decoding hex string %q: %w", s, err)
	}
	return b, nil
}

// sanitizeURL removes query parameters (which may contain SAS tokens) from a
// URL for safe logging.
func sanitizeURL(u string) string {
	if idx := strings.IndexByte(u, '?'); idx >= 0 {
		return u[:idx] + "?<redacted>"
	}
	return u
}

// cosiContentScenario builds a LocalValidator scenario that downloads and
// validates an ACL COSI artifact's contents (tar structure, metadata.json,
// and image SHA-384 hashes) without provisioning a VM. If the COSI
// publishing artifact isn't available, the scenario is still registered but
// with a SkipReason, matching cosiUpdateAMD64Scenario's convention.
func cosiContentScenario(name, description, publishingArtifact, variantLabel string) *Scenario {
	info, ok := loadCOSIPublishingInfo(publishingArtifact)
	if !ok {
		return &Scenario{
			Name:        name,
			Description: description,
			SkipReason:  fmt.Sprintf("COSI artifact not available for %s, skipping COSI content validation", variantLabel),
		}
	}

	return &Scenario{
		Name:        name,
		Description: description,
		Config: Config{
			LocalValidator: func(ctx context.Context) error {
				return validateACLCOSIContent(ctx, info.CosiURL)
			},
		},
	}
}

var _ = Register(cosiContentScenario("ACL_COSIContent_AMD64", "Validates the contents of the AMD64 ACL COSI artifact without provisioning a VM", cosiAMD64PublishingArtifact, "acl-tl-gen2"))

var _ = Register(cosiContentScenario("ACL_COSIContent_ARM64", "Validates the contents of the ARM64 ACL COSI artifact without provisioning a VM", cosiARM64PublishingArtifact, "acl-arm64-tl-gen2"))

var _ = Register(cosiContentScenario("ACL_COSIContent_AMD64_FIPS", "Validates the contents of the AMD64 FIPS ACL COSI artifact without provisioning a VM", cosiAMD64FIPSPublishingArtifact, "acl-fips-tl-gen2"))

var _ = Register(cosiContentScenario("ACL_COSIContent_ARM64_FIPS", "Validates the contents of the ARM64 FIPS ACL COSI artifact without provisioning a VM", cosiARM64FIPSPublishingArtifact, "acl-arm64-fips-tl-gen2"))
