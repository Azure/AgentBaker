BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1','.ps1')
  }

Describe 'GetBroadestRangesForEachAddress' {

    It "Values '<Values>' should return '<Expected>'" -TestCases @(
        @{ Values = @('10.240.0.0/12', '10.0.0.0/8'); Expected = @('10.0.0.0/8', '10.240.0.0/12')}
        @{ Values = @('10.0.0.0/8', '10.0.0.0/16'); Expected = @('10.0.0.0/8')}
        @{ Values = @('10.0.0.0/16', '10.240.0.0/12', '10.0.0.0/8' ); Expected = @('10.0.0.0/8', '10.240.0.0/12')}
        @{ Values = @(); Expected = @()}
        @{ Values = @('foobar'); Expected = @()}
    ){
        param ($Values, $Expected)

        $actual = GetBroadestRangesForEachAddress -values $Values
        $actual | Should -Be $Expected
    }
}

Describe 'Set-AzureCNIConfig' {
    BeforeEach {
        $azureCNIConfDir = "$PSScriptRoot\azurecnifunc.tests.suites"
        $kubeDnsSearchPath = "svc.cluster.local"
        $kubeClusterCIDR = "10.224.0.0/12"
        $kubeServiceCIDR = "10.0.0.0/16"
        $vNetCIDR = "10.224.1.0/12"
        $isDualStackEnabled = $false

        $KubeDnsServiceIp = "10.0.0.10"
        $global:IsDisableWindowsOutboundNat = $false
        $global:KubeproxyFeatureGates = @()

        # AzureCNIConfig uses the default one
        $defaultFile = [Io.path]::Combine($azureCNIConfDir, "AzureCNI.Default.conflist")
        $azureCNIConfigFile = [Io.path]::Combine($azureCNIConfDir, "10-azure.conflist")
        Copy-Item -Path $defaultFile -Destination $azureCNIConfigFile

        # Read Json with the same format (depth = 20) for Json Comparation
        function Read-Format-Json ([string]$JsonFile)
        {
            $json = Get-Content $JsonFile | ConvertFrom-Json
            $json = $json | ConvertTo-Json -depth 20
            return $json
        }
    }

    AfterEach {
        $azureCNIConfigFile = [Io.path]::Combine($azureCNIConfDir, "10-azure.conflist")
        if (Test-Path $azureCNIConfigFile) {
            Remove-Item -Path $azureCNIConfigFile
        }
    }

    Context 'WinDSR is enabled' {
        It "Should remove ROUTE when WinDSR is enabled" {
            $global:KubeproxyFeatureGates = @("WinDSR=true")

            Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                -KubeDnsSearchPath $kubeDnsSearchPath `
                -KubeClusterCIDR $kubeClusterCIDR `
                -KubeServiceCIDR $kubeServiceCIDR `
                -VNetCIDR $vNetCIDR `
                -IsDualStackEnabled $isDualStackEnabled

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.WinDSR.conflist"))
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }

        It "Should replace OutboundNAT with LoopbackDSR when IsDisableWindowsOutboundNat is true" {
            $global:KubeproxyFeatureGates = @("WinDSR=true")
            $global:IsDisableWindowsOutboundNat = $true

            Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                -KubeDnsSearchPath $kubeDnsSearchPath `
                -KubeClusterCIDR $kubeClusterCIDR `
                -KubeServiceCIDR $kubeServiceCIDR `
                -VNetCIDR $vNetCIDR `
                -IsDualStackEnabled $isDualStackEnabled

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.DisableWindowsOutboundNat.conflist"))
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }
    }
}

Describe 'Get-HnsPsm1' {
    Context 'Docker' {
        It "Should download hns.psm1 with the correct URL" {
            Mock DownloadFileOverHttp
            Get-HnsPsm1 -HNSModule "C:\k\hns.psm1"
            Assert-MockCalled -CommandName "DownloadFileOverHttp" -Exactly -Times 1 -ParameterFilter { $Url -eq "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/hns.psm1" -and $DestinationPath -eq "C:\k\hns.psm1" -and $ExitCode -eq $global:WINDOWS_CSE_ERROR_DOWNLOAD_HNS_MODULE }
        }
    }

    Context 'Containerd' {
        It "Should download hns.v2.psm1 with the correct URL" {
            Mock DownloadFileOverHttp
            Get-HnsPsm1 -HNSModule "C:\k\hns.v2.psm1"
            Assert-MockCalled -CommandName "DownloadFileOverHttp" -Exactly -Times 1 -ParameterFilter { $Url -eq "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/hns.v2.psm1" -and $DestinationPath -eq "C:\k\hns.v2.psm1" -and $ExitCode -eq $global:WINDOWS_CSE_ERROR_DOWNLOAD_HNS_MODULE }
        }
    }
}