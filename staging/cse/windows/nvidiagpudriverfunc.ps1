function Start-InstallGPUDriver {
    param(
        [Parameter(Mandatory = $true)]
        [bool]$EnableInstall,
        # when the vm size does not have gpu, this value is an empty string.
        [Parameter(Mandatory = $false)]
        [string]$GpuDriverURL
    )
  
    if (-not $EnableInstall) {
        Write-Log "ConfigGPUDriverIfNeeded is false. GPU driver installation skipped as per configuration."
        return
    }
  
    Write-Log "ConfigGPUDriverIfNeeded is true. GPU driver installation started as per configuration."
  
    if ([string]::IsNullOrEmpty($GpuDriverURL)) {
        $ErrorMsg = "DriverURL is not properly specified."
        Write-Log $ErrorMsg
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_SET -ErrorMessage $ErrorMsg
    }

    $fileName = [IO.Path]::GetFileName($GpuDriverURL)
    if (-not $fileName.EndsWith(".exe")) {
        $ErrorMsg = "DriverURL does not point to a exe file"
        Write-Log $ErrorMsg
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_EXE -ErrorMessage $ErrorMsg
    }

    $Target = "C:\AzureData\$fileName"
    $LogFolder = "C:\AzureData"

    Write-Log "Attempting to install Nvidia driver..."

    Prepare-Installation -GpuDriverURL $GpuDriverURL -Target $Target
    Write-Log "Setup complete"
    $IsSignatureValid = VerifySignature $Target 
    if ($IsSignatureValid -eq $false) {
        $ErrorMsg = "Signature embedded in $($Target) is not valid."
        Write-Log $ErrorMsg
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INVALID_SIGNATURE -ErrorMessage $ErrorMsg
    }
    else {
        Write-Log "Signature embedded in $($Target) is valid."
    }

    Write-Log "Installing $Target ..."
    try {
        $InstallLogFolder = "$LogFolder\NvidiaInstallLog"
        $Arguments = "-s -n -log:$InstallLogFolder -loglevel:6"
    
        $p = Start-Process -FilePath $Target -ArgumentList $Arguments -PassThru

        $Timeout = 10 * 60 # in seconds. 10 minutes for timeout of the installation
        Wait-Process -InputObject $p -Timeout $Timeout -ErrorAction Stop
    
        # check if installation was successful
        if ($p.ExitCode -eq 0 -or $p.ExitCode -eq 1) {
            # 1 is issued when reboot is required after success
            Write-Log "GPU Driver Installation Success. Code: $($p.ExitCode)"
            Remove-InstallerFile -InstallerPath $Target
        } 
        else {
            $ErrorMsg = "GPU Driver Installation Failed! Code: $($p.ExitCode)"
            Write-Log $ErrorMsg
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED -ErrorMessage $ErrorMsg
        }

        # check if reboot is needed
        if ($p.ExitCode -eq 1) {
            Write-Log "Nvidia GPU Driver Installation Finished with code 1. Reboot is needed for this GPU Driver..."
            $global:RebootNeeded = $true
        }
        else {
            Set-RebootNeeded
        }
    }
    catch {
        $ErrorMsg = "Exception: $($_.ToString())"
        Write-Log $ErrorMsg # the status file may get over-written when the agent re-attempts this step
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_EXCEPTION -ErrorMessage $ErrorMsg
    }
}
  
function Prepare-Installation {
    [OutputType([hashtable])]
    param(
        [Parameter(Mandatory = $true)]
        [string]$GpuDriverURL,
        [Parameter(Mandatory = $true)]
        [string]$Target
    )

    $fileName = [IO.Path]::GetFileName($GpuDriverURL)

    Write-Log "gpu url is set to $GpuDriverURL"

    Write-Log "Downloading from $GpuDriverURL to $Target"
    DownloadFileOverHttp -Url $GpuDriverURL -DestinationPath $Target -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_DOWNLOAD_FAILURE
}

function Set-RebootNeeded {
    # check if vm size is nv series. if so, set RebootNeeded to be true
    try {
        $Compute = Get-VmData
        $vmSize = $Compute.vmSize
    }
    catch {
        $ErrorMsg = "Failed to query the SKU information."
        Write-Log $ErrorMsg
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_SKU_INFO_NOT_FOUND -ErrorMessage $ErrorMsg
    }

    $IsNVSeries = $vmSize -ne $null -and $vmSize -match "_NV"

    if ($IsNVSeries) {
        Write-Log "Reboot is needed for this GPU Driver..."
        $global:RebootNeeded = $true
    }
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
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_SKU_INFO_NOT_FOUND -ErrorMessage $ErrorMsg
    }

    return $Compute
}

function VerifySignature([string] $targetFile) {
    Write-Log "VerifySignature - Start"
    Write-Log "Verifying signature for $targetFile"
    $fileCertificate = Get-AuthenticodeSignature $targetFile

    if ($fileCertificate.Status -ne "Valid") {
        Write-Log "Signature for $targetFile is not valid"
        return $false
    }

    if ($fileCertificate.SignerCertificate.Subject -eq $fileCertificate.SignerCertificate.Issuer) {
        Write-Log "Signer certificate's Subject matches the Issuer: The certificate is self-signed"
        return $false
    }

    Write-Log "Signature for $targetFile is valid and is not self-signed"
    Write-Log "VerifySignature - End"
    return $true
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