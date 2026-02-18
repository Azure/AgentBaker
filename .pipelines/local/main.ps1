param (
    [switch]$pushContainer,
    [switch]$pushHost,
    [string]$buildNumber
)

. $PSScriptRoot/setContext.ps1

if ($?) {
. $PSScriptRoot/getContainerBaseAndOpenssh.ps1
. $PSScriptRoot/getContainerLayersFromBuilds.ps1
exit 0
}

else {
    Write-Host "Failed to set context"
}

