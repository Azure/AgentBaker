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
