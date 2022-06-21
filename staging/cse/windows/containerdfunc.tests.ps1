BeforeAll {
  . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
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
    $expectedURL = [string]::Format("https://acs-mirror.azureedge.net/containerd/windows/{0}/binaries/containerd-{0}-windows-amd64.tar.gz", $global:StableContainerdVersion)
    & Install-Containerd-Based-On-Kubernetes-Version -KubernetesVersion "1.25.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }

  It 'k8s version is equal to MinimalKubernetesVersionWithLatestContainerd' {
    $expectedURL = [string]::Format("https://acs-mirror.azureedge.net/containerd/windows/{0}/binaries/containerd-{0}-windows-amd64.tar.gz", $global:LatestContainerdVersion)
    & Install-Containerd-Based-On-Kubernetes-Version -KubernetesVersion "1.26.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }

  It 'k8s version is greater than MinimalKubernetesVersionWithLatestContainerd' {
    $expectedURL = [string]::Format("https://acs-mirror.azureedge.net/containerd/windows/{0}/binaries/containerd-{0}-windows-amd64.tar.gz", $global:LatestContainerdVersion)
    & Install-Containerd-Based-On-Kubernetes-Version -KubernetesVersion "1.26.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }
}