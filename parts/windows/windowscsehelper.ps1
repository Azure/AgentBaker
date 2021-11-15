# This script is used to define basic util functions
# It is better to define functions in the scripts under staging/cse/windows.

# Define all exit codes in Windows CSE
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
$global:WINDOWS_CSE_ERROR_ENABLE_SECURE_TLS=22
$global:WINDOWS_CSE_ERROR_GMSA_EXPAND_ARCHIVE=23
$global:WINDOWS_CSE_ERROR_GMSA_ENABLE_POWERSHELL_PRIVILEGE=24
$global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_PERMISSION=25
$global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_VALUES=26
$global:WINDOWS_CSE_ERROR_GMSA_IMPORT_CCGEVENTS=27
$global:WINDOWS_CSE_ERROR_GMSA_IMPORT_CCGAKVPPLUGINEVENTS=28

# This filter removes null characters (\0) which are captured in nssm.exe output when logged through powershell
filter RemoveNulls { $_ -replace '\0', '' }

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log($message) {
    $msg = $message | Timestamp
    Write-Output $msg
}

function DownloadFileOverHttp {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Url,
        [Parameter(Mandatory = $true)][string]
        $DestinationPath
    )

    # First check to see if a file with the same name is already cached on the VHD
    $fileName = [IO.Path]::GetFileName($Url)

    $search = @()
    if (Test-Path $global:CacheDir) {
        $search = [IO.Directory]::GetFiles($global:CacheDir, $fileName, [IO.SearchOption]::AllDirectories)
    }

    if ($search.Count -ne 0) {
        Write-Log "Using cached version of $fileName - Copying file from $($search[0]) to $DestinationPath"
        Copy-Item -Path $search[0] -Destination $DestinationPath -Force
    }
    else {
        $secureProtocols = @()
        $insecureProtocols = @([System.Net.SecurityProtocolType]::SystemDefault, [System.Net.SecurityProtocolType]::Ssl3)

        foreach ($protocol in [System.Enum]::GetValues([System.Net.SecurityProtocolType])) {
            if ($insecureProtocols -notcontains $protocol) {
                $secureProtocols += $protocol
            }
        }
        [System.Net.ServicePointManager]::SecurityProtocol = $secureProtocols

        $oldProgressPreference = $ProgressPreference
        $ProgressPreference = 'SilentlyContinue'

        $downloadTimer = [System.Diagnostics.Stopwatch]::StartNew()
        try {
            $args = @{Uri=$Url; Method="Get"; OutFile=$DestinationPath}
            Retry-Command -Command "Invoke-RestMethod" -Args $args -Retries 5 -RetryDelaySeconds 10
        } catch {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_FILE_WITH_RETRY -ErrorMessage "Failed in downloading $Url. Error: $_"
        }
        $downloadTimer.Stop()

        if ($global:AppInsightsClient -ne $null) {
            $event = New-Object "Microsoft.ApplicationInsights.DataContracts.EventTelemetry"
            $event.Name = "FileDownload"
            $event.Properties["FileName"] = $fileName
            $event.Metrics["DurationMs"] = $downloadTimer.ElapsedMilliseconds
            $global:AppInsightsClient.TrackEvent($event)
        }

        $ProgressPreference = $oldProgressPreference
        Write-Log "Downloaded file $Url to $DestinationPath"
    }
}

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
        $FullPath,
        [Parameter(Mandatory=$false)][string]
        $DirectoryUsage = "general purpose"
    )
    
    if (-Not (Test-Path $FullPath)) {
        Write-Log "Create directory $FullPath for $DirectoryUsage"
        New-Item -ItemType Directory -Path $FullPath > $null
    } else {
        Write-Log "Directory $FullPath for $DirectoryUsage exists"
    }
}

# https://stackoverflow.com/a/34559554/697126
function New-TemporaryDirectory {
    $parent = [System.IO.Path]::GetTempPath()
    [string] $name = [System.Guid]::NewGuid()
    New-Item -ItemType Directory -Path (Join-Path $parent $name)
}

function Retry-Command {
    Param(
        [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]
        $Command,
        [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][hashtable]
        $Args,
        [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][int]
        $Retries,
        [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][int]
        $RetryDelaySeconds
    )

    for ($i = 0; ; ) {
        try {
            return & $Command @Args
        }
        catch {
            $i++
            if ($i -ge $Retries) {
                throw $_
            }
            Start-Sleep $RetryDelaySeconds
        }
    }
}

function Invoke-Executable {
    Param(
        [string]
        $Executable,
        [string[]]
        $ArgList,
        [int[]]
        $AllowedExitCodes = @(0),
        [int]
        $Retries = 1,
        [int]
        $RetryDelaySeconds = 1
    )

    for ($i = 0; $i -lt $Retries; $i++) {
        Write-Log "Running $Executable $ArgList ..."
        & $Executable $ArgList
        if ($LASTEXITCODE -notin $AllowedExitCodes) {
            Write-Log "$Executable returned unsuccessfully with exit code $LASTEXITCODE"
            Start-Sleep -Seconds $RetryDelaySeconds
            continue
        }
        else {
            Write-Log "$Executable returned successfully"
            return
        }
    }

    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INVOKE_EXECUTABLE -ErrorMessage "Exhausted retries for $Executable $ArgList"
}

function Assert-FileExists {
    Param(
        [Parameter(Mandatory = $true, Position = 0)][string]
        $Filename
    )

    if (-Not (Test-Path $Filename)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_FILE_NOT_EXIST -ErrorMessage "$Filename does not exist"
    }
}
