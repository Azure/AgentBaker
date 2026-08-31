#!/bin/bash
set -euxo pipefail

go_version="1.26.7"

# This script installs Microsoft's Go distribution via apt-get (Ubuntu).
# On hosts without apt-get (e.g. Azure Linux build agents), the build environment
# is expected to provide the required Go version through its own package manager,
# so verify it is on PATH and matches before exiting.
if ! command -v apt-get >/dev/null 2>&1; then
    echo "apt-get not found; skipping Ubuntu-specific Go setup."
    if ! command -v go >/dev/null 2>&1; then
        echo "ERROR: 'go' is not on PATH; the build environment must install Go before invoking setup_golang.sh." >&2
        exit 1
    fi
    actual_go_version=$(go env GOVERSION)
    # shellcheck disable=SC3010
    if [[ "${actual_go_version}" != "go${go_version}" ]]; then
        echo "ERROR: expected Go ${go_version}, found ${actual_go_version}; the build environment must provide the required version." >&2
        exit 1
    fi
    echo "Using Go provided by the build environment:"
    go version
    exit 0
fi

purge_go() {
    sudo apt-get purge golang*
    sudo apt-get update
    sudo rm -rf /usr/local/go
}

setup_pmc() {
    local ubuntu_release="$1"
    # see: https://github.com/microsoft/go/blob/microsoft/main/README.md#ubuntu
    curl -sSL -O "https://packages.microsoft.com/config/ubuntu/${ubuntu_release}/packages-microsoft-prod.deb"
    sudo dpkg -i packages-microsoft-prod.deb
    sudo apt-get update
}

ubuntu_release=$(sudo lsb_release -r -s)

# purge any existing go installation
purge_go

# setup access to packages.microsoft.com for the particular Ubuntu release
setup_pmc "${ubuntu_release}"

# install make
sudo apt-get -y install make

# install msft-golang
sudo apt-get -y install "msft-golang=${go_version}-ubuntu${ubuntu_release}u1"

# make sure go is accessible from the command line
go version
