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

        # $configJson.plugins[0].AdditionalArgs[0] is OutboundNAT. Replace OutBoundNAT with LoopbackDSR for IMDS.
        $configJson.plugins[0].AdditionalArgs[0] = $jsonContent

        # TODO: Remove it after Windows OS fixes the issue.
        Write-Log "Update RegKey to disable the incompatible HNSControlFlag (0x10) for feature DisableWindowsOutboundNat"
        $hnsControlFlag=0x10
        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of HNSControlFlag is $currentValue"
            # -band (-bnot $hnsControlFlag) set the bit to 0 if the bit is 1
            $hnsControlFlag=([int]$currentValue.HNSControlFlag -band (-bnot $hnsControlFlag))
            Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag -Type DWORD -Value $hnsControlFlag
        } else {
            Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag -Type DWORD -Value 0
        }
    } else {
        # Fill in DNS information for kubernetes.
        $exceptionAddresses = @()
        if (!$IsAzureCNIOverlayEnabled) {
            if ($IsDualStackEnabled){
                $subnetToPass = $KubeClusterCIDR -split ","
                $exceptionAddresses += $subnetToPass[0]
            } else {
                $exceptionAddresses += $KubeClusterCIDR
            }
        }
        $vnetCIDRs = $VNetCIDR -split ","
        foreach ($cidr in $vnetCIDRs) {
            $exceptionAddresses += $cidr
        }

        $osBuildNumber = (get-wmiobject win32_operatingsystem).BuildNumber
        if ($osBuildNumber -le 17763){
            # In WS2019 and below rules in the exception list are generated by dropping the prefix lenght and removing duplicate rules.
            # If multiple execptions are specified with different ranges we should only include the broadest range for each address.
            # This issue has been addressed in 19h1+ builds

            $processedExceptions = GetBroadestRangesForEachAddress $exceptionAddresses
            Write-Log "Filtering CNI config exception list values to work around WS2019 issue processing rules. Original exception list: $exceptionAddresses, processed exception list: $processedExceptions"
            $configJson.plugins.AdditionalArgs[0].Value.ExceptionList = $processedExceptions
        }
        else {
            $configJson.plugins.AdditionalArgs[0].Value.ExceptionList = $exceptionAddresses
        }
    }

    if ($global:KubeproxyFeatureGates.Contains("WinDSR=true")) {
        Write-Log "Setting enableLoopbackDSR in Azure CNI conflist for WinDSR"
        $jsonContent = [PSCustomObject]@{
            'enableLoopbackDSR' = $True
        }
        $configJson.plugins[0]|Add-Member -Name "windowsSettings" -Value $jsonContent -MemberType NoteProperty

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
        Action = 'Allow'
        Direction = 'In'
        Priority = 65500
    }
    $aclRule3 = [PSCustomObject]@{
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

    Write-Log "Creating new HNS network `"ext`""
    $externalNetwork = "ext"
    $nas = @(Get-NetAdapter -Physical)

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
}

function Get-HnsPsm1
{
    Param(
        [string]
        $HnsUrl = "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/",
        [Parameter(Mandatory=$true)][string]
        $HNSModule
    )

    # HNSModule is C:\k\hns.psm1 when container runtime is Docker
    # HNSModule is C:\k\hns.v2.psm1 when container runtime is Containerd
    $fileName = [IO.Path]::GetFileName($HNSModule)
    $HnsUrl = [IO.Path]::Combine($HnsUrl, $fileName)
    DownloadFileOverHttp -Url $HnsUrl -DestinationPath "$HNSModule" -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_HNS_MODULE
}
