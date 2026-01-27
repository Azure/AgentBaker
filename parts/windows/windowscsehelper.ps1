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
$global:WINDOWS_CSE_ERROR_DOWNLOAD_SECURE_TLS_BOOTSTRAP_CLIENT=70 # exit code for downloading secure TLS bootstrap client failure
$global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT=71 # exit code for installing secure TLS bootstrap client failure
$global:WINDOWS_CSE_ERROR_WINDOWS_CILIUM_NETWORKING_INSTALL_FAILED=72
$global:WINDOWS_CSE_ERROR_EXTRACT_ZIP=73
$global:WINDOWS_CSE_ERROR_LOAD_METADATA=74
$global:WINDOWS_CSE_ERROR_PARSE_METADATA=75
# WINDOWS_CSE_ERROR_MAX_CODE is only used in unit tests to verify whether new error code name is added in $global:ErrorCodeNames
# Please use the current value of WINDOWS_CSE_ERROR_MAX_CODE as the value of the new error code and increment it by 1
$global:WINDOWS_CSE_ERROR_MAX_CODE=76

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
    "WINDOWS_CSE_ERROR_LOOKUP_INSTANCE_DATA_TAG",
    "WINDOWS_CSE_ERROR_DOWNLOAD_SECURE_TLS_BOOTSTRAP_CLIENT",
    "WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT",
    "WINDOWS_CSE_ERROR_WINDOWS_CILIUM_NETWORKING_INSTALL_FAILED",
    "WINDOWS_CSE_ERROR_EXTRACT_ZIP",
    "WINDOWS_CSE_ERROR_LOAD_METADATA",
    "WINDOWS_CSE_ERROR_PARSE_METADATA"
)

# The package domain to be used
$global:PackageDownloadFqdn = $null
# The preferred package FQDN
$global:PreferredPackageDownloadFqdn = "packages.aks.azure.com"
# Fallback FQDN if preferred cannot be contacted
$global:FallbackPackageDownloadFqdn = "acs-mirror.azureedge.net"

# NOTE: KubernetesVersion does not contain "v"
$global:MinimalKubernetesVersionWithLatestContainerd = "1.28.0" # Will change it to the correct version when we support new Windows containerd version
# The minimum kubernetes version to use containerd 2.x
$global:MinimalKubernetesVersionWithLatestContainerd2 = "1.33.0"
# Although the contianerd package url is set in AKS RP code now, we still need to update the following variables for AgentBaker Windows E2E tests.

# Define containerd version template
$global:ContainerdPackageTemplate = "v{0}-azure.1/binaries/containerd-v{0}-azure.1-windows-amd64.tar.gz"

# Version numbers only - used in various places
$global:StableContainerdVersion = "1.6.35"
$global:LatestContainerdVersion = "1.7.20"
$global:LatestContainerd2Version = "2.0.4"

$global:WindowsVersion2025 = "2025"

# Full package paths are generated using [string]::Format($global:ContainerdPackageTemplate, $version) when needed

$global:EventsLoggingDir = "C:\WindowsAzure\Logs\Plugins\Microsoft.Compute.CustomScriptExtension\Events\"
$global:TaskName = ""
$global:TaskTimeStamp = ""

# This filter removes null characters (\0) which are captured in nssm.exe output when logged through powershell
filter RemoveNulls { $_ -replace '\0', '' }

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log($message) {
    $msg = $message | Timestamp
    Write-Host $msg
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
    $cleanUrl = $Url.Split('?')[0]
    $fileName = [IO.Path]::GetFileName($cleanUrl)

    $search = @()
    if ($global:CacheDir -and (Test-Path $global:CacheDir)) {
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

        $MappedUrl = Update-BaseUrl -InitialUrl $Url
        Write-Log "Updated URL $Url -> $MappedUrl to download $fileName to $DestinationPath"

        $oldProgressPreference = $ProgressPreference
        $ProgressPreference = 'SilentlyContinue'

        $downloadTimer = [System.Diagnostics.Stopwatch]::StartNew()
        try {
            $args = @{Uri=$MappedUrl; Method="Get"; OutFile=$DestinationPath; ErrorAction="Stop"}
            Retry-Command -Command "Invoke-RestMethod" -Args $args -Retries 5 -RetryDelaySeconds 10
        } catch {
            Set-ExitCode -ExitCode $ExitCode -ErrorMessage "Failed in downloading $MappedUrl. Error: $_"
        }
        $downloadTimer.Stop()
        $elapsedMs = $downloadTimer.ElapsedMilliseconds

        if ($global:AppInsightsClient -ne $null) {
            $event = New-Object "Microsoft.ApplicationInsights.DataContracts.EventTelemetry"
            $event.Name = "FileDownload"
            $event.Properties["FileName"] = $fileName
            $event.Metrics["DurationMs"] = $elapsedMs
            $global:AppInsightsClient.TrackEvent($event)
        }

        $ProgressPreference = $oldProgressPreference

        Write-Log "Downloaded file $MappedUrl to $DestinationPath in $elapsedMs ms"
        Get-Item $DestinationPath -ErrorAction Continue | Format-List | Out-String | Write-Log
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

function AKS-Expand-Archive {
    Param(
        [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$Path,
        [Parameter(Mandatory = $true)][ValidateNotNullOrEmpty()][string]$DestinationPath,
        [Parameter(Mandatory = $false)][ValidateNotNullOrEmpty()][boolean]$Force
    )

   try {
        Expand-Archive -Path $Path -DestinationPath ${DestinationPath} -ErrorAction Stop -Force
        Write-Log "Successfully expanded file $Path to $DestinationPath"
    } catch {
        Write-Log "Failed to expand file $Path - Error: $_"
        Get-Item -ErrorAction Continue $Path | Format-List | Out-String | Write-Log
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_EXTRACT_ZIP -ErrorMessage "Unable to extract zip file. Error: $_"
    }
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
        {$_ -ge "25399" -and $_ -le "30397"} { return $global:WindowsVersion2025 }
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
        {$_ -ge "25399" -and $_ -le "30397"} { return  "ltsc2022" }
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

  # Get the current Windows version, this is interim since we are progressively supporting containerd 2.0 for all Windows version. for now only test2025
  $windowsVersion = Get-WindowsVersion
  Write-Log "Install Containerd with ContainerdURL: $ContainerdUrl, KubernetesVersion: $KubernetesVersion, WindowsVersion: $windowsVersion"
  Logs-To-Event -TaskName "AKS.WindowsCSE.InstallContainerdBasedOnKubernetesVersion" -TaskMessage "Start to install ContainerD based on kubernetes version. ContainerdUrl: $global:ContainerdUrl, KubernetesVersion: $global:KubeBinariesVersion, Windows Version: $windowsVersion"

  #  $global:ContainerdUrl is set from RP ContainerService.properties.orchestratorProfile.KubernetesConfig.WindowsContainerdURL
  # it can be
  # - a full URL. e.g.,  "https://packages.aks.azure.com/containerd/windows/v0.0.46/binaries/containerd-v0.0.46-windows-amd64.tar.gz"
  # - an endpoint: e.g., "https://packages.aks.azure.com/containerd/windows/"

  # We only set containerd package based on kubernetes version when $global:ContainerdUrl ends with "/" so we support:
  #   1. Current behavior to set the full URL
  #   2. Setting containerd package in toggle for test purpose or hotfix

  $containerdVersion=$global:StableContainerdVersion
  Write-Log "Install Containerd with request URL : $ContainerdUrl, Kubernetes version: $KubernetesVersion, Windows version: $windowsVersion."

  if ($ContainerdUrl.EndsWith("/")) {
    # for now we only preview containerd 2.0 for Windows 2025
    if ($windowsVersion -eq $global:WindowsVersion2025) {
        $containerdVersion=$global:LatestContainerd2Version
    } elseif (([version]$KubernetesVersion).CompareTo([version]$global:MinimalKubernetesVersionWithLatestContainerd) -ge 0) {
        $containerdVersion=$global:LatestContainerdVersion
    }
    $containerdPackage = [string]::Format($global:ContainerdPackageTemplate, $containerdVersion)
    $ContainerdUrl = $ContainerdUrl + $containerdPackage
  } elseif ( $windowsVersion -eq $global:WindowsVersion2025) {
    # TODO (beileihuang) : remove this else if block when RP is release to set the correct versions for 2025
    $containerdPattern = "v\d+\.\d+\.\d+-azure\.\d+/binaries/containerd-v\d+\.\d+\.\d+-azure\.\d+-windows-amd64\.tar\.gz"
    if ($ContainerdUrl -match $containerdPattern) {
        $matchedPath = $matches[0]
        $containerd2Package = [string]::Format($global:ContainerdPackageTemplate, $global:LatestContainerd2Version)
        $ContainerdUrl = $ContainerdUrl.Replace($matchedPath, $containerd2Package)
    }
  }

  Write-Log "Install Containerd with resolved containerd pacakge url: $ContainerdUrl, Kubernetes version: $KubernetesVersion, Windows version: $windowsVersion."
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
    Write-Output $jsonString | Set-Content ${global:EventsLoggingDir}${eventsFileName}.json
}

# AKS will transition to use packages.aks.azure.com as the default package download acs-mirror.azureedge.net
# on June 11th, 2025.  Just prior to the transition we want to have fallback logic in place to
# ensure that if packages.aks.azure.com is not reachable we can fallback to the old CDN URL
#
# This function sets the global variable $global:PackageDownloadFqdn to the preferred FQDN
# It will attempt to use the preferred FQDN first and if that fails it will fallback to the old CDN URL
function Resolve-PackagesDownloadFqdn {
    Param(
        [Parameter(Mandatory = $true)][string]
        $PreferredFqdn,
        [Parameter(Mandatory = $true)][string]
        $FallbackFqdn,
        [Parameter(Mandatory = $false)][int]
        $Retries = 5,
        [Parameter(Mandatory = $false)][int]
        $WaitSleepSeconds = 1
    )

    $packageDownloadBaseUrl = $PreferredFqdn

    for ($i = 1; $i -le $Retries; $i++) {
        # Confirm that we can establish connectivity to packages.aks.azure.com before node provisioning starts
        try {
            $response = Invoke-WebRequest -Uri "https://${PreferredFqdn}/acs-mirror/healthz" -UseBasicParsing -TimeoutSec 5 -ErrorAction SilentlyContinue
            $responseCode = [int]$response.StatusCode

            if ($responseCode -eq 200) {
                Write-Log "Established connectivity to $PreferredFqdn." | Out-Null
                break
            }
        } catch {
            $responseCode = 0
            Write-Log "Exception while trying to establish connectivity to $PreferredFqdn. Exception: $_" | Out-Null
            if ($_.Exception.Response) {
                $responseCode = [int]$_.Exception.Response.StatusCode
            }
        }

        if ($i -eq $Retries) {
            # If we cannot establish connectivity to packages.aks.azure.com, fallback to old CDN URL
            $packageDownloadBaseUrl = $FallbackFqdn
            break
        } else {
            Start-Sleep -Seconds $WaitSleepSeconds
        }
    }

    $global:PackageDownloadFqdn = $packageDownloadBaseUrl

    Logs-To-Event -TaskName "AKS.WindowsCSE.ResolvedPackageDomain" -TaskMessage "Package download FQDN: $global:PackageDownloadFqdn"
}

# This function will swap the domain in the URL based on the verified package download FQDN
function Update-BaseUrl {
    Param(
        [Parameter(Mandatory = $true)][string]
        $InitialUrl
    )

    $updatedUrl = $InitialUrl

    if (!($InitialUrl -match "acs-mirror\.azureedge\.net|packages\.aks\.azure\.com")) {
        # We're probably not in Public cloud
        return $updatedUrl
    }

    if ($global:PackageDownloadFqdn -eq $null) {
        # We're in public cloud, but we haven't set the package download FQDN yet
        $null = Resolve-PackagesDownloadFqdn -PreferredFqdn $global:PreferredPackageDownloadFqdn -FallbackFqdn $global:FallbackPackageDownloadFqdn
    }

    # Replace domain based on the current package download FQDN
    if (($global:PackageDownloadFqdn -eq "packages.aks.azure.com") -and ($InitialUrl -like "https://acs-mirror.azureedge.net/*")) {
        $updatedUrl = $InitialUrl -replace "acs-mirror.azureedge.net", $global:PackageDownloadFqdn
    } elseif (($global:PackageDownloadFqdn -eq "acs-mirror.azureedge.net") -and ($InitialUrl -like "https://packages.aks.azure.com/*")) {
        $updatedUrl = $InitialUrl -replace "packages.aks.azure.com", $global:PackageDownloadFqdn
    }

    return $updatedUrl
}

function Resolve-Error ($ErrorRecord=$Error[0])
{
   $ErrorRecord | Format-List * -Force
   $ErrorRecord.InvocationInfo |Format-List *
   $Exception = $ErrorRecord.Exception
   for ($i = 0; $Exception; $i++, ($Exception = $Exception.InnerException))
   {   "$i" * 80
       $Exception |Format-List * -Force
   }
}
