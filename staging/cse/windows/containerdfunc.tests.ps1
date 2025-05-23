BeforeAll {
  # Define mock functions before loading the scripts
  function Write-Log {
    param($Message)
    # Do nothing in tests - this is a mock implementation
  }

  function Set-ExitCode {
    param($ExitCode, $ErrorMessage)
    Write-Host "MOCK: Exit Code would be: $ExitCode, Error: $ErrorMessage"
    # Don't actually exit in tests
  }

  # Mock Set-Content to avoid permission denied errors
  Mock Set-Content -MockWith {
    param($Path, $Value)
    Write-Host "SET-CONTENT: Path: $Path, Content: $Value"
  }
  function Get-WindowsPauseVersion {
    return "ltsc2022"
  }

  function Get-WindowsVersion {
    return "ltsc2022"
  }

  . $PSScriptRoot\containerdfunc.ps1
  . $PSCommandPath.Replace('.tests.ps1','.ps1')
  # . $PSScriptRoot\..\..\parts\windows\windowscsehelper.ps1
}

Describe "ProcessAndWriteContainerdConfig" {
  BeforeAll{
    Mock Get-Content -ParameterFilter { $Path -like "*kubeclusterconfig.json" } -MockWith { 
      return "{`"Cri`":{`"Images`":{`"Pause`":`"$pauseImage`"}}}"
    }
  }

  It "Should use <expected> for containerd version <version>" -ForEach @(
    @{ version = "1.7.9"; expected = "containerdtemplate.toml" },
    @{ version = "2.0.4"; expected = "containerd2template.toml" }) {
    $templatePath = GetContainerdTemplatePath -ContainerDVersion $version
    $templatePath | Should -Match $expected
  }
  
  Context 'v1 containerdtemplate.toml' {
    BeforeAll {
      $cniBinDir = 'C:/cni/bin'
      $cniConfDir = 'C:/cni/conf'
      $pauseImage = 'mcr.microsoft.com/oss/kubernetes/pause:3.6'
      $containerdVersion = "1.7.9"

      $containerdDir = "$PSScriptRoot\containerdfunc.tests.suites"
      $templatePath = Join-Path $PSScriptRoot "containerdtemplate.toml"
      $global:KubeClusterConfigPath = [Io.path]::Combine("", "kubeclusterconfig.json")
      $global:ContainerdInstallLocation = $containerdDir
      $global:ContainerdConfigLocation = $PSScriptRoot
      $configPath = Join-Path $global:ContainerdInstallLocation "config.toml"
    }

    It "Should process containerdtemplate.toml with basic configuration" {
      # Set up paths for the test
      $global:DefaultContainerdWindowsSandboxIsolation = "process" # default to process isolation
      $global:ContainerdWindowsRuntimeHandlers = "" # default to no hyperv handlers

      { ProcessAndWriteContainerdConfig -ContainerDVersion $containerdVersion -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw
      
      $configPath | Should -Exist
      $content = Get-Content -Path $configPath -Raw
      $content | Should -Not -BeNullOrEmpty
      
      # Check that placeholders are replaced
      $content | Should -Not -Match ([regex]::Escape("{{"))
      
      # Check that the values were replaced correctly
      $content | Should -Match $pauseImage
      $content | Should -Match $cniBinDir
      $content | Should -Match $cniConfDir
      $content | Should -Match 'SandboxIsolation = 0'
    }

    It "Should include hyperv runtimes when hyperv is enabled" {
      $global:DefaultContainerdWindowsSandboxIsolation = "hyperv"
      $global:ContainerdWindowsRuntimeHandlers = "1234,5678"
      { ProcessAndWriteContainerdConfig -ContainerDVersion $containerdVersion -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw
      
      $content = Get-Content -Path $configPath -Raw
      $content | Should -Match 'plugins.cri.containerd.runtimes.runhcs-wcow-hypervisor-1234'
      $content | Should -Match 'SandboxIsolation = 1'
    }

    It "Should handle older containerd versions (<1.7.9) by removing annotations" {
      # Call the function under test and ensure it doesn't throw
      { ProcessAndWriteContainerdConfig -ContainerDVersion "1.6.10" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw
      
      $content = Get-Content -Path $configPath -Raw
      
      # Should not contain annotation placeholders or values
      $content | Should -Not -Match 'container_annotations'
      $content | Should -Not -Match 'pod_annotations'
    }
  }

  Context 'v2 containerdtemplate.toml' {
    BeforeAll {
      $containerdDir = "$PSScriptRoot\containerdfunc.tests.suites"
      $cniBinDir = 'C:/cni/bin'
      $cniConfDir = 'C:/cni/conf'
      $pauseImage = 'mcr.microsoft.com/oss/kubernetes/pause:3.6'
      $contanerdVersion = "2.0.4"

      $global:KubeClusterConfigPath = [Io.path]::Combine("", "kubeclusterconfig.json")
      $global:ContainerdInstallLocation = $containerdDir
      $global:ContainerdConfigLocation = $PSScriptRoot
      $templatePath = Join-Path $PSScriptRoot "containerdtemplate2.toml"
      $configPath = Join-Path $global:ContainerdInstallLocation "config.toml"
    }

    It "Should process containerd2template.toml with basic configuration" {
      # Set up paths for the test
      $global:DefaultContainerdWindowsSandboxIsolation = "process" # default to process isolation
      $global:ContainerdWindowsRuntimeHandlers = "" # default to no hyperv handlers

      { ProcessAndWriteContainerdConfig  -ContainerDVersion "2.0.4" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw
      
      $configPath | Should -Exist
      $content = Get-Content -Path $configPath -Raw
      $content | Should -Not -BeNullOrEmpty
      
      # Check that placeholders are replaced
      $content | Should -Not -Match ([regex]::Escape("{{"))
      
      # Check that the values were replaced correctly
      $content | Should -Match $pauseImage
      $content | Should -Match $cniBinDir
      $content | Should -Match $cniConfDir
      $content | Should -Match 'SandboxIsolation = 0'
    }

    It "Should include hyperv runtimes when hyperv is enabled" {
      $global:DefaultContainerdWindowsSandboxIsolation = "hyperv"
      $global:ContainerdWindowsRuntimeHandlers = "1234,5678"
      { ProcessAndWriteContainerdConfig  -ContainerDVersion "1.7.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw
      
      $content = Get-Content -Path $configPath -Raw
      $content | Should -Match 'plugins.cri.containerd.runtimes.runhcs-wcow-hypervisor-1234'
      $content | Should -Match 'SandboxIsolation = 1'
    }

    It "Should handle older containerd versions (<1.7.9) by removing annotations" {
      # Call the function under test and ensure it doesn't throw
      { ProcessAndWriteContainerdConfig -TemplatePath $templatePathV1 -ContainerDVersion "1.6.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw
      
      $content = Get-Content -Path $configPath -Raw
      
      # Should not contain annotation placeholders or values
      $content | Should -Not -Match 'container_annotations'
      $content | Should -Not -Match 'pod_annotations'
    }
  }
}