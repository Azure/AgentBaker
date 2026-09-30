# ====== BEGIN AKS BOOTSTRAP VARIABLES ======
# Every value that AgentBaker renders into this script is assigned in this block, and only here.
# The rest of the script is plain PowerShell that reads these variables.
# pkg/agent/windows_bootstrap_template_test.go enforces this.
# The values arrive as data (gzip-compressed JSON in base64, see windowsBootstrapConfig in
# pkg/agent/windows_bootstrap_config.go) and are never run as code. Do not log $cfg: it has secrets.
try {
    $cfgStream=New-Object System.IO.Compression.GZipStream((New-Object System.IO.MemoryStream(,[System.Convert]::FromBase64String('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'))), ([System.IO.Compression.CompressionMode]::Decompress))
    $cfgReader=New-Object System.IO.StreamReader($cfgStream, [System.Text.Encoding]::UTF8)
    $cfg=$cfgReader.ReadToEnd() | ConvertFrom-Json
    $cfgReader.Dispose()
    if ($cfg.SchemaVersion -ne 1) {
        throw "unsupported SchemaVersion"
    }
}
catch {
    # The helper scripts that define Set-ExitCode are not loaded yet. The message must not include
    # the exception text, which can quote the configuration. It also goes to the log because in
    # pre-provision mode csecmd.ps1 only reads base_prep.complete, which must not be written here.
    $cfgError=("ExitCode: |85|, Output: |WINDOWS_CSE_ERROR_LOAD_BOOTSTRAP_CONFIG|, Error: |Failed to load the AKS bootstrap configuration: {0}|" -f $_.Exception.GetType().FullName)
    Write-Output $cfgError
    Set-Content -Path $CSEResultFilePath -Value $cfgError
    exit 85
}

$MasterIP=$cfg.MasterIP
$KubeDnsServiceIp=$cfg.KubeDnsServiceIp
$MasterFQDNPrefix=$cfg.MasterFQDNPrefix
$Location=$cfg.Location
if ($null -ne $cfg.UserAssignedClientID) {
    $UserAssignedClientID=$cfg.UserAssignedClientID
}
$TargetEnvironment=$cfg.TargetEnvironment
$ArmResourceEndpoint=$cfg.ArmResourceEndpoint
$AADClientId=$cfg.AADClientId

$global:SSHKeys=@($cfg.SSHKeys)
$global:CACertificate=$cfg.CACertificate
$global:AgentCertificate=$cfg.AgentCertificate

$global:KubeBinariesPackageSASURL=$cfg.KubeBinariesPackageSASURL
$global:WindowsKubeBinariesURL=$cfg.WindowsKubeBinariesURL
$global:KubeBinariesVersion=$cfg.KubeBinariesVersion
$global:ContainerdUrl=$cfg.ContainerdUrl
$global:ContainerdSdnPluginUrl=$cfg.ContainerdSdnPluginUrl
$global:DockerVersion=$cfg.DockerVersion
$global:DefaultContainerdWindowsSandboxIsolation=$cfg.DefaultContainerdWindowsSandboxIsolation
$global:ContainerdWindowsRuntimeHandlers=$cfg.ContainerdWindowsRuntimeHandlers

$global:WindowsTelemetryGUID=$cfg.WindowsTelemetryGUID
$global:TenantId=$cfg.TenantId
$global:SubscriptionId=$cfg.SubscriptionId
$global:ResourceGroup=$cfg.ResourceGroup
$global:VmType=$cfg.VmType
$global:SubnetName=$cfg.SubnetName
$global:SecurityGroupName=$cfg.SecurityGroupName
$global:VNetName=$cfg.VNetName
$global:RouteTableName=$cfg.RouteTableName
$global:PrimaryAvailabilitySetName=$cfg.PrimaryAvailabilitySetName
$global:PrimaryScaleSetName=$cfg.PrimaryScaleSetName

$global:KubeClusterCIDR=$cfg.KubeClusterCIDR
$global:KubeServiceCIDR=$cfg.KubeServiceCIDR
$global:VNetCIDR=$cfg.VNetCIDR
$global:KubeletNodeLabels=$cfg.KubeletNodeLabels
$global:KubeletConfigArgs=@($cfg.KubeletConfigArgs)
$global:KubeletHealthzEndpoint=$cfg.KubeletHealthzEndpoint
$global:KubeproxyConfigArgs=@($cfg.KubeproxyConfigArgs)
$global:KubeproxyFeatureGates=@($cfg.KubeproxyFeatureGates)

$global:UseManagedIdentityExtension=$cfg.UseManagedIdentityExtension
$global:UseInstanceMetadata=$cfg.UseInstanceMetadata
$global:LoadBalancerSku=$cfg.LoadBalancerSku
$global:ExcludeMasterFromStandardLB=$cfg.ExcludeMasterFromStandardLB
$global:PrivateEgressProxyAddress=$cfg.PrivateEgressProxyAddress

$global:NetworkPlugin=$cfg.NetworkPlugin
$global:VNetCNIPluginsURL=$cfg.VNetCNIPluginsURL
$global:IsDualStackEnabled=$cfg.IsDualStackEnabled
$global:IsAzureCNIOverlayEnabled=$cfg.IsAzureCNIOverlayEnabled
$global:CiliumDataplaneEnabled=$cfg.CiliumDataplaneEnabled
$global:IsIMDSRestrictionEnabled=$cfg.IsIMDSRestrictionEnabled

$global:CredentialProviderURL=$cfg.CredentialProviderURL
$global:EnableCsiProxy=$cfg.EnableCsiProxy
$global:CsiProxyUrl=$cfg.CsiProxyUrl
$global:EnableHostsConfigAgent=$cfg.EnableHostsConfigAgent
$global:CSEScriptsPackageUrl=$cfg.CSEScriptsPackageUrl
$global:GpuDriverURL=$cfg.GpuDriverURL
$global:WindowsPauseImageURL=$cfg.WindowsPauseImageURL
$global:AlwaysPullWindowsPauseImage=$cfg.AlwaysPullWindowsPauseImage
$global:WindowsCalicoPackageURL=$cfg.WindowsCalicoPackageURL
$global:ConfigGPUDriverIfNeeded=$cfg.ConfigGPUDriverIfNeeded
$global:WindowsGmsaPackageUrl=$cfg.WindowsGmsaPackageUrl
$global:TLSBootstrapToken=$cfg.TLSBootstrapToken

$global:EnableSecureTLSBootstrapping=$cfg.EnableSecureTLSBootstrapping
$global:SecureTLSBootstrappingAADResource=$cfg.SecureTLSBootstrappingAADResource
$global:SecureTLSBootstrappingUserAssignedIdentityID=$cfg.SecureTLSBootstrappingUserAssignedIdentityID
$global:CustomSecureTLSBootstrappingClientDownloadURL=$cfg.CustomSecureTLSBootstrappingClientDownloadURL
$global:SecureTLSBootstrappingValidateKubeconfigTimeout=$cfg.SecureTLSBootstrappingValidateKubeconfigTimeout
$global:SecureTLSBootstrappingGetAccessTokenTimeout=$cfg.SecureTLSBootstrappingGetAccessTokenTimeout
$global:SecureTLSBootstrappingGetInstanceDataTimeout=$cfg.SecureTLSBootstrappingGetInstanceDataTimeout
$global:SecureTLSBootstrappingGetNonceTimeout=$cfg.SecureTLSBootstrappingGetNonceTimeout
$global:SecureTLSBootstrappingGetAttestedDataTimeout=$cfg.SecureTLSBootstrappingGetAttestedDataTimeout
$global:SecureTLSBootstrappingGetCredentialTimeout=$cfg.SecureTLSBootstrappingGetCredentialTimeout

$global:IsDisableWindowsOutboundNat=$cfg.IsDisableWindowsOutboundNat
$fipsEnabled=$cfg.fipsEnabled
$global:HNSRemediatorIntervalInMinutes=[System.Convert]::ToUInt32($cfg.HNSRemediatorIntervalInMinutes)
$global:LogGeneratorIntervalInMinutes=[System.Convert]::ToUInt32($cfg.LogGeneratorIntervalInMinutes)
$global:IsSkipCleanupNetwork=$cfg.IsSkipCleanupNetwork
$PreProvisionOnly=$cfg.PreProvisionOnly
$global:EnableKubeletServingCertificateRotation=$cfg.EnableKubeletServingCertificateRotation
$global:EnableWindowsCiliumNetworking=$cfg.EnableWindowsCiliumNetworking
$global:WindowsCiliumNetworkingConfiguration=$cfg.WindowsCiliumNetworkingConfiguration
$global:BootstrapProfileContainerRegistryServer=$cfg.BootstrapProfileContainerRegistryServer
$global:MCRRepositoryBase=$cfg.MCRRepositoryBase
$global:NetworkIsolatedClusterTestMode=$cfg.NetworkIsolatedClusterTestMode

$WindowsSSHEnabled=$cfg.WindowsSSHEnabled
$IsAKSCustomCloud=$cfg.IsAKSCustomCloud
$AKSCustomCloudContainerRegistryDNSSuffix=$cfg.AKSCustomCloudContainerRegistryDNSSuffix
$AKSCustomCloudEnvironmentJSONBase64=$cfg.AKSCustomCloudEnvironmentJSONBase64
$IdentitySystem=$cfg.IdentitySystem

Remove-Variable -Name cfg, cfgReader, cfgStream

# ====== END AKS BOOTSTRAP VARIABLES ======
