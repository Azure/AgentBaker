#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(dirname "${BASH_SOURCE[0]}")/../.."
cd "$ROOT_DIR"

wd=$(mktemp -d)
cd $wd
git clone https://github.com/alexeldeib/cue
cd cue
go build -o $ROOT_DIR/hack/tools/bin/cue cmd/cue/main.go
popd
rm -rf $wd
