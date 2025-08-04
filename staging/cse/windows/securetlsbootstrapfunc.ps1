# Functions used for setting up and running the secure TLS bootstrapping protocol to
# securely generate kubelet client certificates without the usage of bootstrap tokens

$global:SecureTLSBootstrapServiceName = "secure-tls-bootstrap"
$global:SecureTLSBootstrappingDeadline = "120s"

function Install-SecureTLSBootstrapClient {
    Param(
        [Parameter(Mandatory=$true)][string]
        $KubeDir,
        [Parameter(Mandatory=$false)][string]
        $CustomSecureTLSBootstrapClientDownloadUrl,
        [Parameter(Mandatory=$false)][string]
        $SecureTLSBootstrapClientDownloadDir = [Io.path]::Combine("$KubeDir", "aks-secure-tls-bootstrap-client-downloads")
    )

    $secureTLSBootstrapClientDownloadPath = [Io.path]::Combine("$SecureTLSBootstrapClientDownloadDir", "aks-secure-tls-bootstrap-client.zip")
    $secureTLSBootstrapBinPath = [Io.path]::Combine("$KubeDir", "aks-secure-tls-bootstrap-client.exe")
    
    # secure TLS bootstrapping is disabled, cleanup any client binary installations and return
    if (!$global:EnableSecureTLSBootstrapping) {
        Write-Log "Install-SecureTLSBootstrapClient: Secure TLS Bootstrapping is disabled, will remove secure TLS bootstrap client binary installation"
        # binary will be cleaned from aks-cache during nodePrep
        Remove-Item -Path $secureTLSBootstrapBinPath -Force
        Remove-Item -Path $SecureTLSBootstrapClientDownloadDir -Force -Recurse
        return
    }

    # create the ephemeral download directory if needed
    New-Item -ItemType Directory -Force -Path $SecureTLSBootstrapClientDownloadDir > $null

    # we have a URL from which to download a custom version of the client binary, ignoring cached versions
    if (![string]::IsNullOrEmpty($CustomSecureTLSBootstrapClientDownloadUrl)) {
        # remove any cached client binary versions from CacheDir so DownloadFileOverHttp will be forced to download the desired version from remote storage
        Write-Log "Install-SecureTLSBootstrapClient: CustomSecureTLSBootstrapClientDownloadUrl is set to: $CustomSecureTLSBootstrapClientDownloadUrl, will clear aks-cache before downloading custom client"
        Remove-Item -Path [Io.path]::Combine("$global:CacheDir", "aks-secure-tls-bootstrap-client") -Force -Recurse

        Logs-To-Event -TaskName "AKS.WindowsCSE.DownloadSecureTLSBootstrapClient" -TaskMessage "Start to download the secure TLS bootstrap client and unzip. CustomSecureTLSBootstrapClientDownloadUrl: $CustomSecureTLSBootstrapClientDownloadUrl"
        DownloadFileOverHttp -Url $CustomSecureTLSBootstrapClientDownloadUrl -DestinationPath $secureTLSBootstrapClientDownloadPath -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_SECURE_TLS_BOOTSTRAP_CLIENT
        Write-Log "Successfully downloaded custom secure TLS bootstrap client from: $CustomSecureTLSBootstrapClientDownloadUrl"
    } else { # install the cached client binary by moving it from CacheDir to KubeDir
        $search = @()
        if ($global:CacheDir -and (Test-Path $global:CacheDir)) {
            $secureTLSBootstrapClientCacheDir = [Io.path]::Combine("$global:CacheDir", "aks-secure-tls-bootstrap-client")
            $search = [IO.Directory]::GetFiles($secureTLSBootstrapClientCacheDir, "windows-amd64.zip", [IO.SearchOption]::AllDirectories)
        } else {
            Write-Log "CacheDir: $global:CacheDir does not exist, unable to install secure TLS bootstrap client"
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT -ErrorMessage "CacheDir is missing"
        }

        if ($search.Count -ne 0) {
            Write-Log "Using cached version of secure TLS bootstrap client - Copying from $($search[0]) to $secureTLSBootstrapClientDownloadPath"
            Copy-Item -Path $search[0] -Destination $secureTLSBootstrapClientDownloadPath -Force
        } else {
            Write-Log "No cached version of secure TLS bootstrap client found within $secureTLSBootstrapClientCacheDir"
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT -ErrorMessage "Secure TLS bootstrap client is missing from cache"
        }
    }

    Expand-Archive -path $secureTLSBootstrapClientDownloadPath -DestinationPath $global:KubeDir
    if ($LASTEXITCODE -ne 0) {
        Write-Log "Failed to extract secure TLS bootstrap client archive"
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT -ErrorMessage "Failed to extract secure TLS bootstrap client archive"
    }
    if (!(Test-Path -Path [Io.path]::Combine("$KubeDir", "aks-secure-tls-bootstrap-client.exe"))) {
        Write-Log "Secure TLS bootstrap client is missing from KubeDir: $global:KubeDir after zip extraction"
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT -ErrorMessage "Secure TLS bootstrap client is missing from KubeDir after zip extraction"
    }
    
    Remove-Item -Path $SecureTLSBootstrapClientDownloadDir -Force -Recurse
    Write-Log "Successfully extracted secure TLS bootstrap client to: $secureTLSBootstrapClientDownloadPath"
}

function ConfigureAndStart-SecureTLSBootstrapping {
    param(
        [Parameter(Mandatory=$true)][string]
        $KubeDir,
        [Parameter(Mandatory=$true)][string]
        $APIServerFQDN,
        [Parameter(Mandatory=$true)][string]
        $CustomSecureTLSBootstrapAADResource,
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
    $aadResource = ([string]::IsNullOrEmpty($CustomSecureTLSBootstrapAADResource)) ? $global:AKSAADServerAppID : $CustomSecureTLSBootstrapAADResource
    $args = @(
        "--verbose",
        "--ensure-authorized",
        "--next-proto=aks-tls-bootstrap",
        "--aad-resource=$aadResource",
        "--cluster-ca-file=$ClusterCAFilePath",
        "--kubeconfig=$KubeconfigPath",
        "--cred-file=$CredFilePath",
        "--log-file=$LogFilePath",
        "--apiserver-fqdn=$APIServerFQDN",
        "--cloud-provider-config=$AzureConfigPath",
        "--deadline=$global:SecureTLSBootstrappingDeadline"
    )

    Write-Log "Registering and starting the $global:SecureTLSBootstrapServiceName service"

    # Configure the service as a "one-shot", meaning the it will run until the client binary reaches a terminal state
    & "$KubeDir\nssm.exe" install $global:SecureTLSBootstrapServiceName $secureTLSBootstrapBinPath | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SecureTLSBootstrapServiceName AppExit Default Exit
    & "$KubeDir\nssm.exe" set $global:SecureTLSBootstrapServiceName AppDirectory $KubeDir | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SecureTLSBootstrapServiceName AppParameters ($args -join " ") | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SecureTLSBootstrapServiceName DisplayName "$global:SecureTLSBootstrapServiceName" | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SecureTLSBootstrapServiceName Description "$global:SecureTLSBootstrapServiceName" | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SecureTLSBootstrapServiceName Start SERVICE_DEMAND_START | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SecureTLSBootstrapServiceName ObjectName LocalSystem | RemoveNulls
    & "$KubeDir\nssm.exe" set $global:SecureTLSBootstrapServiceName Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls

    # Start the service
    Start-Service $global:SecureTLSBootstrapServiceName
    if ($LASTEXITCODE -ne 0) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_START_SECURE_TLS_BOOTSTRAP_SERVICE -ErrorMessage "failed to start the $global:SecureTLSBootstrapServiceName service"
    }
}
