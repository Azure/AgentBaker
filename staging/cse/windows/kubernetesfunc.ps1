function Get-ProvisioningScripts {
    if (!(Test-Path 'c:\AzureData\windows\provisioningscripts')) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_NOT_FOUND_PROVISIONING_SCRIPTS -ErrorMessage "Failed to found provisioning scripts"
    }
    Write-Log "Copying provisioning scripts"
    Move-Item 'c:\AzureData\windows\provisioningscripts\*' 'c:\k' -Force
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

    Logs-To-Event -TaskName "AKS.WindowsCSE.InitializeDataDirectories" -TaskMessage "Start to create required data directories as needed"

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
    Logs-To-Event -TaskName "AKS.WindowsCSE.RegisterLogsCleanupScriptTask" -TaskMessage "Start to register logs cleanup script task"
    Write-Log "Creating a scheduled task to run windowslogscleanup.ps1"

    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-File `"c:\k\windowslogscleanup.ps1`""
    $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
    $trigger = New-JobTrigger -Daily -At "00:00" -DaysInterval 1
    $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "log-cleanup-task"
    Register-ScheduledTask -TaskName "log-cleanup-task" -InputObject $definition
}

function Register-NodeResetScriptTask {
    Logs-To-Event -TaskName "AKS.WindowsCSE.RegisterNodeResetScriptTask" -TaskMessage "Start to register node reset script task. HNSRemediatorIntervalInMinutes: $global:HNSRemediatorIntervalInMinutes"
    Write-Log "Creating a startup task to run windowsnodereset.ps1"

    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-File `"c:\k\windowsnodereset.ps1`""
    $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
    $trigger = New-JobTrigger -AtStartup -RandomDelay 00:00:05
    $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "k8s-restart-job"
    Register-ScheduledTask -TaskName "k8s-restart-job" -InputObject $definition
}

function Register-CACertificatesRefreshTask {
    Param(
        [Parameter(Mandatory = $false)][string]
        $Location = ""
    )

    Logs-To-Event -TaskName "AKS.WindowsCSE.RegisterCACertificatesRefreshTask" -TaskMessage "Start to register CA certificates refresh task"
    Write-Log "Creating a scheduled task to refresh custom cloud CA certificates"

    $taskName = "aks-ca-certs-refresh-task"
    if (Get-ScheduledTask -TaskName $taskName -ErrorAction Ignore) {
        Write-Log "Scheduled task $taskName already exists, skipping registration"
        return
    }

    # Include -Location only when it was provided, so older VHDs whose Get-CACertificates
    # does not accept -Location can still execute the scheduled task successfully.
    if ([string]::IsNullOrEmpty($Location)) {
        $refreshCommand = "& { . 'C:\AzureData\windows\windowscsehelper.ps1'; . 'C:\AzureData\windows\kubernetesfunc.ps1'; Get-CACertificates | Out-Null }"
    } else {
        $refreshCommand = "& { . 'C:\AzureData\windows\windowscsehelper.ps1'; . 'C:\AzureData\windows\kubernetesfunc.ps1'; Get-CACertificates -Location '$Location' | Out-Null }"
    }
    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-NoProfile -NonInteractive -ExecutionPolicy Bypass -Command `"$refreshCommand`""
    $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
    $trigger = New-JobTrigger -Daily -At "19:00" -DaysInterval 1
    $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "aks-ca-certs-refresh-task"
    Register-ScheduledTask -TaskName $taskName -InputObject $definition
}

# TODO ksubrmnn parameterize this fully
function Write-KubeClusterConfig {
    param(
        [Parameter(Mandatory = $true)][string]
        $MasterIP,
        [Parameter(Mandatory = $true)][string]
        $KubeDnsServiceIp
    )

    Logs-To-Event -TaskName "AKS.WindowsCSE.WriteKubeClusterConfig" -TaskMessage "Start to write KubeCluster Config. WindowsPauseImageURL: $global:WindowsPauseImageURL"

    $Global:ClusterConfiguration = [PSCustomObject]@{ }

    $Global:ClusterConfiguration | Add-Member -MemberType NoteProperty -Name Cri -Value @{
        Name   = "containerd";
        Images = @{
            # e.g. "mcr.microsoft.com/oss/v2/kubernetes/pause:3.6"
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
        IsSkipCleanupNetwork = $global:IsSkipCleanupNetwork;
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
            SecureTLSBootstrapArgs = @{
                Enabled                = $global:EnableSecureTLSBootstrapping;
                Deadline               = $global:SecureTLSBootstrappingDeadline;
                AADResource            = $global:SecureTLSBootstrappingAADResource;
                UserAssignedIdentityID = $global:SecureTLSBootstrappingUserAssignedIdentityID
            };
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
    Logs-To-Event -TaskName "AKS.WindowsCSE.UpdateDefenderPreferences" -TaskMessage "Start to update defender preferences"

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
    Add-MpPreference -ExclusionPath "C:\k\azurecns\azure-endpoints.json"
    Add-MpPreference -ExclusionPath "C:\k\azure-vnet.log"

    if ($global:EnableCsiProxy) {
        Add-MpPreference -ExclusionProcess "c:\k\csi-proxy.exe"
    }

    Add-MpPreference -ExclusionProcess "c:\program files\containerd\containerd.exe"
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
        $MaxRetryCount = 60
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.CheckAPIServerConnectivity" -TaskMessage "Start to check API server connectivity."
    $retryCount=0
    $lastExceptionMessage=$null

    do {
        $retryString="${retryCount}/${MaxRetryCount}"
        try
        {
            $tcpClient = New-Object Net.Sockets.TcpClient
            $tcpClient.SendTimeout = $ConnectTimeout*1000
            $tcpClient.ReceiveTimeout  = $ConnectTimeout*1000

            Write-Log "Retry ${retryString}: Trying to connect to API server $MasterIP"
            $tcpClient.Connect($MasterIP, 443)
            if ($tcpClient.Connected)
            {
                $tcpClient.Close()
                Write-Log "Retry ${retryString}: Connected to API server successfully"
                return
            }
            $tcpClient.Close()
        } catch [System.AggregateException] {
            Logs-To-Event -TaskName "AKS.WindowsCSE.CheckAPIServerConnectivity" -TaskMessage "Retry ${retryString}: Failed to connect to API server $MasterIP. AggregateException: " + $_.Exception.ToString()
            $lastExceptionMessage = $_.Exception.ToString()
        } catch {
            Logs-To-Event -TaskName "AKS.WindowsCSE.CheckAPIServerConnectivity" -TaskMessage "Retry ${retryString}: Failed to connect to API server $MasterIP. Error: $_"
            $lastExceptionMessage = "$_"
        }

        $retryCount++
        Write-Log "Retry ${retryString}: Sleep $RetryInterval and then retry to connect to API server"
        Sleep $RetryInterval
    } while ($retryCount -lt $MaxRetryCount)

    # Normalize any CR/LF in the exception message to spaces to keep ErrorMessage single-line.
    $lastExceptionMessage = $lastExceptionMessage -replace "(`r|`n)+", " "

    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CHECK_API_SERVER_CONNECTIVITY -ErrorMessage "Failed to connect to API server $MasterIP after $retryCount retries. Last exception: $lastExceptionMessage"
}

function Get-CustomCloudCertEndpointModeFromLocation {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Location
    )

    # ussec/usnat regions still use the legacy certificate endpoint contract.
    $normalizedLocation = $Location.ToLowerInvariant()
    if ($normalizedLocation.StartsWith("ussec") -or $normalizedLocation.StartsWith("usnat")) {
        return "legacy"
    }

    # All other regions use the rcv1p endpoint mode with opt-in gating.
    return "rcv1p"
}

function Should-InstallCACertificatesRefreshTask {
    Param(
        [Parameter(Mandatory = $false)][string]
        $Location = ""
    )

    # When Location is not supplied (older callers), default to legacy mode.
    if ([string]::IsNullOrEmpty($Location)) {
        return $true
    }
    $certEndpointMode = Get-CustomCloudCertEndpointModeFromLocation -Location $Location
    if ($certEndpointMode -eq "legacy") {
        return $true
    }

    try {
        $optInUri = 'http://168.63.129.16/acms/isOptedInForRootCerts'
        $optInResponse = Retry-Command -Command 'Invoke-WebRequest' -Args @{Uri=$optInUri; UseBasicParsing=$true} -Retries 5 -RetryDelaySeconds 10
        return ($optInResponse.Content -match 'IsOptedInForRootCerts=true')
    } catch {
        Write-Log "Skipping CA refresh task registration because IsOptedInForRootCerts could not be determined: $_"
        return $false
    }
}

function Get-CACertificates {
    Param(
        [Parameter(Mandatory = $false)][string]
        $Location = ""
    )

    $caFolder = "C:\ca"
    Create-Directory -FullPath $caFolder -DirectoryUsage "storing CA certificates"

    # When Location is not supplied (older callers), fall back to the legacy endpoint
    # which was the original behavior before the rcv1p changes.
    if ([string]::IsNullOrEmpty($Location)) {
        $certEndpointMode = "legacy"
        Write-Log "Get CA certificates. Location not provided, defaulting to legacy endpoint mode"
    } else {
        $certEndpointMode = Get-CustomCloudCertEndpointModeFromLocation -Location $Location
        Write-Log "Get CA certificates. Location: $Location. EndpointMode: $certEndpointMode"
    }

    try {
        if ($certEndpointMode -eq "legacy") {
            $uri = 'http://168.63.129.16/machine?comp=acmspackage&type=cacertificates&ext=json'
            $rawData = Retry-Command -Command 'Invoke-WebRequest' -Args @{Uri=$uri; UseBasicParsing=$true} -Retries 5 -RetryDelaySeconds 10
            $caCerts = ($rawData.Content) | ConvertFrom-Json
            if ($null -eq $caCerts -or $null -eq $caCerts.Certificates -or $caCerts.Certificates.Length -eq 0) {
                Write-Log "Warning: CA certificates rawdata is empty for legacy endpoint"
                return $false
            }

            foreach ($certificate in $caCerts.Certificates) {
                $name = $certificate.Name
                $certFilePath = Join-Path $caFolder $name
                Write-Log "Write certificate $name to $certFilePath"
                $certificate.CertBody > $certFilePath
            }

            return $true
        }

        $optInUri = 'http://168.63.129.16/acms/isOptedInForRootCerts'
        $optInResponse = Retry-Command -Command 'Invoke-WebRequest' -Args @{Uri=$optInUri; UseBasicParsing=$true} -Retries 5 -RetryDelaySeconds 10
        if (($optInResponse.Content -notmatch 'IsOptedInForRootCerts=true')) {
            Write-Log "Skipping custom cloud root cert installation because IsOptedInForRootCerts is not true"
            return $false
        }

        $operationRequestTypes = @("operationrequestsroot", "operationrequestsintermediate")
        $downloadedAny = $false

        foreach ($requestType in $operationRequestTypes) {
            $operationRequestUri = "http://168.63.129.16/machine?comp=acmspackage&type=$requestType&ext=json"
            $operationResponse = Retry-Command -Command 'Invoke-WebRequest' -Args @{Uri=$operationRequestUri; UseBasicParsing=$true} -Retries 5 -RetryDelaySeconds 10
            $operationJson = ($operationResponse.Content) | ConvertFrom-Json

            if ($null -eq $operationJson -or $null -eq $operationJson.OperationRequests) {
                Write-Log "Warning: no operation requests found for $requestType"
                continue
            }

            foreach ($operation in $operationJson.OperationRequests) {
                $resourceFileName = $operation.ResouceFileName
                if ([string]::IsNullOrEmpty($resourceFileName)) {
                    continue
                }

                $resourceType = [IO.Path]::GetFileNameWithoutExtension($resourceFileName)
                $resourceExt = [IO.Path]::GetExtension($resourceFileName).TrimStart('.')
                $resourceUri = "http://168.63.129.16/machine?comp=acmspackage&type=$resourceType&ext=$resourceExt"

                $certContentResponse = Retry-Command -Command 'Invoke-WebRequest' -Args @{Uri=$resourceUri; UseBasicParsing=$true} -Retries 5 -RetryDelaySeconds 10
                if ([string]::IsNullOrEmpty($certContentResponse.Content)) {
                    Write-Log "Warning: empty certificate content for $resourceFileName"
                    continue
                }

                $certFilePath = Join-Path $caFolder $resourceFileName
                Write-Log "Write certificate $resourceFileName to $certFilePath"
                $certContentResponse.Content > $certFilePath
                $downloadedAny = $true
            }
        }

        if (-not $downloadedAny) {
            Write-Log "Warning: no CA certificates were downloaded in rcv1p mode"
        }

        return $downloadedAny
    }
    catch {
        Write-Log "Warning: failed to retrieve CA certificates. Error: $_"
        return $false
    }
}
