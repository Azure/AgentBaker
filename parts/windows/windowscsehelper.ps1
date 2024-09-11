# This script is used to define basic util functions
# It is better to define functions in the scripts under staging/cse/windows.

# Define all exit codes in Windows CSE
# It must match `[A-Z_]+`
$global:WINDOWS_CSE_SUCCESS=0
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
$global:WINDOWS_CSE_ERROR_NOT_FOUND_MANAGEMENT_IP=29
$global:WINDOWS_CSE_ERROR_NOT_FOUND_BUILD_NUMBER=30
$global:WINDOWS_CSE_ERROR_NOT_FOUND_PROVISIONING_SCRIPTS=31
$global:WINDOWS_CSE_ERROR_START_NODE_RESET_SCRIPT_TASK=32
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CSE_PACKAGE=33
$global:WINDOWS_CSE_ERROR_DOWNLOAD_KUBERNETES_PACKAGE=34
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CNI_PACKAGE=35
$global:WINDOWS_CSE_ERROR_DOWNLOAD_HNS_MODULE=36
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CALICO_PACKAGE=37
$global:WINDOWS_CSE_ERROR_DOWNLOAD_GMSA_PACKAGE=38
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CSI_PROXY_PACKAGE=39
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CONTAINERD_PACKAGE=40
$global:WINDOWS_CSE_ERROR_SET_TCP_DYNAMIC_PORT_RANGE=41
$global:WINDOWS_CSE_ERROR_BUILD_DOCKER_PAUSE_CONTAINER=42
$global:WINDOWS_CSE_ERROR_PULL_PAUSE_IMAGE=43
$global:WINDOWS_CSE_ERROR_BUILD_TAG_PAUSE_IMAGE=44
$global:WINDOWS_CSE_ERROR_CONTAINERD_BINARY_EXIST=45
$global:WINDOWS_CSE_ERROR_SET_TCP_EXCLUDE_PORT_RANGE=46
$global:WINDOWS_CSE_ERROR_SET_UDP_DYNAMIC_PORT_RANGE=47
$global:WINDOWS_CSE_ERROR_SET_UDP_EXCLUDE_PORT_RANGE=48
$global:WINDOWS_CSE_ERROR_NO_CUSTOM_DATA_BIN=49 # Return this error code in csecmd.ps1 when C:\AzureData\CustomData.bin does not exist
$global:WINDOWS_CSE_ERROR_NO_CSE_RESULT_LOG=50 # Return this error code in csecmd.ps1 when C:\AzureData\CSEResult.log does not exist
$global:WINDOWS_CSE_ERROR_COPY_LOG_COLLECTION_SCRIPTS=51
$global:WINDOWS_CSE_ERROR_RESIZE_OS_DRIVE=52
$global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED=53
$global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_TIMEOUT=54
$global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_VM_SIZE_NOT_SUPPORTED=55
$global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_SET=56
$global:WINDOWS_CSE_ERROR_GPU_SKU_INFO_NOT_FOUND=57
$global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_DOWNLOAD_FAILURE=58
$global:WINDOWS_CSE_ERROR_GPU_DRIVER_INVALID_SIGNATURE=59
$global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_EXCEPTION=60
$global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_EXE=61
$global:WINDOWS_CSE_ERROR_UPDATING_KUBE_CLUSTER_CONFIG=62
$global:WINDOWS_CSE_ERROR_GET_NODE_IPV6_IP=63
$global:WINDOWS_CSE_ERROR_GET_CONTAINERD_VERSION=64
$global:WINDOWS_CSE_ERROR_INSTALL_CREDENTIAL_PROVIDER = 65 # exit code for installing credential provider
$global:WINDOWS_CSE_ERROR_DOWNLOAD_CREDEDNTIAL_PROVIDER=66 # exit code for downloading credential provider failure
$global:WINDOWS_CSE_ERROR_CREDENTIAL_PROVIDER_CONFIG=67 # exit code for checking credential provider config failure
$global:WINDOWS_CSE_ERROR_ADJUST_PAGEFILE_SIZE=68
$global:WINDOWS_CSE_ERROR_LOOKUP_INSTANCE_DATA_TAG=69 # exit code for looking up nodepool/VM tags via IMDS
# WINDOWS_CSE_ERROR_MAX_CODE is only used in unit tests to verify whether new error code name is added in $global:ErrorCodeNames
# Please use the current value of WINDOWS_CSE_ERROR_MAX_CODE as the value of the new error code and increment it by 1
$global:WINDOWS_CSE_ERROR_MAX_CODE=70

# Please add new error code for downloading new packages in RP code too
$global:ErrorCodeNames = @(
    "WINDOWS_CSE_SUCCESS",
    "WINDOWS_CSE_ERROR_UNKNOWN",
    "WINDOWS_CSE_ERROR_DOWNLOAD_FILE_WITH_RETRY",
    "WINDOWS_CSE_ERROR_INVOKE_EXECUTABLE",
    "WINDOWS_CSE_ERROR_FILE_NOT_EXIST",
    "WINDOWS_CSE_ERROR_CHECK_API_SERVER_CONNECTIVITY",
    "WINDOWS_CSE_ERROR_PAUSE_IMAGE_NOT_EXIST",
    "WINDOWS_CSE_ERROR_GET_SUBNET_PREFIX",
    "WINDOWS_CSE_ERROR_GENERATE_TOKEN_FOR_ARM",
    "WINDOWS_CSE_ERROR_NETWORK_INTERFACES_NOT_EXIST",
    "WINDOWS_CSE_ERROR_NETWORK_ADAPTER_NOT_EXIST",
    "WINDOWS_CSE_ERROR_MANAGEMENT_IP_NOT_EXIST",
    "WINDOWS_CSE_ERROR_CALICO_SERVICE_ACCOUNT_NOT_EXIST",
    "WINDOWS_CSE_ERROR_CONTAINERD_NOT_INSTALLED",
    "WINDOWS_CSE_ERROR_CONTAINERD_NOT_RUNNING",
    "WINDOWS_CSE_ERROR_OPENSSH_NOT_INSTALLED",
    "WINDOWS_CSE_ERROR_OPENSSH_FIREWALL_NOT_CONFIGURED",
    "WINDOWS_CSE_ERROR_INVALID_PARAMETER_IN_AZURE_CONFIG",
    "WINDOWS_CSE_ERROR_NO_DOCKER_TO_BUILD_PAUSE_CONTAINER",
    "WINDOWS_CSE_ERROR_GET_CA_CERTIFICATES",
    "WINDOWS_CSE_ERROR_DOWNLOAD_CA_CERTIFICATES",
    "WINDOWS_CSE_ERROR_EMPTY_CA_CERTIFICATES",
    "WINDOWS_CSE_ERROR_ENABLE_SECURE_TLS",
    "WINDOWS_CSE_ERROR_GMSA_EXPAND_ARCHIVE",
    "WINDOWS_CSE_ERROR_GMSA_ENABLE_POWERSHELL_PRIVILEGE",
    "WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_PERMISSION",
    "WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_VALUES",
    "WINDOWS_CSE_ERROR_GMSA_IMPORT_CCGEVENTS",
    "WINDOWS_CSE_ERROR_GMSA_IMPORT_CCGAKVPPLUGINEVENTS",
    "WINDOWS_CSE_ERROR_NOT_FOUND_MANAGEMENT_IP",
    "WINDOWS_CSE_ERROR_NOT_FOUND_BUILD_NUMBER",
    "WINDOWS_CSE_ERROR_NOT_FOUND_PROVISIONING_SCRIPTS",
    "WINDOWS_CSE_ERROR_START_NODE_RESET_SCRIPT_TASK",
    "WINDOWS_CSE_ERROR_DOWNLOAD_CSE_PACKAGE",
    "WINDOWS_CSE_ERROR_DOWNLOAD_KUBERNETES_PACKAGE",
    "WINDOWS_CSE_ERROR_DOWNLOAD_CNI_PACKAGE",
    "WINDOWS_CSE_ERROR_DOWNLOAD_HNS_MODULE",
    "WINDOWS_CSE_ERROR_DOWNLOAD_CALICO_PACKAGE",
    "WINDOWS_CSE_ERROR_DOWNLOAD_GMSA_PACKAGE",
    "WINDOWS_CSE_ERROR_DOWNLOAD_CSI_PROXY_PACKAGE",
    "WINDOWS_CSE_ERROR_DOWNLOAD_CONTAINERD_PACKAGE",
    "WINDOWS_CSE_ERROR_SET_TCP_DYNAMIC_PORT_RANGE",
    "WINDOWS_CSE_ERROR_BUILD_DOCKER_PAUSE_CONTAINER",
    "WINDOWS_CSE_ERROR_PULL_PAUSE_IMAGE",
    "WINDOWS_CSE_ERROR_BUILD_TAG_PAUSE_IMAGE",
    "WINDOWS_CSE_ERROR_CONTAINERD_BINARY_EXIST",
    "WINDOWS_CSE_ERROR_SET_TCP_EXCLUDE_PORT_RANGE",
    "WINDOWS_CSE_ERROR_SET_UDP_DYNAMIC_PORT_RANGE",
    "WINDOWS_CSE_ERROR_SET_UDP_EXCLUDE_PORT_RANGE",
    "WINDOWS_CSE_ERROR_NO_CUSTOM_DATA_BIN",
    "WINDOWS_CSE_ERROR_NO_CSE_RESULT_LOG",
    "WINDOWS_CSE_ERROR_COPY_LOG_COLLECTION_SCRIPTS",
    "WINDOWS_CSE_ERROR_RESIZE_OS_DRIVE",
    "WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED",
    "WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_TIMEOUT",
    "WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_VM_SIZE_NOT_SUPPORTED",
    "WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_SET",
    "WINDOWS_CSE_ERROR_GPU_SKU_INFO_NOT_FOUND",
    "WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_DOWNLOAD_FAILURE",
    "WINDOWS_CSE_ERROR_GPU_DRIVER_INVALID_SIGNATURE",
    "WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_EXCEPTION",
    "WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_EXE",
    "WINDOWS_CSE_ERROR_UPDATING_KUBE_CLUSTER_CONFIG",
    "WINDOWS_CSE_ERROR_GET_NODE_IPV6_IP",
    "WINDOWS_CSE_ERROR_GET_CONTAINERD_VERSION",
    "WINDOWS_CSE_ERROR_INSTALL_CREDENTIAL_PROVIDER",
    "WINDOWS_CSE_ERROR_DOWNLOAD_CREDEDNTIAL_PROVIDER",
    "WINDOWS_CSE_ERROR_CREDENTIAL_PROVIDER_CONFIG",
    "WINDOWS_CSE_ERROR_ADJUST_PAGEFILE_SIZE",
    "WINDOWS_CSE_ERROR_LOOKUP_INSTANCE_DATA_TAG"
)

# NOTE: KubernetesVersion does not contain "v"
$global:MinimalKubernetesVersionWithLatestContainerd = "1.28.0" # Will change it to the correct version when we support new Windows containerd version
# Although the contianerd package url is set in AKS RP code now, we still need to update the following variables for AgentBaker Windows E2E tests.
$global:StableContainerdPackage = "v1.6.35-azure.1/binaries/containerd-v1.6.35-azure.1-windows-amd64.tar.gz"
# The latest containerd version
$global:LatestContainerdPackage = "v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz"

$global:EventsLoggingDir = "C:\WindowsAzure\Logs\Plugins\Microsoft.Compute.CustomScriptExtension\Events\"
$global:TaskName = ""
$global:TaskTimeStamp = ""

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
        $DestinationPath,
        [Parameter(Mandatory = $true)][int]
        $ExitCode
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
            Set-ExitCode -ExitCode $ExitCode -ErrorMessage "Failed in downloading $Url. Error: $_"
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
    # we use | as the separator as a workaround since " or ' do not work as expected per the testings
    $global:ErrorMessage=($ErrorMessage -replace '\|', '%7C')
    exit $ExitCode
}

function Postpone-RestartComputer 
{
    Logs-To-Event -TaskName "AKS.WindowsCSE.PostponeRestartComputer" -TaskMessage "Start to create an one-time task to restart the VM"
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
            # Do not log Args since Args may contain sensitive data
            Write-Log "Retry $i : $command"
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
        [Parameter(Mandatory=$true)][string]
        $Executable,
        [Parameter(Mandatory=$true)][string[]]
        $ArgList,
        [Parameter(Mandatory=$true)][int]
        $ExitCode,
        [int[]]
        $AllowedExitCodes = @(0),
        [int]
        $Retries = 0,
        [int]
        $RetryDelaySeconds = 1
    )

    for ($i = 0; $i -le $Retries; $i++) {
        Write-Log "$i - Running $Executable $ArgList ..."
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

    Set-ExitCode -ExitCode $ExitCode -ErrorMessage "Exhausted retries for $Executable $ArgList"
}

function Assert-FileExists {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Filename,
        [Parameter(Mandatory = $true)][int]
        $ExitCode
    )

    if (-Not (Test-Path $Filename)) {
        Set-ExitCode -ExitCode $ExitCode -ErrorMessage "$Filename does not exist"
    }
}

function Get-WindowsBuildNumber {
    return (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion").CurrentBuild
}

function Get-WindowsVersion {
    $buildNumber = Get-WindowsBuildNumber
    switch ($buildNumber) {
        "17763" { return "1809" }
        "20348" { return "ltsc2022" }
        "25398" { return "23H2" }
        {$_ -ge "25399" -and $_ -le "30397"} { return "test2025" }
        Default {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_NOT_FOUND_BUILD_NUMBER -ErrorMessage "Failed to find the windows build number: $buildNumber"
        }
    }
}

function Get-WindowsPauseVersion {
    $buildNumber = Get-WindowsBuildNumber
    switch ($buildNumber) {
        "17763" { return "1809" }
        "20348" { return "ltsc2022" }
        "25398" { return "ltsc2022" }
        {$_ -ge "25399" -and $_ -le "30397"} { return "ltsc2022" }
        Default {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_NOT_FOUND_BUILD_NUMBER -ErrorMessage "Failed to find the windows build number: $buildNumber"
        }
    }
}

function Install-Containerd-Based-On-Kubernetes-Version {
  Param(
    [Parameter(Mandatory = $true)][string]
    $ContainerdUrl,
    [Parameter(Mandatory = $true)][string]
    $CNIBinDir,
    [Parameter(Mandatory = $true)][string]
    $CNIConfDir,
    [Parameter(Mandatory = $true)][string]
    $KubeDir,
    [Parameter(Mandatory = $true)][string]
    $KubernetesVersion
  )

  Logs-To-Event -TaskName "AKS.WindowsCSE.InstallContainerdBasedOnKubernetesVersion" -TaskMessage "Start to install ContainerD based on kubernetes version. ContainerdUrl: $global:ContainerdUrl, KubernetesVersion: $global:KubeBinariesVersion"

  # In the past, $global:ContainerdUrl is a full URL to download Windows containerd package.
  # Example: "https://acs-mirror.azureedge.net/containerd/windows/v0.0.46/binaries/containerd-v0.0.46-windows-amd64.tar.gz"
  # To support multiple containerd versions, we only set the endpoint in $global:ContainerdUrl.
  # Example: "https://acs-mirror.azureedge.net/containerd/windows/"
  # We only set containerd package based on kubernetes version when $global:ContainerdUrl ends with "/" so we support:
  #   1. Current behavior to set the full URL
  #   2. Setting containerd package in toggle for test purpose or hotfix
  if ($ContainerdUrl.EndsWith("/")) {
    Write-Log "ContainerdURL is $ContainerdUrl"
    $containerdPackage=$global:StableContainerdPackage
    if (([version]$KubernetesVersion).CompareTo([version]$global:MinimalKubernetesVersionWithLatestContainerd) -ge 0) {
        $containerdPackage=$global:LatestContainerdPackage
        Write-Log "Kubernetes version $KubernetesVersion is greater than or equal to $global:MinimalKubernetesVersionWithLatestContainerd so the latest containerd version $containerdPackage is used"
    } else {
      Write-Log "Kubernetes version $KubernetesVersion is less than $global:MinimalKubernetesVersionWithLatestContainerd so the stable containerd version $containerdPackage is used"
    }
    $ContainerdUrl = $ContainerdUrl + $containerdPackage
  }
  Logs-To-Event -TaskName "AKS.WindowsCSE.InstallContainerd" -TaskMessage "Start to install ContainerD. ContainerdUrl: $ContainerdUrl"
  Install-Containerd -ContainerdUrl $ContainerdUrl -CNIBinDir $CNIBinDir -CNIConfDir $CNIConfDir -KubeDir $KubeDir
}

function Logs-To-Event {
    Param(
        [Parameter(Mandatory = $true)][string]
        $TaskName,
        [Parameter(Mandatory = $true)][string]
        $TaskMessage
    )
    $eventLevel="Informational"
    if ($global:ExitCode -ne 0) {
        $eventLevel="Error"
    }

    $eventsFileName=[DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    $currentTime=$(Get-Date -Format "yyyy-MM-dd HH:mm:ss.fff")
    
    $lastTaskName = ""
    $lastTaskDuration = 0
    if ($global:TaskTimeStamp -ne "") {
        $lastTaskName = $global:TaskName
        $lastTaskDuration = $(New-Timespan -Start $global:TaskTimeStamp -End $currentTime)
    }

    $global:TaskName = $TaskName
    $global:TaskTimeStamp = $currentTime

    Write-Log "$global:TaskName - $TaskMessage"
    $TaskMessage = (echo $TaskMessage | ConvertTo-Json)
    $messageJson = @"
    {
        "HostName": "$env:computername",
        "LastTaskName": "$lastTaskName",
        "LastTaskDuration": "$lastTaskDuration",
        "CurrentTaskMessage": $TaskMessage
    }
"@
    $messageJson = (echo $messageJson | ConvertTo-Json)
    
    $jsonString = @"
    {
        "Timestamp": "$global:TaskTimeStamp",
        "OperationId": "$global:OperationId",
        "Version": "1.10",
        "TaskName": "$global:TaskName",
        "EventLevel": "$eventLevel",
        "Message": $messageJson
    }
"@
    echo $jsonString | Set-Content ${global:EventsLoggingDir}${eventsFileName}.json
}