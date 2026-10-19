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
