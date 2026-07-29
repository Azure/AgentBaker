#!/bin/bash

set -euxo pipefail

: "${TIMEOUT:=90m}"
: "${PARALLEL:=100}"

if [ -n "${VHD_BUILD_ID:-}" ]; then
  echo "VHD_BUILD_ID is specified (${VHD_BUILD_ID}). Running tests using VHDs from that build"
  export SIG_VERSION_TAG_NAME=buildId
  export SIG_VERSION_TAG_VALUE=$VHD_BUILD_ID
fi

go version
# Note, if you run "go test ./..." you won't see the output of the tests until they finish.
# -count 1 disables caching of test results
# default go test timeout is 10 minutes, it's not enough
go test -parallel $PARALLEL -timeout $TIMEOUT -v -count 1
