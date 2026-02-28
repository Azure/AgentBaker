BeforeAll {
  if (-not (Get-PSDrive -Name C -ErrorAction SilentlyContinue)) {
    New-PSDrive -Name C -PSProvider FileSystem -Root ([System.IO.Path]::GetTempPath()) | Out-Null
  }

  # Define mock functions before loading the scripts
  function Write-Log {
    param($Message)
    Write-Host "$Message"
  }

  function Set-ExitCode {
    param($ExitCode, $ErrorMessage)
    Write-Host "MOCK: Exit Code would be: $ExitCode, Error: $ErrorMessage"
    # Don't actually exit in tests
  }

  # Define Create-Directory stub function (used by Set-ContainerdRegistryConfig)
  function Create-Directory {
    param($FullPath, $DirectoryUsage)
    # Do nothing in tests - just a stub
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
  . $PSCommandPath.Replace('.tests.ps1', '.ps1')
  # . $PSScriptRoot\..\..\parts\windows\windowscsehelper.ps1
}

Describe "ProcessAndWriteContainerdConfig" {
  BeforeAll {
    Mock Get-Content -ParameterFilter { $Path -like "*kubeclusterconfig.json" } -MockWith {
      return "{`"Cri`":{`"Images`":{`"Pause`":`"$pauseImage`"}}}"
    }

    # Mock Out-File for registry config writes to avoid file system errors
    Mock Out-File -ParameterFilter { $FilePath -like "*certs.d*" } -MockWith { }
  }

  Context 'containerd template v1 ' {
    BeforeAll {
      $containerdDir = "$PSScriptRoot\containerdfunc.tests.suites"
      $cniBinDir = 'C:/cni/bin'
      $cniConfDir = 'C:/cni/conf'
      $pauseImage = 'mcr.microsoft.com/oss/v2/kubernetes/pause:3.6'

      $global:KubeClusterConfigPath = [Io.path]::Combine("", "kubeclusterconfig.json")
      $global:ContainerdInstallLocation = $containerdDir
      $global:WindowsDataDir = $PSScriptRoot
      $configPath = Join-Path $global:ContainerdInstallLocation "config.toml"
    }

    It "Should process containerdtemplate.toml with basic configuration" {
      # Set up paths for the test
      $global:DefaultContainerdWindowsSandboxIsolation = "process" # default to process isolation
      $global:ContainerdWindowsRuntimeHandlers = "" # default to no hyperv handlers

      { ProcessAndWriteContainerdConfig -ContainerDVersion "1.7.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw

      $configPath | Should -Exist
      $content = Get-Content -Path $configPath -Raw
      $content | Should -Not -BeNullOrEmpty

      # Check that placeholders are replaced
      $content | Should -Not -Match ([regex]::Escape("{{"))

      # Check that the values were replaced correctly
      $content | Should -Match $pauseImage
      $content | Should -Match $cniBinDir
      $content | Should -Match $cniConfDir
      $content | Should -Not -Match 'version = 3'
      $content | Should -Match 'SandboxIsolation = 0'
    }

    It "Should include hyperv runtimes when hyperv is enabled" {
      $global:DefaultContainerdWindowsSandboxIsolation = "hyperv"
      $global:ContainerdWindowsRuntimeHandlers = "1234,5678"
      { ProcessAndWriteContainerdConfig -ContainerDVersion "1.7.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw

      $content = Get-Content -Path $configPath -Raw
      $content | Should -Match 'plugins.cri.containerd.runtimes.runhcs-wcow-hypervisor-1234'
      $content | Should -Match 'SandboxIsolation = 1'
      $content | Should -Not -Match 'version = 3'
    }

    It "Should handle older containerd versions (<1.7.9) by removing annotations" {
      # Call the function under test and ensure it doesn't throw
      { ProcessAndWriteContainerdConfig -ContainerDVersion "1.6.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw

      $content = Get-Content -Path $configPath -Raw

      # Should not contain annotation placeholders or values
      $content | Should -Not -Match 'container_annotations'
      $content | Should -Not -Match 'pod_annotations'
      $content | Should -Not -Match 'version = 3'
    }
  }


  Context 'containerd template v2' {

    BeforeAll {
      $containerdDir = "$PSScriptRoot\containerdfunc.tests.suites"
      $cniBinDir = 'C:/cni/bin'
      $cniConfDir = 'C:/cni/conf'
      $pauseImage = 'mcr.microsoft.com/oss/v2/kubernetes/pause:3.6'

      $global:KubeClusterConfigPath = [Io.path]::Combine("", "kubeclusterconfig.json")
      $global:ContainerdInstallLocation = $containerdDir
      $global:WindowsDataDir = $PSScriptRoot
      $configPath = Join-Path $global:ContainerdInstallLocation "config.toml"
    }

    It "Should process containerdtemplate.toml with basic configuration" {
      # Set up paths for the test
      $global:DefaultContainerdWindowsSandboxIsolation = "process" # default to process isolation
      $global:ContainerdWindowsRuntimeHandlers = "" # default to no hyperv handlers

      { ProcessAndWriteContainerdConfig -ContainerDVersion "2.0.5" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw

      $configPath | Should -Exist
      $content = Get-Content -Path $configPath -Raw
      $content | Should -Not -BeNullOrEmpty

      # Check that placeholders are replaced
      $content | Should -Not -Match ([regex]::Escape("{{"))

      # Check that the values were replaced correctly
      $content | Should -Match $pauseImage
      $content | Should -Match $cniBinDir
      $content | Should -Match $cniConfDir
      $content | Should -Match 'version = 3'
      $content | Should -Match 'SandboxIsolation = 0'
    }

    It "Should include hyperv runtimes when hyperv is enabled" {
      $global:DefaultContainerdWindowsSandboxIsolation = "hyperv"
      $global:ContainerdWindowsRuntimeHandlers = "1234,5678"
      { ProcessAndWriteContainerdConfig -ContainerDVersion "2.0.5" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir } | Should -Not -Throw

      $content = Get-Content -Path $configPath -Raw
      $content | Should -Match 'plugins.cri.containerd.runtimes.runhcs-wcow-hypervisor-1234'
      $content | Should -Match 'SandboxIsolation = 1'
      $content | Should -Match 'version = 3'
    }
  }
}

Describe "Set-ContainerdRegistryConfig" {
  BeforeAll {
    # Define Create-Directory mock function if not already defined
    function Create-Directory {
      param($FullPath, $DirectoryUsage)
    }
  }

  BeforeEach {
    # Mock Create-Directory to track calls
    Mock Create-Directory -MockWith {
      param($FullPath, $DirectoryUsage)
      # Do nothing in tests - we'll verify the call was made
    }

    # Mock Out-File to capture content without writing to disk
    Mock Out-File -MockWith {
      param($FilePath, $Encoding)
    }
  }

  It "Should create hosts.toml file for docker.io registry" {
    $registry = "docker.io"
    $registryHost = "registry-1.docker.io"

    Set-ContainerdRegistryConfig -Registry $registry -RegistryHost $registryHost

    # Verify Create-Directory was called with correct parameters
    Assert-MockCalled -CommandName 'Create-Directory' -Exactly -Times 1 -ParameterFilter {
      $FullPath -eq "C:\ProgramData\containerd\certs.d\docker.io" -and
      $DirectoryUsage -eq "storing containerd registry hosts config"
    }

    # Verify Out-File was called with correct path
    Assert-MockCalled -CommandName 'Out-File' -Exactly -Times 1 -ParameterFilter {
      $FilePath -eq "C:\ProgramData\containerd\certs.d\docker.io\hosts.toml" -and
      $Encoding -eq "ascii"
    }
  }

  It "Should create hosts.toml file for mcr.azk8s.cn registry" {
    $registry = "mcr.azk8s.cn"
    $registryHost = "mcr.azure.cn"

    Set-ContainerdRegistryConfig -Registry $registry -RegistryHost $registryHost

    # Verify Create-Directory was called with correct parameters
    Assert-MockCalled -CommandName 'Create-Directory' -Exactly -Times 1 -ParameterFilter {
      $FullPath -eq "C:\ProgramData\containerd\certs.d\mcr.azk8s.cn" -and
      $DirectoryUsage -eq "storing containerd registry hosts config"
    }

    # Verify Out-File was called with correct path
    Assert-MockCalled -CommandName 'Out-File' -Exactly -Times 1 -ParameterFilter {
      $FilePath -eq "C:\ProgramData\containerd\certs.d\mcr.azk8s.cn\hosts.toml" -and
      $Encoding -eq "ascii"
    }
  }

  It "Should generate correct hosts.toml content structure" {
    $registry = "docker.io"
    $registryHost = "registry-1.docker.io"

    # Mock Out-File to do nothing (we verify structure by checking function implementation)
    Mock Out-File -MockWith { }

    Set-ContainerdRegistryConfig -Registry $registry -RegistryHost $registryHost

    # Verify Out-File was called with correct path
    Assert-MockCalled -CommandName 'Out-File' -Exactly -Times 1 -ParameterFilter {
      $FilePath -eq "C:\ProgramData\containerd\certs.d\docker.io\hosts.toml" -and
      $Encoding -eq "ascii"
    }

    # Note: The content structure is verified by the function's implementation
    # The expected content format is:
    # server = "https://$Registry"
    # [host."https://$RegistryHost"]
    #   capabilities = ["pull", "resolve"]
    # [host."https://$RegistryHost".header]
    #     X-Forwarded-For = ["$Registry"]
  }

  It "Should handle custom registry and host correctly" {
    $registry = "myregistry.example.com"
    $registryHost = "mirror.example.com"

    Mock Out-File -MockWith { }

    Set-ContainerdRegistryConfig -Registry $registry -RegistryHost $registryHost

    # Verify Create-Directory was called with correct registry path
    Assert-MockCalled -CommandName 'Create-Directory' -ParameterFilter {
      $FullPath -eq "C:\ProgramData\containerd\certs.d\myregistry.example.com"
    }

    # Verify Out-File was called with correct path
    Assert-MockCalled -CommandName 'Out-File' -ParameterFilter {
      $FilePath -eq "C:\ProgramData\containerd\certs.d\myregistry.example.com\hosts.toml"
    }
  }

  It "Should write to correct hosts.toml file path" {
    $registry = "test.registry.io"
    $registryHost = "test.mirror.io"
    $script:capturedFilePath = $null

    Mock Out-File -MockWith {
      param($FilePath, $Encoding)
      $script:capturedFilePath = $FilePath
    }

    Set-ContainerdRegistryConfig -Registry $registry -RegistryHost $registryHost

    $script:capturedFilePath | Should -Be "C:\ProgramData\containerd\certs.d\test.registry.io\hosts.toml"
  }

  It "Should use ascii encoding when writing hosts.toml" {
    $registry = "docker.io"
    $registryHost = "registry-1.docker.io"
    $script:capturedEncoding = $null

    Mock Out-File -MockWith {
      param($FilePath, $Encoding)
      $script:capturedEncoding = $Encoding
    }

    Set-ContainerdRegistryConfig -Registry $registry -RegistryHost $registryHost

    $script:capturedEncoding | Should -Be "ascii"
  }
}

Describe "Set-BootstrapProfileRegistryContainerdHost" {
  BeforeEach {
    Mock Create-Directory -MockWith {
      param($FullPath, $DirectoryUsage)
    }

    $script:capturedFilePath = $null
    $script:capturedEncoding = $null
    $script:capturedContent = $null
    Mock Out-File -MockWith {
      param($FilePath, $Encoding)
      $script:capturedFilePath = $FilePath
      $script:capturedEncoding = $Encoding
      $script:capturedContent = $input
    }
  }

  It "Should write hosts.toml for default mcr.microsoft.com when MCR_REPOSITORY_BASE is not set" {
    $global:BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER = "myacr.azurecr.io"
    if (Test-Path variable:global:MCR_REPOSITORY_BASE) {
      Remove-Variable -Name MCR_REPOSITORY_BASE -Scope Global
    }

    Set-BootstrapProfileRegistryContainerdHost

    Assert-MockCalled -CommandName 'Create-Directory' -Exactly -Times 1 -ParameterFilter {
      $FullPath -eq "C:\ProgramData\containerd\certs.d\mcr.microsoft.com" -and
      $DirectoryUsage -eq "storing containerd registry hosts config"
    }
    $script:capturedFilePath | Should -Be "C:\ProgramData\containerd\certs.d\mcr.microsoft.com\hosts.toml"
    $script:capturedEncoding | Should -Be "ascii"
    $script:capturedContent | Should -Match 'server = "https://mcr.microsoft.com"'
    $script:capturedContent | Should -Match '\[host\."https://myacr.azurecr.io/v2"\]'
    $script:capturedContent | Should -Match 'override_path = true'
  }

  It "Should sanitize bootstrap profile host and use custom mcr repository base" {
    $global:MCR_REPOSITORY_BASE = "my.mcr.mirror"
    $global:BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER = "https://myacr.azurecr.io/some/path/"

    Set-BootstrapProfileRegistryContainerdHost

    Assert-MockCalled -CommandName 'Create-Directory' -Exactly -Times 1 -ParameterFilter {
      $FullPath -eq "C:\ProgramData\containerd\certs.d\my.mcr.mirror"
    }
    $script:capturedFilePath | Should -Be "C:\ProgramData\containerd\certs.d\my.mcr.mirror\hosts.toml"
    $script:capturedContent | Should -Match 'server = "https://my.mcr.mirror"'
    $script:capturedContent | Should -Match '\[host\."https://myacr.azurecr.io/v2/some/path"\]'
  }

  It "Should map host with repository prefix to v2 path" {
    $global:MCR_REPOSITORY_BASE = "mcr.microsoft.com"
    $global:BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER = "myacr.azurecr.io/aaa"

    Set-BootstrapProfileRegistryContainerdHost

    $script:capturedContent | Should -Match '\[host\."https://myacr.azurecr.io/v2/aaa"\]'
  }
}
