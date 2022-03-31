#!/usr/bin/env bash
set -euo pipefail

dpkg-query -W -f='["${Package}"]\n' | jq --slurp 'add' > /opt/azure/dpkg-bom.json
# ignore linux package with kernel versions 
# rg -v '"linux-[a-z]+-5\.[0-9]+\.[0-9]+-[0-9]+-azure'
