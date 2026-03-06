
BeforeAll {
  . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
  . $PSCommandPath.Replace('.tests.ps1', '.ps1')

}

Describe "Ensure-Oras" {
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

    { Ensure-Oras } | Should -Not -Throw
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

    { Ensure-Oras } | Should -Not -Throw
    $script:archiveExtractCalls | Should -Be 1
  }

  It "should fail when no cached oras archive exists" {
    Mock Test-Path -MockWith {
      Param($Path)
      return $Path -ne $global:OrasPath
    }

    Mock Get-ChildItem -MockWith { @() }

    {
      Ensure-Oras
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
      Ensure-Oras
    } | Should -Throw "*Set-ExitCode:$($global:WINDOWS_CSE_ERROR_ORAS_NOT_FOUND):Failed to extract oras archive*"
  }
}
