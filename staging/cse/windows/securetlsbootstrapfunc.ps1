# Thin wrapper around [IO.Directory]::GetFiles so we can mock it as needed within Pester tests
function GetCachedSecureTLSBootstrapClientPath {
    Param(
        [Parameter(Mandatory=$true)][string]
        $SecureTLSBootstrapClientCacheDir
    )

    $result = [IO.Directory]::GetFiles($SecureTLSBootstrapClientCacheDir, "windows-amd64.zip", [IO.SearchOption]::AllDirectories)
    return (, $result) # so that Powershell doesn't "unroll" the array returned from [IO.Directory]::GetFiles
}

# Installs the secure TLS bootstrap client from the VHD cache. If a custom URL is specified, then any cached
# versions will be ignored and removed in favor of the custom version
# NOTE: it's possible when calling this function from base CSE scripts that it won't exist. Be careful about
# ensuring this function is available before blindly calling it.
function Install-SecureTLSBootstrapClient {
    Param(
        [Parameter(Mandatory=$true)][string]
        $KubeDir,
        [Parameter(Mandatory=$false)][string]
        $CustomSecureTLSBootstrapClientDownloadUrl
    )

    $secureTLSBootstrapClientDownloadDir = [Io.path]::Combine("$KubeDir", "aks-secure-tls-bootstrap-client-downloads")
    $secureTLSBootstrapClientDownloadPath = [Io.path]::Combine("$secureTLSBootstrapClientDownloadDir", "aks-secure-tls-bootstrap-client.zip")
    $secureTLSBootstrapClientCacheDir = [Io.path]::Combine("$global:CacheDir", "aks-secure-tls-bootstrap-client")
    $secureTLSBootstrapClientBinPath = [Io.path]::Combine("$KubeDir", "aks-secure-tls-bootstrap-client.exe")
    
    # secure TLS bootstrapping is disabled, cleanup any client binary installations and return
    if (!$global:EnableSecureTLSBootstrapping) {
        Write-Log "Install-SecureTLSBootstrapClient: Secure TLS Bootstrapping is disabled, will remove secure TLS bootstrap client binary installation"
        # binary will be cleaned from aks-cache during nodePrep
        if (Test-Path $secureTLSBootstrapClientBinPath) {
            Remove-Item -Path $secureTLSBootstrapClientBinPath -Force
        }
        if (Test-Path $secureTLSBootstrapClientDownloadDir) {
            Remove-Item -Path $secureTLSBootstrapClientDownloadDir -Force -Recurse
        }
        return
    }

    # create the ephemeral download directory if needed
    New-Item -ItemType Directory -Force -Path $secureTLSBootstrapClientDownloadDir > $null

    # we have a URL from which to download a custom version of the client binary, ignoring cached versions
    if (![string]::IsNullOrEmpty($CustomSecureTLSBootstrapClientDownloadUrl)) {
        # remove any cached client binary versions from CacheDir so DownloadFileOverHttp will be forced to download the desired version from remote storage
        Write-Log "Install-SecureTLSBootstrapClient: CustomSecureTLSBootstrapClientDownloadUrl is set to: $CustomSecureTLSBootstrapClientDownloadUrl, will clear aks-cache before downloading custom client"
        Remove-Item -Path $secureTLSBootstrapClientCacheDir -Force -Recurse

        Logs-To-Event -TaskName "AKS.WindowsCSE.DownloadSecureTLSBootstrapClient" -TaskMessage "Start to download the secure TLS bootstrap client and unzip. CustomSecureTLSBootstrapClientDownloadUrl: $CustomSecureTLSBootstrapClientDownloadUrl"
        DownloadFileOverHttp -Url $CustomSecureTLSBootstrapClientDownloadUrl -DestinationPath $secureTLSBootstrapClientDownloadPath -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_SECURE_TLS_BOOTSTRAP_CLIENT
        Write-Log "Successfully downloaded custom secure TLS bootstrap client from: $CustomSecureTLSBootstrapClientDownloadUrl"
    } else { # install the cached client binary by moving it from CacheDir to KubeDir
        $search = @()
        if ($global:CacheDir -and (Test-Path $global:CacheDir)) {
            $search = GetCachedSecureTLSBootstrapClientPath -SecureTLSBootstrapClientCacheDir $secureTLSBootstrapClientCacheDir
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

    AKS-Expand-Archive -Path $secureTLSBootstrapClientDownloadPath -DestinationPath $KubeDir

    if (!(Test-Path -Path $secureTLSBootstrapClientBinPath)) {
        Write-Log "Secure TLS bootstrap client is missing from KubeDir: $KubeDir after zip extraction"
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INSTALL_SECURE_TLS_BOOTSTRAP_CLIENT -ErrorMessage "Secure TLS bootstrap client is missing from KubeDir after zip extraction"
    }
    
    Remove-Item -Path $secureTLSBootstrapClientDownloadDir -Force -Recurse
    Write-Log "Successfully extracted secure TLS bootstrap client to: $KubeDir"
}
