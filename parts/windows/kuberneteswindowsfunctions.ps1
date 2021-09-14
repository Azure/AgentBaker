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

function Get-ProvisioningScripts {
    Write-Log "Getting provisioning scripts"
    DownloadFileOverHttp -Url $global:ProvisioningScriptsPackageUrl -DestinationPath 'c:\k\provisioningscripts.zip'
    Expand-Archive -Path 'c:\k\provisioningscripts.zip' -DestinationPath 'c:\k' -Force
    Remove-Item -Path 'c:\k\provisioningscripts.zip' -Force
}

function Get-WindowsVersion {
    $systemInfo = Get-ItemProperty -Path "HKLM:SOFTWARE\Microsoft\Windows NT\CurrentVersion"
    return "$($systemInfo.CurrentBuildNumber).$($systemInfo.UBR)"
}

function Get-CniVersion {
    switch ($global:NetworkPlugin) {
        "azure" {
            if ($global:VNetCNIPluginsURL -match "(v[0-9`.]+).(zip|tar)") {
                return $matches[1]
            }
            else {
                return ""
            }
            break;
        }
        default {
            return ""
        }
    }
}

function Get-InstanceMetadataServiceTelemetry {
    $keys = @{ }

    try {
        # Write-Log "Querying instance metadata service..."
        # Note: 2019-04-30 is latest api available in all clouds
        $metadata = Invoke-RestMethod -Headers @{"Metadata" = "true" } -URI "http://169.254.169.254/metadata/instance?api-version=2019-04-30" -Method get
        # Write-Log ($metadata | ConvertTo-Json)

        $keys.Add("vm_size", $metadata.compute.vmSize)
    }
    catch {
        Write-Log "Error querying instance metadata service."
    }

    return $keys
}

# https://stackoverflow.com/a/34559554/697126
function New-TemporaryDirectory {
    $parent = [System.IO.Path]::GetTempPath()
    [string] $name = [System.Guid]::NewGuid()
    New-Item -ItemType Directory -Path (Join-Path $parent $name)
}

function Initialize-DataDirectories {
    # Some of the Kubernetes tests that were designed for Linux try to mount /tmp into a pod
    # On Windows, Go translates to c:\tmp. If that path doesn't exist, then some node tests fail

    $requiredPaths = 'c:\tmp'

    $requiredPaths | ForEach-Object {
        Create-Directory -FullPath $_
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

function Get-LogCollectionScripts {
    Write-Log "Getting various log collect scripts and depencencies"
    Create-Directory -FullPath 'c:\k\debug' -DirectoryUsage "storing debug scripts"
    DownloadFileOverHttp -Url 'https://github.com/Azure/AgentBaker/raw/master/vhdbuilder/scripts/windows/collect-windows-logs.ps1' -DestinationPath 'c:\k\debug\collect-windows-logs.ps1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/collectlogs.ps1' -DestinationPath 'c:\k\debug\collectlogs.ps1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/dumpVfpPolicies.ps1' -DestinationPath 'c:\k\debug\dumpVfpPolicies.ps1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/portReservationTest.ps1' -DestinationPath 'c:\k\debug\portReservationTest.ps1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/starthnstrace.cmd' -DestinationPath 'c:\k\debug\starthnstrace.cmd'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/startpacketcapture.cmd' -DestinationPath 'c:\k\debug\startpacketcapture.cmd'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/stoppacketcapture.cmd' -DestinationPath 'c:\k\debug\stoppacketcapture.cmd'
    DownloadFileOverHttp -Url 'https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/debug/VFP.psm1' -DestinationPath 'c:\k\debug\VFP.psm1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/helper.psm1' -DestinationPath 'c:\k\debug\helper.psm1'
    DownloadFileOverHttp -Url 'https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/hns.psm1' -DestinationPath 'c:\k\debug\hns.psm1'
}

function Register-LogsCleanupScriptTask {
    Write-Log "Creating a scheduled task to run windowslogscleanup.ps1"

    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-File `"c:\k\windowslogscleanup.ps1`""
    $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
    $trigger = New-JobTrigger -Daily -At "00:00" -DaysInterval 1
    $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "log-cleanup-task"
    Register-ScheduledTask -TaskName "log-cleanup-task" -InputObject $definition
}

function Register-NodeResetScriptTask {
    Write-Log "Creating a startup task to run windowsnodereset.ps1"

    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-File `"c:\k\windowsnodereset.ps1`""
    $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
    $trigger = New-JobTrigger -AtStartup -RandomDelay 00:00:05
    $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "k8s-restart-job"
    Register-ScheduledTask -TaskName "k8s-restart-job" -InputObject $definition
}

# TODO ksubrmnn parameterize this fully
function Write-KubeClusterConfig {
    param(
        [Parameter(Mandatory = $true)][string]
        $MasterIP,
        [Parameter(Mandatory = $true)][string]
        $KubeDnsServiceIp
    )

    $Global:ClusterConfiguration = [PSCustomObject]@{ }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Cri -Value @{
        Name   = $global:ContainerRuntime;
        Images = @{
            # e.g. "mcr.microsoft.com/oss/kubernetes/pause:1.4.1"
            "Pause" = $global:WindowsPauseImageURL
        }
    }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Cni -Value @{
        Name   = $global:NetworkPlugin;
        Plugin = @{
            Name = "bridge";
        };
    }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Csi -Value @{
        EnableProxy = $global:EnableCsiProxy
    }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Kubernetes -Value @{
        Source       = @{
            Release = $global:KubeBinariesVersion;
        };
        ControlPlane = @{
            IpAddress    = $MasterIP;
            Username     = "azureuser"
            MasterSubnet = $global:MasterSubnet
        };
        Network      = @{
            ServiceCidr = $global:KubeServiceCIDR;
            ClusterCidr = $global:KubeClusterCIDR;
            DnsIp       = $KubeDnsServiceIp
        };
        Kubelet      = @{
            NodeLabels = $global:KubeletNodeLabels;
            ConfigArgs = $global:KubeletConfigArgs
        };
        Kubeproxy    = @{
            FeatureGates = $global:KubeproxyFeatureGates
        };
    }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Install -Value @{
        Destination = "c:\k";
    }

    $Global:ClusterConfiguration | ConvertTo-Json -Depth 10 | Out-File -FilePath $global:KubeClusterConfigPath
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

function Update-DefenderPreferences {
    Add-MpPreference -ExclusionProcess "c:\k\kubelet.exe"

    if ($global:EnableCsiProxy) {
        Add-MpPreference -ExclusionProcess "c:\k\csi-proxy-server.exe"
    }

    if ($global:ContainerRuntime -eq 'containerd') {
        Add-MpPreference -ExclusionProcess "c:\program files\containerd\containerd.exe"
    }
}

function Check-APIServerConnectivity {
    Param(
        [Parameter(Mandatory = $true)][string]
        $MasterIP,
        [Parameter(Mandatory = $false)][int]
        $RetryInterval = 1,
        [Parameter(Mandatory = $false)][int]
        $ConnectTimeout = 10,  #seconds
        [Parameter(Mandatory = $false)][int]
        $MaxRetryCount = 100
    )
    $retryCount=0

    do {
        try {
            $tcpClient=New-Object Net.Sockets.TcpClient
            Write-Log "Retry $retryCount : Trying to connect to API server $MasterIP"
            $tcpClient.ConnectAsync($MasterIP, 443).wait($ConnectTimeout*1000)
            if ($tcpClient.Connected) {
                $tcpClient.Close()
                Write-Log "Retry $retryCount : Connected to API server successfully"
                return
            }
            $tcpClient.Close()
        } catch {
            Write-Log "Retry $retryCount : Failed to connect to API server $MasterIP. Error: $_"
        }
        $retryCount++
        Write-Log "Retry $retryCount : Sleep $RetryInterval and then retry to connect to API server"
        Sleep $RetryInterval
    } while ($retryCount -lt $MaxRetryCount)

    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CHECK_API_SERVER_CONNECTIVITY -ErrorMessage "Failed to connect to API server $MasterIP after $retryCount retries"
}

function Get-CACertificates {
    try {
        Write-Log "Get CA certificates"
        $caFolder = "C:\ca"
        $uri = 'http://168.63.129.16/machine?comp=acmspackage&type=cacertificates&ext=json'

        Create-Directory -FullPath $caFolder -DirectoryUsage "storing CA certificates"

        Write-Log "Download CA certificates rawdata"
        # This is required when the root CA certs are different for some clouds.
        try {
            $rawData = Retry-Command -Command 'Invoke-WebRequest' -Args @{Uri=$uri; UseBasicParsing=$true} -Retries 5 -RetryDelaySeconds 10
        } catch {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_CA_CERTIFICATES -ErrorMessage "Failed to download CA certificates rawdata. Error: $_"
        }

        Write-Log "Convert CA certificates rawdata"
        $caCerts=($rawData.Content) | ConvertFrom-Json
        if ([string]::IsNullOrEmpty($caCerts)) {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_EMPTY_CA_CERTIFICATES -ErrorMessage "CA certificates rawdata is empty"
        }

        $certificates = $caCerts.Certificates
        for ($index = 0; $index -lt $certificates.Length ; $index++) {
            $name=$certificates[$index].Name
            $certFilePath = Join-Path $caFolder $name
            Write-Log "Write certificate $name to $certFilePath"
            $certificates[$index].CertBody > $certFilePath
        }
    }
    catch {
        # Catch all exceptions in this function. NOTE: exit cannot be caught.
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GET_CA_CERTIFICATES -ErrorMessage $_
    }
}