# Secure TLS Bootstrap Functions for Windows Nodes
# This script provides functions to configure and start secure TLS bootstrapping for Windows Kubernetes nodes
# Similar to the Linux systemd implementation but using Windows services

# Global variables for secure TLS bootstrap configuration
$global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME = "secure-tls-bootstrap"

function Install-SecureTLSBootstrapClient {
    Param(
        [Parameter(Mandatory=$true)][bool]
        $EnableSecureTLSBootstrapping,
        [Parameter(Mandatory=$true)][string]
        $KubeDir,
        [Parameter(Mandatory=$true)][string]
        $SecureTLSBootstrapClientDownloadDir,
        [Parameter(Mandatory=$true)][string]
        $SecureTLSBootstrapClientDownloadUrl
    )

    $secureTLSBootstrapClientDownloadPath = [Io.path]::Combine("$SecureTLSBootstrapClientDownloadDir", "aks-secure-tls-bootstrap-client.zip")
    $secureTLSBootstrapBinPath = [Io.path]::Combine("$KubeDir", "aks-secure-tls-bootstrap-client.exe")
    
    if (!($EnableSecureTLSBootstrapping)) {
        Write-Log "Secure TLS Bootstrapping is disabled, will remove secure TLS bootstrap client binary installation"
        Remove-Item -Path $secureTLSBootstrapBinPath -Force
        Remove-Item -Path $SecureTLSBootstrapClientDownloadDir -Force -Recurse
        return
    }

    Logs-To-Event -TaskName "AKS.WindowsCSE.DownloadSecureTLSBootstrapClient" -TaskMessage "Start to download the secure TLS bootstrap client binary and unzip. SecureTLSBootstrapClientDownloadUrl: $SecureTLSBootstrapClientDownloadUrl"
    DownloadFileOverHttp -Url $SecureTLSBootstrapClientDownloadUrl -DestinationPath $SecureTLSBootstrapClientDownloadPath -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_SECURE_TLS_BOOTSTRAP_CLIENT

    Expand-Archive -path $SecureTLSBootstrapClientDownloadPath -DestinationPath $global:KubeDir
    if ($LASTEXITCODE -ne 0) {
        Write-Log "Failed to extract secure TLS bootstrap client archive"
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT -ErrorMessage "Failed to extract secure TLS bootstrap client archive"
    }
    
    Remove-Item -Path $SecureTLSBootstrapClientDownloadDir -Force -Recurse

    Write-Log "Successfully downloaded and extracted secure TLS bootstrap client to: $secureTLSBootstrapClientDownloadPath"
}

function ConfigureAndStart-SecureTLSBootstrapping {
    param(
        [Parameter(Mandatory=$true)][string]
        $KubeDir,
        [Parameter(Mandatory=$true)][string]
        $APIServerFQDN,
        [Parameter(Mandatory=$true)][string]
        $AADResource,
        [Parameter(Mandatory=$false)][string]
        $KubeconfigPath = [Io.path]::Combine("$KubeDir", "config"),
        [Parameter(Mandatory=$false)][string]
        $CredFilePath = [Io.path]::Combine("$KubeDir", "client.pem"),
        [Parameter(Mandatory=$false)][string]
        $AzureConfigPath = [io.path]::Combine($KubeDir, "azure.json"),
        [Parameter(Mandatory=$false)][string]
        $ClusterCAFilePath = [io.path]::Combine($KubeDir, "ca.crt"),
        [Parameter(Mandatory=$false)][string]
        $LogFilePath = [Io.path]::Combine("$KubeDir", "secure-tls-bootstrap.log")
    )

    $secureTLSBootstrapBinPath = [Io.path]::Combine("$KubeDir", "aks-secure-tls-bootstrap-client.exe")
    $args = @(
        "--verbose",
        "--ensure-authorized",
        "--next-proto=aks-tls-bootstrap",
        "--cluster-ca-file=$ClusterCAFilePath",
        "--kubeconfig=$KubeconfigPath",
        "--cred-file=$CredFilePath",
        "--log-file=$LogFilePath",
        "--aad-resource=$AADResource",
        "--apiserver-fqdn=$APIServerFQDN",
        "--cloud-provider-config=$AzureConfigPath"
    )

    Write-Log "Registering and starting the $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME service"

    # Configure the service as a "one-shot", meaning the service will run until the client binary reaches a terminal state
    & "$KubeDir\nssm.exe" install $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME $secureTLSBootstrapBinPath | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME AppExit Default Exit
    & "$KubeDir\nssm.exe" set $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME AppDirectory $KubeDir | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME AppParameters ($args -join " ") | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME DisplayName "$global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME" | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME Description "AKS Secure TLS Bootstrap Client" | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME Start SERVICE_DEMAND_START | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME ObjectName LocalSystem | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls

    # Start the service
    Start-Service $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME
    if ($LASTEXITCODE -ne 0) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_START_SECURE_TLS_BOOTSTRAP_SERVICE -ErrorMessage "failed to start the $global:SECURE_TLS_BOOTSTRAP_SERVICE_NAME service"
    }
}
