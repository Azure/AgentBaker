<#
    .SYNOPSIS
        Used to produce Windows AKS images.

    .DESCRIPTION
        This script is used by packer to produce Windows AKS images.
#>

$ErrorActionPreference = "Stop"

. c:\windows-vhd-configuration.ps1

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-Log($Message) {
    $msg = $message | Timestamp
    Write-Output $msg
}

function Download-File {
    param (
        $URL,
        $Dest,
        $retryCount = 5,
        $retryDelay = 0,
        [Switch]$redactUrl = $false
    )
    curl.exe -f --retry $retryCount --retry-delay $retryDelay -L $URL -o $Dest
    if ($LASTEXITCODE) {
        $logURL = $URL
        if ($redactUrl) {
            $logURL = $logURL.Split("?")[0]
        }
        throw "Curl exited with '$LASTEXITCODE' while attemping to download '$logURL'"
    }
}

function Download-FileWithAzCopy {
    param (
        $URL,
        $Dest
    )


    if (!(Test-Path -Path $global:aksTempDir)) {
        Write-Log "Creating temp dir for tools of building vhd"
        New-Item -ItemType Directory $global:aksTempDir -Force
    }

    if (!(Test-Path -Path "$global:aksTempDir\azcopy.exe")) {
        Write-Log "Downloading azcopy"
        Invoke-WebRequest -UseBasicParsing "https://aka.ms/downloadazcopy-v10-windows" -OutFile "$global:aksTempDir\azcopy.zip"
        Expand-Archive -Path "$global:aksTempDir\azcopy.zip" -DestinationPath "$global:aksTempDir\tmp" -Force
        Move-Item "$global:aksTempDir\tmp\*\azcopy.exe" "$global:aksTempDir\azcopy.exe"
    }

    pushd "$global:aksTempDir"
        $env:AZCOPY_JOB_PLAN_LOCATION="$global:aksTempDir\azcopy"
        $env:AZCOPY_LOG_LOCATION="$global:aksTempDir\azcopy"
        # user_assigned_managed_identities has been bound in vhdbuilder/packer/windows-vhd-builder-sig.json
        .\azcopy.exe login --login-type=MSI
        .\azcopy.exe copy $URL $Dest
    popd
}

function Cleanup-TemporaryFiles {
    if (Test-Path -Path $global:aksTempDir) {
        Remove-Item -Path $global:aksTempDir -Force -Recurse
    }
}

function Retry-Command {
    [CmdletBinding()]
    Param(
        [Parameter(Position=0, Mandatory=$true)]
        [scriptblock]$ScriptBlock,

        [Parameter(Position=1, Mandatory=$true)]
        [string]$ErrorMessage,

        [Parameter(Position=2, Mandatory=$false)]
        [int]$Maximum = 5,

        [Parameter(Position=3, Mandatory=$false)]
        [int]$Delay = 10
    )

    Begin {
        $cnt = 0
    }

    Process {
        do {
            $cnt++
            try {
                $ScriptBlock.Invoke()
                if ($LASTEXITCODE) {
                    throw "Retry $cnt : $ErrorMessage"
                }
                return
            } catch {
                Write-Log $_.Exception.InnerException.Message
                if ($_.Exception.InnerException.Message.Contains("There is not enough space on the disk. (0x70)")) {
                    Write-Error "Exit retry since there is not enough space on the disk"
                    break
                }
                Write-Log "Retry $cnt : $ScriptBlock"
                Start-Sleep $Delay
            }
        } while ($cnt -lt $Maximum)

        # Throw an error after $Maximum unsuccessful invocations. Doesn't need
        # a condition, since the function returns upon successful invocation.
        throw 'All retries failed. $ErrorMessage'
    }
}

function Invoke-Executable {
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
        if ($LASTEXITCODE) {
            Write-Log "$Executable returned unsuccessfully with exit code $LASTEXITCODE"
            Start-Sleep -Seconds $RetryDelaySeconds
            continue
        }
        else {
            Write-Log "$Executable returned successfully"
            return
        }
    }

    Write-Log "Exhausted retries for $Executable $ArgList"
    throw "Exhausted retries for $Executable $ArgList"
}

function Expand-OS-Partition {
    $customizedDiskSize = $env:CustomizedDiskSize
    if ([string]::IsNullOrEmpty($customizedDiskSize)) {
        Write-Log "No need to expand the OS partition size"
        return
    }

    Write-Log "Customized OS disk size is $customizedDiskSize GB"
    [Int32]$osPartitionSize = 0
    if ([Int32]::TryParse($customizedDiskSize, [ref]$osPartitionSize)) {
        # The supportedMaxSize less than the customizedDiskSize because some system usages will occupy disks (about 500M).
        $supportedMaxSize = (Get-PartitionSupportedSize -DriveLetter C).sizeMax
        $currentSize = (Get-Partition -DriveLetter C).Size
        if ($supportedMaxSize -gt $currentSize) {
            Write-Log "Resizing the OS partition size from $currentSize to $supportedMaxSize"
            Resize-Partition -DriveLetter C -Size $supportedMaxSize
            Get-Disk
            Get-Partition
        } else {
            Write-Log "The current size is the max size $currentSize"
        }
    } else {
        Throw "$customizedDiskSize is not a valid customized OS disk size"
    }
}

function Disable-WindowsUpdates {
    # See https://docs.microsoft.com/en-us/windows/deployment/update/waas-wu-settings
    # for additional information on WU related registry settings

    Write-Log "Disabling automatic windows upates"
    $WindowsUpdatePath = "HKLM:SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate"
    $AutoUpdatePath = "HKLM:SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU"

    if (Test-Path -Path $WindowsUpdatePath) {
        Remove-Item -Path $WindowsUpdatePath -Recurse
    }

    New-Item -Path $WindowsUpdatePath | Out-Null
    New-Item -Path $AutoUpdatePath | Out-Null
    Set-ItemProperty -Path $AutoUpdatePath -Name NoAutoUpdate -Value 1 | Out-Null
}

function Get-ContainerImages {
    Write-Log "Pulling images for windows server $windowsSKU" # The variable $windowsSKU will be "2019-containerd", "2022-containerd", ...
    foreach ($image in $imagesToPull) {
        $imagePrefix = $image.Split(":")[0]
        if (($imagePrefix -eq "mcr.microsoft.com/windows/servercore" -and ![string]::IsNullOrEmpty($env:WindowsServerCoreImageURL)) -or
            ($imagePrefix -eq "mcr.microsoft.com/windows/nanoserver" -and ![string]::IsNullOrEmpty($env:WindowsNanoServerImageURL))) {
            $url=""
            if ($image.Contains("mcr.microsoft.com/windows/servercore")) {
                $url=$env:WindowsServerCoreImageURL
            } elseif ($image.Contains("mcr.microsoft.com/windows/nanoserver")) {
                $url=$env:WindowsNanoServerImageURL
            }
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
        } else {
            Write-Log "Pulling image $image"
            Retry-Command -ScriptBlock {
                & crictl.exe pull $image
            } -ErrorMessage "Failed to pull image $image"
        }
    }
    Stop-Job  -Name containerd
    Remove-Job -Name containerd
}

function Get-FilesToCacheOnVHD {
    Write-Log "Caching misc files on VHD"

    foreach ($dir in $map.Keys) {
        New-Item -ItemType Directory $dir -Force | Out-Null

        foreach ($URL in $map[$dir]) {
            $fileName = [IO.Path]::GetFileName($URL)
            $dest = [IO.Path]::Combine($dir, $fileName)

            Write-Log "Downloading $URL to $dest"
            Download-File -URL $URL -Dest $dest
        }
    }
}

function Get-ToolsToVHD {
    # Rely on the completion of Get-FilesToCacheOnVHD
    $cacheDir = "c:\akse-cache\tools"

    if (!(Test-Path -Path $global:aksToolsDir)) {
        New-Item -ItemType Directory -Path $global:aksToolsDir -Force | Out-Null
    }

    Write-Log "Getting DU (Windows Disk Usage)"
    Expand-Archive -Path "$cacheDir\DU.zip" -DestinationPath "$global:aksToolsDir\DU" -Force
}

function Register-ExpandVolumeTask {
    if (!(Test-Path -Path $global:aksToolsDir)) {
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
    if (-not $action) {
        Write-Log "action is null or empty. taskScriptPath: $taskScriptPath. Recreating it"
        $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-File `"$taskScriptPath`""
        if (-not $action) {
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

function Get-PrivatePackagesToCacheOnVHD {
    if (![string]::IsNullOrEmpty($env:WindowsPrivatePackagesURL)) {
        Write-Log "Caching private packages on VHD"
    
        $dir = "c:\akse-cache\private-packages"
        New-Item -ItemType Directory $dir -Force | Out-Null

        $mappingFile = "c:\akse-cache\private-packages\mapping.json"
        $content = @{}
        $urls = $env:WindowsPrivatePackagesURL.Split(",")
        foreach ($url in $urls) {
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

function Install-ContainerD {
    # installing containerd during VHD building is to cache container images into the VHD,
    # and the containerd to managed customer containers after provisioning the vm is not necessary
    # the one used here, considering containerd version/package is configurable, and the first one
    # is expected to override the later one
    Write-Log "Getting containerD binaries from $global:defaultContainerdPackageUrl"

    $installDir = "c:\program files\containerd"
    Write-Log "Installing containerd to $installDir"
    New-Item -ItemType Directory $installDir -Force | Out-Null

    $containerdFilename=[IO.Path]::GetFileName($global:defaultContainerdPackageUrl)
    $containerdTmpDest = [IO.Path]::Combine($installDir, $containerdFilename)
    Download-File -URL $global:defaultContainerdPackageUrl -Dest $containerdTmpDest
    # The released containerd package format is either zip or tar.gz
    if ($containerdFilename.endswith(".zip")) {
        Expand-Archive -path $containerdTmpDest -DestinationPath $installDir -Force
    } else {
        tar -xzf $containerdTmpDest -C $installDir
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
    (containerd config default)  | %{$_ -replace "discard_unpacked_layers = false", "discard_unpacked_layers = true"}  | Out-File  -FilePath $containerdConfigPath -Encoding ascii

    Get-Content $containerdConfigPath

    # start containerd to pre-pull the images to disk on VHD
    # CSE will configure and register containerd as a service at deployment time
    Start-Job -Name containerd -ScriptBlock { containerd.exe }
}

function Install-OpenSSH {
    Write-Log "Installing OpenSSH Server"
    Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0

    # It’s by design that files within the C:\Windows\System32\ folder are not modifiable. 
    # When the OpenSSH Server starts, it copies C:\windows\system32\openssh\sshd_config_default to C:\programdata\ssh\sshd_config, if the file does not already exist.
    $OriginalConfigPath = "C:\windows\system32\OpenSSH\sshd_config_default"
    $ConfigPath = "C:\programdata\ssh\sshd_config"
    Write-Log "Updating $ConfigPath for CVE-2023-48795"
    $ModifiedConfigContents = Get-Content $OriginalConfigPath `
        | %{$_ -replace "#RekeyLimit default none", "$&`r`n# Disable cipher to mitigate CVE-2023-48795`r`nCiphers -chacha20-poly1305@openssh.com`r`nMacs -*-etm@openssh.com`r`n"}
    Write-Log "Updating $ConfigPath for CVE-2006-5051"
    $ModifiedConfigContents = $ModifiedConfigContents.Replace("#LoginGraceTime 2m", "LoginGraceTime 0")
    Stop-Service sshd
    Out-File -FilePath $ConfigPath -InputObject $ModifiedConfigContents -Encoding UTF8
    Start-Service sshd
    Write-Log "Updated $ConfigPath for CVEs"
}

function Install-WindowsPatches {
    Write-Log "Installing Windows patches"
    Write-Log "The length of patchUrls is $($patchUrls.Length)"
    foreach ($patchUrl in $patchUrls) {
        $pathOnly = $patchUrl.Split("?")[0]
        $fileName = Split-Path $pathOnly -Leaf
        $fileExtension = [IO.Path]::GetExtension($fileName)
        $fullPath = [IO.Path]::Combine($env:TEMP, $fileName)

        switch ($fileExtension) {
            ".msu" {
                Write-Log "Downloading windows patch from $pathOnly to $fullPath"
                Download-File -URL $patchUrl -Dest $fullPath -redactUrl
                Write-Log "Starting install of $fileName"
                $proc = Start-Process -Passthru -FilePath wusa.exe -ArgumentList "$fullPath /quiet /norestart"
                Wait-Process -InputObject $proc
                switch ($proc.ExitCode) {
                    0 {
                        Write-Log "Finished install of $fileName"
                    }
                    3010 {
                        WRite-Log "Finished install of $fileName. Reboot required"
                    }
                    default {
                        Write-Log "Error during install of $fileName. ExitCode: $($proc.ExitCode)"
                        throw "Error during install of $fileName. ExitCode: $($proc.ExitCode)"
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

function Set-WinRmServiceAutoStart {
    Write-Log "Setting WinRM service start to auto"
    sc.exe config winrm start=auto
}

function Set-WinRmServiceDelayedStart {
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
function Update-DefenderSignatures {
    Write-Log "Updating windows defender signatures."
    $service = Get-Service "Windefend"
    $service.WaitForStatus("Running","00:5:00")
    Update-MpSignature
}

function Update-WindowsFeatures {
    $featuresToEnable = @(
        "Containers",
        "Hyper-V",
        "Hyper-V-PowerShell")

    foreach ($feature in $featuresToEnable) {
        Write-Log "Enabling Windows feature: $feature"
        Install-WindowsFeature $feature
    }
}

function Enable-WindowsFixInFeatureManagement {
    Param(
      [Parameter(Mandatory = $true)][string]
      $Name,
      [Parameter(Mandatory = $false)][string]
      $Value = "1",
      [Parameter(Mandatory = $false)][string]
      $Type = "DWORD"
    )

    $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft" -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft"
        New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft"
    }

    $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement" -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement"
        New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement"
    }

    $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides"
        New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides"
    }

    $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name $Name -ErrorAction Ignore)
    if (![string]::IsNullOrEmpty($currentValue)) {
        Write-Log "The current value of $Name in FeatureManagement\Overrides is $currentValue"
    }
    Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name $Name -Value $Value -Type $Type
}

function Enable-WindowsFixInHnsState {
    Param(
      [Parameter(Mandatory = $true)][string]
      $Name,
      [Parameter(Mandatory = $false)][string]
      $Value = "1",
      [Parameter(Mandatory = $false)][string]
      $Type = "DWORD"
    )

    $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name $Name -ErrorAction Ignore)
    if (![string]::IsNullOrEmpty($currentValue)) {
        Write-Log "The current value of $Name in hns\State is $currentValue"
    }
    Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name $Name -Value $Value -Type $Type
}

function Enable-WindowsFixInVfpExtParameters {
    Param(
      [Parameter(Mandatory = $true)][string]
      $Name,
      [Parameter(Mandatory = $false)][string]
      $Value = "1",
      [Parameter(Mandatory = $false)][string]
      $Type = "DWORD"
    )

    $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt" -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt"
        New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt"
    }

    $regPath=(Get-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters"
        New-Item -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters"
    }

    $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -Name $Name -ErrorAction Ignore)
    if (![string]::IsNullOrEmpty($currentValue)) {
        Write-Log "The current value of $Name in VfpExt\Parameters is $currentValue"
    }
    Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -Name $Name $Value -Type $Type
}

function Enable-WindowsFixInPath {
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
    $regPath=(Get-Item -Path $Path -ErrorAction Ignore)
    if (!$regPath) {
        Write-Log "Creating $Path"
        New-Item -Path $Path
    }
    $currentValue=(Get-ItemProperty -Path $Path -Name $Name -ErrorAction Ignore)
    if (![string]::IsNullOrEmpty($currentValue)) {
        Write-Log "The current value of $Name in $Path is $currentValue"
    }
    Set-ItemProperty -Path $Path -Name $Name $Value -Type $Type
}

# If you need to add registry key in this function,
# please update $wuRegistryKeys and $wuRegistryNames in vhdbuilder/packer/write-release-notes-windows.ps1 at the same time
function Update-Registry {
    # Enables DNS resolution of SMB shares for containerD
    # https://github.com/kubernetes-sigs/windows-gmsa/issues/30#issuecomment-802240945
    Write-Log "Apply SMB Resolution Fix for containerD"
    Enable-WindowsFixInHnsState -Name EnableCompartmentNamespace

    if ($env:WindowsSKU -Like '2019*') {
        Write-Log "Keep the HNS fix (0x10) even though it is enabled by default. Windows are still using HNSControlFlag and may need it in the future."
        $hnsControlFlag=0x10
        $currentValue=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag -ErrorAction Ignore)
        if (![string]::IsNullOrEmpty($currentValue)) {
            Write-Log "The current value of HNSControlFlag is $currentValue"
            $hnsControlFlag=([int]$currentValue.HNSControlFlag -bor $hnsControlFlag)
        }
        Enable-WindowsFixInHnsState -Name HNSControlFlag -Value $hnsControlFlag

        Write-Log "Enable a WCIFS fix in 2022-10B"
        Enable-WindowsFixInPath -Path "HKLM:\SYSTEM\CurrentControlSet\Services\wcifs" -Name WcifsSOPCountDisabled -Value 0

        Write-Log "Enable 3 fixes in 2023-04B"
        Enable-WindowsFixInHnsState -Name HnsPolicyUpdateChange
        Enable-WindowsFixInHnsState -Name HnsNatAllowRuleUpdateChange
        Enable-WindowsFixInFeatureManagement -Name 3105872524

        Write-Log "Enable 1 fix in 2023-05B"
        Enable-WindowsFixInVfpExtParameters -Name VfpEvenPodDistributionIsEnabled

        Write-Log "Enable 1 fix in 2023-06B"
        Enable-WindowsFixInFeatureManagement -Name 3230913164

        Write-Log "Enable 1 fix in 2023-10B"
        Enable-WindowsFixInVfpExtParameters -Name VfpNotReuseTcpOneWayFlowIsEnabled

        Write-Log "Enable 4 fixes in 2023-11B"
        Enable-WindowsFixInHnsState -Name CleanupReservedPorts
        Enable-WindowsFixInFeatureManagement -Name 652313229
        Enable-WindowsFixInFeatureManagement -Name 2059235981
        Enable-WindowsFixInFeatureManagement -Name 3767762061

        Write-Log "Enable 1 fix in 2024-01B"
        Enable-WindowsFixInFeatureManagement -Name 1102009996

        Write-Log "Enable 2 fixes in 2024-04B"
        Enable-WindowsFixInFeatureManagement -Name 2290715789
        Enable-WindowsFixInFeatureManagement -Name 3152880268

        Write-Log "Enable 1 fix in 2024-06B"
        Enable-WindowsFixInFeatureManagement -Name 1605443213
    }

    if ($env:WindowsSKU -Like '2022*') {
        Write-Log "Enable a WCIFS fix in 2022-10B"
        Enable-WindowsFixInFeatureManagement -Name 2629306509

        Write-Log "Enable 4 fixes in 2023-04B"
        Enable-WindowsFixInHnsState -Name HnsPolicyUpdateChange
        Enable-WindowsFixInHnsState -Name HnsNatAllowRuleUpdateChange
        Enable-WindowsFixInHnsState -Name HnsAclUpdateChange
        Enable-WindowsFixInFeatureManagement -Name 3508525708

        Write-Log "Enable 4 fixes in 2023-05B"
        Enable-WindowsFixInHnsState -Name HnsNpmRefresh
        Enable-WindowsFixInVfpExtParameters -Name VfpEvenPodDistributionIsEnabled
        Enable-WindowsFixInFeatureManagement -Name 1995963020
        Enable-WindowsFixInFeatureManagement -Name 189519500

        Write-Log "Enable 1 fix in 2023-06B"
        Enable-WindowsFixInFeatureManagement -Name 3398685324

        Write-Log "Enable 4 fixes in 2023-07B"
        Enable-WindowsFixInHnsState -Name HnsNodeToClusterIpv6
        Enable-WindowsFixInHnsState -Name HNSNpmIpsetLimitChange
        Enable-WindowsFixInHnsState -Name HNSLbNatDupRuleChange
        Enable-WindowsFixInVfpExtParameters -Name VfpIpv6DipsPrintingIsEnabled

        Write-Log "Enable 3 fixes in 2023-08B"
        Enable-WindowsFixInHnsState -Name HNSUpdatePolicyForEndpointChange
        Enable-WindowsFixInHnsState -Name HNSFixExtensionUponRehydration
        Enable-WindowsFixInFeatureManagement -Name 87798413

        Write-Log "Enable 4 fixes in 2023-09B"
        Enable-WindowsFixInHnsState -Name RemoveSourcePortPreservationForRest
        Enable-WindowsFixInFeatureManagement -Name 4289201804
        Enable-WindowsFixInFeatureManagement -Name 1355135117
        Enable-WindowsFixInFeatureManagement -Name 2214038156

        Write-Log "Enable 3 fixes in 2023-10B"
        Enable-WindowsFixInHnsState -Name FwPerfImprovementChange
        Enable-WindowsFixInVfpExtParameters -Name VfpNotReuseTcpOneWayFlowIsEnabled
        Enable-WindowsFixInFeatureManagement -Name 1673770637

        Write-Log "Enable 4 fixes in 2023-11B"
        Enable-WindowsFixInHnsState -Name CleanupReservedPorts

        Enable-WindowsFixInFeatureManagement -Name 527922829
        # Then based on 527922829 to set DeltaHivePolicy=2: use delta hives, and stop generating rollups
        Enable-WindowsFixInPath -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Windows Containers" -Name DeltaHivePolicy -Value 2

        Enable-WindowsFixInFeatureManagement -Name 2193453709
        Enable-WindowsFixInFeatureManagement -Name 3331554445

        Write-Log "Enable 2 fixes in 2024-01B"
        Enable-WindowsFixInHnsState -Name OverrideReceiveRoutingForLocalAddressesIpv4
        Enable-WindowsFixInHnsState -Name OverrideReceiveRoutingForLocalAddressesIpv6

        Write-Log "Enable 1 fix in 2024-02B"
        Enable-WindowsFixInFeatureManagement -Name 1327590028

        Write-Log "Enable 4 fixes in 2024-03B"
        Enable-WindowsFixInHnsState -Name HnsPreallocatePortRange
        Enable-WindowsFixInFeatureManagement -Name 1114842764
        Enable-WindowsFixInFeatureManagement -Name 4154935436
        Enable-WindowsFixInFeatureManagement -Name 124082829

        Write-Log "Enable 11 fixes in 2024-04B"
        Enable-WindowsFixInFeatureManagement -Name 3744292492
        Enable-WindowsFixInFeatureManagement -Name 3838270605
        Enable-WindowsFixInFeatureManagement -Name 851795084
        Enable-WindowsFixInFeatureManagement -Name 26691724
        Enable-WindowsFixInFeatureManagement -Name 3834988172
        Enable-WindowsFixInFeatureManagement -Name 1535854221
        Enable-WindowsFixInFeatureManagement -Name 3632636556
        Enable-WindowsFixInFeatureManagement -Name 1552261773
        Enable-WindowsFixInFeatureManagement -Name 4186914956
        Enable-WindowsFixInFeatureManagement -Name 3173070476
        Enable-WindowsFixInFeatureManagement -Name 3958450316

        Write-Log "Enable 3 fixes in 2024-06B"
        Enable-WindowsFixInFeatureManagement -Name 2540111500
        Enable-WindowsFixInFeatureManagement -Name 50261647
        Enable-WindowsFixInFeatureManagement -Name 1475968140

        Write-Log "Enable 1 fix in 2024-07B"
        Enable-WindowsFixInFeatureManagement -Name 747051149

        Write-Log "Enable 1 fix in 2024-08B"
        Enable-WindowsFixInFeatureManagement -Name 260097166
    }

    if ($env:WindowsSKU -Like '23H2*') {
        Write-Log "Disable port exclusion change in 23H2"
        Enable-WindowsFixInHnsState -Name PortExclusionChange -Value 0

        Write-Log "Enable 1 fix in 2024-08B"
        Enable-WindowsFixInFeatureManagement -Name 1800977551
    }
}

function Clear-TempFolder {
    $tempFolders = @()
    $tempFolders += [System.Environment]::GetFolderPath('LocalApplicationData') + '\Temp'
    $tempFolders += [System.Environment]::GetFolderPath('InternetCache')
    $tempFolders += [System.Environment]::GetFolderPath('Windows') + '\Temp'

    # Iterate over each temporary folder
    foreach ($folder in $tempFolders) {
        # Check if the folder exists
        if (-not (Test-Path -Path $folder -PathType Container)) {
            Write-Host "The folder '$folder' does not exist."
            continue
        }

        # Get all files in the temporary folder
        $tempFiles = Get-ChildItem -Path $folder -File -Force

        # Delete each file in the temporary folder
        foreach ($file in $tempFiles) {
            # skip file if the file name contains "packer"
            if ($file.Name -like "*packer*") {
                continue
            }

            try {
                Remove-Item -Path $file.FullName -Force
            } catch {
                Write-Host "Failed to remove file: $($file.FullName)"
                continue
            }
        }

        # Confirm completion for each folder
        Write-Host "Temporary files in '$folder' cleaned up successfully."
    }

    # Give the system some time to release the file handles
    Start-Sleep -Seconds 1
}


function Get-SystemDriveDiskInfo {
    Clear-TempFolder
    Write-Log "Get Disk info"
    $disksInfo=Get-CimInstance -ClassName Win32_LogicalDisk
    foreach($disk in $disksInfo) {
        if ($disk.DeviceID -eq "C:") {
            Write-Log "Disk C: Free space: $($disk.FreeSpace), Total size: $($disk.Size)"
        }
    }
}

function Validate-VHDFreeSize {
    Clear-TempFolder
    Write-Log "Get Disk info"
    $disksInfo=Get-CimInstance -ClassName Win32_LogicalDisk
    foreach($disk in $disksInfo) {
        if ($disk.DeviceID -eq "C:") {
            if ($disk.FreeSpace -lt $global:lowestFreeSpace) {
                Write-Log "Disk C: Free space $($disk.FreeSpace) is less than $($global:lowestFreeSpace)"
            }
            break
        }
    }
}

function Get-DefenderPreferenceInfo {
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

function Get-LatestWindowsDefenderPlatformUpdate {
    $downloadFilePath = [IO.Path]::Combine([System.IO.Path]::GetTempPath(), "Mpupdate.exe")
 
    $currentDefenderProductVersion = (Get-MpComputerStatus).AMProductVersion
    $doc = New-Object xml
    $doc.Load("$global:defenderUpdateInfoUrl")
    $latestDefenderProductVersion = $doc.versions.platform
 
    if ($latestDefenderProductVersion -gt $currentDefenderProductVersion) {
        Write-Log "Update started. Current MPVersion: $currentDefenderProductVersion, Expected Version: $latestDefenderProductVersion"
        Download-File -URL $global:defenderUpdateUrl -Dest $downloadFilePath
        $proc = Start-Process -PassThru -FilePath $downloadFilePath -Wait
        Start-Sleep -Seconds 10
        switch ($proc.ExitCode) {
            0 {
                Write-Log "Finished update of $downloadFilePath"
            }
            default {
                Write-Log "Error during update of $downloadFilePath. ExitCode: $($proc.ExitCode)"
                throw "Error during update of $downloadFilePath. ExitCode: $($proc.ExitCode)"
            }
        }
        $currentDefenderProductVersion = (Get-MpComputerStatus).AMProductVersion
        if ($latestDefenderProductVersion -gt $currentDefenderProductVersion) {
            throw "Update failed. Current MPVersion: $currentDefenderProductVersion, Expected Version: $latestDefenderProductVersion"
        }
        else {
            Write-Log "Update succeeded. Current MPVersion: $currentDefenderProductVersion, Expected Version: $latestDefenderProductVersion"
        }
    }
    else {
        Write-Log "Update not required. Current MPVersion: $currentDefenderProductVersion, Expected Version: $latestDefenderProductVersion"
    }
}

function Log-ReofferUpdate {
    try {
        $result=(Get-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Update\TargetingInfo\Installed\Server.OS.amd64" -Name ReofferUpdate)
        if ($result) {
            Write-Log "ReofferUpdate is $($result.ReofferUpdate)"
        }
    } catch {
        Write-Log "ReofferUpdate does not exist"
    }
}

function Test-AzureExtensions {
    # Expect the Windows VHD without any other extensions
    if (Test-Path "C:\Packages\Plugins") {
        $actualExtensions = (Get-ChildItem "C:\Packages\Plugins").Name
        if ($actualExtensions.Length -gt 0) {
            Write-Log "Azure extensions are not expected. Details: $($actualExtensions | Out-String)"
            exit 1
        }
    }
    Write-Log "Azure extensions are not found"
}

# Disable progress writers for this session to greatly speed up operations such as Invoke-WebRequest
$ProgressPreference = 'SilentlyContinue'

try{
    switch ($env:ProvisioningPhase) {
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
            Get-ToolsToVHD # Rely on the completion of Get-FilesToCacheOnVHD
            Get-PrivatePackagesToCacheOnVHD
            Log-ReofferUpdate
        }
        "3" {
            Register-ExpandVolumeTask
            Remove-Item -Path c:\windows-vhd-configuration.ps1
            Cleanup-TemporaryFiles
            (New-Guid).Guid | Out-File -FilePath 'c:\vhd-id.txt'
            Validate-VHDFreeSize
            Test-AzureExtensions
        }
        default {
            Write-Log "Unable to determine provisiong phase... exiting"
            throw "Unable to determine provisiong phase... exiting"
        }
    }
}
finally {
    Get-SystemDriveDiskInfo
    Get-DefenderPreferenceInfo
}
