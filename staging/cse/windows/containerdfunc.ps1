# this is $global to persist across all functions since this is dot-sourced
$global:ContainerdInstallLocation = "$Env:ProgramFiles\containerd"
$global:Containerdbinary = (Join-Path $global:ContainerdInstallLocation containerd.exe)

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
  if ($LASTEXITCODE -ne 0) {
    throw "Failed to extract the '$tarfile' archive."
  }
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
  $windowsVersion = Get-WindowsPauseVersion
  $hypervRuntimes = ""
  $hypervHandlers = $global:ContainerdWindowsRuntimeHandlers.split(",", [System.StringSplitOptions]::RemoveEmptyEntries)
  $containerAnnotations = 'container_annotations = ["io.microsoft.container.processdumplocation", "io.microsoft.wcow.processdumptype", "io.microsoft.wcow.processdumpcount"]'
  $podAnnotations = 'pod_annotations = ["io.microsoft.container.processdumplocation","io.microsoft.wcow.processdumptype", "io.microsoft.wcow.processdumpcount"]'

  # configure
  if ($global:DefaultContainerdWindowsSandboxIsolation -eq "hyperv") {
    Write-Log "default runtime for containerd set to hyperv"
    $sandboxIsolation = 1
  }

  $template = Get-Content -Path "c:\AzureData\windows\containerdtemplate.toml" 
  if ($sandboxIsolation -eq 0 -And $hypervHandlers.Count -eq 0) {
    # remove the value hypervisor place holder
    $template = $template | Select-String -Pattern 'hypervisors' -NotMatch
  }
  else {
    $hypervRuntimes = CreateHypervisorRuntimes -builds @($hypervHandlers) -image $pauseImage
  }

  try {
    # remove the value containerAnnotations and podAnnotations place holder since it is not supported in containerd versions older than 1.7.9
    pushd $global:ContainerdInstallLocation
      # Examples:
      #  - containerd github.com/containerd/containerd v1.6.21+azure 3dce8eb055cbb6872793272b4f20ed16117344f8
      #  - containerd github.com/containerd/containerd v1.7.9+azure 4f03e100cb967922bec7459a78d16ccbac9bb81d
      $versionstring=$(.\containerd.exe -v)
      Write-Log "containerd version: $versionstring"
      $containerdVersion=$versionstring.split(" ")[2].Split("+")[0].substring(1)
    popd

    if (([version]$containerdVersion).CompareTo([version]"1.7.9") -lt 0) {
      # remove the value containerAnnotations place holder
      $template = $template | Select-String -Pattern 'containerAnnotations' -NotMatch
      # remove the value podAnnotations place holder
      $template = $template | Select-String -Pattern 'podAnnotations' -NotMatch
    }
  } catch {
      Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GET_CONTAINERD_VERSION -ErrorMessage "Failed in getting Windows containerd version. Error: $_"
  }

  # Need to convert the template to string to replace the place holders but
  # `Select-String -Pattern [PATTERN] -NotMatch` does not work after converting the template to string
  $template =  $template | Out-String
  $template.Replace('{{sandboxIsolation}}', $sandboxIsolation).
  Replace('{{pauseImage}}', $pauseImage).
  Replace('{{hypervisors}}', $hypervRuntimes).
  Replace('{{cnibin}}', $formatedbin).
  Replace('{{cniconf}}', $formatedconf).
  Replace('{{currentversion}}', $windowsVersion).
  Replace('{{containerAnnotations}}', $containerAnnotations).
  Replace('{{podAnnotations}}', $podAnnotations) | `
    Out-File -FilePath "$configFile" -Encoding ascii

  RegisterContainerDService -KubeDir $KubeDir
  Enable-Logging
}
