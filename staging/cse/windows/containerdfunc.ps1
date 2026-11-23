. $PSScriptRoot\helpers.ps1

# this is $global to persist across all functions since this is dot-sourced
$global:ContainerdInstallLocation = "$Env:ProgramFiles\containerd"
$global:Containerdbinary = (Join-Path $global:ContainerdInstallLocation containerd.exe)
# The minimum kubernetes version to use containerd 2.x
$global:MinimalKubernetesVersionWithLatestContainerd2 = "1.32.0"
$global:WindowsDataDir = "C:\AzureData\windows"

# Function so it can be overridden in tests.
function Get-RootRegistryPath {
  return "C:\ProgramData\containerd\certs.d"
}

# Function so it can be mocked in tests.
function Out-FileAscii {
  [CmdletBinding()]
  param(
    [Parameter(ValueFromPipeline = $true)]$Content,
    [Parameter(Mandatory = $true)][string]$FilePath
  )

  # Due to powershell 5.1 being in use sometimes, we can't specify the encoding to be ([System.Text.Encoding]::ASCII) directly in Out-File,
  # To work around this, we specify the encoding as the string "ascii" which is supported in powershell 5.1 and greater and also results in ascii encoding.
  $Content | Out-File -FilePath $FilePath -Encoding "ascii"
}


function RegisterContainerDService {
  Param(
    [Parameter(Mandatory = $true)][string]
    $kubedir
  )

  Assert-FileExists -Filename $global:Containerdbinary -ExitCode $global:WINDOWS_CSE_ERROR_CONTAINERD_BINARY_EXIST

  # in the past service was not installed via nssm so remove it in case
  try {
    Remove-ServiceIfExists -ServiceName "containerd"
  } catch {
    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CONTAINERD_NOT_INSTALLED -ErrorMessage "Failed to remove existing containerd service before registration. Error: $_"
  }

  Write-Log "Registering containerd as a service"
  try {
    # setup containerd
    Invoke-Nssm -KubeDir $KubeDir install containerd $global:Containerdbinary
    Invoke-Nssm -KubeDir $KubeDir set containerd AppDirectory $KubeDir
    Invoke-Nssm -KubeDir $KubeDir set containerd DisplayName containerd
    Invoke-Nssm -KubeDir $KubeDir set containerd Description containerd
    Invoke-Nssm -KubeDir $KubeDir set containerd Start SERVICE_DEMAND_START
    Invoke-Nssm -KubeDir $KubeDir set containerd ObjectName LocalSystem
    Invoke-Nssm -KubeDir $KubeDir set containerd Type SERVICE_WIN32_OWN_PROCESS
    Invoke-Nssm -KubeDir $KubeDir set containerd AppPriority ABOVE_NORMAL_PRIORITY_CLASS
    Invoke-Nssm -KubeDir $KubeDir set containerd AppThrottle 1500
    Invoke-Nssm -KubeDir $KubeDir set containerd AppStdout "$KubeDir\containerd.log"
    Invoke-Nssm -KubeDir $KubeDir set containerd AppStderr "$KubeDir\containerd.err.log"
    Invoke-Nssm -KubeDir $KubeDir set containerd AppRotateFiles 1
    Invoke-Nssm -KubeDir $KubeDir set containerd AppRotateOnline 1
    Invoke-Nssm -KubeDir $KubeDir set containerd AppRotateSeconds 86400
    Invoke-Nssm -KubeDir $KubeDir set containerd AppRotateBytes 10485760
  } catch {
    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CONTAINERD_NOT_INSTALLED -ErrorMessage "Failed to register containerd as a service. Error: $_"
  }

  $retryCount=0
  $maxRetryCount=6 # 1 minutes

  do {
    $svc = Get-Service -Name "containerd" -ErrorAction SilentlyContinue
    if ($null -eq $svc) {
      Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CONTAINERD_NOT_INSTALLED -ErrorMessage "containerd.exe did not get installed as a service correctly."
    }

    if ($svc.Status -eq "Running") {
      Write-Log "containerd service is running"
      break
    }

    $retryCount++
    Write-Log "Starting containerd service (attempt $retryCount of $maxRetryCount), current status: $($svc.Status)"

    try {
      Start-Service containerd -ErrorAction Stop
      $svc = Get-Service -Name "containerd"
      $svc.WaitForStatus('Running', [timespan]::FromSeconds(5))
      Write-Log "containerd service started successfully"
      break
    } catch {
      Write-Log "Failed to start containerd service: $_"
    }

    if ($retryCount -lt $maxRetryCount) {
      Write-Log "Retry $retryCount failed. Waiting 5 seconds before next attempt..."
      Start-Sleep -Seconds 5
    }
  } while ($retryCount -lt $maxRetryCount)

  if ($svc.Status -ne "Running") {
    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CONTAINERD_NOT_RUNNING -ErrorMessage "containerd service is not running"
  }
}

function CreateHypervisorRuntime {
  Param(
    [Parameter(Mandatory = $true)][string]
    $image,
    [Parameter(Mandatory = $true)][string]
    $version,
    [Parameter(Mandatory = $true)][string]
    $buildNumber
  )

  return @"
        [plugins.{{runtimePlugin}}.containerd.runtimes.runhcs-wcow-hypervisor-$buildnumber]
          runtime_type = "io.containerd.runhcs.v1"
          [plugins.{{runtimePlugin}}.containerd.runtimes.runhcs-wcow-hypervisor-$buildnumber.options]
            Debug = true
            DebugType = 2
            SandboxImage = "$image-windows-$version-amd64"
            SandboxPlatform = "windows/amd64"
            SandboxIsolation = 1
            ScaleCPULimitsToSandbox = true
"@
}

function CreateHypervisorRuntimes {
  Param(
    [Parameter(Mandatory = $true)][string[]]
    $builds,
    [Parameter(Mandatory = $true)][string]
    $image
  )

  Write-Log "Adding hyperv runtimes $builds"
  $hypervRuntimes = ""
  ForEach ($buildNumber in $builds) {
    $windowsVersion = Get-WindowsVersion
    $runtime = createHypervisorRuntime -image $pauseImage -version $windowsVersion -buildNumber $buildNumber
    if ($hypervRuntimes -eq "") {
      $hypervRuntimes = $runtime
    } else {
      $hypervRuntimes = $hypervRuntimes + "`r`n" + $runtime
    }
  }

  return $hypervRuntimes
}

function ProcessAndWriteContainerdConfig {
  Param(
    [Parameter(Mandatory = $true)][string]
    $ContainerDVersion,
    [Parameter(Mandatory = $true)][string]
    $CNIBinDir,
    [Parameter(Mandatory = $true)][string]
    $CNIConfDir
  )

  $sandboxIsolation = 0
  if ($global:DefaultContainerdWindowsSandboxIsolation -eq "hyperv") {
    Write-Log "default runtime for containerd set to hyperv"
    $sandboxIsolation = 1
  }

  $clusterConfig = ConvertFrom-Json ((Get-Content $global:KubeClusterConfigPath -ErrorAction Stop) | Out-String)
  $pauseImage = $clusterConfig.Cri.Images.Pause

  $hypervHandlers = $global:ContainerdWindowsRuntimeHandlers.split(",", [System.StringSplitOptions]::RemoveEmptyEntries)
  $hypervRuntimes = ""

  $templateFilePath =  "containerdtemplate.toml"
  $runtimePlugin = "cri"
  if (([version]$ContainerdVersion).CompareTo([version]"2.0.0") -ge 0) {
    $templateFilePath= "containerd2template.toml"
    $runtimePlugin = "'io.containerd.cri.v1.runtime'"
  }

  $templatePath = [Io.Path]::Combine( $global:WindowsDataDir, $templateFilePath)

  $template = Get-Content -Path $templatePath
  if ($SandboxIsolation -eq 0 -And $hypervHandlers.Count -eq 0) {
    # Remove the hypervisors placeholder when not needed
    $template = $template | Select-String -Pattern 'hypervisors' -NotMatch
  } else {
    $hypervRuntimes = CreateHypervisorRuntimes -builds $hypervHandlers -image $pauseImage
  }

  # Set up registry mirrors
  Set-ContainerdRegistryConfig -Registry "docker.io" -RegistryHost "registry-1.docker.io"
  if ((Test-Path variable:global:BootstrapProfileContainerRegistryServer) -and -not [string]::IsNullOrEmpty($global:BootstrapProfileContainerRegistryServer)) {
    Set-BootstrapProfileRegistryContainerdHost
  } else {
    Set-ContainerdRegistryConfig -Registry "mcr.azk8s.cn" -RegistryHost "mcr.azure.cn"
  }

  if (([version]$ContainerdVersion).CompareTo([version]"1.7.9") -lt 0) {
    # Remove annotations placeholders for older containerd versions
    $template = $template | Select-String -Pattern 'containerAnnotations' -NotMatch
    $template = $template | Select-String -Pattern 'podAnnotations' -NotMatch
  }

  # Convert template to string for placeholder replacements
  $template = $template | Out-String
  $formatedbin = $(($CNIBinDir).Replace("\", "/"))
  $formatedconf = $(($CNIConfDir).Replace("\", "/"))
  $containerAnnotations = 'container_annotations = ["io.microsoft.container.processdumplocation", "io.microsoft.wcow.processdumptype", "io.microsoft.wcow.processdumpcount"]'
  $podAnnotations = 'pod_annotations = ["io.microsoft.container.processdumplocation","io.microsoft.wcow.processdumptype", "io.microsoft.wcow.processdumpcount"]'

  # Replace all placeholders
  $processedTemplate = $template.Replace('{{sandboxIsolation}}', $sandboxIsolation).
  Replace('{{pauseImage}}', $pauseImage).
  Replace('{{hypervisors}}', $hypervRuntimes).
  Replace('{{runtimePlugin}}', $runtimePlugin).
  Replace('{{cnibin}}', $formatedBin).
  Replace('{{cniconf}}', $formatedConf).
  Replace('{{containerAnnotations}}', $containerAnnotations).
  Replace('{{podAnnotations}}', $podAnnotations)

  # Write the processed template to the config file
  $configFile = [Io.Path]::Combine($global:ContainerdInstallLocation, "config.toml")
  Write-Log "using template $templatePath to write containerd config to $configFile"
  $processedTemplate | Out-FileAscii -FilePath $configFile
}

function Enable-Logging {
  if ((Test-Path "$global:ContainerdInstallLocation\diag.ps1") -And (Test-Path "$global:ContainerdInstallLocation\ContainerPlatform.wprp")) {
    $logs = Join-path $pwd.drive.Root logs
    Write-Log "Containerd hyperv logging enabled; temp location $logs"
    $diag = Join-Path $global:ContainerdInstallLocation diag.ps1
    Create-Directory -FullPath $logs -DirectoryUsage "storing containerd logs"
    # !ContainerPlatformPersistent profile is made to work with long term and boot tracing
    & $diag -Start -ProfilePath "$global:ContainerdInstallLocation\ContainerPlatform.wprp!ContainerPlatformPersistent" -TempPath $logs
  } else {
    Write-Log "Containerd hyperv logging script not avalaible"
  }
}

function Set-ContainerdRegistryConfig {
  Param(
    [Parameter(Mandatory = $true)][string] $Registry,
    [Parameter(Mandatory = $true)][string] $RegistryHost
  )

  $certRootRegistryPath = Get-RootRegistryPath

  $registryPath = Join-Path $certRootRegistryPath $Registry
  $hostsTomlPath = Join-Path $registryPath "hosts.toml"

  Create-Directory -FullPath $registryPath -DirectoryUsage "storing containerd registry hosts config"

  $content = @"
server = "https://$Registry"

[host."https://$RegistryHost"]
  capabilities = ["pull", "resolve"]
[host."https://$RegistryHost".header]
    X-Forwarded-For = ["$Registry"]
"@

  $content | Out-FileAscii -FilePath $hostsTomlPath
  Write-Log "Wrote containerd hosts config for registry '$Registry' to '$hostsTomlPath'"
}

function Set-BootstrapProfileRegistryContainerdHost {
  $mcrRegistry = if ((Test-Path variable:global:MCRRepositoryBase) -and
    -not [string]::IsNullOrEmpty($global:MCRRepositoryBase)) {
    [string]$global:MCRRepositoryBase
  } else {
    "mcr.microsoft.com"
  }
  $certRootRegistryPath = Get-RootRegistryPath
  $mcrRegistryPath = Join-Path $certRootRegistryPath $mcrRegistry
  $hostsTomlPath = Join-Path $mcrRegistryPath "hosts.toml"

  $registryHost = [string]$global:BootstrapProfileContainerRegistryServer
  $registryHost = ($registryHost -replace '^https?://', '').TrimEnd('/')

  $registryHostParts = $registryHost.Split('/', 2)
  $registryHostName = $registryHostParts[0]
  $registryRepoPrefix = if ($registryHostParts.Length -gt 1) { $registryHostParts[1].Trim('/') } else { "" }

  $registryHost = if ([string]::IsNullOrEmpty($registryRepoPrefix)) {
    "$registryHostName/v2"
  } else {
    "$registryHostName/v2/$registryRepoPrefix"
  }

  Create-Directory -FullPath $mcrRegistryPath -DirectoryUsage "storing containerd registry hosts config"

  $content = @"
server = "https://$mcrRegistry"

[host."https://$registryHost"]
  capabilities = ["pull", "resolve"]
  override_path = true
"@

  $content | Out-FileAscii -FilePath $hostsTomlPath
  Write-Log "Wrote bootstrap profile container registry hosts config from '$mcrRegistry' to '$registryHost' at '$hostsTomlPath'"
}

function Get-ContainerdVersionFromString {
  Param(
    [Parameter(Mandatory = $true)][string]
    $VersionString
  )

  # Returns major.minor.patch from `containerd -v` output. The leading "v" is optional and any
  # "+build" or "-revision" suffix is dropped, matching how Linux normalizes the package version.
  # Examples:
  #  - containerd github.com/containerd/containerd v1.7.20+azure 4f03e100cb967922bec7459a78d16ccbac9bb81d
  #  - containerd github.com/containerd/containerd/v2 2.3.7-4 6f589cf13dc01a5b493911355427e05b2ff005d7
  $fields = $VersionString.Trim().Split(" ", [System.StringSplitOptions]::RemoveEmptyEntries)
  if ($fields.Count -ge 3 -and $fields[2] -match '^v?(\d+\.\d+\.\d+)') {
    return $matches[1]
  }
  throw "Unable to parse containerd version from '$VersionString'"
}

function Install-Containerd {
  Param(
    [Parameter(Mandatory = $true)][string]
    $ContainerdUrl,
    [Parameter(Mandatory = $true)][string]
    $CNIBinDir,
    [Parameter(Mandatory = $true)][string]
    $CNIConfDir,
    [Parameter(Mandatory = $true)][string]
    $KubeDir
  )

  $cachedPackages = Get-CachedContainerdComponentPackages -ContainerdUrl $ContainerdUrl
  if ($null -ne $cachedPackages) {
    Logs-To-Event -TaskName "AKS.WindowsCSE.InstallContainerdFromCachedComponentPackages" -TaskMessage "Installing cached DALEC packages instead of $ContainerdUrl. Packages: $($cachedPackages -join ', ')"
    Install-ContainerdFromLocalPackages -Packages $cachedPackages -CNIBinDir $CNIBinDir -CNIConfDir $CNIConfDir -KubeDir $KubeDir
    return
  }

  $svc = Get-Service -Name containerd -ErrorAction SilentlyContinue
  # TODO: check if containerd is already installed and is the same version before this.
  if ($null -ne $svc) {
    Write-Log "Stopping containerd service"
    $svc | Stop-Service
  }

  # Extract the package
  # upstream containerd package is a tar
  $tarfile = [Io.path]::Combine($ENV:TEMP, "containerd.tar.gz")

  if ([string]::IsNullOrEmpty($global:BootstrapProfileContainerRegistryServer)) {
    # default path
    # download containerd binaries via http if BootstrapProfileContainerRegistryServer is not set
    DownloadFileOverHttp -Url $ContainerdUrl -DestinationPath $tarfile -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_CONTAINERD_PACKAGE
  } else {
    # ni path
    # download containerd binaries via oras if BootstrapProfileContainerRegistryServer is set
    if (-not (Get-Command 'DownloadFileWithOras' -ErrorAction SilentlyContinue)) {
      Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULL_CONTAINERD -ErrorMessage "DownloadFileWithOras function is not available. networkisolatedclusterfunc.ps1 may not be sourced."
    }

    # Extract containerd version tag from URL for ORAS reference
    # URL format: https://packages.aks.azure.com/containerd/windows/v1.7.20-azure.1/binaries/containerd-v1.7.20-azure.1-windows-amd64.tar.gz
    if ($ContainerdUrl -match "containerd-(v[\d.]+)-azure\.\d+-windows-amd64\.tar\.gz") {
      $containerdVersionTag = $matches[1]
    } else {
      Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULL_CONTAINERD -ErrorMessage "Failed to extract containerd version tag from URL: $ContainerdUrl"
    }

    # Sanitize the registry server value: strip scheme and trailing slash, preserve any repo prefix
    $sanitizedRegistry = ($global:BootstrapProfileContainerRegistryServer -replace '^https?://', '').TrimEnd('/')

    Logs-To-Event -TaskName "AKS.WindowsCSE.DownloadContainerdWithOras" -TaskMessage "Start to download containerd with oras. ContainerdVersionTag: $containerdVersionTag, BootstrapProfileContainerRegistryServer: $global:BootstrapProfileContainerRegistryServer"
    $orasReference = "$sanitizedRegistry/aks/packages/containerd/containerd:$containerdVersionTag"
    $cachedFileName = Get-FileNameFromUrl -Url $ContainerdUrl
    try {
      Retry-Command -Command "DownloadFileWithOras" -Args @{Reference=$orasReference; DestinationPath=$tarfile; CachedFile=$cachedFileName} -Retries 5 -RetryDelaySeconds 10
    } catch {
      Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ORAS_PULL_CONTAINERD -ErrorMessage "Exhausted retries for oras pull $orasReference. Error: $_"
    }
  }
  Create-Directory -FullPath $global:ContainerdInstallLocation -DirectoryUsage "storing containerd"
  tar -xzf $tarfile -C $global:ContainerdInstallLocation
  if ($LASTEXITCODE -ne 0) {
    throw "Failed to extract the '$tarfile' archive."
  }
  Move-Item -Force $global:ContainerdInstallLocation\bin\* $global:ContainerdInstallLocation\
  Remove-Item -Path $tarfile -Force
  Remove-Item -Path $global:ContainerdInstallLocation\bin -Force -Recurse

  Complete-ContainerdInstallation -CNIBinDir $CNIBinDir -CNIConfDir $CNIConfDir -KubeDir $KubeDir
}

# Combined containerd packages that RP requests by default, mapped to their containerd line. A node
# asked for one of these installs the newest DALEC packages cached on the VHD for that line instead.
# Any other combined package, such as a rollback or hotfix set through the windows-containerd-versions
# toggle, is installed as requested, so pointing that toggle elsewhere opts nodes out of DALEC.
$global:DalecReplaceableContainerdPackages = @{
  "containerd-v1.7.20-azure.1-windows-amd64.tar.gz" = "1.7"
  "containerd-v2.0.4-azure.1-windows-amd64.tar.gz"  = "2.0"
}

# Returns the newest cached DALEC ZIP for a package, optionally limited to one major.minor line.
function Select-NewestCachedDalecPackage {
  Param(
    [Parameter(Mandatory = $true)][string]
    $Name,
    [Parameter(Mandatory = $false)][string]
    $Line = ""
  )

  $directory = [Io.path]::Combine($global:CacheDir, "containerd-packages", $Name)
  if (-not (Test-Path -LiteralPath $directory -PathType Container)) {
    return $null
  }

  $pattern = "^" + [regex]::Escape($Name) + "_(?<version>\d+\.\d+\.\d+)-(?<revision>\d+)_amd64\.zip$"
  $selected = $null
  foreach ($file in Get-ChildItem -LiteralPath $directory -File) {
    $match = [regex]::Match($file.Name, $pattern)
    if (-not $match.Success) {
      continue
    }
    $version = [version]$match.Groups['version'].Value
    if ($Line -and "$($version.Major).$($version.Minor)" -ne $Line) {
      continue
    }
    $revision = [int]$match.Groups['revision'].Value
    if ($null -eq $selected -or $version -gt $selected.Version -or ($version -eq $selected.Version -and $revision -gt $selected.Revision)) {
      $selected = [pscustomobject]@{ Version = $version; Revision = $revision; Path = $file.FullName }
    }
  }
  if ($null -eq $selected) {
    return $null
  }
  return $selected.Path
}

# Returns the cached hcsshim, moby-containerd and kubernetes-cri-tools ZIPs that replace the requested
# combined package, or $null to install the combined package.
function Get-CachedContainerdComponentPackages {
  Param(
    [Parameter(Mandatory = $true)][string]
    $ContainerdUrl
  )

  $fileName = [IO.Path]::GetFileName($ContainerdUrl.Split('?')[0])
  $line = $global:DalecReplaceableContainerdPackages[$fileName]
  if ([string]::IsNullOrEmpty($line)) {
    Write-Log "Containerd package $fileName has no DALEC replacement; installing it as requested"
    return $null
  }
  if ([string]::IsNullOrEmpty($global:CacheDir)) {
    Write-Log "No cache directory; installing $fileName"
    return $null
  }

  $packages = @(
    (Select-NewestCachedDalecPackage -Name "hcsshim"),
    (Select-NewestCachedDalecPackage -Name "moby-containerd" -Line $line),
    (Select-NewestCachedDalecPackage -Name "kubernetes-cri-tools")
  )
  if ($packages -contains $null) {
    Write-Log "Cached DALEC packages for containerd $line are incomplete ($($packages -join ', ')); installing $fileName"
    return $null
  }
  return $packages
}

# Gets every package into a staging directory before anything is installed, so a failed download
# leaves the node untouched. Returns the staged paths, or $null if any package can't be fetched.
function Save-ContainerdComponentPackages {
  Param(
    [Parameter(Mandatory = $true)][string[]]
    $Urls,
    [Parameter(Mandatory = $true)][string]
    $StagingDirectory
  )

  New-Item -ItemType Directory -Path $StagingDirectory -Force | Out-Null
  $packages = @()
  foreach ($url in $Urls) {
    $fileName = [IO.Path]::GetFileName($url.Split('?')[0])
    $destination = [Io.path]::Combine($StagingDirectory, $fileName)
    try {
      $cached = @()
      if ($global:CacheDir -and (Test-Path -LiteralPath $global:CacheDir)) {
        $cached = [IO.Directory]::GetFiles($global:CacheDir, $fileName, [IO.SearchOption]::AllDirectories)
      }
      if ($cached.Count -gt 0) {
        Write-Log "Using cached $fileName from $($cached[0])"
        Copy-Item -LiteralPath $cached[0] -Destination $destination -Force
      } else {
        $mappedUrl = $url
        if (Get-Command -Name Update-BaseUrl -ErrorAction SilentlyContinue) {
          $mappedUrl = Update-BaseUrl -InitialUrl $url
        }
        Write-Log "Downloading $mappedUrl to $destination"
        Retry-Command -Command "Invoke-RestMethod" -Args @{Uri=$mappedUrl; Method="Get"; OutFile=$destination; ErrorAction="Stop"} -Retries 5 -RetryDelaySeconds 10
      }
    } catch {
      Write-Log "Failed to get containerd component package $url. Error: $_"
      return $null
    }
    $packages += $destination
  }
  return $packages
}

# Installs containerd from local DALEC hcsshim, moby-containerd and kubernetes-cri-tools Windows ZIPs.
# Each ZIP has its files at the root, so extracting all three gives the same flat layout as the
# combined containerd package, except crictl.yaml, which is written here.
function Install-ContainerdFromLocalPackages {
  Param(
    [Parameter(Mandatory = $true)][string[]]
    $Packages,
    [Parameter(Mandatory = $true)][string]
    $CNIBinDir,
    [Parameter(Mandatory = $true)][string]
    $CNIConfDir,
    [Parameter(Mandatory = $true)][string]
    $KubeDir
  )

  $svc = Get-Service -Name containerd -ErrorAction SilentlyContinue
  if ($null -ne $svc) {
    Write-Log "Stopping containerd service"
    $svc | Stop-Service
  }

  Create-Directory -FullPath $global:ContainerdInstallLocation -DirectoryUsage "storing containerd"
  foreach ($package in $Packages) {
    AKS-Expand-Archive -Path $package -DestinationPath $global:ContainerdInstallLocation
  }

  $crictlConfig = "runtime-endpoint: npipe://\\.\pipe\containerd-containerd`nimage-endpoint: npipe://\\.\pipe\containerd-containerd`ntimeout: 0`ndebug: false`n"
  [System.IO.File]::WriteAllText([Io.path]::Combine($global:ContainerdInstallLocation, "crictl.yaml"), $crictlConfig)

  Complete-ContainerdInstallation -CNIBinDir $CNIBinDir -CNIConfDir $CNIConfDir -KubeDir $KubeDir
}

# Installs the DALEC packages requested by RP. If any of them can't be fetched, falls back to the
# combined package path, which prefers the DALEC packages cached on the VHD.
function Install-ContainerdFromComponentPackages {
  Param(
    [Parameter(Mandatory = $true)][string]
    $HcsshimUrl,
    [Parameter(Mandatory = $true)][string]
    $MobyContainerdUrl,
    [Parameter(Mandatory = $true)][string]
    $CrictlUrl,
    [Parameter(Mandatory = $true)][string]
    $CNIBinDir,
    [Parameter(Mandatory = $true)][string]
    $CNIConfDir,
    [Parameter(Mandatory = $true)][string]
    $KubeDir
  )

  $stagingDirectory = [Io.path]::Combine($ENV:TEMP, "containerd-packages")
  $packages = Save-ContainerdComponentPackages -Urls @($HcsshimUrl, $MobyContainerdUrl, $CrictlUrl) -StagingDirectory $stagingDirectory
  if ($null -ne $packages) {
    Logs-To-Event -TaskName "AKS.WindowsCSE.InstallContainerdFromComponentPackages" -TaskMessage "Installing containerd from the component packages requested by RP. Hcsshim: $HcsshimUrl, MobyContainerd: $MobyContainerdUrl, Crictl: $CrictlUrl"
    Install-ContainerdFromLocalPackages -Packages $packages -CNIBinDir $CNIBinDir -CNIConfDir $CNIConfDir -KubeDir $KubeDir
    Remove-Item -LiteralPath $stagingDirectory -Recurse -Force -ErrorAction SilentlyContinue
    return
  }
  Remove-Item -LiteralPath $stagingDirectory -Recurse -Force -ErrorAction SilentlyContinue

  if ([string]::IsNullOrWhiteSpace($global:ContainerdUrl)) {
    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_CONTAINERD_PACKAGE -ErrorMessage "Failed to get the containerd component packages and no combined containerd package to fall back to"
  }
  Logs-To-Event -TaskName "AKS.WindowsCSE.ContainerdComponentPackagesFallback" -TaskMessage "Failed to get the component packages requested by RP; falling back to $global:ContainerdUrl. Hcsshim: $HcsshimUrl, MobyContainerd: $MobyContainerdUrl, Crictl: $CrictlUrl"
  # Without the component URLs, this resolves a ContainerdUrl endpoint to a package for the Kubernetes
  # version and calls Install-Containerd.
  Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl $global:ContainerdUrl -CNIBinDir $CNIBinDir -CNIConfDir $CNIConfDir -KubeDir $KubeDir -KubernetesVersion $global:KubeBinariesVersion
}

function Complete-ContainerdInstallation {
  Param(
    [Parameter(Mandatory = $true)][string]
    $CNIBinDir,
    [Parameter(Mandatory = $true)][string]
    $CNIConfDir,
    [Parameter(Mandatory = $true)][string]
    $KubeDir
  )

  # get configuration options
  Add-SystemPathEntry $global:ContainerdInstallLocation

  $containerdVersion ="0.0.0"
  try {
    # Check containerd version to determine if it supports annotations
    Push-Location $global:ContainerdInstallLocation
    $versionstring=$(.\containerd.exe -v)
    Write-Log "containerd version: $versionstring"
    $containerdVersion = Get-ContainerdVersionFromString -VersionString $versionstring
    Pop-Location
  } catch {
    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GET_CONTAINERD_VERSION -ErrorMessage "Failed in getting Windows containerd version. Error: $_"
  }

  ProcessAndWriteContainerdConfig `
    -ContainerDVersion $containerdVersion `
    -CNIBinDir $CNIBinDir `
    -CNIConfDir $CNIConfDir

  RegisterContainerDService -KubeDir $KubeDir
  if (-not [string]::IsNullOrEmpty($global:BootstrapProfileContainerRegistryServer)) {
    if (Get-Command -Name Set-PodInfraContainerImage -ErrorAction SilentlyContinue) {
      Set-PodInfraContainerImage
    } else {
      Write-Log "Set-PodInfraContainerImage command not found; skipping pod infra container image configuration."
    }
  }
  Enable-Logging
}
