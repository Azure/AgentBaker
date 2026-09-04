#!/bin/bash

set -euxo pipefail

: "${TIMEOUT:=90m}"
: "${PARALLEL:=100}"
export TIMEOUT PARALLEL

if [ -n "${VHD_BUILD_ID:-}" ]; then
  echo "VHD_BUILD_ID is specified (${VHD_BUILD_ID}). Running tests using VHDs from that build"
  export SIG_VERSION_TAG_NAME=buildId
  export SIG_VERSION_TAG_VALUE=$VHD_BUILD_ID
fi

go version
go run ./cmd/e2e run "$@"
