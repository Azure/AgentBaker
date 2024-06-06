#!/bin/bash
set -uo pipefail

sudo npm install -g bats

filesToCheck=$(find bash_tests -type f -name "*.test.sh" )

echo $filesToCheck

bats "${filesToCheck}"