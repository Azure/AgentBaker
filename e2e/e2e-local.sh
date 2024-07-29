#!/bin/bash

set -euxo pipefail

: "${TIMEOUT:=90m}"
: "${PARALLEL:=100}"

go version
go test -parallel $PARALLEL -timeout $TIMEOUT ./...
