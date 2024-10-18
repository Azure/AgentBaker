function Write-AzureConfig {
    Param(
        [Parameter(Mandatory = $true)][string]
        $AADClientId,
        [Parameter(Mandatory = $true)][string]
        $AADClientSecret,
        [Parameter(Mandatory = $true)][string]
        $TenantId,
        [Parameter(Mandatory = $true)][string]
        $SubscriptionId,
        [Parameter(Mandatory = $true)][string]
        $ResourceGroup,
        [Parameter(Mandatory = $true)][string]
        $Location,
        [Parameter(Mandatory = $true)][string]
        $VmType,
        [Parameter(Mandatory = $true)][string]
        $SubnetName,
        [Parameter(Mandatory = $true)][string]
        $SecurityGroupName,
        [Parameter(Mandatory = $true)][string]
        $VNetName,
        [Parameter(Mandatory = $true)][string]
        $RouteTableName,
        [Parameter(Mandatory = $false)][string] # Need one of these configured
        $PrimaryAvailabilitySetName,
        [Parameter(Mandatory = $false)][string] # Need one of these configured
        $PrimaryScaleSetName,
        [Parameter(Mandatory = $true)][string]
        $UseManagedIdentityExtension,
        [string]
        $UserAssignedClientID,
        [Parameter(Mandatory = $true)][string]
        $UseInstanceMetadata,
        [Parameter(Mandatory = $true)][string]
        $LoadBalancerSku,
        [Parameter(Mandatory = $true)][string]
        $ExcludeMasterFromStandardLB,
        [Parameter(Mandatory = $true)][string]
        $KubeDir,
        [Parameter(Mandatory = $true)][string]
        $TargetEnvironment,
        [Parameter(Mandatory = $false)][bool]
        $UseContainerD = $false
    )

    Logs-To-Event -TaskName "AKS.WindowsCSE.WriteAzureCloudProviderConfig" -TaskMessage "Start to write Azure Cloud Provider Config"

    if ( $VmType -eq "vmss" -And -Not $PrimaryAvailabilitySetName -And -Not $PrimaryScaleSetName ) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INVALID_PARAMETER_IN_AZURE_CONFIG -ErrorMessage "Either PrimaryAvailabilitySetName or PrimaryScaleSetName must be set"
    }

    $azureConfigFile = [io.path]::Combine($KubeDir, "azure.json")

    $azureConfig = @"
{
    "cloud": "$TargetEnvironment",
    "tenantId": "$TenantId",
    "subscriptionId": "$SubscriptionId",
    "aadClientId": "$AADClientId",
    "aadClientSecret": "$AADClientSecret",
    "resourceGroup": "$ResourceGroup",
    "location": "$Location",
    "vmType": "$VmType",
    "subnetName": "$SubnetName",
    "securityGroupName": "$SecurityGroupName",
    "vnetName": "$VNetName",
    "routeTableName": "$RouteTableName",
    "primaryAvailabilitySetName": "$PrimaryAvailabilitySetName",
    "primaryScaleSetName": "$PrimaryScaleSetName",
    "useManagedIdentityExtension": $UseManagedIdentityExtension,
    "userAssignedIdentityID": "$UserAssignedClientID",
    "useInstanceMetadata": $UseInstanceMetadata,
    "loadBalancerSku": "$LoadBalancerSku",
    "excludeMasterFromStandardLB": $ExcludeMasterFromStandardLB
}
"@

    $azureConfig | Out-File -encoding ASCII -filepath "$azureConfigFile"
}


function Write-CACert {
    Param(
        [Parameter(Mandatory = $true)][string]
        $CACertificate,
        [Parameter(Mandatory = $true)][string]
        $KubeDir
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.WriteCACert" -TaskMessage "Start to write ca root"
    $caFile = [io.path]::Combine($KubeDir, "ca.crt")
    [System.Text.Encoding]::ASCII.GetString([System.Convert]::FromBase64String($CACertificate)) | Out-File -Encoding ascii $caFile
}

function Write-KubeConfig {
    Param(
        [Parameter(Mandatory = $true)][string]
        $CACertificate,
        [Parameter(Mandatory = $true)][string]
        $MasterFQDNPrefix,
        [Parameter(Mandatory = $true)][string]
        $MasterIP,
        [Parameter(Mandatory = $true)][string]
        $AgentKey,
        [Parameter(Mandatory = $true)][string]
        $AgentCertificate,
        [Parameter(Mandatory = $true)][string]
        $KubeDir
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.WriteKubeConfig" -TaskMessage "Start to write kube config"

    $kubeConfigFile = [io.path]::Combine($KubeDir, "config")

    $kubeConfig = @"
---
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: "$CACertificate"
    server: https://${MasterIP}:443
  name: "$MasterFQDNPrefix"
contexts:
- context:
    cluster: "$MasterFQDNPrefix"
    user: "$MasterFQDNPrefix-admin"
  name: "$MasterFQDNPrefix"
current-context: "$MasterFQDNPrefix"
kind: Config
users:
- name: "$MasterFQDNPrefix-admin"
  user:
    client-certificate-data: "$AgentCertificate"
    client-key-data: "$AgentKey"
"@

    $kubeConfig | Out-File -encoding ASCII -filepath "$kubeConfigFile"
}

function Write-BootstrapKubeConfig {
    Param(
        [Parameter(Mandatory = $true)][string]
        $CACertificate,
        [Parameter(Mandatory = $true)][string]
        $MasterFQDNPrefix,
        [Parameter(Mandatory = $true)][string]
        $MasterIP,
        [Parameter(Mandatory = $true)][string]
        $TLSBootstrapToken,
        [Parameter(Mandatory = $true)][string]
        $KubeDir
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.WriteBootstrapKubeConfig" -TaskMessage "Start to write TLS bootstrap kubeconfig"

    $bootstrapKubeConfigFile = [io.path]::Combine($KubeDir, "bootstrap-config")

    $bootstrapKubeConfig = @"
---
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: "$CACertificate"
    server: https://${MasterIP}:443
  name: "$MasterFQDNPrefix"
contexts:
- context:
    cluster: "$MasterFQDNPrefix"
    user: "kubelet-bootstrap"
  name: "$MasterFQDNPrefix"
current-context: "$MasterFQDNPrefix"
kind: Config
users:
- name: "kubelet-bootstrap"
  user:
    token: "$TLSBootstrapToken"
"@

    $bootstrapKubeConfig | Out-File -encoding ASCII -filepath "$bootstrapKubeConfigFIle"
}

function Get-KubePackage {
    Param(
        [Parameter(Mandatory = $true)][string]
        $KubeBinariesSASURL
    )
    $mappingFile = [Io.path]::Combine($global:CacheDir, "private-packages\mapping.json")
    if (Test-Path $mappingFile) {
        $urls = @{}
        (ConvertFrom-Json ((Get-Content $mappingFile -ErrorAction Stop) | Out-String)).psobject.properties | Foreach { $urls[$_.Name] = $_.Value }
        if ($urls.ContainsKey($global:KubeBinariesVersion)) {
            Write-Log "Found $global:KubeBinariesVersion in $mappingFile"
            $KubeBinariesSASURL = $urls[$global:KubeBinariesVersion]
        } else {
            Write-Log "Did not find $global:KubeBinariesVersion in $mappingFile"
        }
    }
    Logs-To-Event -TaskName "AKS.WindowsCSE.DownloadKubletBinaries" -TaskMessage "Start to download kubelet binaries and unzip. KubeBinariesPackageSASURL: $KubeBinariesSASURL"

    $zipfile = "c:\k.zip"
    for ($i = 0; $i -le 10; $i++) {
        DownloadFileOverHttp -Url $KubeBinariesSASURL -DestinationPath $zipfile -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_KUBERNETES_PACKAGE
        if ($?) {
            break
        }
        else {
            Write-Log $Error[0].Exception.Message
        }
    }
    Expand-Archive -path $zipfile -DestinationPath C:\
    Remove-Item $zipfile
}

function Remove-KubeletNodeLabel {
    Param(
        [Parameter(Mandatory=$true)][string]
        $KubeletNodeLabels,
        [Parameter(Mandatory=$true)][string]
        $Label
    )

    $labelList = $KubeletNodeLabels -split ","
    $filtered = $labelList | Where-Object { $_ -ne $Label }
    return $filtered -join ","
}

function Get-TagValue {
    Param(
        [Parameter(Mandatory=$true)][string]
        $TagName,
        [Parameter(Mandatory=$true)][string]
        $DefaultValue
    )

    $uri = "http://169.254.169.254/metadata/instance?api-version=2021-02-01"
    try {
        $response = Retry-Command -Command "Invoke-RestMethod" -Args @{Uri=$uri; Method="Get"; ContentType="application/json"; Headers=@{"Metadata"="true"}} -Retries 3 -RetryDelaySeconds 5
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_LOOKUP_INSTANCE_DATA_TAG -ErrorMessage "Unable to lookup VM tag `"$TagName`" from IMDS instance data"
    }

    $tag = $response.compute.tagsList | Where-Object { $_.name -eq $TagName }
    if (!$tag) {
        return $DefaultValue
    }
    return $tag.value
}

# Note: this function modifies global kubelet config args and node labels. Thus, it MUST
# be called before Write-KubeClusterConfig, and any other function that relies on the values of
# kubelet config args and node labels.
function Disable-KubeletServingCertificateRotationForTags {
    Logs-To-Event `
        -TaskName "AKS.WindowsCSE.DisableKubeletServingCertificateRotationForTags" `
        -TaskMessage "EnableKubeletServingCertificateRotation: $global:EnableKubeletServingCertificateRotation. Check whether to disable kubelet serving certificate rotation via nodepool tags."

    Write-Log "Checking whether to disable kubelet serving certificate rotation for nodepool tags"

    if (!($global:EnableKubeletServingCertificateRotation)) {
        Write-Log "Kubelet serving certificate rotation is already disabled"
        return
    }

    $tagName = "aks-disable-kubelet-serving-certificate-rotation"
    $disabled = Get-TagValue -TagName $tagName -DefaultValue "false"
    if ($disabled -ne "true") {
        Write-Log "Nodepool tag `"$tagName`" is missing or not set to true, nothing to disable"
        return
    }

    Write-Log "Kubelet serving certificate rotation is disabled by nodepool tags, will reconfigure kubelet flags and node labels"

    $global:KubeletConfigArgs = $global:KubeletConfigArgs -replace "--rotate-server-certificates=true", "--rotate-server-certificates=false"
    $global:KubeletNodeLabels = Remove-KubeletNodeLabel -KubeletNodeLabels $global:KubeletNodeLabels -Label "kubernetes.azure.com/kubelet-serving-ca=cluster"
}

# TODO: replace KubeletStartFile with a Kubelet config, remove NSSM, and use built-in service integration
function New-NSSMService {
    Param(
        [string]
        [Parameter(Mandatory = $true)]
        $KubeDir,
        [string]
        [Parameter(Mandatory = $true)]
        $KubeletStartFile,
        [string]
        [Parameter(Mandatory = $true)]
        $KubeProxyStartFile
    )

    $kubeletDependOnServices = "containerd"
    if ($global:EnableCsiProxy) {
        $kubeletDependOnServices += " csi-proxy"
    }
    if ($global:EnableHostsConfigAgent) {
        $kubeletDependOnServices += " hosts-config-agent"
    }

    # setup kubelet
    & "$KubeDir\nssm.exe" install Kubelet C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppDirectory $KubeDir | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppParameters $KubeletStartFile | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet DisplayName Kubelet | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppRestartDelay 5000 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet Description Kubelet | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet Start SERVICE_DEMAND_START | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet ObjectName LocalSystem | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppThrottle 1500 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppStdout C:\k\kubelet.log | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppStderr C:\k\kubelet.err.log | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppStdoutCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppStderrCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppRotateFiles 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppRotateOnline 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppRotateSeconds 86400 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubelet AppRotateBytes 10485760 | RemoveNulls
    # Do not use & when calling DependOnService since 'docker csi-proxy'
    # is parsed as a single string instead of two separate strings
    Invoke-Expression "$KubeDir\nssm.exe set Kubelet DependOnService $kubeletDependOnServices | RemoveNulls"

    # setup kubeproxy
    & "$KubeDir\nssm.exe" install Kubeproxy C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppDirectory $KubeDir | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppParameters $KubeProxyStartFile | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy DisplayName Kubeproxy | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy DependOnService Kubelet | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy Description Kubeproxy | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy Start SERVICE_DEMAND_START | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy ObjectName LocalSystem | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppThrottle 1500 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppStdout C:\k\kubeproxy.log | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppStderr C:\k\kubeproxy.err.log | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateFiles 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateOnline 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateSeconds 86400 | RemoveNulls
    & "$KubeDir\nssm.exe" set Kubeproxy AppRotateBytes 10485760 | RemoveNulls
}

# Renamed from Write-KubernetesStartFiles
function Install-KubernetesServices {
    param(
        [Parameter(Mandatory = $true)][string]
        $KubeDir
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.InstallKubernetesServices" -TaskMessage "Start to install kubernetes services"

    # TODO ksbrmnn fix callers to this function

    $KubeletStartFile = [io.path]::Combine($KubeDir, "kubeletstart.ps1")
    $KubeProxyStartFile = [io.path]::Combine($KubeDir, "kubeproxystart.ps1")

    New-NSSMService -KubeDir $KubeDir `
        -KubeletStartFile $KubeletStartFile `
        -KubeProxyStartFile $KubeProxyStartFile
}
