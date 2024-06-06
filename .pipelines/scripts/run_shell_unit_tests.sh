#!/bin/bash
set -uo pipefail

sudo sudo apt-get install -y bats

filesToCheck=$(find bash_tests -type f -name "*.test.bats.sh" )

echo Running bats...

mkdir -p test_results
bats -F junit ${filesToCheck} > test_results/bats_shell.xml