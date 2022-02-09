#!/usr/bin/env bash
set -euxo pipefail

ROOT_DIR="$(dirname "${BASH_SOURCE[0]}")/../../.."

wd=$(mktemp -d)
pushd $wd
git clone https://github.com/alexeldeib/cue
cd cue
cue_bin="$(pwd)/bin/cue"
go build -o "$cue_bin" cmd/cue/main.go
popd
mv "$cue_bin" "${ROOT_DIR}/hack/tools/bin/cue"
rm -rf $wd
