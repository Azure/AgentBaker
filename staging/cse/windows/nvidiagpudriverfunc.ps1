function Start-InstallGPUDriver {
    param(
        [Parameter(Mandatory = $true)]
        [bool]$EnableInstall,
        [Parameter(Mandatory = $true)]
        [PSCustomObject]$DriverConfig
    )
  
    if (-not $EnableInstall) {
        Write-Log "ConfigGPUDriverIfNeeded is false. GPU driver installation skipped as per configuration."
        return
    }
  
    Write-Log "ConfigGPUDriverIfNeeded is true. GPU driver installation started as per configuration."
  
    $RootDir = "C:\AzureData\Windows"
  
    try {
        $FatalError = @()
  
        $LogFolder = "$RootDir\.."
  
        Write-Log "Attempting to install Nvidia driver..."
  
        # Get the SetupTarget based on the input
        $Setup = Get-Setup -DriverConfig $DriverConfig
        $SetupTarget = $Setup.Target
        Write-Log "Setup complete"
        $IsSignatureValid = VerifySignature $SetupTarget 
        if ($IsSignatureValid -eq $false) {
            $ErrorMsg = "Signature embedded in $($SetupTarget) is not valid."
            Write-Log $ErrorMsg
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED -ErrorMessage $ErrorMsg
        }
        else {
            Write-Log "Signature embedded in $($SetupTarget) is valid."
        }

  
        Write-Log "Installing $SetupTarget ..."
        try {
            $InstallLogFolder = "$LogFolder\NvidiaInstallLog"
            $Arguments = "-s -n -log:$InstallLogFolder -loglevel:6"
      
            $p = Start-Process -FilePath $SetupTarget -ArgumentList $Arguments -PassThru
  
            $Timeout = 10 * 60 # in seconds. 10 minutes for timeout of the installation
            Wait-Process -InputObject $p -Timeout $Timeout -ErrorAction Stop
      
            # check if installation was successful
            if ($p.ExitCode -eq 0 -or $p.ExitCode -eq 1) {
                # 1 is issued when reboot is required after success
                Write-Log "GPU Driver Installation Success. Code: $($p.ExitCode)"
            }
            else {
                $ErrorMsg = "GPU Driver Installation Failed! Code: $($p.ExitCode)"
                Write-Log $ErrorMsg
                Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED -ErrorMessage $ErrorMsg
            }
  
            if ($Setup.RebootNeeded -or $p.ExitCode -eq 1) {
                Write-Log "Reboot is needed for this GPU Driver..."
                $global:RebootNeeded = $true
            }
        }
        catch [System.TimeoutException] {
            $ErrorMsg = "Timeout $Timeout s exceeded. Stopping the installation process. Reboot for another attempt."
            Write-Log $ErrorMsg
            Stop-Process -InputObject $p -Force
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_TIMEOUT -ErrorMessage $ErrorMsg
        }
        catch {
            $ErrorMsg = "Exception: $($_.ToString())"
            Write-Log $ErrorMsg # the status file may get over-written when the agent re-attempts this step
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED -ErrorMessage $ErrorMsg
        }
      
    }
    catch {
        $FatalError += $_
        $errorCount = $FatalError.Count
        $ErrorMsg = "A fatal error occurred. Number of errors: $errorCount. Error details: $($_ | Out-String)"
        Write-Log $ErrorMsg
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED -ErrorMessage $ErrorMsg
    }
}
  
function Get-Setup {
    param(
        [Parameter(Mandatory = $true)]
        [PSCustomObject]$DriverConfig
    )

    [OutputType([hashtable])]
    
    $GpuDriverURL = $DriverConfig.GpuDriverURL
    $fileName = [IO.Path]::GetFileName($GpuDriverURL)
      
    if ($GpuDriverURL -eq $null) {
        $ErrorMsg = "DriverURL is not properly specified."
        Write-Log $ErrorMsg
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_SET -ErrorMessage $ErrorMsg
    }

    Write-Log "gpu url is set to $GpuDriverURL"

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
      
    $Setup = @{
        RebootNeeded = ($vmSize -ne $null -and $Compute.vmSize -match "_NV")
        Target       = "$RootDir\..\$fileName"
    }

    Write-Log "Downloading from $GpuDriverURL to $($Setup.Target)"
    DownloadFileOverHttp -Url $GpuDriverURL -DestinationPath $Setup.Target -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_DOWNLOAD_FAILURE
  
    return $Setup
}
  
function Get-VmData {
    [OutputType([hashtable])]
  
    # Retry to overcome failure in retrieving VM metadata
    $Loop = $true
    $RetryCount = 0
    $RetryCountMax = 10
    do {
        try {
            $Compute = Invoke-RestMethod -Headers @{"Metadata" = "true" } -URI http://169.254.169.254/metadata/instance/compute?api-version=2017-08-01 -Method get
            $Loop = $false
        }
        catch {
            if ($RetryCount -gt $RetryCountMax) {
                $Loop = $false
                $ErrorMsg = "Failed to retrieve VM metadata after $RetryCountMax attempts. Exiting! More information on troubleshooting is available at https://aka.ms/NvidiaGpuDriverWindowsExtensionTroubleshoot"
                Write-Log $ErrorMsg
                Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_SKU_INFO_NOT_FOUND -ErrorMessage $ErrorMsg
            }
            else {
                $retryInSeconds = $RetryCount * 2 + 1
                Write-Log "Attempt $RetryCount of $RetryCountMax failed. Retrying in $retryInSeconds seconds."
                Start-Sleep -Seconds ($retryInSeconds)
                $RetryCount++
            }
        }
    } while ($Loop)
  
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