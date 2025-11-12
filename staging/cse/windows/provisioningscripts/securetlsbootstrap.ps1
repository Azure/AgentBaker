Param(
    [Parameter(Mandatory=$true)][string]
    $KubeDir,
    [Parameter(Mandatory=$true)][string]
    $MasterIP,
    [Parameter(Mandatory=$false)][string]
    $AADResource = "6dae42f8-4368-4678-94ff-3960e28e3630", # uniquely identifies AKS's Entra ID application, see: https://learn.microsoft.com/en-us/azure/aks/kubelogin-authentication#how-to-use-kubelogin-with-aks
    [Parameter(Mandatory=$false)][string]
    $UserAssignedIdentityID = "",
    [Parameter(Mandatory=$false)][string]
    $KubeconfigPath = [Io.path]::Combine("$KubeDir", "config"),
    [Parameter(Mandatory=$false)][string]
    $CertDir = [Io.path]::Combine("$KubeDir", "pki"), # use kubelet's default pki directory
    [Parameter(Mandatory=$false)][string]
    $AzureConfigPath = [Io.path]::Combine($KubeDir, "azure.json"),
    [Parameter(Mandatory=$false)][string]
    $ClusterCAFilePath = [Io.path]::Combine($KubeDir, "ca.crt"),
    [Parameter(Mandatory=$false)][string]
    $LogFilePath = [Io.path]::Combine("$KubeDir", "secure-tls-bootstrap.log"),
    [Parameter(Mandatory=$false)][string]
    $Deadline = "120s" # default deadline of 2 minutes
)

$global:BootstrapClientPath = [Io.path]::Combine("$KubeDir", "aks-secure-tls-bootstrap-client.exe")
$global:NextProtoValue = "aks-tls-bootstrap"

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log($message) {
    $msg = $message | Timestamp
    Write-Host $msg
}

if (!(Test-Path $global:BootstrapClientPath)) {
    Write-Log "aks-secure-tls-bootstrap-client.exe was not found within $KubeDir, unable to perform secure TLS bootstrapping"
    exit 0
}

$BootstrapClientArgList = @(
    "--verbose",
    "--ensure-authorized",
    "--next-proto=$global:NextProtoValue",
    "--aad-resource=$AADResource",
    "--apiserver-fqdn=$MasterIP",
    "--cluster-ca-file=$ClusterCAFilePath",
    "--cloud-provider-config=$AzureConfigPath",
    "--cert-dir=$CertDir",
    "--kubeconfig=$KubeconfigPath",
    "--log-file=$LogFilePath",
    "--deadline=$Deadline"
)

if (![string]::IsNullOrEmpty($UserAssignedIdentityID)) {
    Write-Log "secure TLS bootstrapping user-assigned identity ID is specified: $UserAssignedIdentityID"
    $BootstrapClientArgList += "--user-assigned-identity-id=$UserAssignedIdentityID"
}

Write-Log "Starting secure TLS bootstrapping: invoking aks-secure-tls-bootstrap-client.exe"

& $global:BootstrapClientPath $BootstrapClientArgList

if ($?) {
    Write-Log "Secure TLS bootstrapping succeeded"
} else {
    # TODO(cameissner): explicitly fail the kubelet startup process if secure TLS bootstrapping fails once bootstrap tokens are no longer supported
    Write-Log "Secure TLS bootstrapping failed to completed within the alotted deadline of: $Deadline, logs written to $LogFilePath"
}
