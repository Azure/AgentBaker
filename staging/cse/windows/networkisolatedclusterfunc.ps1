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

# For network isolated cluster, containerd has issue to pull pause image via credential provider, thus it will not be able to pull pause image during runtime. We have to pull pause image and set pinned label to avoid gc.
function Set-PodInfraContainerImage {
    $podInfraContainerImageDownloadDir = "C:\k\pod-infra-container-image\downloads"
    $podInfraContainerImageTar = "C:\k\pod-infra-container-image\pod-infra-container-image.tar"

    $clusterConfig = ConvertFrom-Json ((Get-Content $global:KubeClusterConfigPath -ErrorAction Stop) | Out-String)
    $podInfraContainerImage = $clusterConfig.Cri.Images.Pause
    if ([string]::IsNullOrWhiteSpace($podInfraContainerImage)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULL_POD_INFRA_CONTAINER -ErrorMessage "Failed to recognize pod infra container image"
    }

    Write-Log "Checking if '$podInfraContainerImage' already exists locally..."
    $images = & ctr -n k8s.io images list -q 2>$null
    if (($LASTEXITCODE -eq 0) -and ($images -contains $podInfraContainerImage)) {
        Write-Log "Image '$podInfraContainerImage' already exists locally, skipping pull"
        return
    }

    $baseName = $podInfraContainerImage -replace ':[^:]+$'
    $tag = "local"

    $image = $podInfraContainerImage
    if (-not [string]::IsNullOrWhiteSpace($global:BootstrapProfileContainerRegistryServer)) {
        $image = $podInfraContainerImage.Replace("mcr.microsoft.com", $global:BootstrapProfileContainerRegistryServer)
    }

    if (-not (Test-Path -Path $podInfraContainerImageDownloadDir)) {
        New-Item -ItemType Directory -Path $podInfraContainerImageDownloadDir -Force | Out-Null
    }

    Write-Log "Pulling via oras for '$image'"
    $orasCopySucceeded = $false
    $orasDestination = '{0}:{1}' -f $podInfraContainerImageDownloadDir, $tag
    for ($i = 1; $i -le 10; $i++) {
        if ($i -gt 1) {
            Start-Sleep -Seconds 5
        }

        Write-Log "Try $i : oras cp '$image' to '$orasDestination'"
        $orasOutput = & $global:OrasPath cp $image $orasDestination --to-oci-layout --from-registry-config $global:OrasRegistryConfigFile 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Log ("Successfully pulled '$image' via oras on attempt $i")
            $orasCopySucceeded = $true
            break
        }

        Write-Log ('oras cp attempt {0} failed (exit code {1}): {2}' -f $i, $LASTEXITCODE, $orasOutput)
    }

    if (-not $orasCopySucceeded) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULL_POD_INFRA_CONTAINER -ErrorMessage "Failed to pull '$image'"
    }
    tar -cf $podInfraContainerImageTar -C $podInfraContainerImageDownloadDir .
    if ($LASTEXITCODE -ne 0) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULL_POD_INFRA_CONTAINER -ErrorMessage "failed to create tar for pod infra image from '$podInfraContainerImageDownloadDir'"
    }

    $importOutput = $(ctr.exe -n k8s.io image import --base-name $baseName $podInfraContainerImageTar 2>&1)
    if ($LASTEXITCODE -ne 0) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULL_POD_INFRA_CONTAINER -ErrorMessage ('failed to import ''{0}'': {1}' -f $podInfraContainerImage, $importOutput)
    }

    $finalImage = '{0}:{1}' -f $baseName, $tag
    ctr.exe -n k8s.io image tag ${finalImage} $podInfraContainerImage 2>$null
    ctr.exe -n k8s.io images label $podInfraContainerImage io.cri-containerd.pinned=pinned 2>$null
    Write-Log "Successfully imported '$podInfraContainerImage'"

    Remove-Item -Path $podInfraContainerImageDownloadDir -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -Path $podInfraContainerImageTar -Force -ErrorAction SilentlyContinue
}
