function Install-VnetPlugins
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $AzureCNIConfDir,
        [Parameter(Mandatory=$true)][string]
        $AzureCNIBinDir,
        [Parameter(Mandatory=$true)][string]
        $VNetCNIPluginsURL
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.InstallVnetPlugins" -TaskMessage "Start to install Azure VNet plugins. VnetCNIPluginsURL: $global:VNetCNIPluginsURL"

    # Create CNI directories.
    Create-Directory -FullPath $AzureCNIBinDir -DirectoryUsage "storing Azure CNI binaries"
    Create-Directory -FullPath $AzureCNIConfDir -DirectoryUsage "storing Azure CNI configuration"

    # Download Azure VNET CNI plugins.
    # Mirror from https://github.com/Azure/azure-container-networking/releases
    $zipfile =  [Io.path]::Combine("$AzureCNIDir", "azure-vnet.zip")
    DownloadFileOverHttp -Url $VNetCNIPluginsURL -DestinationPath $zipfile -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_CNI_PACKAGE
    Expand-Archive -path $zipfile -DestinationPath $AzureCNIBinDir
    del $zipfile

    # Windows does not need a separate CNI loopback plugin because the Windows
    # kernel automatically creates a loopback interface for each network namespace.
    # Copy CNI network config file and set bridge mode.
    move $AzureCNIBinDir/*.conflist $AzureCNIConfDir
}

function Set-AzureCNIConfig
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $AzureCNIConfDir,
        [Parameter(Mandatory=$true)][string]
        $KubeDnsSearchPath,
        [Parameter(Mandatory=$true)][string]
        $KubeClusterCIDR,
        [Parameter(Mandatory=$true)][string]
        $KubeServiceCIDR,
        [Parameter(Mandatory=$true)][string]
        $VNetCIDR,
        [Parameter(Mandatory=$true)][bool]
        $IsDualStackEnabled,
        [Parameter(Mandatory=$false)][bool]
        $IsAzureCNIOverlayEnabled
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.SetAzureCNIConfig" -TaskMessage "Start to set Azure CNI config. IsDualStackEnabled: $global:IsDualStackEnabled, IsAzureCNIOverlayEnabled: $global:IsAzureCNIOverlayEnabled, IsDisableWindowsOutboundNat: $global:IsDisableWindowsOutboundNat"

    $fileName  = [Io.path]::Combine("$AzureCNIConfDir", "10-azure.conflist")
    $configJson = Get-Content $fileName | ConvertFrom-Json
    $configJson.plugins.dns.Nameservers[0] = $KubeDnsServiceIp
    $configJson.plugins.dns.Search[0] = $KubeDnsSearchPath

    if ($global:IsDisableWindowsOutboundNat) {
        # Replace OutBoundNAT with LoopbackDSR for IMDS acess if AKS cluster disabled Windows OutBoundNAT.
        # The Azure Instance Metadata Service (IMDS) provides information about currently running virtual machine instances.
        # IMDS is a REST API that's available at a well-known, non-routable IP address (169.254.169.254)
        # Details: https://docs.microsoft.com/en-us/azure/virtual-machines/windows/instance-metadata-service?tabs=windows#known-issues-and-faq
        $valueObj = [PSCustomObject]@{
            Type = 'LoopbackDSR'
            IPAddress = '169.254.169.254'
        }
        $jsonContent = [PSCustomObject]@{
            Name = 'EndpointPolicy'
            Value = $valueObj
        }

        # $configJson.plugins[0].AdditionalArgs[0] is OutboundNAT.
        Write-Log "Replace OutBoundNAT with LoopbackDSR for IMDS acess."
        $configJson.plugins[0].AdditionalArgs[0] = $jsonContent

        # Update the corresponding system regkey for DisableWindowsOutboundNat feature.
        $osVersion = Get-WindowsVersion
        if ($osVersion -eq "1809"){
            Write-Log "Update RegKey to disable the incompatible HNSControlFlag (0x10) for feature DisableWindowsOutboundNat"
            $hnsControlFlag=0x10
            $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag -ErrorAction Ignore)
            if (![string]::IsNullOrEmpty($currentValue)) {
                Write-Log "The current value of HNSControlFlag is $currentValue"
                # Set the bit to 0 if the bit is 1
                if ([int]$currentValue.HNSControlFlag -band $hnsControlFlag) {
                    $hnsControlFlag=([int]$currentValue.HNSControlFlag -bxor $hnsControlFlag)
                    Write-Log "HNSControlFlag is updated to $hnsControlFlag to clear the bit 0x10"
                    Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag -Type DWORD -Value $hnsControlFlag
                }
            } else {
                # Set 0 to disable all features under HNSControlFlag (0x10 defaults enable)
                Write-Log "HNSControlFlag is set to 0 to clear the bit 0x10"
                Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag -Type DWORD -Value 0
            }
        } elseif ($osVersion -eq "ltsc2022") {
            Write-Log "SourcePortPreservationForHostPort is set to 0"
            Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name SourcePortPreservationForHostPort -Type DWORD -Value 0
        }
        # Restart hns service if it is exsting and running, to make the system regkey change effective.
        $hnsServiceName = 'hns'
        $hnsService = Get-Service -Name $hnsServiceName -ErrorAction SilentlyContinue
        if ($hnsService -and $hnsService.Status -eq 'Running') {
            Write-Log "hns service is already running. Restart hns."
            Restart-Service -Name $hnsServiceName
        }
    } else {
        # Fill in DNS information for kubernetes.
        $exceptionAddresses = @()
        if ($IsDualStackEnabled) {
            $subnetsToPass = $KubeClusterCIDR -split ","
            foreach ($subnet in $subnetsToPass) {
                $exceptionAddresses += $subnet
            }
        } else {
            $exceptionAddresses += $KubeClusterCIDR
        }

        if (!$IsAzureCNIOverlayEnabled) {
            $vnetCIDRs = $VNetCIDR -split ","
            foreach ($cidr in $vnetCIDRs) {
                $exceptionAddresses += $cidr
            }
        }

        $osVersion = Get-WindowsVersion
        if ($osVersion -eq "1809"){
            # In WS2019 and below rules in the exception list are generated by dropping the prefix lenght and removing duplicate rules.
            # If multiple execptions are specified with different ranges we should only include the broadest range for each address.
            # This issue has been addressed in 19h1+ builds

            $processedExceptions = GetBroadestRangesForEachAddress $exceptionAddresses
            Write-Log "Filtering CNI config exception list values to work around WS2019 issue processing rules. Original exception list: $exceptionAddresses, processed exception list: $processedExceptions"
            $configJson.plugins.AdditionalArgs[0].Value.ExceptionList = $processedExceptions
        } else {
            if ($IsDualStackEnabled) {
                $ipv4Cidrs = @()
                $ipv6Cidrs = @()
                foreach ($cidr in $exceptionAddresses) {
                    # this is the pwsh way of strings.Count(s, ":") >= 2
                    if (($cidr -split ":").Count -ge 3) {
                        $ipv6Cidrs += $cidr
                    } else {
                        $ipv4Cidrs += $cidr
                    }
                }

                # we just assume the first entry in additional Args is the exception
                # list for IPv4 and then append a new EnpointPolicy for IPv6. We
                # probably shouldn't hard code the first one like this and just build
                # 2 EndpointPolicies and append to the AdditionalArgs.
                $configJson.plugins.AdditionalArgs[0].Value.ExceptionList = $ipv4Cidrs

                $outboundException = [PSCustomObject]@{
                    Name = 'EndpointPolicy'
                    Value = [PSCustomObject]@{
                        Type = 'OutBoundNAT'
                        ExceptionList = $ipv6Cidrs
                    }
                }
                $configJson.plugins[0].AdditionalArgs += $outboundException
            } else {
                $configJson.plugins.AdditionalArgs[0].Value.ExceptionList = $exceptionAddresses
            }
        }
    }

    if ($global:KubeproxyFeatureGates.Contains("WinDSR=true")) {
        Write-Log "Setting enableLoopbackDSR in Azure CNI conflist for WinDSR"
        # Add {enableLoopbackDSR:true} if windowsSettings exists, otherwise, add {windowsSettings:{enableLoopbackDSR:true}}
        if (Get-Member -InputObject $configJson.plugins[0] -name "windowsSettings" -Membertype Properties) {
            $configJson.plugins[0].windowsSettings | Add-Member -Name "enableLoopbackDSR" -Value $True -MemberType NoteProperty
        } else {
            $jsonContent = [PSCustomObject]@{
                'enableLoopbackDSR' = $True
            }
            $configJson.plugins[0] | Add-Member -Name "windowsSettings" -Value $jsonContent -MemberType NoteProperty
        }

        # $configJson.plugins[0].AdditionalArgs[1] is ROUTE. Remove ROUTE if WinDSR is enabled.
        $configJson.plugins[0].AdditionalArgs = @($configJson.plugins[0].AdditionalArgs | Where-Object { $_ -ne $configJson.plugins[0].AdditionalArgs[1] })
    } else {
        if ($IsDualStackEnabled){
            $configJson.plugins[0]|Add-Member -Name "ipv6Mode" -Value "ipv6nat" -MemberType NoteProperty
            $serviceCidr = $KubeServiceCIDR -split ","
            $configJson.plugins[0].AdditionalArgs[1].Value.DestinationPrefix = $serviceCidr[0]
            $valueObj = [PSCustomObject]@{
                Type = 'ROUTE'
                DestinationPrefix = $serviceCidr[1]
                NeedEncap = $True
            }

            $jsonContent = [PSCustomObject]@{
                Name = 'EndpointPolicy'
                Value = $valueObj
            }
            $configJson.plugins[0].AdditionalArgs += $jsonContent
        }
        else {
            $configJson.plugins[0].AdditionalArgs[1].Value.DestinationPrefix = $KubeServiceCIDR
        }
    }

    $aclRule1 = [PSCustomObject]@{
        Type = 'ACL'
        Protocols = '6'
        Action = 'Block'
        Direction = 'Out'
        RemoteAddresses = '168.63.129.16/32'
        RemotePorts = '80'
        Priority = 200
        RuleType = 'Switch'
    }
    $aclRule2 = [PSCustomObject]@{
        Type = 'ACL'
        Protocols = '6'
        Action = 'Block'
        Direction = 'Out'
        RemoteAddresses = '168.63.129.16/32'
        RemotePorts = '32526'
        Priority = 200
        RuleType = 'Switch'
    }
    $aclRule3 = [PSCustomObject]@{
        Type = 'ACL'
        Action = 'Allow'
        Direction = 'In'
        Priority = 65500
    }
    $aclRule4 = [PSCustomObject]@{
        Type = 'ACL'
        Action = 'Allow'
        Direction = 'Out'
        Priority = 65500
    }
    $jsonContent = [PSCustomObject]@{
        Name = 'EndpointPolicy'
        Value = $aclRule1
    }
    $configJson.plugins[0].AdditionalArgs += $jsonContent
    $jsonContent = [PSCustomObject]@{
        Name = 'EndpointPolicy'
        Value = $aclRule2
    }
    $configJson.plugins[0].AdditionalArgs += $jsonContent
    $jsonContent = [PSCustomObject]@{
        Name = 'EndpointPolicy'
        Value = $aclRule3
    }
    $configJson.plugins[0].AdditionalArgs += $jsonContent
    $jsonContent = [PSCustomObject]@{
        Name = 'EndpointPolicy'
        Value = $aclRule4
    }
    $configJson.plugins[0].AdditionalArgs += $jsonContent

    $configJson | ConvertTo-Json -depth 20 | Out-File -encoding ASCII -filepath $fileName
}

function GetBroadestRangesForEachAddress{
    param([string[]] $values)

    # Create a map of range values to IP addresses
    $map = @{}

    foreach ($value in $Values) {
        if ($value -match '([0-9\.]+)\/([0-9]+)') {
            if (!$map.contains($matches[1])) {
                $map.Add($matches[1], @())
            }

            $map[$matches[1]] += [int]$matches[2]
        }
    }

    # For each IP address select the range with the lagest scope (smallest value)
    $returnValues = @()
    foreach ($ip in $map.Keys) {
        $range = $map[$ip] | Sort-Object | Select-Object -First 1

        $returnValues += $ip + "/" + $range
    }

    # prefix $returnValues with common to ensure single values get returned as an array otherwise invalid json may be generated
    return ,$returnValues
}

function GetSubnetPrefix
{
    Param(
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $Token,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $SubnetId,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $ResourceManagerEndpoint,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $NetworkAPIVersion
    )

    $uri = "$($ResourceManagerEndpoint)$($SubnetId)?api-version=$NetworkAPIVersion"
    $headers = @{Authorization="Bearer $Token"}

    try {
        $response = Retry-Command -Command "Invoke-RestMethod" -Args @{Uri=$uri; Method="Get"; ContentType="application/json"; Headers=$headers} -Retries 5 -RetryDelaySeconds 10
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GET_SUBNET_PREFIX -ErrorMessage "Error getting subnet prefix. Error: $_"
    }

    $response.properties.addressPrefix
}

function GenerateAzureStackCNIConfig
{
    Param(
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $TenantId,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $SubscriptionId,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $AADClientId,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $AADClientSecret,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $ResourceGroup,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $NetworkAPIVersion,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $AzureEnvironmentFilePath,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $IdentitySystem,
        [Parameter(Mandatory=$true)][ValidateNotNullOrEmpty()][string] $KubeDir

    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.GenerateAzureStackCNIConfig" -TaskMessage "Start to generate Azure Stack CNI config"

    $networkInterfacesFile = "$KubeDir\network-interfaces.json"
    $azureCNIConfigFile = "$KubeDir\interfaces.json"
    $azureEnvironment = Get-Content $AzureEnvironmentFilePath | ConvertFrom-Json

    Write-Log "------------------------------------------------------------------------"
    Write-Log "Parameters"
    Write-Log "------------------------------------------------------------------------"
    Write-Log "TenantId:                  $TenantId"
    Write-Log "SubscriptionId:            $SubscriptionId"
    Write-Log "AADClientId:               ..."
    Write-Log "AADClientSecret:           ..."
    Write-Log "ResourceGroup:             $ResourceGroup"
    Write-Log "NetworkAPIVersion:         $NetworkAPIVersion"
    Write-Log "ServiceManagementEndpoint: $($azureEnvironment.serviceManagementEndpoint)"
    Write-Log "ActiveDirectoryEndpoint:   $($azureEnvironment.activeDirectoryEndpoint)"
    Write-Log "ResourceManagerEndpoint:   $($azureEnvironment.resourceManagerEndpoint)"
    Write-Log "------------------------------------------------------------------------"
    Write-Log "Variables"
    Write-Log "------------------------------------------------------------------------"
    Write-Log "azureCNIConfigFile: $azureCNIConfigFile"
    Write-Log "networkInterfacesFile: $networkInterfacesFile"
    Write-Log "------------------------------------------------------------------------"

    Write-Log "Generating token for Azure Resource Manager"

    $tokenURL = ""
    if($IdentitySystem -ieq "adfs") {
        $tokenURL = "$($azureEnvironment.activeDirectoryEndpoint)adfs/oauth2/token"
    } else {
        $tokenURL = "$($azureEnvironment.activeDirectoryEndpoint)$TenantId/oauth2/token"
    }

    Add-Type -AssemblyName System.Web
    $encodedSecret = [System.Web.HttpUtility]::UrlEncode($AADClientSecret)

    $body = "grant_type=client_credentials&client_id=$AADClientId&client_secret=$encodedSecret&resource=$($azureEnvironment.serviceManagementEndpoint)"
    $args = @{Uri=$tokenURL; Method="Post"; Body=$body; ContentType='application/x-www-form-urlencoded'}
    try {
        $tokenResponse = Retry-Command -Command "Invoke-RestMethod" -Args $args -Retries 5 -RetryDelaySeconds 10
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GENERATE_TOKEN_FOR_ARM -ErrorMessage "Error generating token for Azure Resource Manager. Error: $_"
    }

    $token = $tokenResponse | Select-Object -ExpandProperty access_token

    Write-Log "Fetching network interface configuration for node"

    $interfacesUri = "$($azureEnvironment.resourceManagerEndpoint)subscriptions/$SubscriptionId/resourceGroups/$ResourceGroup/providers/Microsoft.Network/networkInterfaces?api-version=$NetworkAPIVersion"
    $headers = @{Authorization="Bearer $token"}
    $args = @{Uri=$interfacesUri; Method="Get"; ContentType="application/json"; Headers=$headers; OutFile=$networkInterfacesFile}
    try {
        Retry-Command -Command "Invoke-RestMethod" -Args $args -Retries 5 -RetryDelaySeconds 10
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_NETWORK_INTERFACES_NOT_EXIST -ErrorMessage "Error fetching network interface configuration for node. Error: $_"
    }

    Write-Log "Generating Azure CNI interface file"

    $localNics = Get-NetAdapter | Select-Object -ExpandProperty MacAddress | ForEach-Object {$_ -replace "-",""}

    $sdnNics = Get-Content $networkInterfacesFile `
        | ConvertFrom-Json `
        | Select-Object -ExpandProperty value `
        | Where-Object { $localNics.Contains($_.properties.macAddress) } `
        | Where-Object { $_.properties.ipConfigurations.Count -gt 0}

    $interfaces = @{
        Interfaces = @( $sdnNics | ForEach-Object { @{
            MacAddress = $_.properties.macAddress
            IsPrimary = $_.properties.primary
            IPSubnets = @(@{
                Prefix = GetSubnetPrefix `
                            -Token $token `
                            -SubnetId $_.properties.ipConfigurations[0].properties.subnet.id `
                            -NetworkAPIVersion $NetworkAPIVersion `
                            -ResourceManagerEndpoint $($azureEnvironment.resourceManagerEndpoint)
                IPAddresses = $_.properties.ipConfigurations | ForEach-Object { @{
                    Address = $_.properties.privateIPAddress
                    IsPrimary = $_.properties.primary
                }}
            })
        }})
    }

    ConvertTo-Json $interfaces -Depth 6 | Out-File -FilePath $azureCNIConfigFile -Encoding ascii

    Set-ItemProperty -Path $azureCNIConfigFile -Name IsReadOnly -Value $true
}

function New-ExternalHnsNetwork
{
    param (
        [Parameter(Mandatory=$true)][bool]
        $IsDualStackEnabled
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.NewExternalHnsNetwork" -TaskMessage "Start to create new external hns network"

    Write-Log "Creating new HNS network `"ext`""
    $externalNetwork = "ext"
    $nas = @(Get-NetAdapter -Physical)
    $nodeIPs = @()

    if ($nas.Count -eq 0) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_NETWORK_ADAPTER_NOT_EXIST -ErrorMessage "Failed to find any physical network adapters"
    }

    # If there is more than one adapter, use the first adapter that is assigned an ipaddress.
    foreach($na in $nas)
    {
        $netIP = Get-NetIPAddress -ifIndex $na.ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue -ErrorVariable netIPErr
        if ($netIP)
        {
            $managementIP = $netIP.IPAddress
            $adapterName = $na.Name

            Write-Log "Get node IPv4 address assigned to the adapter $($na.Name): $($managementIP)"
            $nodeIPs += $managementIP

            if ($IsDualStackEnabled) {
                $netIPv6s = Get-NetIPAddress -ifIndex $na.ifIndex -AddressFamily IPv6 -ErrorAction SilentlyContinue -ErrorVariable netIPErr
                foreach($ipv6 in $netIPv6s)
                {
                    # On an Azure Windows VM, there are two IPv6 IP addresses. Below is an example. It is same in an Azure Linux VM.
                    # ifIndex IPAddress                                       PrefixLength PrefixOrigin SuffixOrigin AddressState PolicyStore
                    # ------- ---------                                       ------------ ------------ ------------ ------------ -----------
                    # 6       fe80::97bd:baf7:2853:f73d%6                               64 WellKnown    Link         Preferred    ActiveStore
                    # 6       2404:f800:8000:122::4                                    128 Dhcp         Dhcp         Preferred    ActiveStore
                    #
                    # From the found docuements. fe80: with WellKnown is the link-local address so we should ignore it.
                    # IPv6 link-local is a special type of unicast address that is auto-configured on any interface using a combination of 
                    # the link-local prefix FE80::/10 (first 10 bits equal to 1111 1110 10) and the MAC address of the interface.
                    #
                    # https://learn.microsoft.com/en-us/dotnet/api/system.net.networkinformation.prefixorigin?view=net-8.0
                    # WellKnown | 2 | The prefix is a well-known prefix. Well-known prefixes are specified in standard-track Request for 
                    # Comments (RFC) documents and assigned by the Internet Assigned Numbers Authority (Iana) or an address registry. Such 
                    # prefixes are reserved for special purposes. -- | -- | --
                    if ($ipv6.PrefixOrigin -ne "WellKnown")
                    {
                        Write-Log "Get node IPv6 address assigned to the adapter $($na.Name): $($ipv6.IPAddress)"
                        $nodeIPs += $ipv6.IPAddress
                    }
                }
            }

            break
        }
        else {
            Write-Log "No IPv4 found on the network adapter $($na.Name); trying the next adapter ..."
            if ($netIPErr) {
                Write-Log "error when retrieving IPAddress: $netIPErr"
                $netIPErr.Clear()
            }
        }
    }

    if(-Not $managementIP)
    {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_NOT_FOUND_MANAGEMENT_IP -ErrorMessage "None of the physical network adapters has an IP address"
    }

    # https://github.com/kubernetes/kubernetes/pull/121028
    if (([version]$global:KubeBinariesVersion).CompareTo([version]("1.29.0")) -ge 0) {
        Logs-To-Event -TaskName "AKS.WindowsCSE.UpdateKubeClusterConfig" -TaskMessage "Start to update KubeCluster Config. NodeIPs: $nodeIPs"

        # It should always get ipv4 address. Otherwise, it will throw WINDOWS_CSE_ERROR_NOT_FOUND_MANAGEMENT_IP
        if ($IsDualStackEnabled -and $nodeIPs.Count -eq 1) {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GET_NODE_IPV6_IP -ErrorMessage "Failed to get node IPv6 IP address"
        }

        try {
            $clusterConfiguration = ConvertFrom-Json ((Get-Content $global:KubeClusterConfigPath -ErrorAction Stop) | Out-String)
            $clusterConfiguration.Kubernetes.Kubelet.ConfigArgs += "--node-ip=$($nodeIPs -join ',')"
            $clusterConfiguration | ConvertTo-Json -Depth 10 | Out-File -FilePath $global:KubeClusterConfigPath
        } catch {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_UPDATING_KUBE_CLUSTER_CONFIG -ErrorMessage "Failed in updating kube cluster config. Error: $_"
        }
    }

    Write-Log "Using adapter $adapterName with IP address $managementIP"
    $mgmtIPAfterNetworkCreate

    $stopWatch = New-Object System.Diagnostics.Stopwatch
    $stopWatch.Start()

    # Fixme : use a smallest range possible, that will not collide with any pod space
    if ($IsDualStackEnabled) {
        New-HNSNetwork -Type $global:NetworkMode -AddressPrefix @("192.168.255.0/30","192:168:255::0/127") -Gateway @("192.168.255.1","192:168:255::1") -AdapterName $adapterName -Name $externalNetwork -Verbose
    }
    else {
        New-HNSNetwork -Type $global:NetworkMode -AddressPrefix "192.168.255.0/30" -Gateway "192.168.255.1" -AdapterName $adapterName -Name $externalNetwork -Verbose
    }
    # Wait for the switch to be created and the ip address to be assigned.
    for ($i = 0; $i -lt 60; $i++) {
        $mgmtIPAfterNetworkCreate = Get-NetIPAddress $managementIP -ErrorAction SilentlyContinue
        if ($mgmtIPAfterNetworkCreate) {
            break
        }
        Start-Sleep -Milliseconds 500
    }

    $stopWatch.Stop()
    if (-not $mgmtIPAfterNetworkCreate) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_MANAGEMENT_IP_NOT_EXIST -ErrorMessage "Failed to find $managementIP after creating $externalNetwork network"
    }
    Write-Log "It took $($StopWatch.Elapsed.Seconds) seconds to create the $externalNetwork network."

    Write-Log "Log network adapter info after creating $externalNetwork network"
    Get-NetIPConfiguration -AllCompartments -ErrorAction Ignore

    $dnsServers=Get-DnsClientServerAddress -ErrorAction Ignore
    if ($dnsServers) {
        Write-Log "DNS Servers are: $($dnsServers.ServerAddresses)"
    }
}

function Get-HnsPsm1
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $HNSModule
    )
    Logs-To-Event "ASK.WindowsCSE.GetAndImportHNSModule" -TaskMessage "Start to get and import hns module. NetworkPlugin: $global:NetworkPlugin"

    # HNSModule is C:\k\hns.v2.psm1 when container runtime is Containerd
    $fileName = [IO.Path]::GetFileName($HNSModule)
    # Get-LogCollectionScripts will copy hns module file to C:\k\debug
    $sourceFile = [IO.Path]::Combine('C:\k\debug\', $fileName)
    try {
        Write-Log "Copying $sourceFile to $HNSModule."
        Copy-Item -Path $sourceFile -Destination "$HNSModule"
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_HNS_MODULE -ErrorMessage "Failed to copy $sourceFile to $HNSModule. Error: $_"
    }
}
