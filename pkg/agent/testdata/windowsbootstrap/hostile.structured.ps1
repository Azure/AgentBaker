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
