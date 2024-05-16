Param(
    [Parameter(Mandatory=$true)][string]
    $KubeDir,
    [Parameter(Mandatory=$true)][string]
    $APIServerFQDN,
    [Parameter(Mandatory=$true)][string]
    $AADResource,
    [Parameter(Mandatory=$false)][string]
    $BootstrapClientPath = [Io.path]::Combine("$KubeDir", "tls-bootstrap-client.exe"),
    [Parameter(Mandatory=$false)][string]
    $KubeconfigPath = [Io.path]::Combine("$KubeDir", "config"),
    [Parameter(Mandatory=$false)][string]
    $ClientCertPath = [Io.path]::Combine("$KubeDir", "client.crt"),
    [Parameter(Mandatory=$false)][string]
    $ClientKeyPath = [Io.path]::Combine("$KubeDir", "client.key"),
    [Parameter(Mandatory=$false)][string]
    $AzureConfigPath = [io.path]::Combine($KubeDir, "azure.json"),
    [Parameter(Mandatory=$false)][string]
    $ClusterCAFilePath = [io.path]::Combine($KubeDir, "ca.crt"),
    [Parameter(Mandatory=$false)][string]
    $LogFilePath = [Io.path]::Combine("$KubeDir", "securetlsbootstrap.log")
)

# next-proto value sent by the client to the bootstrap server
$global:NextProtoValue = "aks-tls-bootstrap"

function CurrentUnixTime {
    return [DateTimeOffset]::Now.ToUnixTimeSeconds()
}

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log($message) {
    $msg = $message | Timestamp
    Write-Output $msg
}

$now = CurrentUnixTime
$deadline = $now + 180 # 3 minute deadline
Write-Log "Start secure TLS bootstrapping at time: $now"
Write-Log "Secure TLS bootstrapping deadline is: $deadline"

while([DateTime]$now -lt [DateTime]$deadline) {
    & $BootstrapClientPath `
        --aad-resource=$AADResource `
        --apiserver-fqdn=$APIServerFQDN `
        --cluster-ca-file=$ClusterCAFilePath `
        --azure-config=$AzureConfigPath `
        --cert-file=$ClientCertPath `
        --key-file=$ClientKeyPath `
        --next-proto=$global:NextProtoValue `
        --kubeconfig=$KubeconfigPath `
        --log-file=$LogFilePath
    if ($?) {
        Write-Log "Secure TLS bootstrapping succeeded"
        exit 0
    }
    $now = CurrentUnixTime
}

Write-Log "Secure TLS Bootstrapping failed to complete within the alotted time"
exit 1