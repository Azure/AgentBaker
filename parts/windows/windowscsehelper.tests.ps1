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
    $expectedURL = "https://acs-mirror.azureedge.net/containerd/windows/v0.0.47/binaries/containerd-v0.0.47-windows-amd64.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://acs-mirror.azureedge.net/containerd/windows/" -KubernetesVersion "1.24.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }

  It 'k8s version is equal to MinimalKubernetesVersionWithLatestContainerd' {
    $expectedURL = "https://acs-mirror.azureedge.net/containerd/windows/v1.0.46/binaries/containerd-v1.0.46-windows-amd64.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://acs-mirror.azureedge.net/containerd/windows/" -KubernetesVersion "1.30.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }

  It 'k8s version is greater than MinimalKubernetesVersionWithLatestContainerd' {
    $expectedURL = "https://mirror.azk8s.cn/containerd/windows/v1.0.46/binaries/containerd-v1.0.46-windows-amd64.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://mirror.azk8s.cn/containerd/windows/" -KubernetesVersion "1.30.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }

  It 'full URL is set' {
    $expectedURL = "https://privatecotnainer.com/windows-containerd-v1.2.3.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://privatecotnainer.com/windows-containerd-v1.2.3.tar.gz" -KubernetesVersion "1.26.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }
}