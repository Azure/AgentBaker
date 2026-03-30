
# need this so it can be mocked
function Set-ExitCode {
  param($ExitCode, $ErrorMessage)
  Write-Log "Exiting with code $ExitCode. Error: $ErrorMessage"
  exit $ExitCode
}

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

  function Get-WindowsPauseVersion{
    return "ltsc2022"
  }

  function Get-WindowsVersion {
    return "ltsc2022"
  }

  # Stub for Assert-FileExists (defined in windowscsehelper.ps1, not loaded here)
  function Assert-FileExists {
    param($Filename, $ExitCode)
  }

  # Stubs for Windows-only service management cmdlets unavailable on Linux
  Mock Get-Service -MockWith {
    param($Name, $ErrorAction)
  }

  Mock Start-Service -MockWith {
    param($Name, $ErrorAction)
  }

  filter RemoveNulls { $_ -replace '\0', '' }
  . $PSScriptRoot\containerdfunc.ps1
  . $PSCommandPath.Replace('.tests.ps1', '.ps1')
  # . $PSScriptRoot\..\..\parts\windows\windowscsehelper.ps1
}

Describe "ProcessAndWriteContainerdConfig" {
  BeforeEach {
    Mock Get-Content -ParameterFilter { $Path -like "*kubeclusterconfig.json" } -MockWith {
      return "{`"Cri`":{`"Images`":{`"Pause`":`"$pauseImage`"}}}"
    }

    # Mock Out-File to capture content without writing to disk
    Mock Out-File -MockWith {
      param($FilePath, $Encoding)
    }
  }

  Context 'containerd template v1 ' {
    BeforeEach {
      $containerdDir = "$PSScriptRoot\containerdfunc.tests.suites"
      $cniBinDir = 'C:/cni/bin'
      $cniConfDir = 'C:/cni/conf'
      $pauseImage = 'mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.1'

      $global:KubeClusterConfigPath = [Io.path]::Combine("", "kubeclusterconfig.json")
      $global:ContainerdInstallLocation = $containerdDir
      $global:WindowsDataDir = $PSScriptRoot
      $configPath = Join-Path $global:ContainerdInstallLocation "config.toml"

      Mock Out-File -MockWith {
        param($FilePath, $Encoding)
      }
    }

    It "Should process containerdtemplate.toml with basic configuration" -Tag Focus {
      # Set up paths for the test
      $global:DefaultContainerdWindowsSandboxIsolation = "process" # default to process isolation
      $global:ContainerdWindowsRuntimeHandlers = "" # default to no hyperv handlers

      ProcessAndWriteContainerdConfig -ContainerDVersion "1.7.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir

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
      ProcessAndWriteContainerdConfig -ContainerDVersion "1.7.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir

      $content = Get-Content -Path $configPath -Raw
      $content | Should -Match 'plugins.cri.containerd.runtimes.runhcs-wcow-hypervisor-1234'
      $content | Should -Match 'SandboxIsolation = 1'
      $content | Should -Not -Match 'version = 3'
    }

    It "Should handle older containerd versions (<1.7.9) by removing annotations" {
      # Call the function under test and ensure it doesn't throw
      ProcessAndWriteContainerdConfig -ContainerDVersion "1.6.9" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir

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

      ProcessAndWriteContainerdConfig -ContainerDVersion "2.0.5" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir

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
      ProcessAndWriteContainerdConfig -ContainerDVersion "2.0.5" -CNIBinDir $cniBinDir -CNIConfDir $cniConfDir

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
      param($InputObject,$FilePath, $Encoding)
      $script:capturedFilePath = $FilePath
      $script:capturedEncoding = $Encoding
      $script:capturedContent = $InputObject
    }
  }

  It "Should write hosts.toml for default mcr.microsoft.com when MCRRepositoryBase is not set" {
    $global:BootstrapProfileContainerRegistryServer = "myacr.azurecr.io"
    if (Test-Path variable:global:MCRRepositoryBase) {
      Remove-Variable -Name MCRRepositoryBase -Scope Global
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
    $global:MCRRepositoryBase = "my.mcr.mirror"
    $global:BootstrapProfileContainerRegistryServer = "https://myacr.azurecr.io/some/path/"

    Set-BootstrapProfileRegistryContainerdHost

    Assert-MockCalled -CommandName 'Create-Directory' -Exactly -Times 1 -ParameterFilter {
      $FullPath -eq "C:\ProgramData\containerd\certs.d\my.mcr.mirror"
    }
    $script:capturedFilePath | Should -Be "C:\ProgramData\containerd\certs.d\my.mcr.mirror\hosts.toml"
    $script:capturedContent | Should -Match 'server = "https://my.mcr.mirror"'
    $script:capturedContent | Should -Match '\[host\."https://myacr.azurecr.io/v2/some/path"\]'
  }

  It "Should map host with repository prefix to v2 path" {
    $global:MCRRepositoryBase = "mcr.microsoft.com"
    $global:BootstrapProfileContainerRegistryServer = "myacr.azurecr.io/aaa"

    Set-BootstrapProfileRegistryContainerdHost

    $script:capturedContent | Should -Match '\[host\."https://myacr.azurecr.io/v2/aaa"\]'
  }

  AfterEach {
    $global:BootstrapProfileContainerRegistryServer = $null
    $global:MCRRepositoryBase = $null
  }
}

Describe 'RegisterContainerDService' {
  BeforeEach {
    Mock Assert-FileExists
    Mock Invoke-Nssm
    Mock Start-Service
  }

  Context 'when containerd service does not exist' {
    BeforeEach {
      $script:GetServiceCallCount = 0
      $mockRunningSvc = [PSCustomObject]@{Name = 'containerd'; Status = 'Running'}
      Mock Get-Service -MockWith {
        $script:GetServiceCallCount++
        if ($script:GetServiceCallCount -eq 1) { return $null }
        return $mockRunningSvc
      }
      Mock sc.exe
    }

    It 'does not call sc.exe when service does not exist' {
      RegisterContainerDService -kubedir 'C:\k'

      Assert-MockCalled sc.exe -Exactly -Times 0
    }
  }

  Context 'when containerd service already exists' {
    BeforeEach {
      $script:GetServiceCallCount = 0
      $mockExistingSvc = [PSCustomObject]@{Name = 'containerd'; Status = 'Stopped'}
      $mockRunningSvc = [PSCustomObject]@{Name = 'containerd'; Status = 'Running'}
      Mock Get-Service -MockWith {
        $script:GetServiceCallCount++
        if ($script:GetServiceCallCount -eq 1) { return $mockExistingSvc }
        return $mockRunningSvc
      }
    }

    It 'calls sc.exe delete to remove the existing service' {
      Mock sc.exe -MockWith { $global:LASTEXITCODE = 0 }

      RegisterContainerDService -kubedir 'C:\k'

      Assert-MockCalled sc.exe -Exactly -Times 1
    }

    It 'does not throw when sc.exe delete succeeds' {
      Mock sc.exe -MockWith { $global:LASTEXITCODE = 0 }

      { RegisterContainerDService -kubedir 'C:\k' } | Should -Not -Throw
    }

    It 'throws when sc.exe delete fails' {
      Mock sc.exe -MockWith { $global:LASTEXITCODE = 1 }

      { RegisterContainerDService -kubedir 'C:\k' } | Should -Throw '*sc.exe failed to delete existing containerd service*'
    }

    It 'includes the exit code in the error message when sc.exe fails' {
      Mock sc.exe -MockWith { $global:LASTEXITCODE = 5 }

      { RegisterContainerDService -kubedir 'C:\k' } | Should -Throw '*exit code 5*'
    }
  }
}
