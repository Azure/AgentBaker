function Get-ProvisioningScripts {
    if (!(Test-Path 'c:\AzureData\windows\provisioningscripts')) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_NOT_FOUND_PROVISIONING_SCRIPTS -ErrorMessage "Failed to found provisioning scripts"
    }
    Write-Log "Copying provisioning scripts"
    Move-Item 'c:\AzureData\windows\provisioningscripts\*' 'c:\k'
    Remove-Item -Path 'c:\AzureData\windows\provisioningscripts' -Force
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

function Initialize-DataDirectories {
    # Some of the Kubernetes tests that were designed for Linux try to mount /tmp into a pod
    # On Windows, Go translates to c:\tmp. If that path doesn't exist, then some node tests fail

    $requiredPaths = 'c:\tmp'

    $requiredPaths | ForEach-Object {
        Create-Directory -FullPath $_
    }
}

function Get-LogCollectionScripts {
    Write-Log "Moving various log collect scripts and depencencies"
    try {
        Move-Item -Path 'C:\AzureData\windows\debug' -Destination 'c:\k\'
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_COPY_LOG_COLLECTION_SCRIPTS -ErrorMessage "Failed to move log collect scripts and depencencies from C:\AzureData\windows\debug to C:\k. Error: $_"
    }
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

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Services -Value @{
        HNSRemediator       = @{
            IntervalInMinutes = $Global:HNSRemediatorIntervalInMinutes;
        };
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
            FeatureGates = $global:KubeproxyFeatureGates;
            ConfigArgs   = $global:KubeproxyConfigArgs
        };
    }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Install -Value @{
        Destination = "c:\k";
    }

    $Global:ClusterConfiguration | ConvertTo-Json -Depth 10 | Out-File -FilePath $global:KubeClusterConfigPath
}

function Update-DefenderPreferences {
    Add-MpPreference -ExclusionProcess "c:\k\kubelet.exe"
    Add-MpPreference -ExclusionProcess "c:\k\kube-proxy.exe"

    # Azure CNI
    Add-MpPreference -ExclusionProcess "C:\k\azurecni\bin\azure-cns.exe"
    Add-MpPreference -ExclusionProcess "C:\k\azurecni\bin\azure-vnet-ipam.exe"
    Add-MpPreference -ExclusionProcess "C:\k\azurecni\bin\azure-vnet-ipamv6.exe"
    Add-MpPreference -ExclusionProcess "C:\k\azurecni\bin\azure-vnet-telemetry.exe"
    Add-MpPreference -ExclusionProcess "C:\k\azurecni\bin\azure-vnet.exe"
    Add-MpPreference -ExclusionProcess "C:\k\azurecni\bin\AzureNetworkContainer.exe"
    Add-MpPreference -ExclusionProcess "C:\k\azurecni\bin\CnsWrapperService.exe"

    if ($global:EnableCsiProxy) {
        Add-MpPreference -ExclusionProcess "c:\k\csi-proxy.exe"
    }

    if ($global:ContainerRuntime -eq 'containerd') {
        Add-MpPreference -ExclusionProcess "c:\program files\containerd\containerd.exe"
    } else {
        Add-MpPreference -ExclusionProcess "C:\Program Files\Docker\dockerd.exe"
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