BeforeAll {
  # Mock Set-Content to avoid permission denied errors
  Mock Set-Content -MockWith {
    param($Path, $Value)
    Write-Host "SET-CONTENT: Path: $Path, Content: $Value"
  }

  . $PSScriptRoot\windowscsehelper.ps1
  . $PSScriptRoot\..\..\staging\cse\windows\containerdfunc.ps1
  . $PSCommandPath.Replace('.tests.ps1','.ps1')

  $capturedContent = $null
  Mock Set-Content -MockWith {
      param($Path, $Value)
      $script:capturedContent = $Value
  } -Verifiable
}

Describe 'Install-Containerd-Based-On-Kubernetes-Version' {
  BeforeAll{
      Mock Install-Containerd -MockWith {
        Param(
          [Parameter(Mandatory = $true)][string]
          $ContainerdUrl,
          [Parameter(Mandatory = $true)][string]
          $CNIBinDir,
          [Parameter(Mandatory = $true)][string]
          $CNIConfDir,
          [Parameter(Mandatory = $true)][string]
          $KubeDir,
          [Parameter(Mandatory = $false)][string]
          $KubernetesVersion,
          [Parameter(Mandatory = $false)][string]
          $WindowsVersion
        )
        Write-Host $ContainerdUrl
    } -Verifiable

    $ContainerdWindowsPackageDownloadURL = "https://packages.aks.azure.com/containerd/windows/"
    $StableContainerdPackage = [string]::Format($global:ContainerdPackageTemplate, $global:StableContainerdVersion)
    $LatestContainerdPackage = [string]::Format($global:ContainerdPackageTemplate, $global:LatestContainerdVersion)
    $LatestContainerd2Package = [string]::Format($global:ContainerdPackageTemplate, $global:LatestContainerd2Version)

    $ContainerdWindowsPackageDownloadURL = "https://packages.aks.azure.com/containerd/windows/"
    $StableContainerdPackage = [string]::Format($global:ContainerdPackageTemplate, $global:StableContainerdVersion)
    $LatestContainerdPackage = [string]::Format($global:ContainerdPackageTemplate, $global:LatestContainerdVersion)
    $LatestContainerd2Package = [string]::Format($global:ContainerdPackageTemplate, $global:LatestContainerd2Version)
  }

  Context 'Windows Server 2022 (ltsc2022)' {
    # for windows versions other than test2025, containerd version is not changed and should not include containerd2
    BeforeAll {
      Mock Get-WindowsVersion -MockWith { return "ltsc2022" }
    }

    It 'k8s version is less than MinimalKubernetesVersionWithLatestContainerd' {
      $expectedURL = $ContainerdWindowsPackageDownloadURL + $StableContainerdPackage
      & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl $ContainerdWindowsPackageDownloadURL -KubernetesVersion "1.27.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
      Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
    }

    It 'k8s version is equal to MinimalKubernetesVersionWithLatestContainerd' {
      $expectedURL = $ContainerdWindowsPackageDownloadURL + $LatestContainerdPackage
      & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl $ContainerdWindowsPackageDownloadURL -KubernetesVersion "1.28.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
      Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
    }

    It 'k8s version is greater than MinimalKubernetesVersionWithLatestContainerd' {
      $expectedURL = $ContainerdWindowsPackageDownloadURL + $LatestContainerdPackage
      & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl $ContainerdWindowsPackageDownloadURL -KubernetesVersion "1.28.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
      Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
    }

    It 'k8s version is greater than MinimalKubernetesVersionWithLatestContainerd2' {
      $expectedURL = $ContainerdWindowsPackageDownloadURL + $LatestContainerdPackage
      & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl $ContainerdWindowsPackageDownloadURL -KubernetesVersion "1.33.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
      Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
    }

    It 'full URL is set' {
      $expectedURL = "https://privatecotnainer.com/windows-containerd-v1.2.3.tar.gz"
      & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://privatecotnainer.com/windows-containerd-v1.2.3.tar.gz" -KubernetesVersion "1.32.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
      Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
    }
  }

  Context 'Windows Server 2025 (2025)' {
    # for windows versions other than test2025, containerd version is not changed and should not include containerd2
    BeforeAll {
      Mock Get-WindowsVersion -MockWith {
        return $global:WindowsVersion2025
      }
    }

    It 'k8s version is less to MinimalKubernetesVersionWithLatestContainerd2' {
      $expectedURL = $ContainerdWindowsPackageDownloadURL + $LatestContainerd2Package
      & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl $ContainerdWindowsPackageDownloadURL -KubernetesVersion "1.31.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
      Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
    }

    It 'k8s version is equal to MinimalKubernetesVersionWithLatestContainerd2' {
      $expectedURL = $ContainerdWindowsPackageDownloadURL + $LatestContainerd2Package
      & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl $ContainerdWindowsPackageDownloadURL -KubernetesVersion "1.32.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
      Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
    }

    It 'k8s version is greater than MinimalKubernetesVersionWithLatestContainerd2' {
      $expectedURL = $ContainerdWindowsPackageDownloadURL + $LatestContainerd2Package
      & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl $ContainerdWindowsPackageDownloadURL -KubernetesVersion "1.33.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
      Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
    }

    It 'full URL is set' {
      $expectedURL = "https://privatecontainer.com/v2.0.4-azure.1/binaries/containerd-v2.0.4-azure.1-windows-amd64.tar.gz"
      & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://privatecontainer.com/v1.7.0-azure.1/binaries/containerd-v1.7.0-azure.1-windows-amd64.tar.gz" -KubernetesVersion "1.32.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
      Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
    }

    It 'full URL is set however not matching the template, use as passed in we need to handle' {
      $expectedURL = "https://privatecontainer.com/v1.2.3-windows-amd64.tar.gz"
      & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl $expectedURL -KubernetesVersion "1.32.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
      Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
    }
  }
}

Describe 'Get-WindowsVersion and Get-WindowsPauseVersion' {
  BeforeAll {
    Mock Set-ExitCode -MockWith {
        Param(
            [Parameter(Mandatory = $true)][int]
            $ExitCode,
            [Parameter(Mandatory = $true)][string]
            $ErrorMessage
        )
        Write-Host "Set ExitCode to $ExitCode and exit. Error: $ErrorMessage"
        throw $ExitCode
    } -Verifiable
  }

  It 'build number is from Windows 2019' {
    Mock Get-WindowsBuildNumber -MockWith { return "17763" }
    $windowsVersion = Get-WindowsVersion
    $expectedVersion = "1809"
    $windowsVersion | Should -Be $expectedVersion
  }

  It 'build number is from Windows 2022' {
    Mock Get-WindowsBuildNumber -MockWith { return "20348" }
    $windowsVersion = Get-WindowsVersion
    $expectedVersion = "ltsc2022"
    $windowsVersion | Should -Be $expectedVersion
  }

  It 'build number is from 23H2' {
    Mock Get-WindowsBuildNumber -MockWith { return "25398" }
    $windowsVersion = Get-WindowsVersion
    $expectedVersion = "23H2"
    $windowsVersion | Should -Be $expectedVersion
  }

  It 'build number is from prerelease of windows 2025' {
    Mock Get-WindowsBuildNumber -MockWith { return "25399" }
    $windowsVersion = Get-WindowsVersion
    $expectedVersion = "2025"
    $windowsVersion | Should -Be $expectedVersion
  }

  It 'build number is from prerelease of windows 2025' {
    Mock Get-WindowsBuildNumber -MockWith { return "30397" }
    $windowsVersion = Get-WindowsVersion
    $expectedVersion = "2025"
    $windowsVersion | Should -Be $expectedVersion
  }

  It 'build number is unknown' {
    Mock Get-WindowsBuildNumber -MockWith { return "40000" }
    try {
      $windowsVersion = Get-WindowsVersion
    } catch {
      Write-Host "Expected exception: $_"
    }
    Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_NOT_FOUND_BUILD_NUMBER }
  }

  It 'build number is from prerelease of windows 2025' {
    Mock Get-WindowsBuildNumber -MockWith { return "25399" }
    $windowsPauseVersion = Get-WindowsPauseVersion
    $expectedPauseVersion = "ltsc2022"
    $windowsPauseVersion | Should -Be $expectedPauseVersion
  }

  It 'build number is from prerelease of windows 2025' {
    Mock Get-WindowsBuildNumber -MockWith { return "30397" }
    $windowsPauseVersion = Get-WindowsPauseVersion
    $expectedPauseVersion = "ltsc2022"
    $windowsPauseVersion | Should -Be $expectedPauseVersion
  }
}

Describe 'Validate Exit Codes' {
  It 'should succeed' {
    Write-Host "Validating whether new error code name is added with the new error code"
    $global:ErrorCodeNames.Length | Should -Be $global:WINDOWS_CSE_ERROR_MAX_CODE

    for($i=0; $i -lt $global:ErrorCodeNames.Length; $i++) {
      $name=$global:ErrorCodeNames[$i]
      $name | Should -Match '[A-Z_]+'

      Write-Host "Validating $name"
      $ErrorCode = Get-Variable "$name" -ValueOnly
      $ErrorCode | Should -Be $i
      Write-Host "Validated $name"
    }
  }
}

# When using return to return values in a function with using Write-Log, the logs will be returned as well.
Describe "Mock Write-Log" {
  It 'should never exist in ut' {
    # Path to the PowerShell script you want to test
    $scriptPaths = @()
    $cseScripts = Get-ChildItem -Path "$PSScriptRoot\..\..\staging\cse\windows\" -Filter "*tests.ps1"
    foreach($script in $cseScripts) {
      $scriptPaths += $script.FullName
    }

    foreach($scriptPath in $scriptPaths) {
      Write-Host "Validating $scriptPath"
      $scriptContent = Get-Content -Path $scriptPath
      # Uncomment the -Because to find out which script. The version of Pester in the pipeline does not support -Because :cry:
      $scriptContent -join "`n" | Should -Not -Match "Mock Write-Log" # -Because "$scriptPath should not mock Write-Log"
    }
  }
}


# When using return to return values in a function with using Write-Log, the logs will be returned as well.
Describe "Resolve-PackagesSourceUrl" {
  BeforeEach {
    $global:PackageDownloadFqdn = $null
  }

  It 'given valid preferred fqdn, returns preferred' {
    $preferredFqdn = "packages.aks.azure.com"
    $fallbackFqdn = "acs-mirror.azure.com"

    Resolve-PackagesDownloadFqdn -PreferredFqdn $preferredFqdn -FallbackFqdn $fallbackFqdn -Retries 1 -WaitSleepSeconds 1

    $global:PackageDownloadFqdn | Should -Be $preferredFqdn
  }

  It 'given invalid preferred fqdn, returns fallback' {
    $preferredFqdn = "baddomain.aks.azure.com"
    $fallbackFqdn = "acs-mirror.azure.com"

    Resolve-PackagesDownloadFqdn -PreferredFqdn $preferredFqdn -FallbackFqdn $fallbackFqdn -Retries 1 -WaitSleepSeconds 1

    $global:PackageDownloadFqdn | Should -Be $fallbackFqdn
  }

  It 'given all invalid fqdns, still returns fallback' {
    $preferredFqdn = "baddomain.aks.azure.com"
    $fallbackFqdn = "badacs-mirror.azure.com"

    Resolve-PackagesDownloadFqdn -PreferredFqdn $preferredFqdn -FallbackFqdn $fallbackFqdn -Retries 1 -WaitSleepSeconds 1

    $global:PackageDownloadFqdn | Should -Be $fallbackFqdn
  }

  It 'should return fallback fqdn when preferred check returns 404' {
    Mock Invoke-WebRequest -MockWith {
      # Create a custom object that mimics an Invoke-WebRequest response with StatusCode
      [PSCustomObject]@{
        StatusCode = 404  # Set the status code you want to test
        Content = "Not Found"
        Headers = @{}
      }
    }

    $preferredFqdn = "baddomain.aks.azure.com"
    $fallbackFqdn = "badacs-mirror.azure.com"

    Resolve-PackagesDownloadFqdn -PreferredFqdn $preferredFqdn -FallbackFqdn $fallbackFqdn -Retries 1 -WaitSleepSeconds 1

    $global:PackageDownloadFqdn | Should -Be $fallbackFqdn
  }

  It 'should return fallback fqdn when preferred check returns 500' {
    Mock Invoke-WebRequest -MockWith {
      [PSCustomObject]@{
        StatusCode = 500  # Internal server error
        Content = "Internal Server Error"
        Headers = @{}
      }
    }

    $preferredFqdn = "packages.aks.azure.com"
    $fallbackFqdn = "acs-mirror.azure.com"

    Resolve-PackagesDownloadFqdn -PreferredFqdn $preferredFqdn -FallbackFqdn $fallbackFqdn -Retries 1 -WaitSleepSeconds 1

    $global:PackageDownloadFqdn | Should -Be $fallbackFqdn
  }

  It 'should handle exception with no response' {
    Mock Invoke-WebRequest -MockWith {
      throw [System.Net.WebException]::new("Connection Timed Out")
    }

    $preferredFqdn = "packages.aks.azure.com"
    $fallbackFqdn = "acs-mirror.azure.com"

    Resolve-PackagesDownloadFqdn -PreferredFqdn $preferredFqdn -FallbackFqdn $fallbackFqdn -Retries 1 -WaitSleepSeconds 1

    $global:PackageDownloadFqdn | Should -Be $fallbackFqdn
  }

  It 'should correctly handle successful response' {
    Mock Invoke-WebRequest -MockWith {
      [PSCustomObject]@{
        StatusCode = 200  # Success
        Content = "OK"
        Headers = @{}
      }
    }

    $preferredFqdn = "packages.aks.azure.com"
    $fallbackFqdn = "acs-mirror.azure.com"

    Resolve-PackagesDownloadFqdn -PreferredFqdn $preferredFqdn -FallbackFqdn $fallbackFqdn -Retries 1 -WaitSleepSeconds 1

    $global:PackageDownloadFqdn | Should -Be $preferredFqdn
  }

  It 'should call Invoke-WebRequest with 2 times when 2 retries and bad fqdn' {
    Mock Invoke-WebRequest -MockWith {
      [PSCustomObject]@{
        StatusCode = 404  # Success
        Content = "NotFound"
        Headers = @{}
      }
    }

    $preferredFqdn = "packages.aks.azure.com"
    $fallbackFqdn = "acs-mirror.azure.com"

    Resolve-PackagesDownloadFqdn -PreferredFqdn $preferredFqdn -FallbackFqdn $fallbackFqdn -Retries 2 -WaitSleepSeconds 1

    Assert-MockCalled -CommandName "Invoke-WebRequest" -Exactly -Times 2
  }

  It 'should call Invoke-WebRequest with 1 times when 2 retries and valid fqdn' {
    Mock Invoke-WebRequest -MockWith {
      [PSCustomObject]@{
        StatusCode = 200  # Success
        Content = "OK"
        Headers = @{}
      }
    }

    $preferredFqdn = "packages.aks.azure.com"
    $fallbackFqdn = "acs-mirror.azure.com"

    Resolve-PackagesDownloadFqdn -PreferredFqdn $preferredFqdn -FallbackFqdn $fallbackFqdn -Retries 2 -WaitSleepSeconds 1

    Assert-MockCalled -CommandName "Invoke-WebRequest" -Exactly -Times 1
  }
}

Describe "DownloadFileOverHttp" {
  BeforeEach {
    # Reset the PackageDownloadFqdn before each test
    $global:PackageDownloadFqdn = $null
    $global:PreferredPackageDownloadFqdn = "packages.aks.azure.com"
    $global:FallbackPackageDownloadFqdn = "acs-mirror.azureedge.net"

    # Mock the main utilities used in DownloadFileOverHttp
    Mock Invoke-RestMethod -MockWith {} -Verifiable
    Mock Set-ExitCode -MockWith { throw "Set-ExitCode called with: $ExitCode, $ErrorMessage" }
    Mock Write-Log -MockWith {}
    Mock Resolve-PackagesDownloadFqdn -MockWith {} -Verifiable
    Mock Update-BaseUrl -MockWith { return $InitialUrl }
    Mock Get-Item -MockWith { return New-Object -TypeName PSObject -Property @{ FullName = $DestinationPath } }
  }

  It "should use Update-BaseUrl after resolving the FQDN" {
    # Mock Update-BaseUrl to verify it's called with the right parameters
    Mock Update-BaseUrl -MockWith { return "https://updated.domain.com/test/file.zip" } -Verifiable

    # Call the function with a URL containing acs-mirror.azureedge.net
    $destPath = Join-Path -Path (Get-Location) -ChildPath "testfile.zip"
    DownloadFileOverHttp -Url "https://acs-mirror.azureedge.net/test/file.zip" -DestinationPath $destPath -ExitCode 999

    # Verify that Update-BaseUrl was called
    Assert-MockCalled -CommandName "Update-BaseUrl" -Exactly -Times 1 -ParameterFilter {
      $InitialUrl -eq "https://acs-mirror.azureedge.net/test/file.zip"
    }
  }

  It "should download a file when it's not in the cache" {
    # Call the function
    $destPath = Join-Path -Path $(Get-Location) -ChildPath "testfile.zip"
    DownloadFileOverHttp -Url "https://acs-mirror.azureedge.net/test/file.zip" -DestinationPath $destPath -ExitCode 999

    # Verify Invoke-RestMethod was called to download the file
    Assert-MockCalled -CommandName "Invoke-RestMethod" -Exactly -Times 1
  }

  It "should handle URL update after FQDN resolution" {
    # Mock Update-BaseUrl to return a different URL
    Mock Update-BaseUrl -MockWith { return "https://packages.aks.azure.com/test/file.zip" }

    # Mock Invoke-RestMethod with proper parameter capture
    Mock Invoke-RestMethod -MockWith {
      # Implementation doesn't matter for the test
    } -ParameterFilter {
      $Uri -eq "https://packages.aks.azure.com/test/file.zip"
    }

    # Call the function
    $destPath = Join-Path -Path (Get-Location) -ChildPath "testfile.zip"
    DownloadFileOverHttp -Url "https://acs-mirror.azureedge.net/test/file.zip" -DestinationPath $destPath -ExitCode 999

    # Verify Invoke-RestMethod was called with the updated URL
    Assert-MockCalled -CommandName "Invoke-RestMethod" -Exactly -Times 1 -ParameterFilter {
      $Uri -eq "https://packages.aks.azure.com/test/file.zip"
    }
  }
}

Describe "Update-BaseUrl" {
  BeforeEach {
    # Reset the PackageDownloadFqdn before each test
    $global:PackageDownloadFqdn = $null
    $global:PreferredPackageDownloadFqdn = "packages.aks.azure.com"
    $global:FallbackPackageDownloadFqdn = "acs-mirror.azureedge.net"
    Mock Resolve-PackagesDownloadFqdn -MockWith { $global:PackageDownloadFqdn = $PreferredFqdn }
  }

  It "should not modify URL when domain is not a match" {
    $global:PackageDownloadFqdn = "packages.aks.azure.com"
    $initialUrl = "https://some-other-domain.com/path/to/resource"

    $result = Update-BaseUrl -InitialUrl $initialUrl

    $result | Should -Be $initialUrl
  }

  It "should replace acs-mirror.azureedge.net with packages.aks.azure.com when appropriate" {
    $global:PackageDownloadFqdn = "packages.aks.azure.com"
    $initialUrl = "https://acs-mirror.azureedge.net/path/to/resource"
    $expectedUrl = "https://packages.aks.azure.com/path/to/resource"

    $result = Update-BaseUrl -InitialUrl $initialUrl

    $result | Should -Be $expectedUrl
  }

  It "should replace packages.aks.azure.com with acs-mirror.azureedge.net when appropriate" {
    $global:PackageDownloadFqdn = "acs-mirror.azureedge.net"
    $initialUrl = "https://packages.aks.azure.com/path/to/resource"
    $expectedUrl = "https://acs-mirror.azureedge.net/path/to/resource"

    $result = Update-BaseUrl -InitialUrl $initialUrl

    $result | Should -Be $expectedUrl
  }

  It "should handle URLs with query parameters correctly" {
    $global:PackageDownloadFqdn = "packages.aks.azure.com"
    $initialUrl = "https://acs-mirror.azureedge.net/path/to/resource?param=value&param2=value2"
    $expectedUrl = "https://packages.aks.azure.com/path/to/resource?param=value&param2=value2"

    $result = Update-BaseUrl -InitialUrl $initialUrl

    $result | Should -Be $expectedUrl
  }

  It "should handle URLs with special characters correctly" {
    $global:PackageDownloadFqdn = "packages.aks.azure.com"
    $initialUrl = "https://acs-mirror.azureedge.net/path/to/resource-with-hyphens_and_underscores.tar.gz"
    $expectedUrl = "https://packages.aks.azure.com/path/to/resource-with-hyphens_and_underscores.tar.gz"

    $result = Update-BaseUrl -InitialUrl $initialUrl

    $result | Should -Be $expectedUrl
  }

  It "should not modify URL when domain is a match but PackageDownloadFqdn is set to something else" {
    $global:PackageDownloadFqdn = "some-other-fqdn.com"
    $initialUrl = "https://acs-mirror.azureedge.net/path/to/resource"

    $result = Update-BaseUrl -InitialUrl $initialUrl

    $result | Should -Be $initialUrl
  }

  It "should work with domains in the middle of the URL" {
    $global:PackageDownloadFqdn = "packages.aks.azure.com"
    $initialUrl = "https://prefix-acs-mirror.azureedge.net-suffix/path/to/resource"

    $result = Update-BaseUrl -InitialUrl $initialUrl

    # Domain should not be replaced since it's not an exact match
    $result | Should -Be $initialUrl
  }

  It "should handle multiple occurrences of the domain" {
    $global:PackageDownloadFqdn = "packages.aks.azure.com"
    $initialUrl = "https://acs-mirror.azureedge.net/path/to/acs-mirror.azureedge.net/resource"
    $expectedUrl = "https://packages.aks.azure.com/path/to/packages.aks.azure.com/resource"

    $result = Update-BaseUrl -InitialUrl $initialUrl

    $result | Should -Be $expectedUrl
  }

  It "should call Resolve-PackagesDownloadFqdn when PackageDownloadFqdn is null and URL contains acs-mirror.azureedge.net" {
    # Ensure PackageDownloadFqdn is null
    $global:PackageDownloadFqdn = $null

    # Call Update-BaseUrl with a URL containing acs-mirror.azureedge.net
    $initialUrl = "https://acs-mirror.azureedge.net/path/to/resource"
    $result = Update-BaseUrl -InitialUrl $initialUrl

    # Verify Resolve-PackagesDownloadFqdn was called
    Assert-MockCalled -CommandName "Resolve-PackagesDownloadFqdn" -Times 1 -ParameterFilter {
      $PreferredFqdn -eq $global:PreferredPackageDownloadFqdn -and $FallbackFqdn -eq $global:FallbackPackageDownloadFqdn
    }
  }

  It "should call Resolve-PackagesDownloadFqdn when PackageDownloadFqdn is null and URL contains packages.aks.azure.com" {
    # Ensure PackageDownloadFqdn is null
    $global:PackageDownloadFqdn = $null

    # Call Update-BaseUrl with a URL containing packages.aks.azure.com
    $initialUrl = "https://packages.aks.azure.com/path/to/resource"
    $result = Update-BaseUrl -InitialUrl $initialUrl

    # Verify Resolve-PackagesDownloadFqdn was called
    Assert-MockCalled -CommandName "Resolve-PackagesDownloadFqdn" -Times 1 -ParameterFilter {
      $PreferredFqdn -eq $global:PreferredPackageDownloadFqdn -and $FallbackFqdn -eq $global:FallbackPackageDownloadFqdn
    }
  }

  It "should not call Resolve-PackagesDownloadFqdn when PackageDownloadFqdn is null but URL is different domain" {
    # Ensure PackageDownloadFqdn is null
    $global:PackageDownloadFqdn = $null

    # Call Update-BaseUrl with a URL that doesn't contain either domain
    $initialUrl = "https://other-domain.com/path/to/resource"
    $result = Update-BaseUrl -InitialUrl $initialUrl

    # Verify Resolve-PackagesDownloadFqdn was not called
    Assert-MockCalled -CommandName "Resolve-PackagesDownloadFqdn" -Times 0

    # Verify the URL wasn't modified
    $result | Should -Be $initialUrl
  }

  It "should not call Resolve-PackagesDownloadFqdn when PackageDownloadFqdn is already set" {
    # Set PackageDownloadFqdn to a value
    $global:PackageDownloadFqdn = "already-set.example.com"

    # Call Update-BaseUrl with a URL containing acs-mirror.azureedge.net
    $initialUrl = "https://acs-mirror.azureedge.net/path/to/resource"
    $result = Update-BaseUrl -InitialUrl $initialUrl

    # Verify Resolve-PackagesDownloadFqdn was not called
    Assert-MockCalled -CommandName "Resolve-PackagesDownloadFqdn" -Times 0
  }

  It "should correctly update URL after resolving FQDN from null" {
    # Ensure PackageDownloadFqdn is null to trigger resolution
    $global:PackageDownloadFqdn = $null

    # Mock Resolve-PackagesDownloadFqdn to set to the preferred FQDN
    Mock Resolve-PackagesDownloadFqdn -MockWith { $global:PackageDownloadFqdn = "packages.aks.azure.com" }

    # Call Update-BaseUrl with a URL containing the fallback domain
    $initialUrl = "https://acs-mirror.azureedge.net/path/to/resource"
    $result = Update-BaseUrl -InitialUrl $initialUrl

    # Verify Resolve-PackagesDownloadFqdn was called
    Assert-MockCalled -CommandName "Resolve-PackagesDownloadFqdn" -Times 1

    # Verify the URL was updated to use the preferred domain
    $result | Should -Be "https://packages.aks.azure.com/path/to/resource"
  }
}
