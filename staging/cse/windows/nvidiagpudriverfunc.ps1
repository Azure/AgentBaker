function Start-InstallGPUDriver {
    if (-not $PSScriptRoot)
    {
    $PSScriptRoot = Split-Path $MyInvocation.InvocationName
    }
    if ( -not $env:Path.Contains( "$PSScriptRoot;") )
    {
    $env:Path = "$PSScriptRoot;$env:Path"
    }

    try
    {
    $FatalError = @()

    $LogFolder = "$PSScriptRoot\.."

    $Reboot = @{
        Needed = $false
    }

    Write-Host "Attempting to install Nvidia driver..."

    # Get the SetupTarget based on the input
    $Setup = Get-Setup
    $SetupTarget = $Setup.Target
    $Reboot.Needed = $Setup.RebootNeeded
    Write-Host "Setup complete"

    Add-DriverCertificate $Setup.CertificateUrl
    Write-Host "Certificate in store"

    Write-Host "Installing $SetupTarget ..."
    try
    {
    $InstallLogFolder = "$LogFolder\NvidiaInstallLog"
    $Arguments = "-s -n -log:$InstallLogFolder -loglevel:6"
    
    $p = Start-Process -FilePath $SetupTarget -ArgumentList $Arguments -PassThru

    $Timeout = 10*60 # in seconds. 10 minutes for timeout of the installation
    Wait-Process -InputObject $p -Timeout $Timeout -ErrorAction Stop
    
    # check if installation was successful
    if ($p.ExitCode -eq 0 -or $p.ExitCode -eq 1) # 1 is issued when reboot is required after success
    {
        Write-Host "Done. Code: $($p.ExitCode)"
        # todo: need to reboot if $Reboot is true.
    }
    else
    {
        Write-Host "Failed! Code: $($p.ExitCode)"
    }

    #Invoke-CustomScript.ps1 $retEnv $OpStatus $StatusFile
    }
    catch [System.TimeoutException]
    {
    Write-Host "Timeout $Timeout s exceeded. Stopping the installation process. Reboot for another attempt."
    Stop-Process -InputObject $p
    }
    catch
    {
    $Message = $_.ToString()
    Write-Host "Exception: $Message" # the status file may get over-written when the agent re-attempts this step
    throw
    }
    
    }
    catch [NotSupportedException]
    {
    $Message = $_.ToString()
    Write-Host $Message
    exit 52
    }
    catch
    {
    $FatalError += $_
    Write-Host -ForegroundColor Red ($_ | Out-String)
    exit $FatalError.Count
    }
}

function Get-Setup {
    [OutputType([hashtable])]
    
    # Choose driver and specific properties
    $Driver = Select-Driver
    
    if ($Driver.Url -eq $null -or $Driver.CertificateUrl -eq $null)
    {
      $Message = "DriverURL or DriverCertificateURL are not properly specified."
      throw $Message
    }
    
    $Setup = @{
      RebootNeeded   = $Driver.RebootNeeded
      CertificateUrl = $Driver.CertificateUrl
    }
        
    $InstallFolder = "$PSScriptRoot\.."
    
    $source = $Driver.Url
    $dest = "$InstallFolder\$($Driver.InstallExe)"

    Get-DriverFile $source $dest

    $Setup.Target = "$InstallFolder\$($Driver.InstallExe)"

    return $Setup
}

function Select-Driver {
    [OutputType([hashtable])]

    # Set some default values
    $Driver = @{
    # Hard coding the FwLink for resources.json
    ResourceUrl  = "https://go.microsoft.com/fwlink/?linkid=2181101"
    ResourceFile = "$PSScriptRoot\..\resources.json"
    InstallExe   = "install.exe"
    SetupExe     = "setup.exe"
    RebootNeeded = $false
    }
    $Index = @{
    OS           = 0 # Windows
    }

    # Get OS
    # Using the following instead of VM vmetadata ( $Compute.sku -match ('.*2016-Datacenter.*|.*RS.*Pro.*') ) # 2016, Win 10
    # to accomodate various image publishers and their SKUs
    $OSMajorVersion = (Get-CimInstance Win32_OperatingSystem).version.split('.')[0]
    if ( $OSMajorVersion -eq 10 ) # Win 2016, Win 10
    {
    $Index.OSVersion = 0
    }
    else # ( $OSMajorVersion -eq 6 ) Win 2012 R2
    {
    $Index.OSVersion = 1
    }  

    try
    {
    $Compute = Get-VmData
    $vmSize = $Compute.vmSize
    }
    catch
    {
    Write-Host "Failed to query the SKU information. Attempting to install the extension from customer location regardless."
    }

    # Not an AzureStack scenario
    if ($vmSize -ne $null)
    {
    if ( ($Compute.vmSize -Match "_NC") -or
        ($Compute.vmSize -Match "_ND") ) # Override if GRID driver is desired on ND or NC VMs
    {
        $Index.Driver = 0 # CUDA
    }
    elseif ( $Compute.vmSize -Match "_NV" ) # NV or Grid on ND or Grid on NC
    {
        $Index.Driver = 1 # GRID
        $Driver.RebootNeeded = $true
    }
    else
    {
        throw [NotSupportedException] "VM type $($Compute.vmSize) is not an N-series VM. Not attempting to deploy extension! More information on troubleshooting is available at https://aka.ms/NvidiaGpuDriverWindowsExtensionTroubleshoot"
    }
    }


    # Download and read the resources table
    Get-DriverFile $Driver.ResourceUrl $Driver.ResourceFile
    $Resources = (Get-Content $Driver.ResourceFile) -join "`n" | ConvertFrom-JSON

    # Get the certificate url
    $Driver.CertificateUrl = $Resources.OS[$Index.OS].Certificate.FwLink

    # Get the driver version
    $Driver.ObjectArray = $Resources.OS[$Index.OS].Version[$Index.OSVersion].Driver[$Index.Driver].Version
    $Driver.Object = $Driver.ObjectArray[0] # latest driver

    $Driver.Version = $Driver.Object.Num
    $Driver.Url = $Driver.Object.FwLink


    # $Driver.SetupFolder is set based on OS and Driver Type
    # This cannot be made standard currently as there doesn't seem to be a way to pass the extraction/setup folder in silent mode
    if ($Index.Driver -eq 0) # CUDA
    {
    if ($Index.OSVersion -eq 0) # Win 2016, Win 10
    {
        $Driver.SetupFolder = "C:\NVIDIA\DisplayDriver\$($Driver.Version)\Win10_64\International"
    }
    else # Win 2012 R2
    {
        $Driver.SetupFolder = "C:\NVIDIA\DisplayDriver\$($Driver.Version)\Win8_Win7_64\International"
    }
    }
    else # GRID
    {
    $Driver.SetupFolder = "C:\NVIDIA\$($Driver.Version)"
    }


    return $Driver
}

function Get-VmData
{
    [OutputType([hashtable])]

    # Retry to overcome failure in retrieving VM metadata
    $Loop = $true
    $RetryCount = 0
    $RetryCountMax = 10
    do
    {
    try
    {
        $Compute=Invoke-RestMethod -Headers @{"Metadata"="true"} -URI http://169.254.169.254/metadata/instance/compute?api-version=2017-08-01 -Method get
        $Loop = $false
    }
    catch
    {
        if ($RetryCount -gt $RetryCountMax)
        {
        $Loop = $false
        throw [NotSupportedException] "Failed to retrieve VM metadata after $RetryCountMax attempts. Exiting! More information on troubleshooting is available at https://aka.ms/NvidiaGpuDriverWindowsExtensionTroubleshoot"
        }
        else
        {
        Start-Sleep -Seconds ($RetryCount * 2 + 1)
        $RetryCount++
        }
    }
    } while ($Loop)

    return $Compute
}

function Get-DriverFile
{
  param(
    [Parameter(Mandatory=$True)]
    [ValidateNotNullOrEmpty()]
    [string] $source,
  
    [Parameter(Mandatory=$True)]
    [ValidateNotNullOrEmpty()]
    [string] $dest
  )
  
  function GetUsingWebClient
  {
    # Store Security Protocols
    $protocols = [Net.ServicePointManager]::SecurityProtocol
    # Add Tls12 to Security Protocols
    [Net.ServicePointManager]::SecurityProtocol = ([Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12)

    $wc = New-Object System.Net.WebClient
    $start_time = Get-Date
    $wc.DownloadFile($source, $dest)
    Write-Host "Time taken: $((Get-Date).Subtract($start_time).Seconds) second(s)"

    # Reset Security Protocols
    [Net.ServicePointManager]::SecurityProtocol = $protocols
  }
  
  function GetUsingInvokeWebRequest
  {
    $start_time = Get-Date
    Invoke-WebRequest -Uri $source -OutFile $dest
    Write-Host "Time taken: $((Get-Date).Subtract($start_time).Seconds) second(s)"
  }
  
  # Invoke-WebRequest seems slower for large files.
  # Using System.Net/WebClient instead. Apparently wget (and csource) are aliases

  Write-Host "Downloading from $source to $dest"

  # Retry to overcome failure in downloading
  $Loop = $true
  $RetryCount = 0
  $RetryCountMax = 10
  do
  {
    try
    {
      GetUsingWebClient
      $Loop = $false
    }
    catch
    {
      if ($RetryCount -gt $RetryCountMax)
      {
        $Loop = $false
        throw [NotSupportedException] "Failed to download $source after $RetryCountMax attempts. Exiting! More information on troubleshooting is available at https://aka.ms/NvidiaGpuDriverWindowsExtensionTroubleshoot"
      }
      else
      {
        Start-Sleep -Seconds ($RetryCount * 2 + 1)
        $RetryCount++
      }
    }
  } while ($Loop)
}

function Add-DriverCertificate
{
  param(
    [Parameter(Mandatory=$True)]
    [ValidateNotNullOrEmpty()]
    [string] $link
  )

  $Cert = Get-DriverCertificate $link
  if ( $Cert.OldObject )
  {
    Write-Host 'Certificate already in store.'
  }
  else
  {
    Write-Host 'Adding Certificate ...'
    $Store = Get-Item $Cert.Store
    $Store.Open([System.Security.Cryptography.X509Certificates.OpenFlags]::ReadWrite)
    Import-Certificate -Filepath $Cert.File -CertStoreLocation $Cert.Store
    Write-Host 'Certificate added.'
    $Store.Close()
 }
}
 
function Get-DriverCertificate
{
    param(
        [Parameter(Mandatory=$True)]
        [ValidateNotNullOrEmpty()]
        [string] $link
    )

    # Defaults
    $Cert = @{
        File = "$PSScriptRoot\..\nvidia.cer"
        Store = "Cert:\LocalMachine\TrustedPublisher\"
        OldObject = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2 #placeholder
        NewObject = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2
    }

    # Download the .cer file if it doesn't already exist.
    if ( !(Test-Path -Path $Cert.File -PathType Leaf) )
    {
        $source = $link
        $dest = $Cert.File
        Get-DriverFile $source $dest
    }

    # New Certificate object to check for properties
    $Cert.NewObject.Import($Cert.File)

    # Checking for existing cert with a unique thumbprint
    $Cert.OldObject = ( Get-ChildItem -Path $Cert.Store | Where-Object {$_.Thumbprint -Match $Cert.NewObject.Thumbprint} )

    return $Cert
}