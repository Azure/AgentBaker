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
