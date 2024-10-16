#!/bin/bash
set -ex

HOST=windows

scp running_on_vanilla_windows_vm/setup.ps1 ${HOST}:/
ssh ${HOST} "pwsh -C \". /setup.ps1; Update-Registry ; Write-Log 'Shutting Down' ; shutdown /r \" "
