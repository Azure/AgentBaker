#!/usr/bin/env bash

set -euo pipefail

GOLANG_VERSION="go1.18.9"
echo "Downloading ${GOLANG_VERSION}"
curl -O "https://dl.google.com/go/${GOLANG_VERSION}.linux-amd64.tar.gz"

echo "unpacking go"
sudo mkdir -p /usr/local/go
sudo chown -R "$(whoami):$(whoami)" /usr/local/go 
sudo tar -xvf "${GOLANG_VERSION}.linux-amd64.tar.gz" -C /usr/local
rm "${GOLANG_VERSION}.linux-amd64.tar.gz"

export PATH="/usr/local/go/bin:$PATH"
GOPATH="/home/$(whoami)/go"
export GOPATH
