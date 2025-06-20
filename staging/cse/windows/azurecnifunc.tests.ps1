BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1','.ps1')

    $capturedContent = $null
    Mock Set-Content -MockWith { 
        param($Path, $Value)
        $script:capturedContent = $Value 
    } -Verifiable

    Mock Set-ItemProperty -MockWith {
        Param(
            $Path,
            $Name,
            $Type,
            $Value
        )
        Write-Host "Set-ItemProperty -Path $Path -Name $Name -Type $Type -Value $Value"
    } -Verifiable
}

Describe 'GetBroadestRangesForEachAddress' {

    It "Values '<Values>' should return '<Expected>'" -TestCases @(
        @{ Values = @('10.240.0.0/12', '10.0.0.0/8'); Expected = @('10.0.0.0/8', '10.240.0.0/12')}
        @{ Values = @('10.0.0.0/8', '10.0.0.0/16'); Expected = @('10.0.0.0/8')}
        @{ Values = @('10.0.0.0/16', '10.240.0.0/12', '10.0.0.0/8' ); Expected = @('10.0.0.0/8', '10.240.0.0/12')}
    ){
        param ($Values, $Expected)
        $actual = GetBroadestRangesForEachAddress -values $Values
        $actual | Should -BeIn $Expected
    }
}
    
Describe 'Set-AzureCNIConfig' {
    BeforeEach {
        $azureCNIConfDir = "$PSScriptRoot\azurecnifunc.tests.suites"
        $kubeDnsSearchPath = "svc.cluster.local"
        $kubeClusterCIDR = "10.224.0.0/12"
        $kubeServiceCIDR = "10.0.0.0/16"
        $vNetCIDR = "10.224.1.0/12"
        $isDualStackEnabled = $False
        $KubeDnsServiceIp = "10.0.0.10"
        $global:IsDisableWindowsOutboundNat = $false
        $global:IsIMDSRestrictionEnabled = $false
        $global:CiliumDataplaneEnabled = $false
        $global:KubeproxyFeatureGates = @("WinDSR=true")
        $azureCNIConfigFile = [Io.path]::Combine($azureCNIConfDir, "10-azure.conflist")

        # Set the default AzureCNI (mock the file download from https://acs-mirror.azureedge.net/azure-cni/.../10-azure.conflist)
        function Set-Default-AzureCNI ([string]$fileName) {
            $defaultFile = [Io.path]::Combine($azureCNIConfDir, $fileName)    
            Copy-Item -Path $defaultFile -Destination $azureCNIConfigFile
        }       

        function Read-Format-Json ([string]$JsonFile) {
            function Sort-ArraysInObject {
                param(
                    [Parameter(ValueFromPipeline = $true)]
                    $InputObject
                )
                
                process {
                    if ($null -eq $InputObject) {
                        return $null
                    }
                    # Handle arrays
                    elseif ($InputObject -is [System.Collections.IList]) {
                        $result = @()
                        # Process each item in the array recursively
                        foreach ($item in $InputObject) {
                            $result += (Sort-ArraysInObject -InputObject $item)
                        }
                        # Sort string arrays
                        if ($result.Count -gt 0 -and $result[0] -is [string]) {
                            return ($result | Sort-Object)
                        }
                        return $result
                    }
                    # Handle objects
                    elseif ($InputObject -is [PSCustomObject]) {
                        $result = [PSCustomObject]@{}
                        # Process each property
                        foreach ($prop in $InputObject.PSObject.Properties.Name) {
                            $result | Add-Member -MemberType NoteProperty -Name $prop -Value (Sort-ArraysInObject -InputObject $InputObject.$prop)
                        }
                        return $result
                    }
                    # Return primitives as is
                    else {
                        return $InputObject
                    }
                }
            }
            
            # Parse JSON and sort arrays
            $json = Get-Content $JsonFile | ConvertFrom-Json
            $sortedJson = Sort-ArraysInObject -InputObject $json
            
            # Convert back to JSON string
            $formattedJson = $sortedJson | ConvertTo-Json -depth 20
            return $formattedJson
        } 
        Mock Get-WindowsVersion -MockWith { return "ltsc2022" }
        Mock Restart-Service -MockWith { Write-Host "Restart-Service -Name $hnsServiceName" } -Verifiable
    }

    AfterEach {
        $azureCNIConfigFile = [Io.path]::Combine($azureCNIConfDir, "10-azure.conflist")
        if (Test-Path $azureCNIConfigFile) {
            Remove-Item -Path $azureCNIConfigFile
        }
    }

    Context 'Cilium (ebpf dataplane) is enabled' {
        It "Should use azure-cns as IPAM" {
            Set-Default-AzureCNI "AzureCNI.Default.conflist"

            $global:CiliumDataplaneEnabled = $true
            Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                -KubeDnsSearchPath $kubeDnsSearchPath `
                -KubeClusterCIDR $kubeClusterCIDR `
                -KubeServiceCIDR $kubeServiceCIDR `
                -VNetCIDR $vNetCIDR `
                -IsDualStackEnabled $isDualStackEnabled   

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.CiliumNodeSubnet.conflist"))
              
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }
    }

    Context 'WinDSR is enabled, ebpf dataplane disabled by default' {
        It "Should remove ROUTE and use azure-vnet-ipam for IPAM" {
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
            $diffence | Should -Be $null    }
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
                -IsDualStackEnabled $isDualStackEnabled `
                -IsAzureCNIOverlayEnabled $false

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.DisableWinDSR.conflist"))
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }
    }

    Context 'DisableOutboundNAT' {
        Context "WS2019 should replace OutboundNAT with LoopbackDSR and update regkey HNSControlFlag" {
            BeforeEach {
                Mock Get-WindowsVersion -MockWith { return "1809" }
                Mock Get-ItemProperty -MockWith {
					Param(
					  $Path,
					  $Name,
					  $ErrorAction
					)
					Write-Host "Get-ItemProperty -Path $Path -Name $Name : Return 0x50"
					return [PSCustomObject]@{
						HNSControlFlag = 0x50
					}
				} -Verifiable
            }

            It "Should clear 0x10 in HNSControlFlag when HNSControlFlag exists" {
			
                $global:IsDisableWindowsOutboundNat = $true
                Set-Default-AzureCNI "AzureCNI.Default.conflist"
    
                Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                    -KubeDnsSearchPath $kubeDnsSearchPath `
                    -KubeClusterCIDR $kubeClusterCIDR `
                    -KubeServiceCIDR $kubeServiceCIDR `
                    -VNetCIDR $vNetCIDR `
                    -IsDualStackEnabled $isDualStackEnabled
				Assert-MockCalled -CommandName "Get-ItemProperty" -Exactly -Times 1 -ParameterFilter { $Path -eq "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -and $Name -eq "HNSControlFlag" }
				Assert-MockCalled -CommandName "Set-ItemProperty" -Exactly -Times 1 -ParameterFilter { $Path -eq "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -and $Name -eq "HNSControlFlag" -and $Type -eq "DWORD" -and $Value -eq 0x40 }
    
                $actualConfigJson = Read-Format-Json $azureCNIConfigFile
                $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.DisableOutboundNat.conflist"))
                $diffence = Compare-Object $actualConfigJson $expectedConfigJson
                $diffence | Should -Be $null
            }

            It "Should set HNSControlFlag to 0 when HNSControlFlag does not exist" {
                Mock Get-ItemProperty -MockWith {
                    Param(
                        $Path,
                        $Name,
                        $ErrorAction
                    )
                    Write-Host "Get-ItemProperty -Path $Path -Name $Name : Return null"
                    return $null
                } -Verifiable

                $global:IsDisableWindowsOutboundNat = $true
                Set-Default-AzureCNI "AzureCNI.Default.conflist"
    
                Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                    -KubeDnsSearchPath $kubeDnsSearchPath `
                    -KubeClusterCIDR $kubeClusterCIDR `
                    -KubeServiceCIDR $kubeServiceCIDR `
                    -VNetCIDR $vNetCIDR `
                    -IsDualStackEnabled $isDualStackEnabled
				Assert-MockCalled -CommandName "Get-ItemProperty" -Exactly -Times 1 -ParameterFilter { $Path -eq "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -and $Name -eq "HNSControlFlag" -and $ErrorAction -eq "Ignore" }
				Assert-MockCalled -CommandName "Set-ItemProperty" -Exactly -Times 1 -ParameterFilter { $Path -eq "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -and $Name -eq "HNSControlFlag" -and $Type -eq "DWORD" -and $Value -eq 0 }
    
                $actualConfigJson = Read-Format-Json $azureCNIConfigFile
                $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.DisableOutboundNat.conflist"))
                $diffence = Compare-Object $actualConfigJson $expectedConfigJson
                $diffence | Should -Be $null
            }
        }

        Context "WS2022 should replace OutboundNAT with LoopbackDSR and update regkey SourcePortPreservationForHostPort" {
            BeforeEach {
                Mock Get-WindowsVersion -MockWith { return "ltsc2022" }
                
            }
            
            It "Should update SourcePortPreservationForHostPort to 0" {
                $global:IsDisableWindowsOutboundNat = $true
                Set-Default-AzureCNI "AzureCNI.Default.conflist"

                Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                    -KubeDnsSearchPath $kubeDnsSearchPath `
                    -KubeClusterCIDR $kubeClusterCIDR `
                    -KubeServiceCIDR $kubeServiceCIDR `
                    -VNetCIDR $vNetCIDR `
                    -IsDualStackEnabled $isDualStackEnabled

                Assert-MockCalled -CommandName "Set-ItemProperty" -Exactly -Times 1 -ParameterFilter { $Path -eq "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -and $Name -eq "SourcePortPreservationForHostPort" -and $Type -eq "DWORD" -and $Value -eq 0 }

                $actualConfigJson = Read-Format-Json $azureCNIConfigFile
                $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.DisableOutboundNat.conflist"))
                $diffence = Compare-Object $actualConfigJson $expectedConfigJson
                $diffence | Should -Be $null
            }
        }
    }

    Context 'AzureCNIOverlay is enabled' {
        BeforeEach {
            $hnsServiceName = "hns"
            Mock Get-Service -MockWith { return $hnsServiceName }
            Mock Restart-Service -MockWith { Write-Host "Restart-Service -Name $hnsServiceName" } -Verifiable
        }

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

        It "Should include cluster CIDRs and Vnet CIDRs included IPv6 in exceptionList" {
            Set-Default-AzureCNI "AzureCNI.Default.OverlayDualStack.conflist"

            $dualStackKubeClusterCIDR = "10.244.0.0/16,fd12:3456::/64"
            $dualStackvNetCIDR = "10.0.0.0/8,2001:abcd::/56"
            Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                -KubeDnsSearchPath $kubeDnsSearchPath `
                -KubeClusterCIDR $dualStackKubeClusterCIDR `
                -KubeServiceCIDR $kubeServiceCIDR `
                -VNetCIDR $dualStackvNetCIDR `
                -IsDualStackEnabled $true `
                -IsAzureCNIOverlayEnabled $true

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.OverlayDualStack.conflist"))
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }
    }

    Context 'SwiftCNI' {
        BeforeEach {
            $hnsServiceName = "hns"
            Mock Get-Service -MockWith { return $hnsServiceName }
            Mock Restart-Service -MockWith { Write-Host "Restart-Service -Name $hnsServiceName" } -Verifiable
        }

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

    Context 'IMDS restriction' {
        BeforeEach {
            $hnsServiceName = "hns"
            Mock Get-Service -MockWith { return $hnsServiceName }
            Mock Restart-Service -MockWith { Write-Host "Restart-Service -Name $hnsServiceName" } -Verifiable
        }
        It "should include IMDS restriction ACL rule when IMDS restriction is enabled" {
            Set-Default-AzureCNI "AzureCNI.Default.conflist"
            $global:IsIMDSRestrictionEnabled = $true
            Set-AzureCNIConfig -AzureCNIConfDir $azureCNIConfDir `
                -KubeDnsSearchPath $kubeDnsSearchPath `
                -KubeClusterCIDR $kubeClusterCIDR `
                -KubeServiceCIDR $kubeServiceCIDR `
                -VNetCIDR $vNetCIDR `
                -IsDualStackEnabled $isDualStackEnabled

            $actualConfigJson = Read-Format-Json $azureCNIConfigFile
            $expectedConfigJson = Read-Format-Json ([Io.path]::Combine($azureCNIConfDir, "AzureCNI.Expect.EnableIMDSRestriction.conflist"))
            $diffence = Compare-Object $actualConfigJson $expectedConfigJson
            $diffence | Should -Be $null
        }
    }
}

Describe 'Get-HnsPsm1' {
    Context 'Containerd' {
        It "Should copy hns.v2.psm1 with the correct source file" {
            Mock Copy-Item {}
            Get-HnsPsm1 -HNSModule "C:\k\hns.v2.psm1"
            Assert-MockCalled -CommandName "Copy-Item" -Exactly -Times 1 -ParameterFilter { $Path -eq 'C:\k\debug\hns.v2.psm1' -and $Destination -eq "C:\k\hns.v2.psm1" }
        }
    }
}
