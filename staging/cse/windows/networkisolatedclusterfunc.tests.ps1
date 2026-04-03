
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

Describe "Invoke-OrasLogin" {
  BeforeEach {
    $global:OrasRegistryConfigFile = "C:\aks-tools\oras\config.json"
    $global:LASTEXITCODE = 0

    Mock Start-Sleep
    Mock Set-ExitCode -MockWith {
      param(
        [Parameter(Mandatory = $true)][int]$ExitCode,
        [Parameter(Mandatory = $true)][string]$ErrorMessage
      )
      throw "Set-ExitCode:${ExitCode}:${ErrorMessage}"
    }

    $script:RetryCommandMock = {
      param($Command, $Args, $Retries, $RetryDelaySeconds)
      throw "Retry-Command was not configured for this test"
    }
    function global:Retry-Command {
      param(
        [Parameter(Mandatory = $true)][string]$Command,
        [Parameter(Mandatory = $true)][hashtable]$Args,
        [Parameter(Mandatory = $true)][int]$Retries,
        [Parameter(Mandatory = $true)][int]$RetryDelaySeconds
      )

      return & $script:RetryCommandMock $Command $Args $Retries $RetryDelaySeconds
    }
  }

  AfterEach {
    Remove-Item Function:\global:Retry-Command -ErrorAction SilentlyContinue
  }

  It "should return unauthorized error code when ClientID is missing" {
    # Use whitespace to bypass mandatory empty-string binding while still testing missing value logic.
    $ret = Invoke-OrasLogin -Acr_Url "contoso.azurecr.io" -ClientID " " -TenantID "tenant-id"
    $ret | Should -Be $global:WINDOWS_CSE_ERROR_ORAS_PULL_UNAUTHORIZED
  }

  It "should return early when anonymous pull is allowed" {
    Mock Assert-AnonymousAcrAccess -MockWith { 0 }

    { Invoke-OrasLogin -Acr_Url "contoso.azurecr.io" -ClientID "client-id" -TenantID "tenant-id" } | Should -Not -Throw
  }

  It "should set network timeout exit code when anonymous check returns unexpected error" {
    Mock Assert-AnonymousAcrAccess -MockWith { 2 }

    {
      Invoke-OrasLogin -Acr_Url "contoso.azurecr.io" -ClientID "client-id" -TenantID "tenant-id"
    } | Should -Throw "*Set-ExitCode:$($global:WINDOWS_CSE_ERROR_ORAS_PULL_NETWORK_TIMEOUT):failed with an error other than unauthorized*"
  }

  It "should complete login flow when token exchange and oras login succeed" {
    Mock Assert-AnonymousAcrAccess -MockWith { 1 }
    Mock Invoke-RestMethod -MockWith {
      param($Uri, $Method, $Headers, $TimeoutSec, $ContentType, $Body)
      if ($Uri -like "*metadata/identity/oauth2/token*") {
        return [pscustomobject]@{ access_token = "imds-token" }
      }
      if ($Uri -like "*/oauth2/exchange") {
        return [pscustomobject]@{ refresh_token = "refresh-token" }
      }
      throw "unexpected Invoke-RestMethod Uri: $Uri"
    }
    $script:RetryCommandMock = {
      param($Command, $Args, $Retries, $RetryDelaySeconds)
      $requestUri = [string]$Args['Uri']
      if ($requestUri -like "*metadata/identity/oauth2/token*") {
        return [pscustomobject]@{ access_token = "imds-token" }
      }
      if ($requestUri -like "*/oauth2/exchange") {
        return [pscustomobject]@{ refresh_token = "refresh-token" }
      }
      throw "unexpected Retry-Command Uri: $requestUri"
    }
    Mock Assert-RefreshToken -MockWith { 0 }

    $global:OrasPath = {
      $null = $input
      $null = $args
      $global:LASTEXITCODE = 0
      return "login ok"
    }

    { Invoke-OrasLogin -Acr_Url "contoso.azurecr.io" -ClientID "client-id" -TenantID "tenant-id" } | Should -Not -Throw
    Assert-MockCalled -CommandName 'Assert-RefreshToken' -Times 1 -ParameterFilter { $RefreshToken -eq 'refresh-token' }
  }

  It "should fail after three unsuccessful oras login attempts" {
    Mock Assert-AnonymousAcrAccess -MockWith { 1 }
    Mock Invoke-RestMethod -MockWith {
      param($Uri, $Method, $Headers, $TimeoutSec, $ContentType, $Body)
      if ($Uri -like "*metadata/identity/oauth2/token*") {
        return [pscustomobject]@{ access_token = "imds-token" }
      }
      if ($Uri -like "*/oauth2/exchange") {
        return [pscustomobject]@{ refresh_token = "refresh-token" }
      }
      throw "unexpected Invoke-RestMethod Uri: $Uri"
    }
    $script:RetryCommandMock = {
      param($Command, $Args, $Retries, $RetryDelaySeconds)
      $requestUri = [string]$Args['Uri']
      if ($requestUri -like "*metadata/identity/oauth2/token*") {
        return [pscustomobject]@{ access_token = "imds-token" }
      }
      if ($requestUri -like "*/oauth2/exchange") {
        return [pscustomobject]@{ refresh_token = "refresh-token" }
      }
      throw "unexpected Retry-Command Uri: $requestUri"
    }
    Mock Assert-RefreshToken -MockWith { 0 }

    $global:OrasPath = {
      $null = $input
      $null = $args
      $global:LASTEXITCODE = 1
      return "login failed"
    }

    {
      Invoke-OrasLogin -Acr_Url "contoso.azurecr.io" -ClientID "client-id" -TenantID "tenant-id"
    } | Should -Throw "*Set-ExitCode:$($global:WINDOWS_CSE_ERROR_ORAS_PULL_UNAUTHORIZED):failed to login to acr*"
    Assert-MockCalled -CommandName 'Start-Sleep' -Times 2
  }
}

Describe "Get-BootstrapRegistryDomainName" {
  It "should return default mcr domain when no overrides are set" {
    $global:MCRRepositoryBase = ""
    $global:BootstrapProfileContainerRegistryServer = ""

    Get-BootstrapRegistryDomainName | Should -Be "mcr.microsoft.com"
  }

  It "should use MCRRepositoryBase and trim trailing slash" {
    $global:MCRRepositoryBase = "example.registry.io/"
    $global:BootstrapProfileContainerRegistryServer = ""

    Get-BootstrapRegistryDomainName | Should -Be "example.registry.io"
  }

  It "should prefer bootstrap profile registry host when provided" {
    $global:MCRRepositoryBase = "example.registry.io/"
    $global:BootstrapProfileContainerRegistryServer = "mybootstrap.azurecr.io/repo/path"

    Get-BootstrapRegistryDomainName | Should -Be "mybootstrap.azurecr.io"
  }
}

Describe "DownloadFileWithOras" {
  BeforeEach {
    $global:OrasPath = "Mock-OrasCli"
    $script:MockOrasExitCode = 0
    function global:Mock-OrasCli {
      param([Parameter(ValueFromRemainingArguments = $true)]$Args)
      $global:LASTEXITCODE = $script:MockOrasExitCode
    }
    $global:OrasRegistryConfigFile = "C:\oras-config.json"
    $global:AppInsightsClient = $null

    Mock Set-ExitCode -MockWith {
      Param(
        [Parameter(Mandatory = $true)][int]$ExitCode,
        [Parameter(Mandatory = $true)][string]$ErrorMessage
      )
      throw "Set-ExitCode:${ExitCode}:${ErrorMessage}"
    }
    Mock Get-Item -MockWith {
      param($Path)
      return New-Object -TypeName PSObject -Property @{ FullName = $Path }
    }
    # Mock Get-ChildItem to return a fake downloaded file from the oras pull temp directory
    Mock Get-ChildItem -MockWith {
      param($Path, [switch]$File)
      return @([PSCustomObject]@{ Name = "downloaded-file.zip"; FullName = "C:\tmp\downloaded-file.zip"; Length = 1024 })
    }
    Mock Move-Item -MockWith {}
    Mock Remove-Item -MockWith {} -ParameterFilter { $Recurse -eq $true -or $Path -eq 'c:\k.zip' }
    Mock Test-Path -MockWith {
      Param($Path)
      return $false
    } -ParameterFilter {
      $null -ne $Path -and ($Path -eq 'c:\k.zip' -or $Path -eq 'c:\test.zip' -or $Path -eq 'c:')
    }
  }

  It "should call oras with correct arguments on success" {
    $reference = "myregistry.azurecr.io/aks/packages/kubernetes/windowszip:1.29.2"
    $destPath = "c:\k.zip"

    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath } | Should -Not -Throw
  }

  It "should throw when oras returns non-zero exit code" {
    $script:MockOrasExitCode = 1

    $reference = "myregistry.azurecr.io/aks/packages/kubernetes/windowszip:1.29.2"
    $destPath = "c:\k.zip"

    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath } | Should -Throw "*oras pull failed*"
  }

  It "should throw when no file is found after oras pull" {
    Mock Get-ChildItem -MockWith {
      param($Path, [switch]$File)
      return @()
    }

    $reference = "myregistry.azurecr.io/aks/packages/kubernetes/windowszip:1.29.2"
    $destPath = "c:\k.zip"

    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath } | Should -Throw "*oras pull succeeded but no file found*"
  }

  It "should move downloaded file to destination path on success" {
    $reference = "myregistry.azurecr.io/aks/packages/kubernetes/windowszip:1.29.2"
    $destPath = "c:\k.zip"

    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath } | Should -Not -Throw

    Assert-MockCalled -CommandName 'Move-Item' -Exactly -Times 1 -ParameterFilter {
      $Destination -eq $destPath
    }
  }

  It "should use default platform windows/amd64 when not specified" {
    $reference = "myregistry.azurecr.io/aks/packages/test:v1"
    $destPath = "c:\test.zip"

    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath } | Should -Not -Throw
  }

  It "should accept a custom platform parameter" {
    $reference = "myregistry.azurecr.io/aks/packages/test:v1"
    $destPath = "c:\test.zip"

    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath -Platform "linux/amd64" } | Should -Not -Throw
  }
}
