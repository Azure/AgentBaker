BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1','.ps1')
}

Describe 'Get-KubePackage' {
    BeforeEach {
        Mock Expand-Archive
        Mock Remove-Item
        Mock Logs-To-Event
        Mock DownloadFileOverHttp -MockWith {
            Param(
                $Url,
                $DestinationPath,
                $ExitCode
            )
            Write-Host "DownloadFileOverHttp -Url $Url -DestinationPath $DestinationPath -ExitCode $ExitCode"
        } -Verifiable

        $global:CacheDir = 'c:\akse-cache'
    }

    Context 'mapping file exists' {
        BeforeEach {
            Mock Test-Path -MockWith { $true }
            Mock Get-Content -MockWith {
                Param(
                    $Path
                )
@'
                {
                    "1.29.5":  "https://xxx.blob.core.windows.net/kubernetes/v1.29.5-hotfix.20240101/windowszip/v1.29.5-hotfix.20240101-1int.zip",
                    "1.29.2":  "https://xxx.blob.core.windows.net/kubernetes/v1.29.2-hotfix.20240101/windowszip/v1.29.2-hotfix.20240101-1int.zip",
                    "1.29.0":  "https://xxx.blob.core.windows.net/kubernetes/v1.29.0-hotfix.20240101/windowszip/v1.29.0-hotfix.20240101-1int.zip",
                    "1.28.3":  "https://xxx.blob.core.windows.net/kubernetes/v1.28.3-hotfix.20240101/windowszip/v1.28.3-hotfix.20240101-1int.zip",
                    "1.28.0":  "https://xxx.blob.core.windows.net/kubernetes/v1.28.0-hotfix.20240101/windowszip/v1.28.0-hotfix.20240101-1int.zip",
                    "1.27.1":  "https://xxx.blob.core.windows.net/kubernetes/v1.27.1-hotfix.20240101/windowszip/v1.27.1-hotfix.20240101-1int.zip",
                    "1.27.0":  "https://xxx.blob.core.windows.net/kubernetes/v1.27.0-hotfix.20240101/windowszip/v1.27.0-hotfix.20240101-1int.zip"
                }
'@
            }
        }

        It "KubeBinariesSASURL should be changed when the version exists in the mapping file" {
            $global:KubeBinariesVersion = '1.29.2'
            Get-KubePackage -KubeBinariesSASURL 'https://xxx.blob.core.windows.net/kubernetes/v1.29.2/windowszip/v1.29.2-1int.zip'
            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq 'https://xxx.blob.core.windows.net/kubernetes/v1.29.2-hotfix.20240101/windowszip/v1.29.2-hotfix.20240101-1int.zip' }
        }

        It "KubeBinariesSASURL should not be changed when the version does not exist in the mapping file" {
            $global:KubeBinariesVersion = '1.30.0'
            Get-KubePackage -KubeBinariesSASURL 'https://xxx.blob.core.windows.net/kubernetes/v1.30.0/windowszip/v1.30.0-1int.zip'
            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq 'https://xxx.blob.core.windows.net/kubernetes/v1.30.0/windowszip/v1.30.0-1int.zip' }
        }
    }

    Context 'mapping file does not exist' {
        It "KubeBinariesSASURL should not be changed" {
            Mock Test-Path -MockWith { $false }
            $global:KubeBinariesVersion = '1.29.2'
            Get-KubePackage -KubeBinariesSASURL 'https://xxx.blob.core.windows.net/kubernetes/v1.29.2/windowszip/v1.29.2-1int.zip'
            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq 'https://xxx.blob.core.windows.net/kubernetes/v1.29.2/windowszip/v1.29.2-1int.zip' }
        }
    }
}

Describe 'Disable-KubeletServingCertificateRotationForTags' {
    BeforeEach {
        Mock Logs-To-Event
    }

    It "Should no-op when EnableKubeletServingCertificateRotation is already disabled" {
        Mock Get-TagValue -MockWith { "false" }
        $kubeletConfigArgs = "--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false"
        $kubeletNodeLabels = "kubernetes.azure.com/agentpool=wp0"
        $global:KubeletNodeLabels = $kubeletNodeLabels
        $global:KubeletConfigArgs = $kubeletConfigArgs
        $global:EnableKubeletServingCertificateRotation = $false
        Disable-KubeletServingCertificateRotationForTags
        Compare-Object $global:KubeletConfigArgs $kubeletConfigArgs | Should -Be $null
        Compare-Object $global:KubeletNodeLabels $kubeletNodeLabels | Should -Be $null
        Assert-MockCalled -CommandName 'Get-TagValue' -Exactly -Times 0
    }

    It "Should no-op when the aks-disable-kubelet-serving-certificate-rotation tag is not true" {
        Mock Get-TagValue -MockWith { "false" }
        $kubeletConfigArgs = "--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
        $kubeletNodeLabels = "kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster"
        $global:KubeletConfigArgs = $kubeletConfigArgs
        $global:KubeletNodeLabels = $kubeletNodeLabels
        $global:EnableKubeletServingCertificateRotation = $true
        Disable-KubeletServingCertificateRotationForTags
        Compare-Object $global:KubeletConfigArgs $kubeletConfigArgs | Should -Be $null
        Compare-Object $global:KubeletNodeLabels $kubeletNodeLabels | Should -Be $null
        Assert-MockCalled -CommandName 'Get-TagValue' -Exactly -Times 1 -ParameterFilter { $TagName -eq 'aks-disable-kubelet-serving-certificate-rotation' -and $DefaultValue -eq "false" }
    }

    It "Should reconfigure kubelet config args and node labels when aks-disable-kubelet-serving-certificate-rotation is true" {
        Mock Get-TagValue -MockWith { "true" }
        $global:KubeletConfigArgs = "--rotate-certificates=true,--rotate-server-certificates=true,--node-ip=10.0.0.1,anonymous-auth=false"
        $global:KubeletNodeLabels = "kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster"
        $global:EnableKubeletServingCertificateRotation = $true
        Disable-KubeletServingCertificateRotationForTags
        Compare-Object $global:KubeletConfigArgs "--rotate-certificates=true,--rotate-server-certificates=false,--node-ip=10.0.0.1,anonymous-auth=false" | Should -Be $null
        Compare-Object $global:KubeletNodeLabels "kubernetes.azure.com/agentpool=wp0" | Should -Be $null
        Assert-MockCalled -CommandName 'Get-TagValue' -Exactly -Times 1 -ParameterFilter { $TagName -eq 'aks-disable-kubelet-serving-certificate-rotation' -and $DefaultValue -eq "false" }
    }
}

Describe 'Get-TagValue' {
    Context 'IMDS returns a valid response' {
        It "Should return the tag value if it is present within the IMDS response" {
            Mock Invoke-RestMethod -MockWith {
                return (Get-Content "$PSScriptRoot\kubeletfunc.tests.suites\IMDS.Instance.TagExists.json" | Out-String | ConvertFrom-Json)
            }
            $result = Get-TagValue -TagName "aks-disable-kubelet-serving-certificate-rotation" -DefaultValue "false"
            $expected = "true"
            Compare-Object $result $expected | Should -Be $null
            Assert-MockCalled -CommandName 'Invoke-RestMethod' -Exactly -Times 1 -ParameterFilter { $Uri -eq 'http://169.254.169.254/metadata/instance?api-version=2021-02-01' }
        }

        It "Should return the default value of the tag is not present within the response" {
            Mock Invoke-RestMethod -MockWith {
                return (Get-Content "$PSScriptRoot\kubeletfunc.tests.suites\IMDS.Instance.TagDoesNotExist.json" | Out-String | ConvertFrom-Json)
            }
            $result = Get-TagValue -TagName "aks-disable-kubelet-serving-certificate-rotation" -DefaultValue "false"
            $expected = "false"
            Compare-Object $result $expected | Should -Be $null
            Assert-MockCalled -CommandName 'Invoke-RestMethod' -Exactly -Times 1 -ParameterFilter { $Uri -eq 'http://169.254.169.254/metadata/instance?api-version=2021-02-01' }
        }
    }

    Context 'Unable to call IMDS' {
        BeforeEach {
            Mock Set-ExitCode
            Mock Invoke-RestMethod -MockWith { 
                Throw 'IMDS is down' 
            }
        }

        It "Should return the default value when an error is encountered while calling IMDS" {
            Get-TagValue -TagName "aks-disable-kubelet-serving-certificate-rotation" -DefaultValue "false"
            Assert-MockCalled -CommandName 'Invoke-RestMethod' -Exactly -Times 3 -ParameterFilter { $Uri -eq 'http://169.254.169.254/metadata/instance?api-version=2021-02-01' }
            Assert-MockCalled -CommandName 'Set-ExitCode' -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_LOOKUP_INSTANCE_DATA_TAG }
        }
    }
}

Describe 'Remove-KubeletNodeLabel' {
    It "Should remove the specified label when it exists within the label string" {
        $labelString = "kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/kubelet-serving-ca=cluster,kubernetes.azure.com/agentpool=wp0"
        $label = "kubernetes.azure.com/kubelet-serving-ca=cluster"
        $expected = "kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0"
        $result = Remove-KubeletNodeLabel -KubeletNodeLabels $labelString -Label $label
        Compare-Object $result $expected | Should -Be $null
    }

    It "Should remove the specified label when it is the first label within the label string" {
        $labelString = "kubernetes.azure.com/kubelet-serving-ca=cluster,kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0"
        $label = "kubernetes.azure.com/kubelet-serving-ca=cluster"
        $expected = "kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0"
        $result = Remove-KubeletNodeLabel -KubeletNodeLabels $labelString -Label $label
        Compare-Object $result $expected | Should -Be $null
    }

    It "Should remove the specified label when it is the last label within the label string" {
        $labelString = "kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0,kubernetes.azure.com/kubelet-serving-ca=cluster"
        $label = "kubernetes.azure.com/kubelet-serving-ca=cluster"
        $expected = "kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0"
        $result = Remove-KubeletNodeLabel -KubeletNodeLabels $labelString -Label $label
        Compare-Object $result $expected | Should -Be $null
    }

    It "Should not alter the specified label string if the target label does not exist" {
        $labelString = "kubernetes.azure.com/nodepool-type=VirtualMachineScaleSets,kubernetes.azure.com/agentpool=wp0"
        $label = "kubernetes.azure.com/kubelet-serving-ca=cluster"
        $expected = $labelString
        $result = Remove-KubeletNodeLabel -KubeletNodeLabels $labelString -Label $label
        Compare-Object $result $expected | Should -Be $null
    }

    It "Should return an empty string if the only label within the label string is the target" {
        $labelString = "kubernetes.azure.com/kubelet-serving-ca=cluster"
        $label = "kubernetes.azure.com/kubelet-serving-ca=cluster"
        $expected = ""
        $result = Remove-KubeletNodeLabel -KubeletNodeLabels $labelString -Label $label
        Compare-Object $result $expected | Should -Be $null
    }
}