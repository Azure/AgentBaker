
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

Describe "DownloadFileWithOras" {
  BeforeEach {
    $global:OrasPath = "C:\aks-tools\oras\oras.exe"
    $global:OrasRegistryConfigFile = "C:\oras-config.json"
    $global:AppInsightsClient = $null

    Mock Set-ExitCode -MockWith { throw "Set-ExitCode:$($ExitCode):$ErrorMessage" }
    Mock Get-Item -MockWith { return New-Object -TypeName PSObject -Property @{ FullName = $DestinationPath } }
    # Mocks for temp directory and file move operations used by oras pull workflow
    Mock New-Item -MockWith {} -ParameterFilter { $ItemType -eq 'Directory' }
    Mock Get-ChildItem -MockWith {
      return @([PSCustomObject]@{ FullName = "/tmp/downloaded-file.zip" })
    }
    Mock Move-Item -MockWith {}
    Mock Remove-Item -MockWith {}
    Mock Test-Path -MockWith { $false }
  }

  It "should call oras with correct arguments on success" {
    $reference = "myregistry.azurecr.io/aks/packages/kubernetes/windowszip:1.29.2"
    $destPath = "c:\k.zip"

    $global:OrasPath = "Write-Output"
    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath -ExitCode 80 } | Should -Not -Throw
  }

  It "should call Set-ExitCode when oras returns non-zero exit code" {
    $global:OrasPath = "cmd.exe"
    Mock cmd.exe -MockWith { $global:LASTEXITCODE = 1 }

    $reference = "myregistry.azurecr.io/aks/packages/kubernetes/windowszip:1.29.2"
    $destPath = "c:\k.zip"

    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath -ExitCode 80 } | Should -Throw "*Set-ExitCode:80:oras pull failed*"
  }

  It "should call Set-ExitCode when no file is found after oras pull" {
    $global:OrasPath = "Write-Output"
    Mock Get-ChildItem -MockWith { return @() }

    $reference = "myregistry.azurecr.io/aks/packages/kubernetes/windowszip:1.29.2"
    $destPath = "c:\k.zip"

    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath -ExitCode 80 } | Should -Throw "*Set-ExitCode:80:oras pull succeeded but no file found*"
  }

  It "should move downloaded file to destination path on success" {
    $global:OrasPath = "Write-Output"
    $reference = "myregistry.azurecr.io/aks/packages/kubernetes/windowszip:1.29.2"
    $destPath = "c:\k.zip"

    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath -ExitCode 80 } | Should -Not -Throw

    Assert-MockCalled -CommandName 'Move-Item' -Exactly -Times 1 -ParameterFilter {
      $Destination -eq $destPath
    }
  }

  It "should use default platform windows/amd64 when not specified" {
    $global:OrasPath = "Write-Output"
    $reference = "myregistry.azurecr.io/aks/packages/test:v1"
    $destPath = "c:\test.zip"

    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath -ExitCode 80 } | Should -Not -Throw

    Assert-MockCalled -CommandName 'Write-Log' -ParameterFilter {
      $message -like "*platform=windows/amd64*"
    }
  }

  It "should accept a custom platform parameter" {
    $global:OrasPath = "Write-Output"
    $reference = "myregistry.azurecr.io/aks/packages/test:v1"
    $destPath = "c:\test.zip"

    { DownloadFileWithOras -Reference $reference -DestinationPath $destPath -ExitCode 80 -Platform "linux/amd64" } | Should -Not -Throw

    Assert-MockCalled -CommandName 'Write-Log' -ParameterFilter {
      $message -like "*platform=linux/amd64*"
    }
  }
}
