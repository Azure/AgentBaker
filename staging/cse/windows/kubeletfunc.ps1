# Invokes nssm.exe with the given arguments and throws if the exit code is non-zero.
function Invoke-Nssm
{
    param(
        [Parameter(Mandatory = $true)][string]$KubeDir,
        [Parameter(Mandatory = $true, ValueFromRemainingArguments = $true)][string[]]$NssmArguments
    )
    & "$KubeDir\nssm.exe" @NssmArguments | RemoveNulls
    if ($LASTEXITCODE -ne 0)
    {
        throw "nssm.exe $( $NssmArguments -join ' ' ) failed (exit code $LASTEXITCODE)"
    }
}

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
        }
        else {
            Write-Log "Did not find $global:KubeBinariesVersion in $mappingFile"
        }
    }

    $zipfile = "c:\k.zip"


    if ([string]::IsNullOrEmpty($global:BootstrapProfileContainerRegistryServer)) {
        # default path
        # download kubelet binaries via http if BootstrapProfileContainerRegistryServer is not set
        Logs-To-Event -TaskName "AKS.WindowsCSE.DownloadKubeletBinaries" -TaskMessage "Start to download kubelet binaries and unzip. KubeBinariesPackageSASURL: $KubeBinariesSASURL"

        for ($i = 0; $i -le 10; $i++) {
            DownloadFileOverHttp -Url $KubeBinariesSASURL -DestinationPath $zipfile -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_KUBERNETES_PACKAGE
            if ($?) {
                break
            }
            else {
                Write-Log $Error[0].Exception.Message
            }
        }
    } else {
        # ni path
        # download kubelet binaries via oras if BootstrapProfileContainerRegistryServer is set
        if (-not (Get-Command 'DownloadFileWithOras' -ErrorAction SilentlyContinue)) {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULL_WINDOWSZIP_FAIL -ErrorMessage "DownloadFileWithOras function is not available. networkisolatedclusterfunc.ps1 may not be sourced."
        }
        Logs-To-Event -TaskName "AKS.WindowsCSE.DownloadKubeletBinariesWithOras" -TaskMessage "Start to download kubelet binaries with oras. KubeBinariesVersion: $global:KubeBinariesVersion, BootstrapProfileContainerRegistryServer: $global:BootstrapProfileContainerRegistryServer"
        $orasReference = "$($global:BootstrapProfileContainerRegistryServer)/aks/packages/kubernetes/windowszip:v$($global:KubeBinariesVersion)"
        try {
            Retry-Command -Command "DownloadFileWithOras" -Args @{Reference=$orasReference; DestinationPath=$zipfile} -Retries 5 -RetryDelaySeconds 10
        } catch {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULL_WINDOWSZIP_FAIL -ErrorMessage "Exhausted retries for oras pull $orasReference. Error: $_"
        }
    }

    AKS-Expand-Archive -Path $zipfile -DestinationPath C:\
    Remove-Item $zipfile
}

function Add-KubeletNodeLabel {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Label
    )

    $labelList = $global:KubeletNodeLabels -split ","
    foreach ($existingLabel in $labelList) {
        if ($existingLabel -eq $Label) {
            Write-Log "found existing kubelet node label $existingLabel, will continue without adding anything"
            return
        }
    }
    Write-Log "adding label $Label to kubelet node labels..."
    $labelList += $Label
    $global:KubeletNodeLabels = $labelList -join ","
}

function Remove-KubeletNodeLabel {
    Param(
        [Parameter(Mandatory = $true)][string]
        $Label
    )

    $labelList = $global:KubeletNodeLabels -split ","
    $filtered = $labelList | Where-Object { $_ -ne $Label }
    $global:KubeletNodeLabels = $filtered -join ","
}

function Get-TagValue {
    Param(
        [Parameter(Mandatory = $true)][string]
        $TagName,
        [Parameter(Mandatory = $true)][string]
        $DefaultValue
    )

    $uri = "http://169.254.169.254/metadata/instance?api-version=2021-02-01"
    try {
        $response = Retry-Command -Command "Invoke-RestMethod" -Args @{Uri = $uri; Method = "Get"; ContentType = "application/json"; Headers = @{"Metadata" = "true" } } -Retries 3 -RetryDelaySeconds 5
    }
    catch {
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
function Configure-KubeletServingCertificateRotation {
    Logs-To-Event `
        -TaskName "AKS.WindowsCSE.ConfigureKubeletServingCertificateRotation" `
        -TaskMessage "EnableKubeletServingCertificateRotation: $global:EnableKubeletServingCertificateRotation. Configure kubelet config args and node labels for serving certificate rotation"

    if (!($global:EnableKubeletServingCertificateRotation)) {
        Write-Log "Kubelet serving certificate rotation is disabled, nothing to configure"
        return
    }

    $nodeLabel = "kubernetes.azure.com/kubelet-serving-ca=cluster"

    # check if kubelet serving certificate rotation is disabled via customer-specified nodepool tags
    $tagName = "aks-disable-kubelet-serving-certificate-rotation"
    $disabled = Get-TagValue -TagName $tagName -DefaultValue "false"
    if ($disabled -eq "true") {
        Write-Log "Kubelet serving certificate rotation is disabled by nodepool tags, will reconfigure kubelet flags and node labels"
        $global:KubeletConfigArgs = $global:KubeletConfigArgs -replace "--rotate-server-certificates=true", "--rotate-server-certificates=false"
        Remove-KubeletNodeLabel -Label $nodeLabel
        return
    }

    Write-Log "Kubelet serving certificate rotation is enabled, will add node label if needed"
    Add-KubeletNodeLabel -Label $nodeLabel
}

# DEPRECATED - TODO(cameissner): remove once k8s setup script has been updated
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
    Remove-KubeletNodeLabel -Label "kubernetes.azure.com/kubelet-serving-ca=cluster"
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
    Invoke-Nssm -KubeDir $KubeDir install Kubelet C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppDirectory $KubeDir
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppParameters $KubeletStartFile
    Invoke-Nssm -KubeDir $KubeDir set Kubelet DisplayName Kubelet
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppRestartDelay 5000
    Invoke-Nssm -KubeDir $KubeDir set Kubelet Description Kubelet
    Invoke-Nssm -KubeDir $KubeDir set Kubelet Start SERVICE_DEMAND_START
    Invoke-Nssm -KubeDir $KubeDir set Kubelet ObjectName LocalSystem
    Invoke-Nssm -KubeDir $KubeDir set Kubelet Type SERVICE_WIN32_OWN_PROCESS
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppThrottle 1500
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppStdout C:\k\kubelet.log
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppStderr C:\k\kubelet.err.log
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppStdoutCreationDisposition 4
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppStderrCreationDisposition 4
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppRotateFiles 1
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppRotateOnline 1
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppRotateSeconds 86400
    Invoke-Nssm -KubeDir $KubeDir set Kubelet AppRotateBytes 10485760

    # Do not use Invoke-Nssm when calling DependOnService since 'docker csi-proxy'
    # is parsed as a single string instead of two separate strings
    $LASTEXITCODE = 0
    Invoke-Expression "$KubeDir\nssm.exe set Kubelet DependOnService $kubeletDependOnServices | RemoveNulls"
    if (-not $?) { throw "Invoke-Expression failed to invoke before calling nssm.exe (PowerShell invocation failed - exit code $?)" }
    if ($LASTEXITCODE -ne 0) { throw "nssm.exe failed to set Kubelet DependOnService (exit code $LASTEXITCODE)" }

    # setup kubeproxy
    Invoke-Nssm -KubeDir $KubeDir install Kubeproxy C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy AppDirectory $KubeDir
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy AppParameters $KubeProxyStartFile
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy DisplayName Kubeproxy
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy DependOnService Kubelet
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy Description Kubeproxy
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy Start SERVICE_DEMAND_START
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy ObjectName LocalSystem
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy Type SERVICE_WIN32_OWN_PROCESS
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy AppThrottle 1500
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy AppStdout C:\k\kubeproxy.log
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy AppStderr C:\k\kubeproxy.err.log
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy AppRotateFiles 1
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy AppRotateOnline 1
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy AppRotateSeconds 86400
    Invoke-Nssm -KubeDir $KubeDir set Kubeproxy AppRotateBytes 10485760
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
