# Define all exit codes in Windows CSE

function Set-ExitCode
{
    Param(
        [Parameter(Mandatory=$true)][int]
        $ExitCode,
        [Parameter(Mandatory=$true)][string]
        $ErrorMessage
    )
    Write-Log "Set ExitCode to $ExitCode and exit. Error: $ErrorMessage"
    $global:ExitCode=$ExitCode
    $global:ErrorMessage=$ErrorMessage
    exit $ExitCode
}

function Postpone-RestartComputer
{
    Write-Log "Creating an one-time task to restart the VM"
    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument " -Command `"Restart-Computer -Force`""
    $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
    # trigger this task once
    $trigger = New-JobTrigger -At  (Get-Date).AddSeconds(15).DateTime -Once
    $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "Restart computer after provisioning the VM"
    Register-ScheduledTask -TaskName "restart-computer" -InputObject $definition
    Write-Log "Created an one-time task to restart the VM"
}

function Create-Directory
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $Folder,
        [Parameter(Mandatory=$false)][string]
        $FolderUsage = "general purpose"
    )
    
    if (-Not (Test-Path $Folder)) {
        Write-Log "Create folder $Folder for $FolderUsage"
        New-Item -ItemType Directory -Path $Folder > $null
    } else {
        Write-Log "Folder $Folder for $FolderUsage exists"
    }
}

$global:WINDOWS_CSE_ERROR_UNKNOWN=1 # For unexpected error caught by the catch block in kuberneteswindowssetup.ps1
$global:WINDOWS_CSE_ERROR_DOWNLOAD_FILE_WITH_RETRY=2
$global:WINDOWS_CSE_ERROR_INVOKE_EXECUTABLE=3
$global:WINDOWS_CSE_ERROR_FILE_NOT_EXIST=4
$global:WINDOWS_CSE_ERROR_CHECK_API_SERVER_CONNECTIVITY=5
$global:WINDOWS_CSE_ERROR_PAUSE_IMAGE_NOT_EXIST=6
$global:WINDOWS_CSE_ERROR_GET_SUBNET_PREFIX=7
$global:WINDOWS_CSE_ERROR_GENERATE_TOKEN_FOR_ARM=8
$global:WINDOWS_CSE_ERROR_NETWORK_INTERFACES_NOT_EXIST=9
$global:WINDOWS_CSE_ERROR_NETWORK_ADAPTER_NOT_EXIST=10
$global:WINDOWS_CSE_ERROR_MANAGEMENT_IP_NOT_EXIST=11
$global:WINDOWS_CSE_ERROR_CALICO_SERVICE_ACCOUNT_NOT_EXIST=12
$global:WINDOWS_CSE_ERROR_CONTAINERD_NOT_INSTALLED=13
$global:WINDOWS_CSE_ERROR_CONTAINERD_NOT_RUNNING=14
$global:WINDOWS_CSE_ERROR_OPENSSH_NOT_INSTALLED=15
$global:WINDOWS_CSE_ERROR_OPENSSH_FIREWALL_NOT_CONFIGURED=16
$global:WINDOWS_CSE_ERROR_INVALID_PARAMETER_IN_AZURE_CONFIG=17
$global:WINDOWS_CSE_ERROR_NO_DOCKER_TO_BUILD_PAUSE_CONTAINER=18
$global:WINDOWS_CSE_ERROR_GET_CA_CERTIFICATES=19
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CA_CERTIFICATES=20
$global:WINDOWS_CSE_ERROR_EMPTY_CA_CERTIFICATES=21