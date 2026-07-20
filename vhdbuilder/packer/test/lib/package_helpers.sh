#!/bin/bash
# Shared helper functions for VHD content testing.
# Sourced by both linux-vhd-content-test.sh (runtime on VM) and ShellSpec unit tests.

# shellcheck disable=SC2016

err() {
  echo "$1:Error: $2" >>/dev/stderr
}

# assertPackageVersion verifies that the installed deb/rpm package version matches
# the expected full version string from components.json (including hotfix suffix).
# This catches drift between what the package manager installs and what components.json
# specifies at VHD build time rather than in e2e.
assertPackageVersion() {
  local test="$1"
  local packageName="$2"
  local expectedVersion="$3"

  local installedVersion=""
  if command -v dpkg-query >/dev/null 2>&1 && dpkg-query -W -f='${Status}' "$packageName" 2>/dev/null | grep -q "install ok installed"; then
    # dpkg versions may include an epoch prefix (e.g. "1:..."); strip it for comparison with components.json.
    installedVersion=$(dpkg-query -W -f='${Version}' "$packageName" 2>/dev/null | sed 's/^[0-9]*://')
  elif command -v rpm >/dev/null 2>&1 && rpm -q "$packageName" >/dev/null 2>&1; then
    installedVersion=$(rpm -q --queryformat '%{VERSION}-%{RELEASE}' "$packageName" 2>/dev/null)
  else
    err "$test" "$packageName is not installed"
    return 1
  fi

  echo "$test: checking if installed $packageName version '$installedVersion' matches expected '$expectedVersion'"
  if [ "$installedVersion" != "$expectedVersion" ]; then
    err "$test" "installed $packageName version '$installedVersion' does not match expected '$expectedVersion' from components.json"
    return 1
  fi
  return 0
}
