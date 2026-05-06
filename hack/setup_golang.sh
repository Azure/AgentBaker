#!/bin/bash
set -euxo pipefail

# This script installs Microsoft's Go distribution via apt-get on Ubuntu hosts.
# On hosts without apt-get (e.g. Azure Linux build agents), assume Go is already
# provided by the build environment and skip the Ubuntu-specific setup.
if ! command -v apt-get >/dev/null 2>&1; then
    echo "apt-get not found; skipping Ubuntu-specific Go setup."
    echo "Assuming Go is provided by the build environment:"
    go version
    exit 0
fi

purge_go() {
    sudo apt-get purge golang*
    sudo apt-get update
    sudo rm -rf /usr/local/go
}

setup_pmc() {
    # see: https://github.com/microsoft/go/blob/microsoft/main/README.md#ubuntu
    UBUNTU_RELEASE=$(sudo lsb_release -r -s 2>/dev/null || echo "")
    curl -sSL -O https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/packages-microsoft-prod.deb
    sudo dpkg -i packages-microsoft-prod.deb
    sudo apt-get update
}

# purge any existing go installation
purge_go

# setup access to packages.microsoft.com for the particular Ubuntu release
setup_pmc

# install make
sudo apt-get -y install make

# install msft-golang
sudo apt-get -y install msft-golang

# make sure go is accessible from the command line
go version
