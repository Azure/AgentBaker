
BeforeAll {
  . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
  . $PSCommandPath.Replace('.tests.ps1', '.ps1')

}

Describe "Install-Oras" {
  BeforeEach {
    $global:OrasPath = "C:\aks-tools\oras\oras.exe"
    $global:OrasCacheDir = "C:\akse-cache\oras"
    $script:archiveExtractCalls = 0

    Mock New-Item -MockWith {}
    Mock Expand-Archive -MockWith { $script:archiveExtractCalls++ }
    Mock AKS-Expand-Archive -MockWith {
      param($Path, $DestinationPath, $Force)
      $script:archiveExtractCalls++
    }
    Mock tar -MockWith {}
    Mock Set-ExitCode -MockWith {
      Param(
        [Parameter(Mandatory = $true)][int]$ExitCode,
        [Parameter(Mandatory = $true)][string]$ErrorMessage
      )
      throw "Set-ExitCode:${ExitCode}:${ErrorMessage}"
    }
  }

  It "should return early when oras executable already exists" {
    Mock Test-Path -MockWith {
      Param($Path)
      return $Path -eq $global:OrasPath
    }

    { Install-Oras } | Should -Not -Throw
    Assert-MockCalled -CommandName 'New-Item' -Times 0
    Assert-MockCalled -CommandName 'Expand-Archive' -Times 0
    Assert-MockCalled -CommandName 'AKS-Expand-Archive' -Times 0
  }

  It "should extract cached zip archive and install oras" {
    $script:orasInstalled = $false

    Mock Test-Path -MockWith {
      Param($Path)
      switch ($Path) {
        { $_ -eq $global:OrasPath } { return $script:orasInstalled }
        { $_ -eq $global:OrasCacheDir } { return $true }
        { $_ -eq "C:\aks-tools\oras" } { return $false }
        default { return $true }
      }
    }

    Mock Get-ChildItem -MockWith {
      return [pscustomobject]@{ Name = "oras_1.3.0_windows_amd64.zip"; FullName = "C:\akse-cache\oras\oras_1.3.0_windows_amd64.zip" }
    }

    Mock Expand-Archive -MockWith {
      $script:archiveExtractCalls++
      $script:orasInstalled = $true
    }
    Mock AKS-Expand-Archive -MockWith {
      param($Path, $DestinationPath, $Force)
      $script:archiveExtractCalls++
      $script:orasInstalled = $true
    }

    { Install-Oras } | Should -Not -Throw
    $script:archiveExtractCalls | Should -Be 1
  }

  It "should fail when no cached oras archive exists" {
    Mock Test-Path -MockWith {
      Param($Path)
      return $Path -ne $global:OrasPath
    }

    Mock Get-ChildItem -MockWith { @() }

    {
      Install-Oras
    } | Should -Throw "*Set-ExitCode:$($global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND):No oras archive*"
  }

  It "should fail when tar extraction returns non-zero exit code" {
    Mock Test-Path -MockWith {
      Param($Path)
      switch ($Path) {
        { $_ -eq $global:OrasPath } { return $false }
        { $_ -eq $global:OrasCacheDir } { return $true }
        { $_ -eq "C:\aks-tools\oras" } { return $true }
        default { return $true }
      }
    }

    Mock Get-ChildItem -MockWith {
      return [pscustomobject]@{ Name = "oras_1.3.0_windows_amd64.tar.gz"; FullName = "C:\akse-cache\oras\oras_1.3.0_windows_amd64.tar.gz" }
    }

    Mock tar -MockWith { $global:LASTEXITCODE = 1 }

    {
      Install-Oras
    } | Should -Throw "*Set-ExitCode:$($global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND):Failed to extract oras archive*"
  }
}

Describe "Set-PodInfraContainerImage" {
  BeforeEach {
    $global:KubeClusterConfigPath = "C:\k\kubeclusterconfig.json"
    $global:BootstrapProfileContainerRegistryServer = "myacr.azurecr.io/aks-managed-repository"
    $global:OrasRegistryConfigFile = "C:\aks-tools\oras\config.json"
    $global:OrasPath = "Mock-OrasCli"
    $global:WINDOWS_CSE_ERROR_ORAS_PULL_POD_INFRA_CONTAINER = 82

    Mock Start-Sleep
    Mock New-Item
    Mock Remove-Item
    Mock tar -MockWith { $global:LASTEXITCODE = 0 }
    $script:CtrExeInvocations = @()
    $script:CtrExeMock = {
      param($Args)
      return "ok"
    }
    function global:ctr.exe {
      param([Parameter(ValueFromRemainingArguments = $true)]$Args)
      $script:CtrExeInvocations += , @($Args)
      $global:LASTEXITCODE = 0
      return & $script:CtrExeMock $Args
    }
    Mock Test-Path -MockWith { $false }
    Mock Set-ExitCode -MockWith {
      param(
        [Parameter(Mandatory = $true)][int]$ExitCode,
        [Parameter(Mandatory = $true)][string]$ErrorMessage
      )
      throw "Set-ExitCode:${ExitCode}:${ErrorMessage}"
    }

    Mock Get-Content -MockWith {
@'
{
  "Cri": {
    "Images": {
      "Pause": "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.1"
    }
  }
}
'@
    }
  }

  AfterEach {
    Remove-Item Function:\global:ctr.exe -ErrorAction SilentlyContinue
  }

  It "fails when pod infra image is empty" {
    Mock Get-Content -MockWith {
@'
{
  "Cri": {
    "Images": {
      "Pause": ""
    }
  }
}
'@
    }

    {
      Set-PodInfraContainerImage
    } | Should -Throw "*Set-ExitCode:82:Failed to recognize pod infra container image*"
  }

  It "returns early when image already exists locally" {
    $script:CtrExeMock = {
      param($Args)
      return @("mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.1")
    }

    function global:Mock-OrasCli {
      param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
      $global:LASTEXITCODE = 0
      return "ok"
    }

    { Set-PodInfraContainerImage } | Should -Not -Throw
    Assert-MockCalled -CommandName 'tar' -Times 0
    $script:CtrExeInvocations.Count | Should -Be 1
  }

  It "pulls via oras and imports image when not found locally" {
    $script:CtrExeMock = {
      param($Args)
      if ($Args -contains 'list') {
        return @()
      }
      return "ok"
    }

    function global:Mock-OrasCli {
      param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
      $global:LASTEXITCODE = 0
      return "oras ok"
    }

    { Set-PodInfraContainerImage } | Should -Not -Throw
    Assert-MockCalled -CommandName 'tar' -Times 1
    $script:CtrExeInvocations.Count | Should -Be 4
    Assert-MockCalled -CommandName 'Remove-Item' -Times 2
  }

  It "fails after oras retry exhaustion" {
    $script:CtrExeMock = {
      param($Args)
      return @()
    }

    function global:Mock-OrasCli {
      param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
      $global:LASTEXITCODE = 1
      return "oras failed"
    }

    {
      Set-PodInfraContainerImage
    } | Should -Throw "*Set-ExitCode:82:Failed to pull*"
    Assert-MockCalled -CommandName 'Start-Sleep' -Times 9
    $script:CtrExeInvocations.Count | Should -Be 1
  }
}
