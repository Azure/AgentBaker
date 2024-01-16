BeforeAll {
  . $PSScriptRoot\windowscsehelper.ps1
  . $PSScriptRoot\..\..\staging\cse\windows\containerdfunc.ps1
  . $PSCommandPath.Replace('.tests.ps1','.ps1')
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
          $KubeDir
        )
        Write-Host $ContainerdUrl
    } -Verifiable
  }

  It 'k8s version is less than MinimalKubernetesVersionWithLatestContainerd' {
    $expectedURL = "https://acs-mirror.azureedge.net/containerd/windows/v1.6.21-azure.1/binaries/containerd-v1.6.21-azure.1-windows-amd64.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://acs-mirror.azureedge.net/containerd/windows/" -KubernetesVersion "1.27.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }

  It 'k8s version is equal to MinimalKubernetesVersionWithLatestContainerd' {
    $expectedURL = "https://acs-mirror.azureedge.net/containerd/windows/v1.7.9-azure.1/binaries/containerd-v1.7.9-azure.1-windows-amd64.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://acs-mirror.azureedge.net/containerd/windows/" -KubernetesVersion "1.28.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }

  It 'k8s version is greater than MinimalKubernetesVersionWithLatestContainerd' {
    $expectedURL = "https://mirror.azk8s.cn/containerd/windows/v1.7.9-azure.1/binaries/containerd-v1.7.9-azure.1-windows-amd64.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://mirror.azk8s.cn/containerd/windows/" -KubernetesVersion "1.28.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }

  It 'full URL is set' {
    $expectedURL = "https://privatecotnainer.com/windows-containerd-v1.2.3.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://privatecotnainer.com/windows-containerd-v1.2.3.tar.gz" -KubernetesVersion "1.26.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
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
    $expectedVersion = "test2025"
    $windowsVersion | Should -Be $expectedVersion
  }

  It 'build number is from prerelease of windows 2025' {
    Mock Get-WindowsBuildNumber -MockWith { return "30397" }
    $windowsVersion = Get-WindowsVersion
    $expectedVersion = "test2025"
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