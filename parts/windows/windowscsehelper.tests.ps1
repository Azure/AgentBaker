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
    $expectedURL = "https://acs-mirror.azureedge.net/containerd/windows/v0.0.56/binaries/containerd-v0.0.56-windows-amd64.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://acs-mirror.azureedge.net/containerd/windows/" -KubernetesVersion "1.27.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }

  It 'k8s version is equal to MinimalKubernetesVersionWithLatestContainerd' {
    $expectedURL = "https://acs-mirror.azureedge.net/containerd/windows/v1.7.1-azure.1/binaries/containerd-v1.7.1-azure.1-windows-amd64.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://acs-mirror.azureedge.net/containerd/windows/" -KubernetesVersion "1.28.0" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }

  It 'k8s version is greater than MinimalKubernetesVersionWithLatestContainerd' {
    $expectedURL = "https://mirror.azk8s.cn/containerd/windows/v1.7.1-azure.1/binaries/containerd-v1.7.1-azure.1-windows-amd64.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://mirror.azk8s.cn/containerd/windows/" -KubernetesVersion "1.28.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }

  It 'full URL is set' {
    $expectedURL = "https://privatecotnainer.com/windows-containerd-v1.2.3.tar.gz"
    & Install-Containerd-Based-On-Kubernetes-Version -ContainerdUrl "https://privatecotnainer.com/windows-containerd-v1.2.3.tar.gz" -KubernetesVersion "1.26.1" -CNIBinDir "cniBinPath" -CNIConfDir "cniConfigPath" -KubeDir "kubeDir"
    Assert-MockCalled -CommandName "Install-Containerd" -Exactly -Times 1 -ParameterFilter { $ContainerdUrl -eq $expectedURL }
  }
}