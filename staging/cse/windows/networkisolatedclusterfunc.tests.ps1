
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

Describe "Invoke-OrasLogin" {
  BeforeEach {
    $global:OrasRegistryConfigFile = "C:\aks-tools\oras\config.json"
    $global:LASTEXITCODE = 0

    Mock Write-Log
    Mock Start-Sleep
    Mock Set-ExitCode -MockWith {
      param(
        [Parameter(Mandatory = $true)][int]$ExitCode,
        [Parameter(Mandatory = $true)][string]$ErrorMessage
      )
      throw "Set-ExitCode:${ExitCode}:${ErrorMessage}"
    }
  }

  It "should return unauthorized error code when ClientID is missing" {
    $ret = Invoke-OrasLogin -Acr_Url "contoso.azurecr.io" -ClientID "" -TenantID "tenant-id"
    $ret | Should -Be $global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED
  }

  It "should return early when anonymous pull is allowed" {
    Mock Assert-AnonymousAcrAccess -MockWith { 0 }
    Mock Retry-Command

    { Invoke-OrasLogin -Acr_Url "contoso.azurecr.io" -ClientID "client-id" -TenantID "tenant-id" } | Should -Not -Throw
    Assert-MockCalled -CommandName 'Retry-Command' -Times 0
  }

  It "should set network timeout exit code when anonymous check returns unexpected error" {
    Mock Assert-AnonymousAcrAccess -MockWith { 2 }

    {
      Invoke-OrasLogin -Acr_Url "contoso.azurecr.io" -ClientID "client-id" -TenantID "tenant-id"
    } | Should -Throw "*Set-ExitCode:$($global:WINDOWS_CSE_ERROR_ORAS_PULL_NETWORK_TIMEOUT):failed with an error other than unauthorized*"
  }

  It "should complete login flow when token exchange and oras login succeed" {
    Mock Assert-AnonymousAcrAccess -MockWith { 1 }
    Mock Retry-Command -MockWith {
      param($Command, $Args)
      if ($Args.Uri -like "*metadata/identity/oauth2/token*") {
        return @{ access_token = "imds-token" }
      }
      return @{ refresh_token = "refresh-token" }
    }
    Mock Assert-RefreshToken -MockWith { 0 }

    function global:Mock-OrasCli {
      param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
      $global:LASTEXITCODE = 0
      return "login ok"
    }
    $global:OrasPath = "Mock-OrasCli"

    { Invoke-OrasLogin -Acr_Url "contoso.azurecr.io" -ClientID "client-id" -TenantID "tenant-id" } | Should -Not -Throw
    Assert-MockCalled -CommandName 'Retry-Command' -Times 2
    Assert-MockCalled -CommandName 'Assert-RefreshToken' -Times 1 -ParameterFilter { $RefreshToken -eq 'refresh-token' }
  }

  It "should fail after three unsuccessful oras login attempts" {
    Mock Assert-AnonymousAcrAccess -MockWith { 1 }
    Mock Retry-Command -MockWith {
      param($Command, $Args)
      if ($Args.Uri -like "*metadata/identity/oauth2/token*") {
        return @{ access_token = "imds-token" }
      }
      return @{ refresh_token = "refresh-token" }
    }
    Mock Assert-RefreshToken -MockWith { 0 }

    function global:Mock-OrasCli {
      param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
      $global:LASTEXITCODE = 1
      return "login failed"
    }
    $global:OrasPath = "Mock-OrasCli"

    {
      Invoke-OrasLogin -Acr_Url "contoso.azurecr.io" -ClientID "client-id" -TenantID "tenant-id"
    } | Should -Throw "*Set-ExitCode:$($global:WINDOWS_CSE_ERROR_ORAS_PULLUNAUTHORIZED):failed to login to acr*"
    Assert-MockCalled -CommandName 'Start-Sleep' -Times 2
  }
}
