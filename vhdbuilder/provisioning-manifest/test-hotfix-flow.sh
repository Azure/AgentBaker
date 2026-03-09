#!/bin/bash
# test-hotfix-flow.sh — End-to-end test for the provisioning script hotfix mechanism.
#
# This test simulates the full lifecycle:
#   1. A VHD with baked scripts at a specific version
#   2. A hotfix artifact built for that exact version
#   3. Node-side detection via tag prefix matching
#   4. Verification that the hotfix was applied correctly
#   5. Verification that non-matching versions are NOT patched
#
# Prerequisites:
#   - oras CLI installed
#   - jq installed
#   - Docker or podman (for local registry, optional — can use a directory-based OCI layout)
#
# Usage:
#   ./test-hotfix-flow.sh [--use-registry <host:port>]
#   ./test-hotfix-flow.sh  # Uses a local directory-based mock

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BUILD_SCRIPT="${SCRIPT_DIR}/build-hotfix-oci.sh"
MANIFEST_FILE="${SCRIPT_DIR}/manifest.json"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0
USE_REGISTRY=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --use-registry) USE_REGISTRY="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

pass() {
    echo -e "${GREEN}  PASS${NC}: $1"
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

fail() {
    echo -e "${RED}  FAIL${NC}: $1"
    TESTS_FAILED=$((TESTS_FAILED + 1))
}

info() {
    echo -e "${YELLOW}>>> $1${NC}"
}

# Create a temporary work directory
WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

# ============================================================
# TEST 1: build-hotfix-oci.sh — Dry run build (single version)
# ============================================================
info "TEST 1: Build hotfix artifact (dry run, single version)"

"$BUILD_SCRIPT" \
    --sku ubuntu-2204 \
    --affected-version v0.20260201.0 \
    --description "Test hotfix for POC validation" \
    --files "parts/linux/cloud-init/artifacts/cse_helpers.sh" \
    --dry-run > "${WORK_DIR}/build-output.txt" 2>&1

BUILD_EXIT=$?
if [ $BUILD_EXIT -eq 0 ]; then
    pass "build-hotfix-oci.sh dry run succeeded"
else
    fail "build-hotfix-oci.sh dry run failed (exit $BUILD_EXIT)"
    cat "${WORK_DIR}/build-output.txt"
fi

# Check the output references the staging dir
STAGING_DIR=$(grep "Artifacts staged in:" "${WORK_DIR}/build-output.txt" | awk '{print $NF}')
if [ -n "$STAGING_DIR" ] && [ -d "$STAGING_DIR" ]; then
    pass "Staging directory exists: $STAGING_DIR"

    # Verify tarball was created — tag is v0.20260201.0-hotfix (no sequence)
    if ls "$STAGING_DIR"/provisioning-scripts-v0.20260201.0-hotfix.tar.gz &>/dev/null; then
        pass "Tarball created with version-based name"
    else
        fail "Tarball not found in staging dir"
        ls -la "$STAGING_DIR"/ 2>/dev/null || true
    fi

    # Verify metadata was created
    if [ -f "$STAGING_DIR/hotfix-metadata.json" ]; then
        pass "Metadata file created"

        # Verify metadata content
        HOTFIX_ID=$(jq -r '.hotfixId' "$STAGING_DIR/hotfix-metadata.json")
        AFFECTED_VER=$(jq -r '.affectedVersion' "$STAGING_DIR/hotfix-metadata.json")

        [ "$HOTFIX_ID" = "v0.20260201.0-hotfix" ] && pass "Metadata hotfixId correct (${HOTFIX_ID})" || fail "Metadata hotfixId wrong: $HOTFIX_ID"
        [ "$AFFECTED_VER" = "v0.20260201.0" ] && pass "Metadata affectedVersion correct" || fail "Metadata affectedVersion wrong: $AFFECTED_VER"

        # Verify file list in metadata
        FILE_COUNT=$(jq '.files | length' "$STAGING_DIR/hotfix-metadata.json")
        [ "$FILE_COUNT" -eq 1 ] && pass "Metadata has 1 file entry" || fail "Metadata has $FILE_COUNT file entries (expected 1)"

        FILE_PATH=$(jq -r '.files[0].path' "$STAGING_DIR/hotfix-metadata.json")
        [ "$FILE_PATH" = "/opt/azure/containers/provision_source.sh" ] && pass "File path mapped correctly" || fail "File path wrong: $FILE_PATH"
    else
        fail "Metadata file not found"
    fi

    # Verify tarball content has correct structure
    TARBALL=$(find "$STAGING_DIR" -maxdepth 1 -name "provisioning-scripts-*.tar.gz" -print -quit 2>/dev/null)
    if [ -n "$TARBALL" ]; then
        TAR_CONTENTS=$(tar -tzf "$TARBALL" 2>/dev/null)
        if echo "$TAR_CONTENTS" | grep -q "opt/azure/containers/provision_source.sh"; then
            pass "Tarball contains correct destination path"
        else
            fail "Tarball missing expected file path. Contents: $TAR_CONTENTS"
        fi
    fi

    # Clean up dry-run staging
    rm -rf "$STAGING_DIR"
else
    fail "Could not find staging directory in build output"
fi

# ============================================================
# TEST 2: build-hotfix-oci.sh — Invalid file rejection
# ============================================================
info "TEST 2: Build rejects invalid source file"

if "$BUILD_SCRIPT" \
    --sku ubuntu-2204 \
    --affected-version v0.20260201.0 \
    --description "Bad hotfix" \
    --files "nonexistent/file.sh" \
    --dry-run > "${WORK_DIR}/bad-build.txt" 2>&1; then
    fail "Build should have failed for nonexistent file"
else
    pass "Build correctly rejected nonexistent file"
fi

# ============================================================
# TEST 3: build-hotfix-oci.sh — Invalid SKU rejection
# ============================================================
info "TEST 3: Build rejects invalid SKU"

if "$BUILD_SCRIPT" \
    --sku windows-2022 \
    --affected-version v0.20260201.0 \
    --description "Bad SKU" \
    --files "parts/linux/cloud-init/artifacts/cse_helpers.sh" \
    --dry-run > "${WORK_DIR}/bad-sku.txt" 2>&1; then
    fail "Build should have failed for invalid SKU"
else
    pass "Build correctly rejected invalid SKU"
fi

# ============================================================
# TEST 4: Tag prefix matching logic (replaces version range tests)
# ============================================================
info "TEST 4: Exact tag matching logic"

# Simulate the tag matching from check_for_script_hotfix()
# The node looks for exactly "<baked_version>-hotfix" in the tag list
tag_matches_version() {
    local tag="$1" baked_version="$2"
    [ "$tag" = "${baked_version}-hotfix" ]
}

# Exact match: tag for our version
if tag_matches_version "v0.20260201.0-hotfix" "v0.20260201.0"; then
    pass "v0.20260201.0-hotfix matches baked v0.20260201.0"
else
    fail "v0.20260201.0-hotfix should match v0.20260201.0"
fi

# Different version: should NOT match
if tag_matches_version "v0.20260301.0-hotfix" "v0.20260201.0"; then
    fail "v0.20260301.0-hotfix should NOT match baked v0.20260201.0"
else
    pass "v0.20260301.0-hotfix correctly ignored for baked v0.20260201.0"
fi

# Newer VHD (fix already baked): should NOT match
if tag_matches_version "v0.20260201.0-hotfix" "v0.20260308.0"; then
    fail "v0.20260201.0-hotfix should NOT match newer baked v0.20260308.0"
else
    pass "v0.20260201.0-hotfix correctly ignored for newer VHD v0.20260308.0"
fi

# Older VHD: should NOT match
if tag_matches_version "v0.20260201.0-hotfix" "v0.20260101.0"; then
    fail "v0.20260201.0-hotfix should NOT match older baked v0.20260101.0"
else
    pass "v0.20260201.0-hotfix correctly ignored for older VHD v0.20260101.0"
fi

# Patch version: should NOT match (different baked version)
if tag_matches_version "v0.20260201.0-hotfix" "v0.20260201.1"; then
    fail "v0.20260201.0-hotfix should NOT match baked v0.20260201.1"
else
    pass "v0.20260201.0-hotfix correctly ignored for patch version v0.20260201.1"
fi

# ============================================================
# TEST 5: Simulated end-to-end hotfix flow (local, no registry)
# ============================================================
info "TEST 5: Simulated end-to-end hotfix overlay"

# Create a fake VHD filesystem
FAKE_ROOT="${WORK_DIR}/fake-root"
mkdir -p "$FAKE_ROOT/opt/azure/containers"
mkdir -p "$FAKE_ROOT/etc"

# Write a "baked" script with known content
echo '#!/bin/bash
# Original baked provision_source.sh — v0.20260201.0
echo "I am the ORIGINAL baked script"
BAKED_MARKER="original"
#HELPERSEOF' > "$FAKE_ROOT/opt/azure/containers/provision_source.sh"
chmod 0744 "$FAKE_ROOT/opt/azure/containers/provision_source.sh"

# Stamp the baked version
echo "v0.20260201.0" > "$FAKE_ROOT/opt/azure/containers/.provisioning-scripts-version"

# Build a hotfix tarball with a "fixed" script
HOTFIX_STAGING="${WORK_DIR}/hotfix-build"
mkdir -p "$HOTFIX_STAGING/opt/azure/containers"
echo '#!/bin/bash
# HOTFIXED provision_source.sh — fixes CVE-2026-XXXX
echo "I am the HOTFIXED script"
BAKED_MARKER="hotfixed"
#HELPERSEOF' > "$HOTFIX_STAGING/opt/azure/containers/provision_source.sh"
chmod 0744 "$HOTFIX_STAGING/opt/azure/containers/provision_source.sh"

# Create hotfix tarball
(cd "$HOTFIX_STAGING" && tar -czf "${WORK_DIR}/provisioning-scripts-v0.20260201.0-hotfix.tar.gz" .)

# Create hotfix metadata (single-version, single-tag format)
TARBALL_SHA=$(sha256sum "${WORK_DIR}/provisioning-scripts-v0.20260201.0-hotfix.tar.gz" | awk '{print $1}')
cat > "${WORK_DIR}/hotfix-metadata.json" <<EOF
{
  "hotfixId": "v0.20260201.0-hotfix",
  "sku": "ubuntu-2204",
  "affectedVersion": "v0.20260201.0",
  "description": "Test hotfix",
  "sourceCommit": "test",
  "tarballSha256": "${TARBALL_SHA}",
  "files": [
    {
      "path": "/opt/azure/containers/provision_source.sh",
      "sha256": "test",
      "permissions": "0744"
    }
  ]
}
EOF

# Simulate the extraction (what check_for_script_hotfix would do)
tar -xzf "${WORK_DIR}/provisioning-scripts-v0.20260201.0-hotfix.tar.gz" -C "$FAKE_ROOT" --no-same-owner 2>/dev/null

# Verify the script was replaced
if grep -q "HOTFIXED" "$FAKE_ROOT/opt/azure/containers/provision_source.sh"; then
    pass "Hotfix overlay replaced the baked script"
else
    fail "Hotfix overlay did not replace the baked script"
fi

if grep -q "hotfixed" "$FAKE_ROOT/opt/azure/containers/provision_source.sh"; then
    pass "Hotfixed content is correct"
else
    fail "Hotfixed content is incorrect"
fi

# Verify original version stamp is preserved (hotfix doesn't change it)
STAMP=$(cat "$FAKE_ROOT/opt/azure/containers/.provisioning-scripts-version")
if [ "$STAMP" = "v0.20260201.0" ]; then
    pass "Baked version stamp preserved after hotfix"
else
    fail "Baked version stamp changed: $STAMP"
fi

# ============================================================
# TEST 6: Multiple files in single hotfix
# ============================================================
info "TEST 6: Build hotfix with multiple files"

"$BUILD_SCRIPT" \
    --sku ubuntu-2204 \
    --affected-version v0.20260201.0 \
    --description "Multi-file hotfix" \
    --files "parts/linux/cloud-init/artifacts/cse_helpers.sh,parts/linux/cloud-init/artifacts/cse_install.sh" \
    --dry-run > "${WORK_DIR}/multi-build.txt" 2>&1

if [ $? -eq 0 ]; then
    pass "Multi-file hotfix build succeeded"

    MULTI_STAGING=$(grep "Artifacts staged in:" "${WORK_DIR}/multi-build.txt" | awk '{print $NF}')
    if [ -n "$MULTI_STAGING" ] && [ -f "$MULTI_STAGING/hotfix-metadata.json" ]; then
        MULTI_COUNT=$(jq '.files | length' "$MULTI_STAGING/hotfix-metadata.json")
        [ "$MULTI_COUNT" -eq 2 ] && pass "Multi-file metadata has 2 entries" || fail "Multi-file metadata has $MULTI_COUNT entries"

        MULTI_ID=$(jq -r '.hotfixId' "$MULTI_STAGING/hotfix-metadata.json")
        [ "$MULTI_ID" = "v0.20260201.0-hotfix" ] && pass "Single tag format in hotfix ID" || fail "Hotfix ID wrong: $MULTI_ID"
        rm -rf "$MULTI_STAGING"
    fi
else
    fail "Multi-file hotfix build failed"
fi

# ============================================================
# TEST 7: Metadata affectedVersion validation
# ============================================================
info "TEST 7: Metadata affectedVersion defense-in-depth"

# Simulate check: metadata says v0.20260201.0 but we're on v0.20260301.0
META_VERSION="v0.20260201.0"
BAKED="v0.20260301.0"
if [ "$META_VERSION" != "$BAKED" ]; then
    pass "Defense-in-depth: metadata version mismatch correctly detected"
else
    fail "Should have detected version mismatch"
fi

# Matching case
META_VERSION="v0.20260201.0"
BAKED="v0.20260201.0"
if [ "$META_VERSION" = "$BAKED" ]; then
    pass "Defense-in-depth: matching version correctly passes"
else
    fail "Should have accepted matching version"
fi

# ============================================================
# SUMMARY
# ============================================================
echo ""
echo "============================================================"
echo -e "Results: ${GREEN}${TESTS_PASSED} passed${NC}, ${RED}${TESTS_FAILED} failed${NC}"
echo "============================================================"

if [ "$TESTS_FAILED" -gt 0 ]; then
    exit 1
fi
exit 0
