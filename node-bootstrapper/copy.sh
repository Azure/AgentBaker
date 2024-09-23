#!/bin/bash
set -ex

./build.sh && scp nbc.json prepare.ps1 dist/node-bootstrapper-windows-amd64.exe windows:/AzureData/
