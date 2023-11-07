function Start-InstallGPUDriver {
    param(
        [Parameter(Mandatory = $true)]
        [bool]$EnableInstall,
        [Parameter(Mandatory = $true)]
        [PSCustomObject]$DriverUrlConfig
    )
  
    if (-not $EnableInstall) {
        Write-ConsoleLog "ConfigGPUDriverIfNeeded is false. GPU driver installation skipped as per configuration."
        return $false
    }
  
    Write-ConsoleLog "ConfigGPUDriverIfNeeded is true. GPU driver installation started as per configuration."
  
    $RebootNeeded = $false
  
    $RootDir = "C:\AzureData\Windows"
  
    try {
        $FatalError = @()
  
        $LogFolder = "$RootDir\.."
  
        Write-ConsoleLog "Attempting to install Nvidia driver..."
  
        # Get the SetupTarget based on the input
        $Setup = Get-Setup -DriverUrlConfig $DriverUrlConfig
        $SetupTarget = $Setup.Target
        Write-ConsoleLog "Setup complete"
        $IsSignatureValid = VerifySignature $SetupTarget 
        if ($IsSignatureValid -eq $false) {
            $ErrorMsg = "Signature embedded in $($SetupTarget) is not valid."
            Write-ConsoleLog $ErrorMsg
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED -ErrorMessage $ErrorMsg
        }
        else {
            Write-ConsoleLog "Signature embedded in $($SetupTarget) is valid."
        }

  
        Write-ConsoleLog "Installing $SetupTarget ..."
        try {
            $InstallLogFolder = "$LogFolder\NvidiaInstallLog"
            $Arguments = "-s -n -log:$InstallLogFolder -loglevel:6"
      
            $p = Start-Process -FilePath $SetupTarget -ArgumentList $Arguments -PassThru
  
            $Timeout = 10 * 60 # in seconds. 10 minutes for timeout of the installation
            Wait-Process -InputObject $p -Timeout $Timeout -ErrorAction Stop
      
            # check if installation was successful
            if ($p.ExitCode -eq 0 -or $p.ExitCode -eq 1) {
                # 1 is issued when reboot is required after success
                Write-ConsoleLog "GPU Driver Installation Success. Code: $($p.ExitCode)"
            }
            else {
                $ErrorMsg = "GPU Driver Installation Failed! Code: $($p.ExitCode)"
                Write-ConsoleLog $ErrorMsg
                Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED -ErrorMessage $ErrorMsg
            }
  
            if ($Setup.RebootNeeded -or $p.ExitCode -eq 1) {
                Write-ConsoleLog "Reboot is needed for this GPU Driver..."
                $RebootNeeded = $true
            }
            return $RebootNeeded
        }
        catch [System.TimeoutException] {
            $ErrorMsg = "Timeout $Timeout s exceeded. Stopping the installation process. Reboot for another attempt."
            Write-ConsoleLog $ErrorMsg
            Stop-Process -InputObject $p -Force
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_TIMEOUT -ErrorMessage $ErrorMsg
        }
        catch {
            $ErrorMsg = "Exception: $($_.ToString())"
            Write-ConsoleLog $ErrorMsg # the status file may get over-written when the agent re-attempts this step
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED -ErrorMessage $ErrorMsg
        }
      
    }
    catch {
        $FatalError += $_
        $errorCount = $FatalError.Count
        $ErrorMsg = "A fatal error occurred. Number of errors: $errorCount. Error details: $($_ | Out-String)"
        Write-ConsoleLog $ErrorMsg
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_FAILED -ErrorMessage $ErrorMsg
    }
}
  
function Get-Setup {
    param(
        [Parameter(Mandatory = $true)]
        [PSCustomObject]$DriverUrlConfig
    )

    [OutputType([hashtable])]
    
    $GpuDriverURL = $DriverUrlConfig.GpuDriverURL
      
    if ($GpuDriverURL -eq $null) {
        $ErrorMsg = "DriverURL is not properly specified."
        Write-ConsoleLog $ErrorMsg
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_URL_NOT_SET -ErrorMessage $ErrorMsg
    }

    Write-ConsoleLog "gpu url is set to $GpuDriverURL"

    # check if vm size is nv series. if so, set RebootNeeded to be true
    try {
        $Compute = Get-VmData
        $vmSize = $Compute.vmSize
    }
    catch {
        $ErrorMsg = "Failed to query the SKU information."
        Write-ConsoleLog $ErrorMsg
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_SKU_INFO_NOT_FOUND -ErrorMessage $ErrorMsg
    }
      
    $Setup = @{
        RebootNeeded = ($vmSize -ne $null -and $Compute.vmSize -match "_NV")
        Target       = "$RootDir\..\install.exe"
    }

    Get-DriverFile $GpuDriverURL $Setup.Target
  
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
                Write-ConsoleLog $ErrorMsg
                Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_SKU_INFO_NOT_FOUND -ErrorMessage $ErrorMsg
            }
            else {
                $retryInSeconds = $RetryCount * 2 + 1
                Write-ConsoleLog "Attempt $RetryCount of $RetryCountMax failed. Retrying in $retryInSeconds seconds."
                Start-Sleep -Seconds ($retryInSeconds)
                $RetryCount++
            }
        }
    } while ($Loop)
  
    return $Compute
}
  
function Get-DriverFile {
    param(
        [Parameter(Mandatory = $True)]
        [ValidateNotNullOrEmpty()]
        [string] $source,
    
        [Parameter(Mandatory = $True)]
        [ValidateNotNullOrEmpty()]
        [string] $dest
    )
  
    Write-ConsoleLog "Downloading from $source to $dest"
  
    # Retry to overcome failure in downloading
    $Loop = $true
    $RetryCount = 0
    $RetryCountMax = 10
    do {
        try {
            Get-UsingWebClient -Url $source -OutputPath $dest
            $Loop = $false
            Write-ConsoleLog "Downloaded file successfully."
            break
        }
        catch {
            if ($RetryCount -gt $RetryCountMax) {
                $Loop = $false
                $ErrorMsg = "Failed to download $source after $RetryCountMax attempts. Exiting! More information on troubleshooting is available at https://aka.ms/NvidiaGpuDriverWindowsExtensionTroubleshoot"
                Write-ConsoleLog $ErrorMsg
                Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GPU_DRIVER_INSTALLATION_DOWNLOAD_FAILURE -ErrorMessage $ErrorMsg
            }
            else {
                Start-Sleep -Seconds ($RetryCount * 2 + 1)
                $RetryCount++
            }
        }
    } while ($Loop)
}
function Get-UsingWebClient {
    param (
        [string] $Url,
        [string] $OutputPath
    )

    try {
        # Store Security Protocols
        $protocols = [Net.ServicePointManager]::SecurityProtocol
        # Add Tls12 to Security Protocols
        [Net.ServicePointManager]::SecurityProtocol = ([Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12)

        $wc = New-Object System.Net.WebClient
        $start_time = Get-Date
        $wc.DownloadFile($Url, $OutputPath)
        Write-ConsoleLog "Time taken: $((Get-Date).Subtract($start_time).Seconds) second(s)"
    }
    finally {
        # Reset Security Protocols
        [Net.ServicePointManager]::SecurityProtocol = $protocols
    }
}

function VerifySignature([string] $targetFile) {
    Write-ConsoleLog "VerifySignature - Start"
    Write-ConsoleLog "Verifying signature for $targetFile"
    $fileCertificate = Get-AuthenticodeSignature $targetFile

    if ($fileCertificate.Status -ne "Valid") {
        Write-ConsoleLog "Signature for $targetFile is not valid"
        return $false
    }

    if ($fileCertificate.SignerCertificate.Subject -eq $fileCertificate.SignerCertificate.Issuer) {
        Write-ConsoleLog "Signer certificate's Subject matches the Issuer: The certificate is self-signed"
        return $false
    }

    Write-ConsoleLog "Signature for $targetFile is valid and is not self-signed"
    Write-ConsoleLog "VerifySignature - End"
    return $true
}
function Write-ConsoleLog($message) {
    $msg = $message | Timestamp
    Write-Host $msg
}
