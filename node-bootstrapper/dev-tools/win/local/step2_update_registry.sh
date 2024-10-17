#!/bin/bash
set -ex

HOST=windows
SCRIPTDIR=$(dirname "$0")

scp "${SCRIPTDIR}/setup.ps1" ${HOST}:/
ssh ${HOST} "pwsh -C \". /setup.ps1; Update-Registry ; Write-Log 'Shutting Down' ; shutdown /r \" "
