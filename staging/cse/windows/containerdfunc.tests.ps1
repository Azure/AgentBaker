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

  # Now load the scripts
  . $PSScriptRoot\containerdfunc.ps1
  . $PSCommandPath.Replace('.tests.ps1','.ps1')
  # . $PSScriptRoot\..\..\parts\windows\windowscsehelper.ps1
}

Describe "ProcessAndWriteContainerdConfig" {
  Context 'v1 containerdtemplate.toml' {
    BeforeEach {
      # Set up global variables required by the function
      $containerdDir = "$PSScriptRoot\containerdfunc.tests.suites"
      $global:ContainerdInstallLocation = $containerdDir
      $global:KubeClusterConfigPath = [Io.path]::Combine("", "kubeclusterconfig.json")
      $global:DefaultContainerdWindowsSandboxIsolation = "process" # default to process isolation
      $global:ContainerdWindowsRuntimeHandlers = "" # default to no hyperv handlers
      
      # Mock Get-Content -ParameterFilter { $Path -like "*containerdtemplate.toml*" }
      # Mock Get-Content -ParameterFilter { $Path -like "*config.toml" } 
      Mock Get-Content -ParameterFilter { $Path -like "*kubeclusterconfig.json" } -MockWith { 
        return '{"Cri":{"Images":{"Pause":"mcr.microsoft.com/oss/kubernetes/pause"}}}'   
      }
    }

    It "Should process containerdtemplate.toml with basic configuration" {
      # Set up paths for the test
      $templatePath = Join-Path $PSScriptRoot "containerdtemplate.toml"
      $cniBinDir = "C:\cni\bin"
      $cniConfDir = "C:\cni\conf"
      $configPath = Join-Path $global:ContainerdInstallLocation "config.toml"
      
      # Call the function under test
      { ProcessAndWriteContainerdConfig -TemplatePath $templatePath -ContainerDVersion "1.7.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw
      
      # Check that config.toml was created
      $configPath | Should -Exist
      
      # Verify content
      $content = Get-Content -Path $configPath -Raw
      $content | Should -Not -BeNullOrEmpty
      
      # Check that placeholders are replaced
      $content | Should -Not -Match '{{pauseImage}}'
      $content | Should -Not -Match '{{currentversion}}'
      $content | Should -Not -Match '{{sandboxIsolation}}'
      $content | Should -Not -Match '{{cnibin}}'
      $content | Should -Not -Match '{{cniconf}}'
      
      # Check that the values were replaced correctly
      $content | Should -Match 'mcr.microsoft.com/oss/kubernetes/pause'
      $content | Should -Match 'C:/cni/bin'
      $content | Should -Match 'C:/cni/conf'
      $content | Should -Match 'SandboxIsolation = 0'
    }

    # It "Should include hyperv runtimes when ContainerdWindowsRuntimeHandlers is set" {
    #   # Set up paths for the test
    #   $templatePath = Join-Path $TestDrive "containerdtemplate.toml"
    #   $cniBinDir = "C:\cni\bin"
    #   $cniConfDir = "C:\cni\conf"
      
    #   # Set global variables to enable hyperv
    #   $global:ContainerdWindowsRuntimeHandlers = "1234,5678"
      
    #   # Mock the calls that try to check containerd version
    #   Mock Start-Process -MockWith { 
    #     return @{
    #       ExitCode = 0
    #     }
    #   }
      
    #   # Create a stub for the containerd.exe version check
    #   $containerdExePath = Join-Path $global:ContainerdInstallLocation "containerd.exe"
    #   New-Item -ItemType File -Force -Path $containerdExePath
      
    #   # Call the function under test
    #   { ProcessAndWriteContainerdConfig -TemplatePath $templatePath -ContainerDVersion "1.7.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw
      
    #   # Check config content
    #   $configPath = Join-Path $global:ContainerdInstallLocation "config.toml"
    #   $content = Get-Content -Path $configPath -Raw
      
    #   # Should include our mock hyperv runtime content
    #   $content | Should -Match 'mock-hyperv-runtime-content'
    # }

    # It "Should set SandboxIsolation to 1 when DefaultContainerdWindowsSandboxIsolation is hyperv" {
    #   # Set up paths for the test
    #   $templatePath = Join-Path $TestDrive "containerdtemplate.toml"
    #   $cniBinDir = "C:\cni\bin"
    #   $cniConfDir = "C:\cni\conf"
      
    #   # Set global variables to enable hyperv isolation
    #   $global:DefaultContainerdWindowsSandboxIsolation = "hyperv"
      

    #   # Create a stub for the containerd.exe version check
    #   $containerdExePath = Join-Path $global:ContainerdInstallLocation "containerd.exe"
    #   New-Item -ItemType File -Force -Path $containerdExePath
      
    #   # Call the function under test
    #   { ProcessAndWriteContainerdConfig -TemplatePath $templatePath -ContainerDVersion "1.7.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw
      
    #   # Check config content
    #   $configPath = Join-Path $global:ContainerdInstallLocation "config.toml"
    #   $content = Get-Content -Path $configPath -Raw
      
    #   # Should set SandboxIsolation to 1
    #   $content | Should -Match 'SandboxIsolation = 1'
    # }

    # It "Should handle older containerd versions (<1.7.9) by removing annotations" {
    #   # Set up paths for the test
    #   $templatePath = Join-Path $TestDrive "containerdtemplate.toml"
    #   $cniBinDir = "C:\cni\bin"
    #   $cniConfDir = "C:\cni\conf"
      
    #   # Set global mock to return an older containerd version
    #   $Global:MockedScriptOutput = "containerd github.com/containerd/containerd v1.6.21+azure 3dce8eb055cbb6872793272b4f20ed16117344f8"
      
    #   # Create a stub for the containerd.exe version check
    #   $containerdExePath = Join-Path $global:ContainerdInstallLocation "containerd.exe"
    #   New-Item -ItemType File -Force -Path $containerdExePath
      
    #   # Call the function under test and ensure it doesn't throw
    #   { ProcessAndWriteContainerdConfig -TemplatePath $templatePath -ContainerDVersion "1.7.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw
      
    #   # Check config content
    #   $configPath = Join-Path $global:ContainerdInstallLocation "config.toml"
    #   $content = Get-Content -Path $configPath -Raw
      
    #   # Should not contain annotation placeholders or values
    #   $content | Should -Not -Match 'container_annotations'
    #   $content | Should -Not -Match 'pod_annotations'
    # }
  }
}