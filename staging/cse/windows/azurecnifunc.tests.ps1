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
        $global:KubeproxyFeatureGates = @("WinDSR=true")
        $azureCNIConfigFile = [Io.path]::Combine($azureCNIConfDir, "10-azure.conflist")

        # Set the default AzureCNI (mock the file download from https://acs-mirror.azureedge.net/azure-cni/.../10-azure.conflist)
        function Set-Default-AzureCNI ([string]$fileName) {
            $defaultFile = [Io.path]::Combine($azureCNIConfDir, $fileName)    
            Copy-Item -Path $defaultFile -Destination $azureCNIConfigFile
        }

        # Read Json with the same format (depth = 20) for Json Comparation
        function Read-Format-Json ([string]$JsonFile) {
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

    Context 'WinDSR is enabled by default' {
        It "Should remove ROUTE" {
            Set-Default-AzureCNI "AzureCNI.Default.conflist"

            Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                -KubeDnsSearchPath $kubeDnsSearchPath `
                -KubeClusterCIDR $kubeClusterCIDR `
                -KubeServiceCIDR $kubeServiceCIDR `
                -VNetCIDR $vNetCIDR `
                -IsDualStackEnabled $isDualStackEnabled

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.EnableWinDSR.conflist"))
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }
    }

    Context 'WinDSR is disabled' {
        It "Should reserve ROUTE and no WinDSR setting" {
            $global:KubeproxyFeatureGates = @("")
            Set-Default-AzureCNI "AzureCNI.Default.conflist"

            Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                -KubeDnsSearchPath $kubeDnsSearchPath `
                -KubeClusterCIDR $kubeClusterCIDR `
                -KubeServiceCIDR $kubeServiceCIDR `
                -VNetCIDR $vNetCIDR `
                -IsDualStackEnabled $isDualStackEnabled

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.DisableWinDSR.conflist"))
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }
    }

    Context 'DisableOutboundNAT' {
        It "Should replace OutboundNAT with LoopbackDSR" {
            $global:IsDisableWindowsOutboundNat = $true
            Set-Default-AzureCNI "AzureCNI.Default.conflist"

            Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                -KubeDnsSearchPath $kubeDnsSearchPath `
                -KubeClusterCIDR $kubeClusterCIDR `
                -KubeServiceCIDR $kubeServiceCIDR `
                -VNetCIDR $vNetCIDR `
                -IsDualStackEnabled $isDualStackEnabled

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.DisableOutboundNat.conflist"))
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }
    }

    Context 'AzureCNIOverlay is enabled' {
        It "Should not include Cluster CIDR when AzureCNIOverlay is enabled" {
            Set-Default-AzureCNI "AzureCNI.Default.Overlay.conflist"

            Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                -KubeDnsSearchPath $kubeDnsSearchPath `
                -KubeClusterCIDR $kubeClusterCIDR `
                -KubeServiceCIDR $kubeServiceCIDR `
                -VNetCIDR $vNetCIDR `
                -IsDualStackEnabled $isDualStackEnabled `
                -IsAzureCNIOverlayEnabled $true

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.Overlay.conflist"))
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }
    }

    Context 'SwiftCNI' {
        It "Should has hnsTimeoutDurationInSeconds and enableLoopbackDSR" {
            Set-Default-AzureCNI  "AzureCNI.Default.Swift.conflist"

            Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                -KubeDnsSearchPath $kubeDnsSearchPath `
                -KubeClusterCIDR $kubeClusterCIDR `
                -KubeServiceCIDR $kubeServiceCIDR `
                -VNetCIDR $vNetCIDR `
                -IsDualStackEnabled $isDualStackEnabled

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.Swift.conflist"))
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }

        It "DisableOutboundNAT with SwiftCNI" {
            $global:IsDisableWindowsOutboundNat = $true
            Set-Default-AzureCNI  "AzureCNI.Default.Swift.conflist"

            Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                -KubeDnsSearchPath $kubeDnsSearchPath `
                -KubeClusterCIDR $kubeClusterCIDR `
                -KubeServiceCIDR $kubeServiceCIDR `
                -VNetCIDR $vNetCIDR `
                -IsDualStackEnabled $isDualStackEnabled

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.Swift.DisableOutboundNAT.conflist"))
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }
    }
}

Describe 'Get-HnsPsm1' {
    Context 'Docker' {
        It "Should copy hns.psm1 with the correct source file" {
            Mock Copy-Item {}
            Get-HnsPsm1 -HNSModule "C:\k\hns.psm1"
            Assert-MockCalled -CommandName "Copy-Item" -Exactly -Times 1 -ParameterFilter { $Path -eq 'C:\k\debug\hns.psm1' -and $Destination -eq "C:\k\hns.psm1" }
        }
    }

    Context 'Containerd' {
        It "Should copy hns.v2.psm1 with the correct source file" {
            Mock Copy-Item {}
            Get-HnsPsm1 -HNSModule "C:\k\hns.v2.psm1"
            Assert-MockCalled -CommandName "Copy-Item" -Exactly -Times 1 -ParameterFilter { $Path -eq 'C:\k\debug\hns.v2.psm1' -and $Destination -eq "C:\k\hns.v2.psm1" }
        }
    }
}
