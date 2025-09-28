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

        # Set the default AzureCNI (mock the file download from https://packages.aks.azure.com/azure-cni/.../10-azure.conflist)
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
        Mock Restart-Service -MockWith { } -Verifiable
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
            Mock Restart-Service -MockWith { } -Verifiable
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
            Mock Restart-Service -MockWith { } -Verifiable
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
            Mock Restart-Service -MockWith { } -Verifiable
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

Describe 'GetIpv6AddressFromParsedContent' {
    Context 'When parsed content contains valid IPv6 address' {
        It "Should return the first IPv6 private address when available" {
            $parsedContent = @(
                @{
                    ipv6 = @{
                        ipAddress = @(
                            @{
                                privateIpAddress = "2001:db8::1"
                                publicIpAddress = "2001:db8:85a3::8a2e:370:7334"
                            },
                            @{
                                privateIpAddress = "2001:db8::2"
                            }
                        )
                    }
                }
            )
            
            $result = GetIpv6AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be "2001:db8::1"
        }
        
        It "Should return IPv6 address when only one address exists" {
            $parsedContent = @(
                @{
                    ipv6 = @{
                        ipAddress = @(
                            @{
                                privateIpAddress = "fe80::1"
                            }
                        )
                    }
                }
            )
            
            $result = GetIpv6AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be "fe80::1"
        }
    }
    
    Context 'When parsed content does not contain IPv6 address' {
        It "Should return null when ipv6 property is missing" {
            $parsedContent = @(
                @{
                    ipv4 = @{
                        ipAddress = @(
                            @{
                                privateIpAddress = "10.0.0.1"
                            }
                        )
                    }
                }
            )
            
            $result = GetIpv6AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should return null when ipv6.ipAddress property is missing" {
            $parsedContent = @(
                @{
                    ipv6 = @{
                        subnet = "2001:db8::/64"
                    }
                }
            )
            
            $result = GetIpv6AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should return null when ipv6.ipAddress array is empty" {
            $parsedContent = @(
                @{
                    ipv6 = @{
                        ipAddress = @()
                    }
                }
            )
            
            $result = GetIpv6AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should return null when ipv6 is null" {
            $parsedContent = @(
                @{
                    ipv6 = $null
                }
            )
            
            $result = GetIpv6AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should return null when ParsedContent is empty array" {
            $parsedContent = @()
            
            $result = GetIpv6AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
    }
    
    Context 'Edge cases' {
        It "Should return null when first element of ParsedContent is null" {
            $parsedContent = @($null)
            
            $result = GetIpv6AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should return null when ipv6.ipAddress[0] exists but privateIpAddress is missing" {
            $parsedContent = @(
                @{
                    ipv6 = @{
                        ipAddress = @(
                            @{
                                publicIpAddress = "2001:db8:85a3::8a2e:370:7334"
                            }
                        )
                    }
                }
            )
            
            $result = GetIpv6AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should handle multiple network interfaces but only check first one" {
            $parsedContent = @(
                @{
                    ipv6 = @{
                        ipAddress = @(
                            @{
                                privateIpAddress = "2001:db8::1"
                            }
                        )
                    }
                },
                @{
                    ipv6 = @{
                        ipAddress = @(
                            @{
                                privateIpAddress = "2001:db8::2"
                            }
                        )
                    }
                }
            )
            
            $result = GetIpv6AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be "2001:db8::1"
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

Describe 'GetIpv4AddressFromParsedContent' {
    Context 'When parsed content contains valid IPv4 address' {
        It "Should return the first IPv4 private address when available" {
            $parsedContent = @(
                @{
                    ipv4 = @{
                        ipAddress = @(
                            @{
                                privateIpAddress = "10.0.0.1"
                                publicIpAddress = "203.0.113.1"
                            },
                            @{
                                privateIpAddress = "10.0.0.2"
                            }
                        )
                    }
                }
            )
            
            $result = GetIpv4AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be "10.0.0.1"
        }
        
        It "Should return IPv4 address when only one address exists" {
            $parsedContent = @(
                @{
                    ipv4 = @{
                        ipAddress = @(
                            @{
                                privateIpAddress = "192.168.1.100"
                            }
                        )
                    }
                }
            )
            
            $result = GetIpv4AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be "192.168.1.100"
        }
    }
    
    Context 'When parsed content does not contain IPv4 address' {
        It "Should return null when ipv4 property is missing" {
            $parsedContent = @(
                @{
                    ipv6 = @{
                        ipAddress = @(
                            @{
                                privateIpAddress = "2001:db8::1"
                            }
                        )
                    }
                }
            )
            
            $result = GetIpv4AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should return null when ipv4.ipAddress property is missing" {
            $parsedContent = @(
                @{
                    ipv4 = @{
                        subnet = "10.0.0.0/24"
                    }
                }
            )
            
            $result = GetIpv4AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should return null when ipv4.ipAddress array is empty" {
            $parsedContent = @(
                @{
                    ipv4 = @{
                        ipAddress = @()
                    }
                }
            )
            
            $result = GetIpv4AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should return null when ipv4 is null" {
            $parsedContent = @(
                @{
                    ipv4 = $null
                }
            )
            
            $result = GetIpv4AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should return null when ParsedContent is empty array" {
            $parsedContent = @()
            
            $result = GetIpv4AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
    }
    
    Context 'Edge cases' {
        It "Should return null when first element of ParsedContent is null" {
            $parsedContent = @($null)
            
            $result = GetIpv4AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should return null when ipv4.ipAddress[0] exists but privateIpAddress is missing" {
            $parsedContent = @(
                @{
                    ipv4 = @{
                        ipAddress = @(
                            @{
                                publicIpAddress = "203.0.113.1"
                            }
                        )
                    }
                }
            )
            
            $result = GetIpv4AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be $null
        }
        
        It "Should handle multiple network interfaces but only check first one" {
            $parsedContent = @(
                @{
                    ipv4 = @{
                        ipAddress = @(
                            @{
                                privateIpAddress = "10.0.0.1"
                            }
                        )
                    }
                },
                @{
                    ipv4 = @{
                        ipAddress = @(
                            @{
                                privateIpAddress = "10.0.0.2"
                            }
                        )
                    }
                }
            )
            
            $result = GetIpv4AddressFromParsedContent -ParsedContent $parsedContent
            $result | Should -Be "10.0.0.1"
        }
        
        It "Should handle different private IP address formats" {
            $testCases = @(
                @{ IpAddress = "192.168.1.1"; Description = "Class C private IP" },
                @{ IpAddress = "172.16.0.1"; Description = "Class B private IP" },
                @{ IpAddress = "10.255.255.254"; Description = "Class A private IP" },
                @{ IpAddress = "169.254.1.1"; Description = "Link-local IP" }
            )
            
            foreach ($testCase in $testCases) {
                $parsedContent = @(
                    @{
                        ipv4 = @{
                            ipAddress = @(
                                @{
                                    privateIpAddress = $testCase.IpAddress
                                }
                            )
                        }
                    }
                )
                
                $result = GetIpv4AddressFromParsedContent -ParsedContent $parsedContent
                $result | Should -Be $testCase.IpAddress -Because $testCase.Description
            }
        }
    }
}

Describe 'GetMetadataContent' {
    BeforeEach {
        # Mock Start-Sleep to speed up tests
        Mock Start-Sleep -MockWith { } -Verifiable
    }
    
    Context 'Successful metadata retrieval' {
        It "Should return parsed content when metadata service responds with valid IPv4 address" {
            $mockMetadataResponse = @{
                Content = @'
[
    {
        "ipv4": {
            "ipAddress": [
                {
                    "privateIpAddress": "10.0.0.1",
                    "publicIpAddress": "203.0.113.1"
                }
            ]
        }
    }
]
'@
            }
            
            Mock Invoke-WebRequest -MockWith { return $mockMetadataResponse } -Verifiable
            Mock GetIpv4AddressFromParsedContent -MockWith { return "10.0.0.1" } -Verifiable
            
            $result = GetMetadataContent
            
            $result | Should -Not -Be $null
            $result[0].ipv4.ipAddress[0].privateIpAddress | Should -Be "10.0.0.1"
            
            Assert-MockCalled -CommandName "Invoke-WebRequest" -Exactly -Times 1 -ParameterFilter { 
                $Uri -eq "http://169.254.169.254/metadata/instance/network/interface?api-version=2021-02-01" -and
                $Headers["metadata"] -eq "true" -and
                $TimeoutSec -eq 10 -and
                $UseBasicParsing -eq $true
            }
            Assert-MockCalled -CommandName "GetIpv4AddressFromParsedContent" -Exactly -Times 1
        }
        
        It "Should return parsed content with both IPv4 and IPv6 addresses" {
            $mockMetadataResponse = @{
                Content = @'
[
    {
        "ipv4": {
            "ipAddress": [
                {
                    "privateIpAddress": "10.0.0.1"
                }
            ]
        },
        "ipv6": {
            "ipAddress": [
                {
                    "privateIpAddress": "2001:db8::1"
                }
            ]
        }
    }
]
'@
            }
            
            Mock Invoke-WebRequest -MockWith { return $mockMetadataResponse } -Verifiable
            Mock GetIpv4AddressFromParsedContent -MockWith { return "10.0.0.1" } -Verifiable
            
            $result = GetMetadataContent
            
            $result | Should -Not -Be $null
            $result[0].ipv4.ipAddress[0].privateIpAddress | Should -Be "10.0.0.1"
            $result[0].ipv6.ipAddress[0].privateIpAddress | Should -Be "2001:db8::1"
        }
    }
    
    Context 'Retry scenarios with eventual success' {
        It "Should retry when IPv4 address is not found initially but succeeds on second attempt" {
            $callCount = 0
            Mock Invoke-WebRequest -MockWith { 
                $script:callCount++
                return @{
                    Content = @'
[
    {
        "ipv4": {
            "ipAddress": [
                {
                    "privateIpAddress": "10.0.0.1"
                }
            ]
        }
    }
]
'@
                }
            } -Verifiable
            
            Mock GetIpv4AddressFromParsedContent -MockWith { 
                if ($script:callCount -eq 1) {
                    return $null  # First call fails
                } else {
                    return "10.0.0.1"  # Second call succeeds
                }
            } -Verifiable
            
            $result = GetMetadataContent
            
            $result | Should -Not -Be $null
            Assert-MockCalled -CommandName "Invoke-WebRequest" -Exactly -Times 2
            Assert-MockCalled -CommandName "GetIpv4AddressFromParsedContent" -Exactly -Times 2
            Assert-MockCalled -CommandName "Start-Sleep" -Exactly -Times 1
        }
        
        It "Should retry when Invoke-WebRequest throws exception but succeeds on retry" {
            $script:callCount = 0
            Mock Invoke-WebRequest -MockWith { 
                $script:callCount++
                if ($script:callCount -eq 1) {
                    throw "Connection timeout"
                } else {
                    return @{
                        Content = @'
[
    {
        "ipv4": {
            "ipAddress": [
                {
                    "privateIpAddress": "10.0.0.1"
                }
            ]
        }
    }
]
'@
                    }
                }
            } -Verifiable
            
            Mock GetIpv4AddressFromParsedContent -MockWith { return "10.0.0.1" } -Verifiable
            
            $result = GetMetadataContent
            
            $result | Should -Not -Be $null
            Assert-MockCalled -CommandName "Invoke-WebRequest" -Exactly -Times 2
            Assert-MockCalled -CommandName "Start-Sleep" -Exactly -Times 1
        }
    }
    
    Context 'Failure scenarios' {
        It "Should throw exception when all retries are exhausted due to no IPv4 address" {
            Mock Invoke-WebRequest -MockWith { 
                return @{
                    Content = @'
[
    {
        "ipv6": {
            "ipAddress": [
                {
                    "privateIpAddress": "2001:db8::1"
                }
            ]
        }
    }
]
'@
                }
            } -Verifiable
            
            Mock GetIpv4AddressFromParsedContent -MockWith { return $null } -Verifiable
            
            { GetMetadataContent } | Should -Throw "No IPv4 address found in metadata."
            
            # Should attempt all 120 retries
            Assert-MockCalled -CommandName "Invoke-WebRequest" -Exactly -Times 120
            Assert-MockCalled -CommandName "GetIpv4AddressFromParsedContent" -Exactly -Times 120
            Assert-MockCalled -CommandName "Start-Sleep" -Exactly -Times 119  # No sleep after last attempt
        }
        
        It "Should throw exception when all retries are exhausted due to network errors" {
            Mock Invoke-WebRequest -MockWith { 
                throw "Network unreachable"
            } -Verifiable
            
            { GetMetadataContent } | Should -Throw "No IPv4 address found in metadata."
            
            # Should attempt all 120 retries
            Assert-MockCalled -CommandName "Invoke-WebRequest" -Exactly -Times 120
            Assert-MockCalled -CommandName "Start-Sleep" -Exactly -Times 119  # No sleep after last attempt
        }
        
        It "Should handle ConvertFrom-Json errors gracefully" {
            Mock Invoke-WebRequest -MockWith { 
                return @{
                    Content = "invalid json content"
                }
            } -Verifiable
            
            { GetMetadataContent } | Should -Throw "No IPv4 address found in metadata."
            
            Assert-MockCalled -CommandName "Invoke-WebRequest" -Exactly -Times 120
        }
    }
    
    Context 'Logging behavior' {
            
        It "Should not sleep on the last retry attempt" {
            $script:callCount = 0
            Mock Invoke-WebRequest -MockWith { 
                $script:callCount++
                if ($script:callCount -lt 120) {
                    throw "Network error"
                } else {
                    return @{
                        Content = @'
[
    {
        "ipv4": {
            "ipAddress": [
                {
                    "privateIpAddress": "10.0.0.1"
                }
            ]
        }
    }
]
'@
                    }
                }
            } -Verifiable
            
            Mock GetIpv4AddressFromParsedContent -MockWith { return "10.0.0.1" } -Verifiable
            
            $result = GetMetadataContent
            
            $result | Should -Not -Be $null
            # Should sleep 119 times (not on the last successful attempt)
            Assert-MockCalled -CommandName "Start-Sleep" -Exactly -Times 119
        }
    }
    
    Context 'Edge cases' {
        It "Should handle empty metadata response" {
            Mock Invoke-WebRequest -MockWith { 
                return @{
                    Content = "[]"
                }
            } -Verifiable
            
            Mock GetIpv4AddressFromParsedContent -MockWith { return $null } -Verifiable
            
            { GetMetadataContent } | Should -Throw "No IPv4 address found in metadata."
        }
        
        It "Should succeed immediately if first attempt returns valid IPv4" {
            Mock Invoke-WebRequest -MockWith { 
                return @{
                    Content = @'
[
    {
        "ipv4": {
            "ipAddress": [
                {
                    "privateIpAddress": "10.0.0.1"
                }
            ]
        }
    }
]
'@
                }
            } -Verifiable
            
            Mock GetIpv4AddressFromParsedContent -MockWith { return "10.0.0.1" } -Verifiable
            
            $result = GetMetadataContent
            
            $result | Should -Not -Be $null
            Assert-MockCalled -CommandName "Invoke-WebRequest" -Exactly -Times 1
            Assert-MockCalled -CommandName "Start-Sleep" -Exactly -Times 0
        }
    }
}
