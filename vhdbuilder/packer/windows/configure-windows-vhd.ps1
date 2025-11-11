<#
    .SYNOPSIS
        Used to produce Windows AKS images.

    .DESCRIPTION
        This script is used by packer to produce Windows AKS images.
#>

param(
    [string]$windowsSKUParam,
    [string]$provisioningPhaseParam,
    [string]$customizedDiskSizeParam
)

if (![string]::IsNullOrEmpty($windowsSKUParam))
{
    Write-Log "Setting Windows SKU to $windowsSKUParam"
    $env:WindowsSKU = $windowsSKUParam
}

if (![string]::IsNullOrEmpty($provisioningPhaseParam))
{
    Write-Log "Setting Provisioning Phase to $provisioningPhaseParam"
    $env:ProvisioningPhase = $provisioningPhaseParam
}

if (![string]::IsNullOrEmpty($customizedDiskSizeParam))
{
    Write-Log "Setting Customized Disk Size to $customizedDiskSizeParam"
    $env:CustomizedDiskSize = $customizedDiskSizeParam
}

$ErrorActionPreference = "Stop"

filter Timestamp
{
    "$( Get-Date -Format o ): $_"
}

function Write-Log($Message)
{
    $msg = $message | Timestamp
    Write-Host $msg
}

. c:/k/windows-vhd-configuration.ps1


function Log-VHDFreeSize
{
    Write-Log "Get Disk info"
    $disksInfo = Get-CimInstance -ClassName Win32_LogicalDisk
    foreach ($disk in $disksInfo)
    {
        if ($disk.DeviceID -eq "C:")
        {
            if ($disk.FreeSpace -lt $global:lowestFreeSpace)
            {
                Write-Log "Disk C: Free space $( $disk.FreeSpace ) is less than $( $global:lowestFreeSpace )"
            }
            break
        }

        # the break above means we'll only print this where there is no error.
        Write-Log "Disk $( $disk.DeviceID ) has free space $( $disk.FreeSpace )"
    }
}

function Download-File
{
    param (
        $URL,
        $Dest,
        $retryCount = 5,
        $retryDelay = 0,
        [Switch]$redactUrl = $false
    )
    # replicate same check as in windowscsehelper.ps1, check if the file already exists before downloading
    $cleanUrl = $URL.Split('?')[0]
    $fileName = [IO.Path]::GetFileName($cleanUrl)

    $search = @()
    if ($global:cacheDir -and (Test-Path $global:cacheDir)) {
        $search = [IO.Directory]::GetFiles($global:cacheDir, $fileName, [IO.SearchOption]::AllDirectories)
    }

    if ($search.Count -ne 0) {
        Write-Log "Package exist $fileName in cache dir $global:CacheDir, skipping download"
        Get-ChildItem "$Dest"
        return
    }

    Write-Log "Downloading $URL to $Dest"
    curl.exe -f --retry $retryCount --retry-delay $retryDelay -L $URL -o $Dest
    $curlExitCode = $LASTEXITCODE
    if ($curlExitCode)
    {
        $logURL = $URL
        if ($redactUrl)
        {
            $logURL = $logURL.Split("?")[0]
        }
        Log-VHDFreeSize
        curl.exe --version
        if ("$curlExitCode" -eq "23") {
            throw "Curl exited with '$curlExitCode' while attempting to download '$logURL' to '$Dest'. This often means VHD out of space."
        }
        throw "Curl exited with '$curlExitCode' while attempting to download '$logURL' to '$Dest'"
    }
    Get-ChildItem "$Dest"
}

function Download-FileWithAzCopy
{
    param (
        $URL,
        $Dest
    )


    if (!(Test-Path -Path $global:aksTempDir))
    {
        Write-Log "Creating temp dir for tools of building vhd"
        New-Item -ItemType Directory $global:aksTempDir -Force
    }

    if (!(Test-Path -Path "$global:aksTempDir\azcopy.exe"))
    {
        Write-Log "Downloading azcopy"
        Invoke-WebRequest -UseBasicParsing "https://aka.ms/downloadazcopy-v10-windows" -OutFile "$global:aksTempDir\azcopy.zip"
        Expand-Archive -Path "$global:aksTempDir\azcopy.zip" -DestinationPath "$global:aksTempDir\tmp" -Force
        Move-Item "$global:aksTempDir\tmp\*\azcopy.exe" "$global:aksTempDir\azcopy.exe"
    }

    pushd "$global:aksTempDir"
    $env:AZCOPY_JOB_PLAN_LOCATION = "$global:aksTempDir\azcopy"
    $env:AZCOPY_LOG_LOCATION = "$global:aksTempDir\azcopy"

    mkdir -Force $env:AZCOPY_LOG_LOCATION
    if (Test-Path -Path "$env:AZCOPY_LOG_LOCATION\*.log")
    {
        rm -Force "$env:AZCOPY_LOG_LOCATION\*.log"
    }

    Write-Log "Logging in to AzCopy"
    # user_assigned_managed_identities has been bound in vhdbuilder/packer/windows/windows-vhd-builder-sig.json
    .\azcopy.exe login --login-type=MSI

    Write-Log "Copying $URL to $Dest"
    .\azcopy.exe copy "$URL" "$Dest"

    dir "$Dest"

    Write-Log "--- START AzCopy Log"
    Get-Content "$env:AZCOPY_LOG_LOCATION\*.log" | Write-Log
    Write-Log "--- END AzCopy Log"
    popd
}

function Pull-OCIArtifact
{
    param (
        $Name,
        $Dest
    )

    if (!(Test-Path -Path "$global:aksTempDir\oras.exe"))
    {
        if (!(Test-Path -Path "$global:aksTempDir"))
        {
            Write-Log "Creating temp dir $global:aksTempDir for oras"
            New-Item -ItemType Directory $global:aksTempDir -Force
        }

        $orasVersion = '1.2.3'
        $orasZip = "oras_${orasVersion}_windows_amd64.zip"
        $orasUrl = "https://github.com/oras-project/oras/releases/download/v${orasVersion}/${orasZip}"

        Write-Log "Downloading oras v${orasVersion}"
        Invoke-WebRequest -UseBasicParsing $orasUrl -OutFile "$global:aksTempDir\$orasZip"
        Expand-Archive -Path "$global:aksTempDir\$orasZip" -DestinationPath "$global:aksTempDir" -Force
    }

    Push-Location $global:aksTempDir

    try
    {
        Write-Log "Pulling OCI artifact $Name"

        if (!(Test-Path -Path $Dest))
        {
            Write-Log "Creating destination directory $Dest"
            New-Item -ItemType Directory -Path $Dest -Force | Out-Null
        }

        .\oras.exe pull $Name -o $Dest
        $orasExitCode = $LASTEXITCODE
        if ($orasExitCode)
        {
            Log-VHDFreeSize
            .\oras.exe version
            throw "Oras pull failed with exit code $orasExitCode"
        }

        Get-ChildItem "$Dest"
    }
    finally
    {
        Pop-Location
    }
}

function Cleanup-TemporaryFiles
{
    if (Test-Path -Path $global:aksTempDir)
    {
        Remove-Item -Path $global:aksTempDir -Force -Recurse
    }
}

function Retry-Command
{
    [CmdletBinding()]
    Param(
        [Parameter(Position = 0, Mandatory = $true)]
        [scriptblock]$ScriptBlock,

        [Parameter(Position = 1, Mandatory = $true)]
        [string]$ErrorMessage,

        [Parameter(Position = 2, Mandatory = $false)]
        [int]$Maximum = 5,

        [Parameter(Position = 3, Mandatory = $false)]
        [int]$Delay = 10
    )

    Begin {
        $cnt = 0
    }

    Process {
        do
        {
            $cnt++
            try
            {
                $ScriptBlock.Invoke()
                if ($LASTEXITCODE)
                {
                    throw "Retry $cnt : $ErrorMessage"
                }
                return
            }
            catch
            {
                Write-Log $_.Exception.InnerException.Message
                if ( $_.Exception.InnerException.Message.Contains("There is not enough space on the disk."))
                {
                    Write-Error "Exit retry since there is not enough space on the disk"
                    break
                }
                if ( $_.Exception.InnerException.Message.Contains("The device is not connected.: unknown."))
                {
                    Write-Error "Exit retry since drive disconnected (usually means that disk is out of space)"
                    break
                }
                Write-Log "Retry $cnt : $ScriptBlock"
                Start-Sleep -Seconds $Delay
            }
        } while ($cnt -lt $Maximum)

        # Throw an error after $Maximum unsuccessful invocations. Doesn't need
        # a condition, since the function returns upon successful invocation.
        throw 'All retries failed. $ErrorMessage'
    }
}

function Invoke-Executable
{
    Param(
        [string]
        $Executable,
        [string[]]
        $ArgList,
        [int]
        $Retries = 0,
        [int]
        $RetryDelaySeconds = 1
    )

    for ($i = 0; $i -le $Retries; $i++) {
        Write-Log "$i - Running $Executable $ArgList ..."
        & $Executable $ArgList
        if ($LASTEXITCODE)
        {
            Write-Log "$Executable returned unsuccessfully with exit code $LASTEXITCODE"
            Start-Sleep -Seconds $RetryDelaySeconds
            continue
        }
        else
        {
            Write-Log "$Executable returned successfully"
            return
        }
    }

    Write-Log "Exhausted retries for $Executable $ArgList"
    throw "Exhausted retries for $Executable $ArgList"
}

function Expand-OS-Partition
{
    $customizedDiskSize = $env:CustomizedDiskSize
    if ( [string]::IsNullOrEmpty($customizedDiskSize))
    {
        Write-Log "No need to expand the OS partition size"
        return
    }

    Write-Log "Customized OS disk size is $customizedDiskSize GB"
    [Int32]$osPartitionSize = 0
    if ( [Int32]::TryParse($customizedDiskSize, [ref]$osPartitionSize))
    {
        # The supportedMaxSize less than the customizedDiskSize because some system usages will occupy disks (about 500M).
        $supportedMaxSize = (Get-PartitionSupportedSize -DriveLetter C).sizeMax
        $currentSize = (Get-Partition -DriveLetter C).Size
        if ($supportedMaxSize -gt $currentSize)
        {
            Write-Log "Resizing the OS partition size from $currentSize to $supportedMaxSize"
            Resize-Partition -DriveLetter C -Size $supportedMaxSize
            Get-Disk
            Get-Partition
        }
        else
        {
            Write-Log "The current size is the max size $currentSize"
        }
    }
    else
    {
        Throw "$customizedDiskSize is not a valid customized OS disk size"
    }
}

function Disable-WindowsUpdates
{
    # See https://docs.microsoft.com/en-us/windows/deployment/update/waas-wu-settings
    # for additional information on WU related registry settings

    Write-Log "Disabling automatic windows upates"
    $WindowsUpdatePath = "HKLM:SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate"
    $AutoUpdatePath = "HKLM:SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU"

    if (Test-Path -Path $WindowsUpdatePath)
    {
        Remove-Item -Path $WindowsUpdatePath -Recurse
    }

    New-Item -Path $WindowsUpdatePath | Out-Null
    New-Item -Path $AutoUpdatePath | Out-Null
    Set-ItemProperty -Path $AutoUpdatePath -Name NoAutoUpdate -Value 1 | Out-Null
}

function Get-ContainerImages
{
    Write-Log "Pulling images for windows server $windowsSKU" # The variable $windowsSKU will be "2019-containerd", "2022-containerd", ...
    foreach ($image in $imagesToPull)
    {
        Write-Host "* $image"
    }

    # ./.clusterfuzzliteThere is a regression in crictl.exe in kube-tools 1.33 for windows that it cannot find the default config file. Has been discussed with upstream and pending fix
    $crictlPath = (Get-Command crictl.exe -ErrorAction SilentlyContinue).Path
    $configPath = Join-Path (Split-Path -Parent $crictlPath) "crictl.yaml"

    foreach ($image in $imagesToPull)
    {
        $imagePrefix = $image.Split(":")[0]
        if (($imagePrefix -eq "mcr.microsoft.com/windows/servercore" -and ![string]::IsNullOrEmpty($env:WindowsServerCoreImageURL)) -or
                ($imagePrefix -eq "mcr.microsoft.com/windows/nanoserver" -and ![string]::IsNullOrEmpty($env:WindowsNanoServerImageURL)))
        {
            $url = ""
            if ( $image.Contains("mcr.microsoft.com/windows/servercore"))
            {
                $url = $env:WindowsServerCoreImageURL
            }
            elseif ($image.Contains("mcr.microsoft.com/windows/nanoserver"))
            {
                $url = $env:WindowsNanoServerImageURL
            }

            # In 2025 case, we will need to cache container base images for both 2022 and 2025
            # To support multiple versions of container base images, we expect the URL to be provided as a list of strings
            # seperated by commas or semicolons, while each string specify a version of the image.
            # For example: mcr.microsoft.com/windows/servercore:ltsc2022, mcr.microsoft.com/windows/servercore:ltsc2025
            $containerBaseImageurls = $url -split '\s*[;,]\s*'

            foreach ($url in $containerBaseImageurls)
            {
                $fileName = [IO.Path]::GetFileName($url.Split("?")[0])
                $tmpDest = [IO.Path]::Combine([System.IO.Path]::GetTempPath(), $fileName)
                Write-Log "Downloading image $image to $tmpDest"
                Download-FileWithAzCopy -URL $url -Dest $tmpDest

                Write-Log "Loading image $image from $tmpDest"
                Retry-Command -ScriptBlock {
                    & ctr -n k8s.io images import $tmpDest
                } -ErrorMessage "Failed to load image $image from $tmpDest"

                Write-Log "Removing tmp tar file $tmpDest"
                Remove-Item -Path $tmpDest
            }
        }
        else
        {
            Write-Log "Pulling image $image"
            Retry-Command -ScriptBlock {
                & crictl.exe -c $configPath pull $image
            } -ErrorMessage "Failed to pull image $image"
        }
    }

    # before stopping containerd, let's echo the cached images and their sizes.
    crictl -c $configPath images show

    Stop-Job  -Name containerd
    Remove-Job -Name containerd
}

function Get-PackagesToCacheOnVHD
{
    Write-Log "Caching packages on VHD"

    foreach ($dir in $map.Keys)
    {
        New-Item -ItemType Directory $dir -Force | Out-Null

        foreach ($URL in $map[$dir])
        {
            $fileName = [IO.Path]::GetFileName($URL)
            $dest = [IO.Path]::Combine($dir, $fileName)

            Write-Log "Downloading $URL to $dest"
            Download-File -URL $URL -Dest $dest
        }
    }


    foreach ($dir in $map.Keys)
    {
        LogFilesInDirectory "$dir"
    }   return $global:map.Keys
}

function Get-OCIArtifactsToCacheOnVHD
{
    Write-Log "Caching $($ociArtifactsToPull.Count) OCI artifacts on VHD"

    foreach ($dir in $ociArtifactsToPull.Keys)
    {
        New-Item -ItemType Directory $dir -Force | Out-Null

        foreach ($ociArtifactName in $global:ociArtifactsToPull[$dir])
        {
            Write-Log "Pulling OCI artifact $ociArtifactName to $dir"
            Pull-OCIArtifact -Name $ociArtifactName -Dest $dir
        }
    }


    foreach ($dir in $ociArtifactsToPull.Keys)
    {
        LogFilesInDirectory "$dir"
    }
}

function Get-FilesToCacheOnVHD
{
    Write-Log "Caching misc files on VHD"
    Get-OCIArtifactsToCacheOnVHD
    Get-PackagesToCacheOnVHD
}

function LogFilesInDirectory
{
    Param(
        [string]
        $Directory
    )

    Get-ChildItem -Path "$Directory" | ForEach-Object {
        $sizeKB = [math]::Round($_.Length / 1KB, 2)
        Write-Host "$( $_.Name ) - $sizeKB KB"
    }
}

function Get-ToolsToVHD
{
    if (!(Test-Path -Path $global:aksToolsDir))
    {
        New-Item -ItemType Directory -Path $global:aksToolsDir -Force | Out-Null
    }

    Write-Log "Getting DU (Windows Disk Usage)"
    Download-File -URL "https://download.sysinternals.com/files/DU.zip" -Dest "$global:aksToolsDir\DU.zip"
    Expand-Archive -Path "$global:aksToolsDir\DU.zip" -DestinationPath "$global:aksToolsDir\DU" -Force
    Remove-Item -Path "$global:aksToolsDir\DU.zip" -Force

    LogFilesInDirectory "$global:aksToolsDir\DU"
}

function Register-ExpandVolumeTask
{
    if (!(Test-Path -Path $global:aksToolsDir))
    {
        New-Item -ItemType Directory -Path $global:aksToolsDir -Force | Out-Null
    }

    # Leverage existing folder 'c:\aks-tools' to store the task scripts
    $taskScript = @'
        $osDrive = ((Get-WmiObject Win32_OperatingSystem -ErrorAction Stop).SystemDrive).TrimEnd(":")
        $diskpartScriptPath = "c:\aks-tools\diskpart.script"
        [String]::Format("select volume {0}`nextend`nexit", $osDrive) | Out-File -Encoding "UTF8" $diskpartScriptPath -Force
        Start-Process -FilePath diskpart.exe -ArgumentList "/s $diskpartScriptPath" -Wait
        # Run once and remove the task. Sequence: taks invokes ps1, ps1 invokes diskpart.
        Unregister-ScheduledTask -TaskName "aks-expand-volume" -Confirm:$false
        Remove-Item -Path "c:\aks-tools\expand-volume.ps1" -Force
        Remove-Item -Path $diskpartScriptPath -Force
'@

    $taskScriptPath = Join-Path $global:aksToolsDir "expand-volume.ps1"
    $taskScript | Set-Content -Path $taskScriptPath -Force

    # It sometimes failed with below error
    # New-ScheduledTask : Cannot validate argument on parameter 'Action'. The argument is null or empty. Provide an argument
    # that is not null or empty, and then try the command again.
    # Add below logs and retry logic to test it
    $scriptContent = Get-Content -Path $taskScriptPath
    Write-Log "Task script content: $scriptContent"

    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-File `"$taskScriptPath`""
    if (-not $action)
    {
        Write-Log "action is null or empty. taskScriptPath: $taskScriptPath. Recreating it"
        $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-File `"$taskScriptPath`""
        if (-not $action)
        {
            Write-Log "action is still null"
            exit 1
        }
    }
    $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
    $trigger = New-JobTrigger -AtStartup
    $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "aks-expand-volume"
    Register-ScheduledTask -TaskName "aks-expand-volume" -InputObject $definition
    Write-Log "Registered ScheduledTask aks-expand-volume"
}

function Get-PrivatePackagesToCacheOnVHD
{
    if (![string]::IsNullOrEmpty($env:WindowsPrivatePackagesURL))
    {
        Write-Log "Caching private packages on VHD"

        $dir = "c:\akse-cache\private-packages"
        New-Item -ItemType Directory $dir -Force | Out-Null

        $mappingFile = "c:\akse-cache\private-packages\mapping.json"
        $content = @{ }
        $urls = $env:WindowsPrivatePackagesURL.Split(",")
        foreach ($url in $urls)
        {
            $fileName = [IO.Path]::GetFileName($url.Split("?")[0])
            $dest = [IO.Path]::Combine($dir, $fileName)

            Write-Log "Downloading a private package to $dest"
            Download-FileWithAzCopy -URL $URL -Dest $dest

            # Example: v1.29.2-hotfix.2024101-1int.zip
            $version = $fileName.Split('-')[0].SubString(1)
            Write-Log "Adding $version to $mappingFile"
            $content[$version] = $url
        }

        Write-Log "Writing mapping file to $mappingFile"
        $content | ConvertTo-Json -Depth 10 | Out-File -FilePath $mappingFile
    }
}

function Install-ContainerD
{
    # installing containerd during VHD building is to cache container images into the VHD,
    # and the containerd to managed customer containers after provisioning the vm is not necessary
    # the one used here, considering containerd version/package is configurable, and the first one
    # is expected to override the later one
    Write-Log "Getting containerD binaries from $global:defaultContainerdPackageUrl"

    $installDir = "c:\program files\containerd"
    Write-Log "Installing containerd to $installDir"
    New-Item -ItemType Directory $installDir -Force | Out-Null

    $containerdFilename = [IO.Path]::GetFileName($global:defaultContainerdPackageUrl)
    $containerdTmpDest = [IO.Path]::Combine($installDir, $containerdFilename)
    Download-File -URL $global:defaultContainerdPackageUrl -Dest $containerdTmpDest
    # The released containerd package format is either zip or tar.gz
    if ( $containerdFilename.endswith(".zip"))
    {
        Expand-Archive -path $containerdTmpDest -DestinationPath $installDir -Force
    }
    else
    {
        tar -xzf $containerdTmpDest -C $installDir
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to extract the '$containerdTmpDest' archive."
        }
        mv -Force $installDir\bin\* $installDir
        Remove-Item -Path $installDir\bin -Force -Recurse
    }
    Remove-Item -Path $containerdTmpDest | Out-Null

    $newPaths = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine) + ";$installDir"
    [Environment]::SetEnvironmentVariable("Path", $newPaths, [EnvironmentVariableTarget]::Machine)
    $env:Path += ";$installDir"

    $containerdConfigPath = [Io.Path]::Combine($installDir, "config.toml")
    # enabling discard_unpacked_layers allows GC to remove layers from the content store after
    # successfully unpacking these layers to the snapshotter to reduce the disk space caching Windows containerd images
    (containerd config default)  | %{ $_ -replace "discard_unpacked_layers = false", "discard_unpacked_layers = true" }  | Out-File  -FilePath $containerdConfigPath -Encoding ascii

    Get-Content $containerdConfigPath

    # start containerd to pre-pull the images to disk on VHD
    # CSE will configure and register containerd as a service at deployment time
    Start-Job -Name containerd -ScriptBlock { containerd.exe }
}

function Install-OpenSSH
{
    if ($env:INSTALL_OPEN_SSH_SERVER -eq 'False')
    {
        Write-Log "Not installing Windows OpenSSH Server as this is disabled in the pipeline"
        return
    }

    Write-Log "Installing OpenSSH Server"

    # Somehow openssh client got added to Windows 2019 base image.
    if ($env:WindowsSKU -Like '2019*')
    {
        Remove-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0
    }

    Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0

    # It’s by design that files within the C:\Windows\System32\ folder are not modifiable.
    # When the OpenSSH Server starts, it copies C:\windows\system32\openssh\sshd_config_default to C:\programdata\ssh\sshd_config, if the file does not already exist.
    $OriginalConfigPath = "C:\windows\system32\OpenSSH\sshd_config_default"
    $ConfigDirectory = "C:\programdata\ssh"
    New-Item -ItemType Directory -Force -Path $ConfigDirectory
    $ConfigPath = $ConfigDirectory + "\sshd_config"
    Write-Log "Updating $ConfigPath for CVE-2023-48795"
    $ModifiedConfigContents = Get-Content $OriginalConfigPath `
        | %{ $_ -replace "#RekeyLimit default none", "$&`r`n# Disable cipher to mitigate CVE-2023-48795`r`nCiphers -chacha20-poly1305@openssh.com`r`nMacs -*-etm@openssh.com`r`n" }
    Write-Log "Updating $ConfigPath for CVE-2006-5051"
    $ModifiedConfigContents = $ModifiedConfigContents.Replace("#LoginGraceTime 2m", "LoginGraceTime 0")
    Stop-Service sshd
    Out-File -FilePath $ConfigPath -InputObject $ModifiedConfigContents -Encoding UTF8
    Start-Service sshd
    Write-Log "Updated $ConfigPath for CVEs"
}

function Install-WindowsPatches
{
    Write-Log "Installing Windows patches"
    Write-Log "The length of patchUrls is $( $patchUrls.Length )"
    foreach ($patchUrl in $patchUrls)
    {
        $pathOnly = $patchUrl.Split("?")[0]
        $fileName = Split-Path $pathOnly -Leaf
        $fileExtension = [IO.Path]::GetExtension($fileName)
        $fullPath = [IO.Path]::Combine($env:TEMP, $fileName)

        switch ($fileExtension)
        {
            ".msu" {
                Write-Log "Downloading windows patch from $pathOnly to $fullPath"
                Download-File -URL $patchUrl -Dest $fullPath -redactUrl
                Write-Log "Starting install of $fileName"
                $proc = Start-Process -Passthru -FilePath wusa.exe -ArgumentList "$fullPath /quiet /norestart"
                Wait-Process -InputObject $proc
                switch ($proc.ExitCode)
                {
                    0 {
                        Write-Log "Finished install of $fileName"
                    }
                    3010 {
                        WRite-Log "Finished install of $fileName. Reboot required"
                    }
                    2359302 {
                        # https://learn.microsoft.com/en-gb/windows/win32/wua_sdk/wua-success-and-error-codes-?redirectedfrom=MSDN
                        # this number is 0x00240006 and means already installed
                        Write-Log "The update was already installed. Ignoring $fileName"
                    }
                    default {
                        Write-Log "Error during install of $fileName. ExitCode: $( $proc.ExitCode )"
                        throw "Error during install of $fileName. ExitCode: $( $proc.ExitCode )"
                    }
                }
            }
            default {
                Write-Log "Installing patches with extension $fileExtension is not currently supported."
                throw "Installing patches with extension $fileExtension is not currently supported."
            }
        }
    }
}

function Install-WindowsCiliumNetworking
{
    $wcnDirectory = Join-Path -Path $global:cacheDir -ChildPath 'wcn'
    $wcnInstallDirectory = Join-Path -Path $wcnDirectory -ChildPath 'install'
    $wcnScriptsDirectory = Join-Path -Path $wcnInstallDirectory -ChildPath 'scripts'
    $wcnInstallScript = Join-Path -Path $wcnScriptsDirectory -ChildPath 'install' | Join-Path -ChildPath 'install.ps1'

    if (!(Test-Path -PathType Container -Path $wcnDirectory))
    {
        Write-Log "Windows Cilium Networking (WCN) installation package not staged; skipping installation."
        return
    }

    # Select the highest versioned package available.
    $wcnPackageNuget = (Get-ChildItem -Path $wcnDirectory -File -Filter '*.nupkg' | Sort-Object -Property Name -Descending) | Select-Object -First 1
    if (!$wcnPackageNuget -or !(Test-Path -Path $wcnPackageNuget.FullName))
    {
        Write-Log "No Windows Cilium Networking package found in $wcnDirectory"
        throw "No Windows Cilium Networking package found in $wcnDirectory"
    }

    Write-Log "Installing Windows Cilium Networking (WCN) Platform with '$wcnPackageNuget'"

    # Extract NuGet package contents.
    New-Item -ItemType Directory -Path $wcnInstallDirectory -Force
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($wcnPackageNuget.FullName, $wcnInstallDirectory)

    # Invoke install script.
    try {
        & $wcnInstallScript -DisableCiliumStack -SourceDirectory $wcnDirectory -SkipNugetUnpack
    }
    catch {
        Write-Log "Error occurred while installing Windows Cilium Networking: $_"
        throw "Error occurred while installing Windows Cilium Networking: $_"
    }
}

function Set-WinRmServiceAutoStart
{
    Write-Log "Setting WinRM service start to auto"
    sc.exe config winrm start=auto
}

function Set-WinRmServiceDelayedStart
{
    # Hyper-V messes with networking components on startup after the feature is enabled
    # causing issues with communication over winrm and setting winrm to delayed start
    # gives Hyper-V enough time to finish configuration before having packer continue.
    Write-Log "Setting WinRM service start to delayed-auto"
    sc.exe config winrm start=delayed-auto
}

# Best effort to update defender signatures
# This can fail if there is already a signature
# update running which means we will get them anyways
# Also at the time the VM is provisioned Defender will trigger any required updates
function Update-DefenderSignatures
{
    Write-Log "Updating windows defender signatures."
    $service = Get-Service "Windefend"
    $service.WaitForStatus("Running", "00:5:00")
    Update-MpSignature
}

function Update-WindowsFeatures
{
    $featuresToEnable = @(
        "Containers",
        "Hyper-V",
        "Hyper-V-PowerShell")

    foreach ($feature in $featuresToEnable)
    {
        Write-Log "Enabling Windows feature: $feature"
        Install-WindowsFeature $feature
    }
}

function Enable-WindowsFixInPath
{
    Param(
        [Parameter(Mandatory = $true)][string]
        $Path,
        [Parameter(Mandatory = $true)][string]
        $Name,
        [Parameter(Mandatory = $false)][string]
        $Value = "1",
        [Parameter(Mandatory = $false)][string]
        $Type = "DWORD"
    )
    $regPath = (Get-Item -Path $Path -ErrorAction Ignore)
    if (!$regPath)
    {
        Write-Log "Creating $Path"
        # assigning to a variable stops logs of logging of successful creations.
        $newRegDir = New-Item -Force -Path $Path
    }
    $currentValue = (Get-ItemProperty -Path $Path -Name $Name -ErrorAction Ignore)
    if (![string]::IsNullOrEmpty($currentValue))
    {
        Write-Log "The current value of $Name in $Path is $currentValue"
    }
    Set-ItemProperty -Path $Path -Name $Name $Value -Type $Type
}

function Update-Registry
{
    foreach ($key in $global:keysToSet)
    {
        $keyPath = $key.Path
        $keyName = $key.Name
        $keyValue = $key.Value
        $keyType = $key.Type
        $keyComment = $key.Comment
        $keyOperation = $key.Operation

        Write-Log "$keyPath\$keyName = $keyValue : $keyComment"
        if ($keyOperation -eq "bor")
        {
            $currentValue = (Get-ItemProperty -Path $keyPath -Name $keyName -ErrorAction Ignore)
            if (![string]::IsNullOrEmpty($currentValue))
            {
                Write-Log "The current value of $keyName is $currentValue"
                $keyValue = ([int]$currentValue.$keyName -bor $keyValue)
            }
            Enable-WindowsFixInPath -Path $keyPath -Name $keyName -Value $keyValue -Type $keyType
        }
        else
        {
            Enable-WindowsFixInPath -Path $keyPath -Name $keyName -Value $keyValue -Type $keyType
        }
    }
}

function Clear-TempFolder
{
    $tempFolders = @()
    $tempFolders += [System.Environment]::GetFolderPath('LocalApplicationData') + '\Temp'
    $tempFolders += [System.Environment]::GetFolderPath('InternetCache')
    $tempFolders += [System.Environment]::GetFolderPath('Windows') + '\Temp'

    # Iterate over each temporary folder
    foreach ($folder in $tempFolders)
    {
        # Check if the folder exists
        if (-not (Test-Path -Path $folder -PathType Container))
        {
            Write-Host "The folder '$folder' does not exist."
            continue
        }

        # Get all files in the temporary folder
        $tempFiles = Get-ChildItem -Path $folder -File -Force

        # Delete each file in the temporary folder
        foreach ($file in $tempFiles)
        {
            # skip file if the file name contains "packer"
            if ($file.Name -like "*packer*")
            {
                continue
            }

            try
            {
                Remove-Item -Path $file.FullName -Force
            }
            catch
            {
                Write-Host "Failed to remove file: $( $file.FullName )"
                continue
            }
        }

        # Confirm completion for each folder
        Write-Host "Temporary files in '$folder' cleaned up successfully."
    }

    # Give the system some time to release the file handles
    Start-Sleep -Seconds 1
}


function Get-SystemDriveDiskInfo
{
    Clear-TempFolder
    Write-Log "Get Disk info"
    $disksInfo = Get-CimInstance -ClassName Win32_LogicalDisk
    foreach ($disk in $disksInfo)
    {
        if ($disk.DeviceID -eq "C:")
        {
            Write-Log "Disk C: Free space: $( $disk.FreeSpace ), Total size: $( $disk.Size )"
        }
    }
}

function Get-DefenderPreferenceInfo
{
    Write-Log "Get preferences for the Windows Defender scans and updates"
    Write-Log(Get-MpPreference | Format-List | Out-String)
}

function Exclude-ReservedUDPSourcePort()
{
    # https://docs.microsoft.com/en-us/azure/virtual-network/virtual-networks-faq#what-protocols-can-i-use-within-vnets
    # Default UDP Dynamic Port Range in Windows server: Start Port: 49152, Number of Ports : 16384. Range: [49152, 65535]
    # Exclude UDP source port 65330. This only excludes the port in AKS Windows nodes but will not impact Windows containers.
    # Reference: https://github.com/Azure/AKS/issues/2988
    # List command: netsh int ipv4 show excludedportrange udp
    Invoke-Executable -Executable "netsh.exe" -ArgList @("int", "ipv4", "add", "excludedportrange", "udp", "65330", "1", "persistent")
}

function Get-LatestWindowsDefenderPlatformUpdate
{
    $downloadFilePath = [IO.Path]::Combine([System.IO.Path]::GetTempPath(), "Mpupdate.exe")

    $currentDefenderProductVersion = (Get-MpComputerStatus).AMProductVersion
    $doc = New-Object xml
    $doc.Load("$global:defenderUpdateInfoUrl")
    $latestDefenderProductVersion = $doc.versions.platform

    if ($latestDefenderProductVersion -gt $currentDefenderProductVersion)
    {
        Write-Log "Update started. Current MPVersion: $currentDefenderProductVersion, Expected Version: $latestDefenderProductVersion"
        Download-File -URL $global:defenderUpdateUrl -Dest $downloadFilePath
        $proc = Start-Process -PassThru -FilePath $downloadFilePath -Wait
        Start-Sleep -Seconds 10
        switch ($proc.ExitCode)
        {
            0 {
                Write-Log "Finished update of $downloadFilePath"
            }
            default {
                Write-Log "Error during update of $downloadFilePath. ExitCode: $( $proc.ExitCode )"
                throw "Error during update of $downloadFilePath. ExitCode: $( $proc.ExitCode )"
            }
        }
        $currentDefenderProductVersion = (Get-MpComputerStatus).AMProductVersion
        if ($latestDefenderProductVersion -gt $currentDefenderProductVersion)
        {
            throw "Update failed. Current MPVersion: $currentDefenderProductVersion, Expected Version: $latestDefenderProductVersion"
        }
        else
        {
            Write-Log "Update succeeded. Current MPVersion: $currentDefenderProductVersion, Expected Version: $latestDefenderProductVersion"
        }
    }
    else
    {
        Write-Log "Update not required. Current MPVersion: $currentDefenderProductVersion, Expected Version: $latestDefenderProductVersion"
    }
}

function Log-ReofferUpdate
{
    try
    {
        $result = (Get-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Update\TargetingInfo\Installed\Server.OS.amd64" -Name ReofferUpdate)
        if ($result)
        {
            Write-Log "ReofferUpdate is $( $result.ReofferUpdate )"
        }
    }
    catch
    {
        Write-Log "ReofferUpdate registry setting does not exist"
    }
}

function Test-AzureExtensions
{
    if ($env:SKIP_EXTENSION_CHECK -eq "True")
    {
        Write-Log "Skipping extension check because SKIP_EXTENSION_CHECK is set to True"
        return
    }

    # Expect the Windows VHD without any other extensions
    if (Test-Path "C:\Packages\Plugins")
    {
        $actualExtensions = (Get-ChildItem "C:\Packages\Plugins").Name
        if ($actualExtensions.Length -gt 0)
        {
            Write-Log "Azure extensions are not expected and skip extension checks was $env:SKIP_EXTENSION_CHECK. Details:"
            foreach ($extension in $actualExtensions)
            {
                Write-Log "*  $extension"
            }
            exit 1
        }
    }
    Write-Log "Azure extensions are not found"
}

# Disable progress writers for this session to greatly speed up operations such as Invoke-WebRequest
$ProgressPreference = 'SilentlyContinue'

try
{
    switch ($env:ProvisioningPhase)
    {
        "1" {
            Write-Log "Performing actions for provisioning phase 1"
            Expand-OS-Partition
            Exclude-ReservedUDPSourcePort
            Get-LatestWindowsDefenderPlatformUpdate
            Disable-WindowsUpdates
            Set-WinRmServiceDelayedStart
            Update-DefenderSignatures
            Log-ReofferUpdate
            Install-OpenSSH
            Log-ReofferUpdate
            Install-WindowsPatches
            Update-WindowsFeatures
        }
        "2" {
            Write-Log "Performing actions for provisioning phase 2"
            Log-ReofferUpdate
            Set-WinRmServiceAutoStart
            Install-ContainerD
            Update-Registry
            Get-ContainerImages
            Get-FilesToCacheOnVHD
            Get-ToolsToVHD
            Get-PrivatePackagesToCacheOnVHD
            Install-WindowsCiliumNetworking
            # Update all the registry keys again in case the steps in between reset them. Ok, some of the steps in between do reset them. But there's a risk that the steps also need
            # the keys set. So we kinda have to do both now :cry:
            Update-Registry
            Log-ReofferUpdate
        }
        "3" {
            Register-ExpandVolumeTask
            Cleanup-TemporaryFiles
            (New-Guid).Guid | Out-File -FilePath 'c:\vhd-id.txt'
            Clear-TempFolder
            Log-VHDFreeSize
            Test-AzureExtensions
        }
        default {
            Write-Log "Unable to determine provisiong phase... exiting"
            throw "Unable to determine provisiong phase... exiting"
        }
    }
}
finally
{
    Get-SystemDriveDiskInfo
    Get-DefenderPreferenceInfo
}
