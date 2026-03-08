# functions for network isolated cluster

# Initialize-Oras will install oras and login the registry if anonymous access is disabled. This is required for network isolated cluster to pull windowszip from private container registry.
function Initialize-Oras {
    Install-Oras
    # reserve for Invoke-OrasLogin to avoid frequent code changes in parts/windows/
}

# unpackage and install oras from cache
# Oras is used for pulling windows binaries, e.g. windowszip, from private container registry when it is network isolated cluster.
function Install-Oras {
    # Check if OrasPath variable exists to avoid latest cached cse in vhd with possible old ab svc
    $orasPathVarExists = Test-Path variable:global:OrasPath
    if (-not $orasPathVarExists) {
        Write-Log "OrasPath variable does not exist. Setting OrasPath to default value C:\aks-tools\oras\oras.exe"
        $global:OrasPath = "C:\aks-tools\oras\oras.exe"
    }

    if (Test-Path -Path $global:OrasPath) {
        # oras already installed, skip
        Write-Log "Oras already installed at $($global:OrasPath)"
        return
    }
    # Ensure cache directory exists before checking for archives or downloading
    if (-Not (Test-Path $global:OrasCacheDir)) {
        New-Item -ItemType Directory -Path $global:OrasCacheDir -Force | Out-Null
    }

    if (-Not (Test-Path $global:OrasCacheDir)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Oras cache directory not found at $($global:OrasCacheDir)"
    }

    # Look for a cached oras archive (.tar.gz or .zip) in the oras cache directory
    $orasArchive = Get-ChildItem -Path $global:OrasCacheDir -File |
        Where-Object { $_.Name -like "*.tar.gz" -or $_.Name -like "*.zip" } |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
    if (-Not $orasArchive) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "No oras archive (.tar.gz or .zip) found in $($global:OrasCacheDir)"
    }

    # Extract the archive to the oras install directory
    $orasInstallDir = [IO.Path]::GetDirectoryName($global:OrasPath)
    if (-Not (Test-Path $orasInstallDir)) {
        New-Item -ItemType Directory -Path $orasInstallDir -Force | Out-Null
    }

    Write-Log "Extracting oras from $($orasArchive.FullName) to $orasInstallDir"
    if ($orasArchive.Name -like "*.zip") {
        AKS-Expand-Archive -Path $orasArchive.FullName -DestinationPath $orasInstallDir
    } elseif ($orasArchive.Name -like "*.tar.gz") {
        try {
            tar -xzf $orasArchive.FullName -C $orasInstallDir
            if ($LASTEXITCODE -ne 0) {
                Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Failed to extract oras archive $($orasArchive.FullName) (tar exit code $LASTEXITCODE)"
            }
        } catch {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Exception while extracting oras archive $($orasArchive.FullName): $($_.Exception.Message)"
        }
    }

    if (-Not (Test-Path $global:OrasPath)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Oras executable not found at $($global:OrasPath) after extraction"
    }

    Write-Log "Oras installed successfully at $($global:OrasPath)"
}

function Oras-Login {
    param(
        [Parameter(Mandatory = $true)][string]
        $Acr_Url,
        [Parameter(Mandatory = $true)][string]
        $ClientID,
        [Parameter(Mandatory = $true)][string]
        $TenantID
    )

    Ensure-Oras

    # Check for required variables
    if ([string]::IsNullOrWhiteSpace($ClientID) -or [string]::IsNullOrWhiteSpace($TenantID)) {
        Write-Host "ClientID or TenantID are not set. Oras login is not possible, proceeding with anonymous pull"
        return $global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED
    }

    # Attempt anonymous pull check (assumes helper function exists)
    $retCode = retrycmd_can_oras_ls_acr_anonymously 10 5 $Acr_Url
    if ($retCode -eq 0) {
        Write-Host "anonymous pull is allowed for acr '$Acr_Url', proceeding with anonymous pull"
        return
    }
    elseif ($retCode -ne 1) {
        Write-Host "failed with an error other than unauthorized, exiting.."
        Set-ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULL_NETWORK_TIMEOUT -ErrorMessage "failed with an error other than unauthorized, exiting"
    }

    # Get AAD Access Token using Managed Identity Metadata Service
    $accessUrl = "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://management.azure.com/&client_id=$ClientID"
    try {
        $args = @{
            Uri     = $accessUrl
            Method  = "Get"
            Headers = @{ Metadata = "true" }
        }
        $rawAccessTokenResponse = Retry-Command -Command "Invoke-RestMethod" -Args $args -Retries 10 -RetryDelaySeconds 5
        $accessToken = $rawAccessTokenResponse.access_token
    }
    catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_IMDS_TIMEOUT -ErrorMessage "failed to retrieve AAD access token: $($_.Exception.Message)"
    }

    if ([string]::IsNullOrWhiteSpace($accessToken)) {
        Set-ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED -ErrorMessage "failed to parse imds access token"
    }

    # Exchange AAD Access Token for ACR Refresh Token
    try {
        $exchangeUrl = "https://$Acr_Url/oauth2/exchange"
        $body = "grant_type=access_token&service=$Acr_Url&tenant=$TenantID&access_token=$accessToken"
        $args = @{
            Uri         = $exchangeUrl
            Method      = "Post"
            ContentType = "application/x-www-form-urlencoded"
            Body        = $body
            TimeoutSec  = 60
        }
        $rawRefreshTokenResponse = Retry-Command -Command "Invoke-RestMethod" -Args $args -Retries 10 -RetryDelaySeconds 5
        $refreshToken = $rawRefreshTokenResponse.refresh_token
    }
    catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED -ErrorMessage "failed to retrieve refresh token: $($_.Exception.Message)"
    }

    if ([string]::IsNullOrWhiteSpace($refreshToken)) {
        Set-ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED -ErrorMessage "failed to parse refresh token"
    }

    # Pre-validate refresh token permissions
    $retCode = Assert-RefreshToken -RefreshToken $refreshToken -RequiredActions @("read")
    if ($retCode -ne 0) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED -ErrorMessage "failed to validate refresh token permissions"
    }

    # Perform Oras Login (pipe refresh token to stdin for --identity-token-stdin)
    $loginSuccess = $false
    for ($i = 1; $i -le 3; $i++) {
        try {
            Write-Log "Retry $i : oras login $Acr_Url"
            $loginOutput = $refreshToken | & $global:OrasPath login $Acr_Url --identity-token-stdin --registry-config $global:OrasRegistryConfigFile 2>&1
            if ($LASTEXITCODE -eq 0) {
                $loginSuccess = $true
                break
            }
            Write-Log "oras login attempt $i failed (exit code $LASTEXITCODE): $loginOutput"
        }
        catch {
            Write-Log "oras login attempt $i exception: $($_.Exception.Message)"
        }
        if ($i -lt 3) {
            Start-Sleep -Seconds 5
        }
    }
    if (-Not $loginSuccess) {
        Set-ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED -ErrorMessage "failed to login to acr '$Acr_Url' with identity token"
    }

    # Clean up sensitive data
    Remove-Variable accessToken, refreshToken -ErrorAction SilentlyContinue

    Write-Host "successfully logged in to acr '$Acr_Url' with identity token"
}

function Ensure-Oras {
    if (Test-Path $global:OrasPath) {
        return
    }
    # Ensure cache directory exists before checking for archives or downloading
    if (-Not (Test-Path $global:OrasCacheDir)) {
        New-Item -ItemType Directory -Path $global:OrasCacheDir -Force | Out-Null
    }

    ### FOR TEMP TEST USE ONLY - Download oras if not found in cache. This is to unblock Windows 2025 testing since we don't have oras in the cache for 2025 image yet. We will remove this logic after we have oras in the cache for 2025 image.
    if (-Not (Get-ChildItem -Path $global:OrasCacheDir -File | Where-Object { $_.Name -like "*.tar.gz" -or $_.Name -like "*.zip" })) {
        $orasVersion = "1.3.0"
        $orasZip = "oras_${orasVersion}_windows_amd64.zip"
        $orasDownloadUrl = "https://github.com/oras-project/oras/releases/download/v${orasVersion}/${orasZip}"
        Write-Log "Downloading oras v${orasVersion} from $orasDownloadUrl"
        try {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Invoke-WebRequest -UseBasicParsing $orasDownloadUrl -OutFile "$($global:OrasCacheDir)\$orasZip"
        }
        catch {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Failed to download oras from $orasDownloadUrl. Error: $_"
        }
    }
    ######################## END TEMP TEST USE ONLY ######################################

    if (-Not (Test-Path $global:OrasCacheDir)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Oras cache directory not found at $($global:OrasCacheDir)"
    }

    # Look for a cached oras archive (.tar.gz or .zip) in the oras cache directory
    $orasArchive = Get-ChildItem -Path $global:OrasCacheDir -File | Where-Object { $_.Name -like "*.tar.gz" -or $_.Name -like "*.zip" } | Select-Object -First 1
    if (-Not $orasArchive) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "No oras archive (.tar.gz or .zip) found in $($global:OrasCacheDir)"
    }

    # Extract the archive to the oras install directory
    $orasInstallDir = [IO.Path]::GetDirectoryName($global:OrasPath)
    if (-Not (Test-Path $orasInstallDir)) {
        New-Item -ItemType Directory -Path $orasInstallDir -Force | Out-Null
    }

    Write-Log "Extracting oras from $($orasArchive.FullName) to $orasInstallDir"
    if ($orasArchive.Name -like "*.zip") {
        Expand-Archive -Path $orasArchive.FullName -DestinationPath $orasInstallDir -Force
    }
    elseif ($orasArchive.Name -like "*.tar.gz") {
        tar -xzf $orasArchive.FullName -C $orasInstallDir
        if ($LASTEXITCODE -ne 0) {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Failed to extract oras archive $($orasArchive.FullName)"
        }
    }

    if (-Not (Test-Path $global:OrasPath)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND -ErrorMessage "Oras executable not found at $($global:OrasPath) after extraction"
    }

    Write-Log "Oras installed successfully at $($global:OrasPath)"
}

function retrycmd_can_oras_ls_acr_anonymously {
    Param(
        [Parameter(Mandatory = $true)][int]$Retries,
        [Parameter(Mandatory = $true)][int]$WaitSleep,
        [Parameter(Mandatory = $true)][string]$AcrUrl
    )

    for ($i = 1; $i -le $Retries; $i++) {
        # Logout first to ensure insufficient ABAC token won't affect anonymous judging
        try { & $global:OrasPath logout $AcrUrl  --registry-config $global:OrasRegistryConfigFile 2>$null } catch { }

        $output = $null
        try {
            $output = & $global:OrasPath repo ls $AcrUrl  --registry-config $global:OrasRegistryConfigFile 2>&1
        }
        catch {
            $output = $_.Exception.Message
        }

        if ($LASTEXITCODE -eq 0) {
            Write-Host "acr is anonymously reachable"
            return 0
        }

        if ($output -and ($output -like "*unauthorized: authentication required*")) {
            Write-Host "ACR is not anonymously reachable: $output"
            return 1
        }

        Start-Sleep -Seconds $WaitSleep
    }

    Write-Host "unexpected response from acr: $output"
    return $global:WINDOWS_CSE_ERROR_ORAS_PULL_NETWORK_TIMEOUT
}

function Assert-RefreshToken {
    Param(
        [Parameter(Mandatory = $true)][string]$RefreshToken,
        [Parameter(Mandatory = $true)][string[]]$RequiredActions
    )

    # Decode the refresh token (JWT format: header.payload.signature)
    # Extract the payload (second part) and decode from base64
    $tokenParts = $RefreshToken.Split('.')
    if ($tokenParts.Length -lt 2) {
        Write-Host "Invalid JWT token format"
        return $global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED
    }

    $tokenPayload = $tokenParts[1]
    # Add padding if needed for base64url decoding
    switch ($tokenPayload.Length % 4) {
        2 { $tokenPayload += "==" }
        3 { $tokenPayload += "=" }
    }
    # Replace base64url characters with standard base64
    $tokenPayload = $tokenPayload -replace '-', '+' -replace '_', '/'

    try {
        $decodedBytes = [System.Convert]::FromBase64String($tokenPayload)
        $decodedToken = [System.Text.Encoding]::UTF8.GetString($decodedBytes)
    }
    catch {
        Write-Host "Failed to decode token payload: $($_.Exception.Message)"
        return $global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED
    }

    if (-Not [string]::IsNullOrWhiteSpace($decodedToken)) {
        try {
            $tokenObj = $decodedToken | ConvertFrom-Json
        }
        catch {
            Write-Host "Failed to parse token JSON: $($_.Exception.Message)"
            return $global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED
        }

        # Check if permissions field exists (RBAC token vs ABAC token)
        if ($null -ne $tokenObj.permissions) {
            Write-Host "RBAC token detected, validating permissions"

            $tokenActions = @()
            if ($null -ne $tokenObj.permissions.actions) {
                $tokenActions = @($tokenObj.permissions.actions)
            }

            foreach ($action in $RequiredActions) {
                if ($tokenActions -notcontains $action) {
                    Write-Host "Required action '$action' not found in token permissions"
                    return $global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED
                }
            }
            Write-Host "Token validation passed: all required actions present"
        }
        else {
            Write-Host "No permissions field found in token. Assuming ABAC token, skipping permission validation"
        }
    }

    return 0
}
