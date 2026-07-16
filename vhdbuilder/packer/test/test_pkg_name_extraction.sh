#!/bin/bash
# Test that getPackageJSON + renovateTag extraction resolves correct package names
# for containerd and kubernetes-cri-tools across all supported OS variants.
#
# This is a local-only test script (not run in CI). ShellSpec integration was
# deferred because cse_helpers.sh has system dependencies that are hard to
# satisfy in the shellspec Docker environment.
#
# Usage: bash vhdbuilder/packer/test/test_pkg_name_extraction.sh

set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Source cse_helpers.sh for getPackageJSON
# Set defaults for variables that cse_helpers.sh expects
OS_VARIANT="${OS_VARIANT:-DEFAULT}"
if ! source "$REPO_ROOT/parts/linux/cloud-init/artifacts/cse_helpers.sh" 2>/dev/null || ! type getPackageJSON >/dev/null 2>&1; then
  echo "Failed to source cse_helpers.sh: getPackageJSON is not defined" >&2
  exit 1
fi

COMPONENTS_FILEPATH="$REPO_ROOT/parts/common/components.json"

# Helper: extract package name from renovateTag via getPackageJSON
# Uses sed instead of grep -P for macOS compatibility
extractPkgName() {
  local packageJson="$1"
  local os="$2"
  local osVersion="$3"
  local osVariant="${4:-DEFAULT}"
  getPackageJSON "$packageJson" "$os" "$osVersion" "$osVariant" \
    | jq -r '.versionsV2[0].renovateTag // empty' \
    | sed -n 's/.*name=\([^,]*\).*/\1/p' \
    | xargs
}

PASS=0
FAIL=0

assertPkgName() {
  local description="$1"
  local expected="$2"
  local actual="$3"

  if [ "$actual" = "$expected" ]; then
    echo "✓ $description: got '$actual'"
    PASS=$((PASS + 1))
  else
    echo "✗ $description: expected '$expected', got '$actual'"
    FAIL=$((FAIL + 1))
  fi
}

# Load containerd package JSON
containerdPkg=$(jq -c '.Packages[] | select(.name == "containerd")' "$COMPONENTS_FILEPATH")
criToolsPkg=$(jq -c '.Packages[] | select(.name == "kubernetes-cri-tools")' "$COMPONENTS_FILEPATH")

echo "=== containerd package name extraction ==="

# Ubuntu variants
result=$(extractPkgName "$containerdPkg" "UBUNTU" "24.04")
assertPkgName "containerd on Ubuntu 24.04" "moby-containerd" "$result"

result=$(extractPkgName "$containerdPkg" "UBUNTU" "22.04")
assertPkgName "containerd on Ubuntu 22.04" "moby-containerd" "$result"

result=$(extractPkgName "$containerdPkg" "UBUNTU" "20.04")
assertPkgName "containerd on Ubuntu 20.04" "moby-containerd" "$result"

# Azure Linux
result=$(extractPkgName "$containerdPkg" "AZURELINUX" "3.0")
assertPkgName "containerd on Azure Linux 3.0" "containerd2" "$result"

# Mariner
result=$(extractPkgName "$containerdPkg" "MARINER" "2.0" "DEFAULT")
assertPkgName "containerd on Mariner 2.0" "moby-containerd" "$result"

echo ""
echo "=== kubernetes-cri-tools package name extraction ==="

result=$(extractPkgName "$criToolsPkg" "UBUNTU" "24.04")
assertPkgName "cri-tools on Ubuntu 24.04" "kubernetes-cri-tools" "$result"

result=$(extractPkgName "$criToolsPkg" "UBUNTU" "22.04")
assertPkgName "cri-tools on Ubuntu 22.04" "kubernetes-cri-tools" "$result"

result=$(extractPkgName "$criToolsPkg" "AZURELINUX" "3.0")
assertPkgName "cri-tools on Azure Linux 3.0" "kubernetes-cri-tools" "$result"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
