<#
    .SYNOPSIS
        Uploads a log bundle to the host for retrieval via GuestVMLogs.

    .DESCRIPTION
        Uploads a log bundle to the host for retrieval via GuestVMLogs.

        Takes a parameter of a ZIP file name to upload, which is sent to the HostAgent
        via the /vmAgentLog endpoint.
#>
[CmdletBinding()]
param(
    [string]
    $Path
)

if (!(Test-Path $Path)) {
    return
}

$GoalStateArgs = @{
    "Method"="Get";
    "Uri"="http://168.63.129.16/machine/?comp=goalstate";
    "Headers"=@{"x-ms-version"="2012-11-30"}
}
$GoalState = $(Invoke-RestMethod @GoalStateArgs).GoalState

$UploadArgs = @{
    "Method"="Put";
    "Uri"="http://168.63.129.16:32526/vmAgentLog";
    "InFile"=$Path;
    "Headers"=@{
        "x-ms-version"="2015-09-01";
        "x-ms-client-correlationid"="";
        "x-ms-client-name"="AKSCSEPlugin";
        "x-ms-client-version"="0.1.0";
        "x-ms-containerid"=$GoalState.Container.ContainerId;
        "x-ms-vmagentlog-deploymentid"=($GoalState.Container.RoleInstanceList.RoleInstance.Configuration.ConfigName -split "\.")[0]
    }
}
Invoke-RestMethod @UploadArgs