#!/bin/bash
set -ex

./build.sh
scp reset.ps1 windows:/
ssh windows "pwsh -C /reset.ps1"
ssh windows "pwsh -C \"mkdir /AzureData\""
scp nbc.json prepare.ps1 dist/node-bootstrapper-windows-amd64.exe windows:/AzureData/
ssh windows "/AzureData/node-bootstrapper-windows-amd64.exe provision --provision-config /AzureData/nbc.json"
