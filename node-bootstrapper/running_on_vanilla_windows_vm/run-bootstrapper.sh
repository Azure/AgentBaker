#!/bin/bash
set -ex

HOST=windows

./build.sh
#scp test_scripts/reset.ps1 ${HOST}:/
#ssh ${HOST} "pwsh -C /reset.ps1"
ssh ${HOST} "pwsh -C \"mkdir -Force /AzureData\""
scp running_on_vanilla_windows_vm/sample_node_bootstrapping_config.json running_on_vanilla_windows_vm/setup.ps1 dist/node-bootstrapper-windows-amd64.exe ${HOST}:/AzureData/
ssh ${HOST} "/AzureData/node-bootstrapper-windows-amd64.exe provision --provision-config /AzureData/sample_node_bootstrapping_config.json"
