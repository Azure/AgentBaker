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
