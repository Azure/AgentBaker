function Start-InstallGPUDriver {
    param(
        [Parameter(Mandatory = $true)]
        [bool]$EnableInstall,
        # when the vm size does not have gpu, this value is an empty string.
        [Parameter(Mandatory = $false)]
        [string]$GpuDriverURL
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.InstallGPUDriver" -TaskMessage "Start to install GPU driver. ConfigGPUDriverIfNeeded: $global:ConfigGPUDriverIfNeeded, GpuDriverURL: $global:GpuDriverURL"
  
    if (-not $EnableInstall) {
        Write-Log "ConfigGPUDriverIfNeeded is false. GPU driver installation skipped as per configuration."
        return
    }
  
    Write-Log "ConfigGPUDriverIfNeeded is true. GPU driver installation started as per configuration."
  
    if ([string]::IsNullOrEmpty($GpuDriverURL)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_SET -ErrorMessage "DriverURL is not properly specified."
    }

    $fileName = [IO.Path]::GetFileName($GpuDriverURL)
    if (-not $fileName.EndsWith(".exe")) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_EXE -ErrorMessage "DriverURL does not point to a exe file"
    }

    $Target = "C:\AzureData\$fileName"
    $LogFolder = "C:\AzureData"

    Write-Log "Attempting to install Nvidia driver..."

    Write-Log "Downloading from $GpuDriverURL to $Target"
    DownloadFileOverHttp -Url $GpuDriverURL -DestinationPath $Target -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_DOWNLOAD_FAILURE
    Write-Log "Installer download complete"
    
    VerifySignature $Target 

    Write-Log "Installing $Target ..."
    try {
        $InstallLogFolder = "$LogFolder\NvidiaInstallLog"
        $Arguments = "-s -n -log:$InstallLogFolder -loglevel:6"
    
        $p = Start-Process -FilePath $Target -ArgumentList $Arguments -PassThru
        
        $Timeout = 10 * 60 # in seconds. 10 minutes for timeout of the installation

        # This is for testability. Start-Process mock returns a hashtable.
        if (-not ($p -is [hashtable])) {
            Wait-Process -InputObject $p -Timeout $Timeout -ErrorAction Stop
        }

        Handle-InstallationResult -ErrorCode $p.ExitCode
    }
    catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_EXCEPTION -ErrorMessage "Exception: $($_.ToString())"
    }
}

function Handle-InstallationResult {
    param(
        [Parameter(Mandatory = $true)]
        [int]$ErrorCode
    )

    if ($ErrorCode -ne 0 -and $ErrorCode -ne 1) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED -ErrorMessage "GPU Driver Installation Failed! Code: $ErrorCode"
    }

    if ($ErrorCode -eq 0) {
        Write-Log "GPU Driver Installation Success. Code: $ErrorCode"
        # check if vm size is nv series. if so, set RebootNeeded to be true
        try {
            $Compute = Get-VmData
            $vmSize = $Compute.vmSize
        }
        catch {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_SKU_INFO_NOT_FOUND -ErrorMessage "Failed to query the SKU information."
        }

        $IsNVSeries = $vmSize -ne $null -and $vmSize -match "standard_nv" #case insensitive

        if ($IsNVSeries -eq $true) {
            Write-Log "Reboot is needed for this GPU Driver..."
            $global:RebootNeeded = $true
        }
    }
    elseif ($ErrorCode -eq 1) {
        Write-Log "GPU Driver Installation Success. Code: $ErrorCode. Reboot is needed for this installation..."
        $global:RebootNeeded = $true
    }

    Remove-InstallerFile -InstallerPath $Target
}
  
function Get-VmData {
    [OutputType([hashtable])]

    $arguments = @{
        Headers = @{"Metadata" = "true" }
        URI     = "http://169.254.169.254/metadata/instance/compute?api-version=2017-08-01"
        Method  = "get"
    }

    try {
        $Compute = Retry-Command -Command Invoke-RestMethod -Args $arguments -Retries 5 -RetryDelaySeconds 10
    }
    catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_SKU_INFO_NOT_FOUND -ErrorMessage "Failed to query the SKU information."
    }

    return $Compute
}

function VerifySignature([string] $targetFile) {
    Write-Log "VerifySignature - Start"
    Write-Log "Verifying signature for $targetFile"
    $fileCertificate = Get-AuthenticodeSignature $targetFile

    if ($fileCertificate.Status -ne "Valid") {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INVALID_SIGNATURE -ErrorMessage "Signature embedded in $($Target) is not valid."
    }

    if ($fileCertificate.SignerCertificate.Subject -eq $fileCertificate.SignerCertificate.Issuer) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INVALID_SIGNATURE -ErrorMessage "Signer certificate's Subject in $($Target) matches the Issuer: The certificate is self-signed"
    }

    Write-Log "Signature for $targetFile is valid and is not self-signed"
    Write-Log "VerifySignature - End"
}

function Remove-InstallerFile {
    param(
        [Parameter(Mandatory = $true)]
        [string]$InstallerPath
    )
  
    Write-Log "Attempting to remove installer file at $InstallerPath..."
  
    try {
        Remove-Item -Path $InstallerPath -Force
        Write-Log "Installer file removed successfully."
    }
    catch {
        Write-Log "Failed to remove installer file. Error: $($_.Exception.Message)"
    }
}