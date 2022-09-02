# this is $global to persist across all functions since this is dot-sourced
$global:ContainerdInstallLocation = "$Env:ProgramFiles\containerd"
$global:Containerdbinary = (Join-Path $global:ContainerdInstallLocation containerd.exe)

# NOTE: KubernetesVersion does not contain "v"
$global:MinimalKubernetesVersionWithLatestContainerd = "1.30.0" # Will change it to the correct version when we support new Windows containerd version
$global:StableContainerdPackage = "v0.0.47/binaries/containerd-v0.0.47-windows-amd64.tar.gz"
# The containerd package name may be changed in future
$global:LatestContainerdPackage = "v1.0.46/binaries/containerd-v1.0.46-windows-amd64.tar.gz" # It does not exist and is only for test for now

function RegisterContainerDService {
  Param(
    [Parameter(Mandatory = $true)][string]
    $kubedir
  )

  Assert-FileExists -Filename $global:Containerdbinary -ExitCode $global:WINDOWS_CSE_ERROR_CONTAINERD_BINARY_EXIST

  # in the past service was not installed via nssm so remove it in case
  $svc = Get-Service -Name "containerd" -ErrorAction SilentlyContinue
  if ($null -ne $svc) {
    sc.exe delete containerd
  }

  Write-Log "Registering containerd as a service"
  # setup containerd
  & "$KubeDir\nssm.exe" install containerd $global:Containerdbinary | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppDirectory $KubeDir | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd DisplayName containerd | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd Description containerd | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd Start SERVICE_DEMAND_START | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd ObjectName LocalSystem | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppThrottle 1500 | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppStdout "$KubeDir\containerd.log" | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppStderr "$KubeDir\containerd.err.log" | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppRotateFiles 1 | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppRotateOnline 1 | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppRotateSeconds 86400 | RemoveNulls
  & "$KubeDir\nssm.exe" set containerd AppRotateBytes 10485760 | RemoveNulls

  $retryCount=0
  $retryInterval=10
  $maxRetryCount=6 # 1 minutes

  do {
    $svc = Get-Service -Name "containerd" -ErrorAction SilentlyContinue
    if ($null -eq $svc) {
      Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CONTAINERD_NOT_INSTALLED -ErrorMessage "containerd.exe did not get installed as a service correctly."
    }
    if ($svc.Status -eq "Running") {
      break
    }
    Write-Log "Starting containerd, current status: $svc.Status"
    Start-Service containerd
    $retryCount++
    Write-Log "Retry $retryCount : Sleep $retryInterval and check containerd status"
    Sleep $retryInterval
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
        [plugins.cri.containerd.runtimes.runhcs-wcow-hypervisor-$buildnumber]
          runtime_type = "io.containerd.runhcs.v1"
          [plugins.cri.containerd.runtimes.runhcs-wcow-hypervisor-$buildnumber.options]
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
    }
    else {
      $hypervRuntimes = $hypervRuntimes + "`r`n" + $runtime
    }
  }

  return $hypervRuntimes
}

function Enable-Logging {
  if ((Test-Path "$global:ContainerdInstallLocation\diag.ps1") -And (Test-Path "$global:ContainerdInstallLocation\ContainerPlatform.wprp")) {
    $logs = Join-path $pwd.drive.Root logs
    Write-Log "Containerd hyperv logging enabled; temp location $logs"
    $diag = Join-Path $global:ContainerdInstallLocation diag.ps1
    Create-Directory -FullPath $logs -DirectoryUsage "storing containerd logs"
    # !ContainerPlatformPersistent profile is made to work with long term and boot tracing
    & $diag -Start -ProfilePath "$global:ContainerdInstallLocation\ContainerPlatform.wprp!ContainerPlatformPersistent" -TempPath $logs
  }
  else {
    Write-Log "Containerd hyperv logging script not avalaible"
  }
}

function Install-Containerd-Based-On-Kubernetes-Version {
  Param(
    [Parameter(Mandatory = $true)][string]
    $ContainerdUrl,
    [Parameter(Mandatory = $true)][string]
    $CNIBinDir,
    [Parameter(Mandatory = $true)][string]
    $CNIConfDir,
    [Parameter(Mandatory = $true)][string]
    $KubeDir,
    [Parameter(Mandatory = $true)][string]
    $KubernetesVersion
  )

  # In the past, $global:ContainerdUrl is a full URL to download Windows containerd package.
  # Example: "https://acs-mirror.azureedge.net/containerd/windows/v0.0.46/binaries/containerd-v0.0.46-windows-amd64.tar.gz"
  # To support multiple containerd versions, we only set the endpoint in $global:ContainerdUrl.
  # Example: "https://acs-mirror.azureedge.net/containerd/windows/"
  # We only set containerd package based on kubernetes version when $global:ContainerdUrl ends with "/" so we support:
  #   1. Current behavior to set the full URL
  #   2. Setting containerd package in toggle for test purpose or hotfix
  if ($ContainerdUrl.EndsWith("/")) {
    Write-Log "ContainerdURL is $ContainerdUrl"
    $containerdPackage=$global:StableContainerdPackage
    if (([version]$KubernetesVersion).CompareTo([version]$global:MinimalKubernetesVersionWithLatestContainerd) -ge 0) {
      $containerdPackage=$global:LatestContainerdPackage
      Write-Log "Kubernetes version $KubernetesVersion is greater than or equal to $global:MinimalKubernetesVersionWithLatestContainerd so the latest containerd version $containerdPackage is used"
    } else {
      Write-Log "Kubernetes version $KubernetesVersion is less than $global:MinimalKubernetesVersionWithLatestContainerd so the stable containerd version $containerdPackage is used"
    }
    $ContainerdUrl = $ContainerdUrl + $containerdPackage
  }
  Install-Containerd -ContainerdUrl $ContainerdUrl -CNIBinDir $CNIBinDir -CNIConfDir $CNIConfDir -KubeDir $KubeDir
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

  $svc = Get-Service -Name containerd -ErrorAction SilentlyContinue
  if ($null -ne $svc) {
    Write-Log "Stoping containerd service"
    $svc | Stop-Service
  }

  # TODO: check if containerd is already installed and is the same version before this.
  
  # Extract the package
  # upstream containerd package is a tar 
  $tarfile = [Io.path]::Combine($ENV:TEMP, "containerd.tar.gz")
  DownloadFileOverHttp -Url $ContainerdUrl -DestinationPath $tarfile -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_CONTAINERD_PACKAGE
  Create-Directory -FullPath $global:ContainerdInstallLocation -DirectoryUsage "storing containerd"
  tar -xzf $tarfile -C $global:ContainerdInstallLocation

  mv -Force $global:ContainerdInstallLocation\bin\* $global:ContainerdInstallLocation\
  Remove-Item -Path $tarfile -Force
  Remove-Item -Path $global:ContainerdInstallLocation\bin -Force -Recurse

  # get configuration options
  Add-SystemPathEntry $global:ContainerdInstallLocation
  $configFile = [Io.Path]::Combine($global:ContainerdInstallLocation, "config.toml")
  $clusterConfig = ConvertFrom-Json ((Get-Content $global:KubeClusterConfigPath -ErrorAction Stop) | Out-String)
  $pauseImage = $clusterConfig.Cri.Images.Pause
  $formatedbin = $(($CNIBinDir).Replace("\", "/"))
  $formatedconf = $(($CNIConfDir).Replace("\", "/"))
  $sandboxIsolation = 0
  $windowsVersion = Get-WindowsVersion
  $hypervRuntimes = ""
  $hypervHandlers = $global:ContainerdWindowsRuntimeHandlers.split(",", [System.StringSplitOptions]::RemoveEmptyEntries)

  # configure
  if ($global:DefaultContainerdWindowsSandboxIsolation -eq "hyperv") {
    Write-Log "default runtime for containerd set to hyperv"
    $sandboxIsolation = 1
  }

  $template = Get-Content -Path "c:\AzureData\windows\containerdtemplate.toml" 
  if ($sandboxIsolation -eq 0 -And $hypervHandlers.Count -eq 0) {
    # remove the value hypervisor place holder
    $template = $template | Select-String -Pattern 'hypervisors' -NotMatch | Out-String
  }
  else {
    $hypervRuntimes = CreateHypervisorRuntimes -builds @($hypervHandlers) -image $pauseImage
  }

  $template.Replace('{{sandboxIsolation}}', $sandboxIsolation).
  Replace('{{pauseImage}}', $pauseImage).
  Replace('{{hypervisors}}', $hypervRuntimes).
  Replace('{{cnibin}}', $formatedbin).
  Replace('{{cniconf}}', $formatedconf).
  Replace('{{currentversion}}', $windowsVersion) | `
    Out-File -FilePath "$configFile" -Encoding ascii

  RegisterContainerDService -KubeDir $KubeDir
  Enable-Logging
}
