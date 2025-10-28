BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1', '.ps1')

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

    function Invoke-WebRequest {
        return  @"
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
"@
    }

    Mock Write-Host -MockWith { } -Verifiable
    Mock Start-Sleep -MockWith { } -Verifiable
}

Describe 'GetBroadestRangesForEachAddress' {

    It "Values '<Values>' should return '<Expected>'" -TestCases @(
        @{ Values = @('10.240.0.0/12', '10.0.0.0/8'); Expected = @('10.0.0.0/8', '10.240.0.0/12') }
        @{ Values = @('10.0.0.0/8', '10.0.0.0/16'); Expected = @('10.0.0.0/8') }
        @{ Values = @('10.0.0.0/16', '10.240.0.0/12', '10.0.0.0/8' ); Expected = @('10.0.0.0/8', '10.240.0.0/12') }
    ) {
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
            $diffence | Should -Be $null }
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
                                publicIpAddress  = "2001:db8:85a3::8a2e:370:7334"
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
                                publicIpAddress  = "203.0.113.1"
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
                $result | Should -Be $testCase.IpAddress
            }
        }
    }
}

Describe 'Get-Node-Ipv4-Address' {
    It 'Retrieves the private ip address' {
        $content = @"
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
"@ | ConvertFrom-Json

        $result = GetIpv4AddressFromParsedContent -ParsedContent $content
        $result | Should -Be "10.0.0.1"
    }

    It 'Retrieves the private ip address even when Write-Log prints garbage to stdout' {
        function Write-Log {
            Write-Output "GARBAGE OUTPUT"
        }

        $content = @"
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
"@ | ConvertFrom-Json

        $result = GetIpv4AddressFromParsedContent -ParsedContent $content
        $result | Should -Be "10.0.0.1"
    }

}

Describe 'Get-Node-Ipv6-Address' {
    BeforeEach {
        # Mock dependencies
        Mock Set-ExitCode -MockWith {
            param($ExitCode, $ErrorMessage)
            throw $ErrorMessage
        } -Verifiable
    }

    Context 'Successful IPv6 address retrieval' {
        It "Should return IPv6 address when metadata content and parsing are successful" {
            $mockParsedContent = @"
            [
                {
                    "ipv6": {
                        "ipAddress": [
                            {
                                "privateIpAddress": "10.0.0.1",
                                "publicIpAddress": "203.0.113.1"
                            }
                        ]
                    }
                }
            ]
"@ | ConvertFrom-Json

            Mock GetMetadataContent -MockWith { return $mockParsedContent } -Verifiable

            $result = Get-Node-Ipv6-Address

            $result | Should -Be "10.0.0.1"

            Assert-MockCalled -CommandName "GetMetadataContent" -Exactly -Times 1
        }


        It "Should return IPv6 address when write-log dumps garbage to stdout" {
            function Write-Log {
                Write-Output "GARBAGE OUTPUT"
            }

            $mockParsedContent = @"
            [
                {
                    "ipv6": {
                        "ipAddress": [
                            {
                                "privateIpAddress": "10.0.0.1",
                                "publicIpAddress": "203.0.113.1"
                            }
                        ]
                    }
                }
            ]
"@ | ConvertFrom-Json

            Mock GetMetadataContent -MockWith { return $mockParsedContent } -Verifiable

            $result = Get-Node-Ipv6-Address

            $result | Should -Be "10.0.0.1"
        }
    }
}


Describe 'Get-Node-Ipv4-Address' {
    BeforeEach {
        # Mock dependencies
        Mock Set-ExitCode -MockWith {
            param($ExitCode, $ErrorMessage)
            throw $ErrorMessage
        } -Verifiable
    }

    Context 'Successful IPv4 address retrieval' {
        It "Should return IPv4 address when metadata content and parsing are successful" {
            $mockParsedContent = @"
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
"@ | ConvertFrom-Json

            Mock GetMetadataContent -MockWith { return $mockParsedContent } -Verifiable

            $result = Get-Node-Ipv4-Address

            $result | Should -Be "10.0.0.1"

            Assert-MockCalled -CommandName "GetMetadataContent" -Exactly -Times 1
        }


        It "Should return IPv4 address when write-log dumps garbage to stdout" {
            function Write-Log {
                Write-Output "GARBAGE OUTPUT"
            }

            $mockParsedContent = @"
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
"@ | ConvertFrom-Json

            Mock GetMetadataContent -MockWith { return $mockParsedContent } -Verifiable

            $result = Get-Node-Ipv4-Address

            $result | Should -Be "10.0.0.1"
        }


        It "Should handle different valid IPv4 addresses correctly" {
            $testCases = @(
                @{ IpAddress = "192.168.1.1"; Description = "Class C private IP" },
                @{ IpAddress = "172.16.0.1"; Description = "Class B private IP" },
                @{ IpAddress = "10.255.255.254"; Description = "Class A private IP" },
                @{ IpAddress = "169.254.1.1"; Description = "Link-local IP" }
            )

            foreach ($testCase in $testCases) {
                $mockParsedContent = @(
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

                Mock GetMetadataContent -MockWith { return $mockParsedContent } -Verifiable
                Mock GetIpv4AddressFromParsedContent -MockWith { return $testCase.IpAddress } -Verifiable

                $result = Get-Node-Ipv4-Address

                $result | Should -Be $testCase.IpAddress
            }
        }
    }

    Context 'Metadata content retrieval failures' {
        It "Should handle null metadata content and set exit code" {
            Mock GetMetadataContent -MockWith { return $null } -Verifiable

            { Get-Node-Ipv4-Address } | Should -Throw "Failed to load metadata content"

            Assert-MockCalled -CommandName "GetMetadataContent" -Exactly -Times 1
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter {
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_LOAD_METADATA -and
                $ErrorMessage -eq "Failed to load metadata content"
            }
        }

        It "Should handle empty metadata content and set exit code" {
            Mock GetMetadataContent -MockWith { return "[{}]" | ConvertFrom-Json } -Verifiable

            { Get-Node-Ipv4-Address } | Should -Throw "No IPv4 address found in metadata"

            Assert-MockCalled -CommandName "GetMetadataContent" -Exactly -Times 1
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter {
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_PARSE_METADATA -and
                $ErrorMessage -eq "No IPv4 address found in metadata"
            }
        }

        It "Should handle GetMetadataContent throwing exception" {
            Mock GetMetadataContent -MockWith { throw "Network timeout" } -Verifiable

            { Get-Node-Ipv4-Address } | Should -Throw "Network timeout"

            Assert-MockCalled -CommandName "GetMetadataContent" -Exactly -Times 1
        }
    }

    Context 'IPv4 address parsing failures' {
        It "Should handle null IPv4 address and set exit code" {
            $mockParsedContent = @"
            [
                {
                    "ipv6": {
                        "ipAddress": [
                            {
                                "privateIpAddress": "10.0.0.1",
                                "publicIpAddress": "203.0.113.1"
                            }
                        ]
                    }
                }
            ]
"@ | ConvertFrom-Json

            Mock GetMetadataContent -MockWith { return $mockParsedContent } -Verifiable

            { Get-Node-Ipv4-Address } | Should -Throw "No IPv4 address found in metadata"

            Assert-MockCalled -CommandName "GetMetadataContent" -Exactly -Times 1
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter {
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_PARSE_METADATA -and
                $ErrorMessage -eq "No IPv4 address found in metadata"
            }
        }

        It "Should handle empty string IPv4 address and set exit code" {
            $mockParsedContent = @"
            [
                {
                    "ipv4": {
                        "ipAddress": [
                            {
                                "privateIpAddress": ""
                            }
                        ]
                    }
                }
            ]
"@ | ConvertFrom-Json

            Mock GetMetadataContent -MockWith { return $mockParsedContent } -Verifiable

            { Get-Node-Ipv4-Address } | Should -Throw "No IPv4 address found in metadata"

            Assert-MockCalled -CommandName "GetMetadataContent" -Exactly -Times 1
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter {
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_PARSE_METADATA -and
                $ErrorMessage -eq "No IPv4 address found in metadata"
            }
        }

        It "Should handle whitespace-only IPv4 address and set exit code" {
            $mockParsedContent = @"
            [
                {
                    "ipv4": {
                        "ipAddress": [
                            {
                                "privateIpAddress": "   "
                            }
                        ]
                    }
                }
            ]
"@ | ConvertFrom-Json

            Mock GetMetadataContent -MockWith { return $mockParsedContent } -Verifiable

            { Get-Node-Ipv4-Address } | Should -Throw "empty IPv4 address found in metadata"

            Assert-MockCalled -CommandName "GetMetadataContent" -Exactly -Times 1
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter {
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_PARSE_METADATA -and
                $ErrorMessage -eq "empty IPv4 address found in metadata"
            }
        }
    }

    Context 'Error handling and exit codes' {
        It "Should call Set-ExitCode with WINDOWS_CSE_ERROR_LOAD_METADATA when metadata loading fails" {
            Mock GetMetadataContent -MockWith { return $null } -Verifiable

            { Get-Node-Ipv4-Address } | Should -Throw

            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter {
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_LOAD_METADATA -and
                $ErrorMessage -eq "Failed to load metadata content"
            }
        }

        It "Should call Set-ExitCode with WINDOWS_CSE_ERROR_PARSE_METADATA when IPv4 parsing fails" {
            $mockParsedContent = @(@{})

            Mock GetMetadataContent -MockWith { return $mockParsedContent } -Verifiable
            Mock GetIpv4AddressFromParsedContent -MockWith { return $null } -Verifiable

            { Get-Node-Ipv4-Address } | Should -Throw

            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter {
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_PARSE_METADATA -and
                $ErrorMessage -eq "No IPv4 address found in metadata"
            }
        }

        It "Should not call Set-ExitCode when function succeeds" {
            $mockParsedContent = @"
            [
                {
                    "ipv4": {
                        "ipAddress": [
                            {
                                "privateIpAddress": "10.240.0.4"
                            }
                        ]
                    }
                }
            ]
"@ | ConvertFrom-Json

            Mock GetMetadataContent -MockWith { return $mockParsedContent } -Verifiable

            $result = Get-Node-Ipv4-Address

            $result | Should -Be "10.240.0.4"
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 0
        }
    }

    Context 'Edge cases and boundary conditions' {
        It "Should handle complex metadata structure with multiple interfaces" {
            $complexParsedContent = @"
 [
{
  "ipv4": {
        "ipAddress": [
            {
                "privateIpAddress": "10.0.0.1",
                "publicIpAddress": "203.0.113.1"
            },
            {
                "privateIpAddress": "10.0.0.2"
            }
        ]
    },
    "ipv6": {
        "ipAddress": [
            {
                "privateIpAddress": "2001:db8::1"
            }
        ]
    },
    "macAddress": "00:11:22:33:44:55"
},
{
    "ipv4": {
        "ipAddress": [
            {
                "privateIpAddress": "192.168.1.1"
            }
        ]
    }
}
]
"@ | ConvertFrom-Json

            Mock GetMetadataContent -MockWith { return $complexParsedContent } -Verifiable

            $result = Get-Node-Ipv4-Address

            $result | Should -Be "10.0.0.1"
        }

        It "Should handle the scenario where GetIpv4AddressFromParsedContent throws an exception" {
            $mockParsedContent = @(@{})

            Mock GetMetadataContent -MockWith { return $mockParsedContent } -Verifiable
            Mock GetIpv4AddressFromParsedContent -MockWith { throw "Parsing error" } -Verifiable

            { Get-Node-Ipv4-Address } | Should -Throw "Parsing error"

            Assert-MockCalled -CommandName "GetMetadataContent" -Exactly -Times 1
            # Should not reach the logging or Set-ExitCode calls due to exception
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 0
        }
    }
}

Describe 'Get-AKS-NodeIPs' {
    BeforeEach {
        # Mock dependencies
        Mock Set-ExitCode -MockWith {
            param($ExitCode, $ErrorMessage)
            throw $ErrorMessage
        } -Verifiable

        # Reset global variable to default state
        $global:IsDualStackEnabled = $false
    }

    Context 'IPv4-only scenarios (dual stack disabled)' {
        It "Should return only IPv4 address when dual stack is disabled" {
            $global:IsDualStackEnabled = $false

            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return "2001:db8::1" } -Verifiable

            $result = Get-AKS-NodeIPs

            $result | Should -Be "10.0.0.1"

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-Node-Ipv6-Address" -Exactly -Times 0
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 0
        }

        It "Should handle different IPv4 address formats when dual stack is disabled" {
            $testCases = @(
                @{ IpAddress = "192.168.1.1"; Description = "Class C private IP" },
                @{ IpAddress = "172.16.0.1"; Description = "Class B private IP" },
                @{ IpAddress = "10.255.255.254"; Description = "Class A private IP" },
                @{ IpAddress = "169.254.1.1"; Description = "Link-local IP" }
            )

            foreach ($testCase in $testCases) {
                $global:IsDualStackEnabled = $false

                Mock Get-Node-Ipv4-Address -MockWith { return $testCase.IpAddress } -Verifiable

                $result = Get-AKS-NodeIPs

                $result | Should -Be $testCase.IpAddress
            }
        }

        It "Should propagate exceptions from Get-Node-Ipv4-Address when dual stack is disabled" {
            $global:IsDualStackEnabled = $false

            Mock Get-Node-Ipv4-Address -MockWith { throw "IPv4 retrieval failed" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return "asd:asdf:Asfd:asf" } -Verifiable

            { Get-AKS-NodeIPs } | Should -Throw "IPv4 retrieval failed"

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-Node-Ipv6-Address" -Exactly -Times 0
        }
    }

    Context 'Dual stack enabled scenarios with successful IPv6 retrieval' {
        It "Should return both IPv4 and IPv6 addresses when dual stack is enabled and IPv6 is available" {
            $global:IsDualStackEnabled = $true

            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return "2001:db8::1" } -Verifiable

            $result = Get-AKS-NodeIPs

            $result | Should -Be "10.0.0.1,2001:db8::1"

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-Node-Ipv6-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 0
        }

        It "Should handle various IPv4 and IPv6 address combinations" {
            $testCases = @(
                @{
                    IPv4        = "192.168.1.100";
                    IPv6        = "fe80::1";
                    Expected    = "192.168.1.100,fe80::1";
                    Description = "Standard private IPv4 with link-local IPv6"
                },
                @{
                    IPv4        = "172.16.0.50";
                    IPv6        = "2001:db8:85a3::8a2e:370:7334";
                    Expected    = "172.16.0.50,2001:db8:85a3::8a2e:370:7334";
                    Description = "Class B IPv4 with global unicast IPv6"
                },
                @{
                    IPv4        = "10.240.0.4";
                    IPv6        = "fd12:3456:789a::1";
                    Expected    = "10.240.0.4,fd12:3456:789a::1";
                    Description = "Class A IPv4 with unique local IPv6"
                }
            )

            foreach ($testCase in $testCases) {
                $global:IsDualStackEnabled = $true

                Mock Get-Node-Ipv4-Address -MockWith { return $testCase.IPv4 } -Verifiable
                Mock Get-Node-Ipv6-Address -MockWith { return $testCase.IPv6 } -Verifiable

                $result = Get-AKS-NodeIPs

                $result | Should -Be $testCase.Expected
            }
        }

        It "Should maintain correct order of IP addresses (IPv4 first, then IPv6)" {
            $global:IsDualStackEnabled = $true

            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return "2001:db8::1" } -Verifiable

            $result = Get-AKS-NodeIPs
            $addresses = $result -split ','

            $addresses.Count | Should -Be 2
            $addresses[0] | Should -Be "10.0.0.1"
            $addresses[1] | Should -Be "2001:db8::1"
        }
    }

    Context 'Dual stack enabled scenarios with IPv6 retrieval failures' {
        It "Should call Set-ExitCode when dual stack is enabled but IPv6 address is null" {
            $global:IsDualStackEnabled = $true

            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return $null } -Verifiable

            { Get-AKS-NodeIPs } | Should -Throw "Failed to get node IPv6 IP address"

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-Node-Ipv6-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter {
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_NETWORK_INTERFACES_NOT_EXIST -and
                $ErrorMessage -eq "Failed to get node IPv6 IP address"
            }
        }

        It "Should call Set-ExitCode when dual stack is enabled but IPv6 address is empty string" {
            $global:IsDualStackEnabled = $true

            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return "" } -Verifiable

            { Get-AKS-NodeIPs } | Should -Throw "Failed to get node IPv6 IP address"

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-Node-Ipv6-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter {
                $ExitCode -eq $global:WINDOWS_CSE_ERROR_NETWORK_INTERFACES_NOT_EXIST -and
                $ErrorMessage -eq "Failed to get node IPv6 IP address"
            }
        }

        It "Should propagate exceptions from Get-Node-Ipv6-Address when dual stack is enabled" {
            $global:IsDualStackEnabled = $true

            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { throw "IPv6 retrieval failed" } -Verifiable

            { Get-AKS-NodeIPs } | Should -Throw "IPv6 retrieval failed"

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-Node-Ipv6-Address" -Exactly -Times 1
            # Should not reach Set-ExitCode due to exception
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 0
        }
    }

    Context 'Error handling and exception propagation' {
        It "Should propagate IPv4 exceptions regardless of dual stack setting" {
            $global:IsDualStackEnabled = $true

            Mock Get-Node-Ipv4-Address -MockWith { throw "IPv4 network error" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return "2001:db8::1" } -Verifiable

            { Get-AKS-NodeIPs } | Should -Throw "IPv4 network error"

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            # Should not reach IPv6 call due to IPv4 exception
            Assert-MockCalled -CommandName "Get-Node-Ipv6-Address" -Exactly -Times 0
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 0
        }

        It "Should handle the case where IsDualStackEnabled variable doesn't exist" {
            # Remove the global variable to simulate it not being set
            if (Get-Variable -Name "IsDualStackEnabled" -Scope Global -ErrorAction SilentlyContinue) {
                Remove-Variable -Name "IsDualStackEnabled" -Scope Global
            }

            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return "2001:db8::1" } -Verifiable

            $result = Get-AKS-NodeIPs

            # Should behave as if dual stack is disabled when variable doesn't exist
            $result | Should -Be "10.0.0.1"

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-Node-Ipv6-Address" -Exactly -Times 0
        }
    }

    Context 'Return value formatting' {
        It "Should return comma-separated string with no spaces around comma" {
            $global:IsDualStackEnabled = $true

            Mock Get-Node-Ipv4-Address -MockWith { return "192.168.1.100" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return "fe80::1234:5678:90ab:cdef" } -Verifiable

            $result = Get-AKS-NodeIPs

            $result | Should -Be "192.168.1.100,fe80::1234:5678:90ab:cdef"
            $result | Should -Not -Match "\s"  # Should not contain any whitespace
            $result | Should -Match "^[^,]+,[^,]+$"  # Should match pattern: text,text
        }

        It "Should return single IP address without comma when dual stack is disabled" {
            $global:IsDualStackEnabled = $false

            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable

            $result = Get-AKS-NodeIPs

            $result | Should -Be "10.0.0.1"
            $result | Should -Not -Match ","  # Should not contain comma
        }

        It "Should handle special characters in IP addresses correctly" {
            $global:IsDualStackEnabled = $true

            # Using IPv6 address with various characters that might need special handling
            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return "2001:db8:85a3::8a2e:370:7334" } -Verifiable

            $result = Get-AKS-NodeIPs

            $result | Should -Be "10.0.0.1,2001:db8:85a3::8a2e:370:7334"

            # Verify we can split it back correctly
            $splitResult = $result -split ','
            $splitResult.Count | Should -Be 2
            $splitResult[0] | Should -Be "10.0.0.1"
            $splitResult[1] | Should -Be "2001:db8:85a3::8a2e:370:7334"
        }
    }

    Context 'Edge cases and boundary conditions' {
        It "Should handle very long IPv6 addresses" {
            $global:IsDualStackEnabled = $true

            $longIPv6 = "2001:0db8:85a3:0000:0000:8a2e:0370:7334"  # Full form IPv6

            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return $longIPv6 } -Verifiable

            $result = Get-AKS-NodeIPs

            $result | Should -Be "10.0.0.1,$longIPv6"
        }

        It "Should handle minimal valid IPv6 addresses" {
            $global:IsDualStackEnabled = $true

            $minimalIPv6 = "::1"  # Loopback IPv6

            Mock Get-Node-Ipv4-Address -MockWith { return "127.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return $minimalIPv6 } -Verifiable

            $result = Get-AKS-NodeIPs

            $result | Should -Be "127.0.0.1,$minimalIPv6"
        }

        It "Should handle the case where IPv6 returns exactly empty string (not null)" {
            $global:IsDualStackEnabled = $true

            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return [string]::Empty } -Verifiable

            { Get-AKS-NodeIPs } | Should -Throw "Failed to get node IPv6 IP address"

            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1
        }

        It "Should handle boolean evaluation of IPv6 address correctly" {
            $global:IsDualStackEnabled = $true

            # Test with IPv6 address that might be tricky for boolean evaluation
            Mock Get-Node-Ipv4-Address -MockWith { return "10.0.0.1" } -Verifiable
            Mock Get-Node-Ipv6-Address -MockWith { return "0:0:0:0:0:0:0:1" } -Verifiable  # Another form of ::1

            $result = Get-AKS-NodeIPs

            $result | Should -Be "10.0.0.1,0:0:0:0:0:0:0:1"
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 0
        }
    }
}

Describe 'Get-AKS-NetworkAdaptor' {
    BeforeEach {
        # Mock dependencies
        Mock Set-ExitCode -MockWith {
            param($ExitCode, $ErrorMessage)
            throw $ErrorMessage
        } -Verifiable
    }

    Context 'Successful network adapter retrieval' {
        It "Should return network adapter when IP address and adapter are found successfully" {
            $mockIPv4Address = "10.0.0.1"
            $mockNetIP = [PSCustomObject]@{
                ifIndex   = 5
                IPAddress = $mockIPv4Address
            }
            $mockNetAdapter = [PSCustomObject]@{
                Name    = "Ethernet"
                ifIndex = 5
                Status  = "Up"
            }

            Mock Get-Node-Ipv4-Address -MockWith { return $mockIPv4Address } -Verifiable
            Mock Get-NetIPAddress -MockWith { return $mockNetIP } -Verifiable
            Mock Get-NetAdapter -MockWith { return $mockNetAdapter } -Verifiable
            Mock Get-NetworkAdaptor-Fallback -MockWith { return $null } -Verifiable

            $result = Get-AKS-NetworkAdaptor

            $result | Should -Be $mockNetAdapter
            $result.Name | Should -Be "Ethernet"
            $result.ifIndex | Should -Be 5

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-NetIPAddress" -Exactly -Times 1 -ParameterFilter {
                $AddressFamily -eq "IPv4" -and
                $ErrorAction -eq "Stop" -and
                $IpAddress -eq $mockIPv4Address
            }
            Assert-MockCalled -CommandName "Get-NetAdapter" -Exactly -Times 1 -ParameterFilter {
                $ifindex -eq 5
            }
            Assert-MockCalled -CommandName "Get-NetworkAdaptor-Fallback" -Exactly -Times 0
        }

        It "Should handle different IPv4 addresses and interface indexes correctly" {
            $testCases = @(
                @{
                    IPv4        = "192.168.1.100";
                    IfIndex     = 3;
                    AdapterName = "Wi-Fi";
                    Description = "Wi-Fi adapter with Class C IP"
                },
                @{
                    IPv4        = "172.16.0.50";
                    IfIndex     = 12;
                    AdapterName = "Ethernet 2";
                    Description = "Ethernet adapter with Class B IP"
                },
                @{
                    IPv4        = "10.240.0.4";
                    IfIndex     = 7;
                    AdapterName = "vEthernet";
                    Description = "Virtual Ethernet with Class A IP"
                }
            )

            foreach ($testCase in $testCases) {
                $mockNetIP = [PSCustomObject]@{
                    ifIndex   = $testCase.IfIndex
                    IPAddress = $testCase.IPv4
                }
                $mockNetAdapter = [PSCustomObject]@{
                    Name    = $testCase.AdapterName
                    ifIndex = $testCase.IfIndex
                    Status  = "Up"
                }

                Mock Get-Node-Ipv4-Address -MockWith { return $testCase.IPv4 } -Verifiable
                Mock Get-NetIPAddress -MockWith { return $mockNetIP } -Verifiable
                Mock Get-NetAdapter -MockWith { return $mockNetAdapter } -Verifiable

                $result = Get-AKS-NetworkAdaptor

                $result.Name | Should -Be $testCase.AdapterName
                $result.ifIndex | Should -Be $testCase.IfIndex

                Assert-MockCalled -CommandName "Get-NetIPAddress" -ParameterFilter {
                    $IpAddress -eq $testCase.IPv4
                }
                Assert-MockCalled -CommandName "Get-NetAdapter" -ParameterFilter {
                    $ifindex -eq $testCase.IfIndex
                }
            }
        }

        It "Should log the IPv4 address correctly" {
            $testIPv4Address = "10.240.0.100"
            $mockNetIP = [PSCustomObject]@{ ifIndex = 8; IPAddress = $testIPv4Address }
            $mockNetAdapter = [PSCustomObject]@{ Name = "TestAdapter"; ifIndex = 8 }

            Mock Get-Node-Ipv4-Address -MockWith { return $testIPv4Address } -Verifiable
            Mock Get-NetIPAddress -MockWith { return $mockNetIP } -Verifiable
            Mock Get-NetAdapter -MockWith { return $mockNetAdapter } -Verifiable
            Mock Logs-To-Event -MockWith { } -Verifiable

            Get-AKS-NetworkAdaptor

            Assert-MockCalled -CommandName "Logs-To-Event" -Exactly -Times 1 -ParameterFilter {
                $TaskName -eq "AKS.WindowsCSE.NewExternalHnsNetwork" -and
                $TaskMessage -eq "Found IPv4 address from metadata: $testIPv4Address"
            }
        }
    }

    Context 'Get-NetIPAddress failures that trigger fallback' {
        It "Should call fallback when Get-NetIPAddress returns null" {
            $mockIPv4Address = "10.0.0.1"
            $mockFallbackAdapter = [PSCustomObject]@{
                Name    = "FallbackAdapter"
                ifIndex = 10
                Status  = "Up"
            }

            Mock Get-Node-Ipv4-Address -MockWith { return $mockIPv4Address } -Verifiable
            Mock Get-NetIPAddress -MockWith { return $null } -Verifiable
            Mock Get-NetAdapter -MockWith { return $null } -Verifiable
            Mock Get-NetworkAdaptor-Fallback -MockWith { return $mockFallbackAdapter } -Verifiable

            $result = Get-AKS-NetworkAdaptor

            $result | Should -Be $mockFallbackAdapter
            $result.Name | Should -Be "FallbackAdapter"

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-NetIPAddress" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-NetAdapter" -Exactly -Times 0
            Assert-MockCalled -CommandName "Get-NetworkAdaptor-Fallback" -Exactly -Times 1
        }

        It "Should log error and call fallback when Get-NetIPAddress fails" {
            $mockIPv4Address = "192.168.1.50"
            $mockFallbackAdapter = [PSCustomObject]@{
                Name    = "FallbackEthernet"
                ifIndex = 15
            }

            Mock Get-Node-Ipv4-Address -MockWith { return $mockIPv4Address } -Verifiable
            Mock Get-NetIPAddress -MockWith {
                throw @("Network interface not found")
            } -Verifiable
            Mock Get-NetworkAdaptor-Fallback -MockWith { return $mockFallbackAdapter } -Verifiable
            Mock Logs-To-Event -MockWith { } -Verifiable

            $result = Get-AKS-NetworkAdaptor

            $result | Should -Be $mockFallbackAdapter

            Assert-MockCalled -CommandName "Logs-To-Event" -ParameterFilter {
                $TaskName -eq "AKS.WindowsCSE.NewExternalHnsNetwork" -and
                $TaskMessage -like "*Failed to find IP address info for ip address $mockIPv4Address*"
            }
            Assert-MockCalled -CommandName "Get-NetworkAdaptor-Fallback" -Exactly -Times 1
        }

        It "Should handle different error conditions from Get-NetIPAddress" {
            $testErrorCases = @(
                @{
                    IPv4     = "10.0.0.50";
                    ErrorVar = @("Interface disabled");
                },
                @{
                    IPv4     = "172.16.1.100";
                    ErrorVar = @("Access denied");
                },
                @{
                    IPv4     = "192.168.10.20";
                    ErrorVar = @("Network unreachable");
                }
            )

            foreach ($testCase in $testErrorCases) {
                $mockFallbackAdapter = [PSCustomObject]@{ Name = "TestFallback"; ifIndex = 99 }

                Mock Get-Node-Ipv4-Address -MockWith { return $testCase.IPv4 } -Verifiable
                Mock Get-NetIPAddress -MockWith {
                    $global:netIPErr = $testCase.ErrorVar
                    return $null
                } -Verifiable
                Mock Get-NetworkAdaptor-Fallback -MockWith { return $mockFallbackAdapter } -Verifiable

                $result = Get-AKS-NetworkAdaptor

                $result | Should -Be $mockFallbackAdapter
            }
        }
    }

    Context 'Get-NetAdapter failures that trigger fallback' {
        It "Should call fallback when Get-NetAdapter returns null" {
            $mockIPv4Address = "10.0.0.1"
            $mockNetIP = [PSCustomObject]@{ ifIndex = 5; IPAddress = $mockIPv4Address }
            $mockFallbackAdapter = [PSCustomObject]@{
                Name    = "FallbackFromAdapter"
                ifIndex = 20
            }

            Mock Get-Node-Ipv4-Address -MockWith { return $mockIPv4Address } -Verifiable
            Mock Get-NetIPAddress -MockWith { return $mockNetIP } -Verifiable
            Mock Get-NetAdapter -MockWith { return $null } -Verifiable
            Mock Get-NetworkAdaptor-Fallback -MockWith { return $mockFallbackAdapter } -Verifiable

            $result = Get-AKS-NetworkAdaptor

            $result | Should -Be $mockFallbackAdapter
            $result.Name | Should -Be "FallbackFromAdapter"

            Assert-MockCalled -CommandName "Get-NetIPAddress" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-NetAdapter" -Exactly -Times 1 -ParameterFilter {
                $ifindex -eq 5
            }
            Assert-MockCalled -CommandName "Get-NetworkAdaptor-Fallback" -Exactly -Times 1
        }

        It "Should handle different interface indexes when Get-NetAdapter fails" {
            $testCases = @(
                @{ IPv4 = "10.1.1.1"; IfIndex = 3; Description = "Low interface index" },
                @{ IPv4 = "172.16.0.1"; IfIndex = 25; Description = "High interface index" },
                @{ IPv4 = "192.168.0.1"; IfIndex = 1; Description = "Minimum interface index" }
            )

            foreach ($testCase in $testCases) {
                $mockNetIP = [PSCustomObject]@{
                    ifIndex   = $testCase.IfIndex;
                    IPAddress = $testCase.IPv4
                }
                $mockFallbackAdapter = [PSCustomObject]@{ Name = "TestFallback"; ifIndex = 99 }

                Mock Get-Node-Ipv4-Address -MockWith { return $testCase.IPv4 } -Verifiable
                Mock Get-NetIPAddress -MockWith { return $mockNetIP } -Verifiable
                Mock Get-NetAdapter -MockWith { return $null } -Verifiable
                Mock Get-NetworkAdaptor-Fallback -MockWith { return $mockFallbackAdapter } -Verifiable

                $result = Get-AKS-NetworkAdaptor

                $result | Should -Be $mockFallbackAdapter

                Assert-MockCalled -CommandName "Get-NetAdapter" -ParameterFilter {
                    $ifindex -eq $testCase.IfIndex
                }
            }
        }
    }

    Context 'Exception handling and error propagation' {
        It "Should propagate exceptions from Get-Node-Ipv4-Address" {
            Mock Get-Node-Ipv4-Address -MockWith { throw "IPv4 address retrieval failed" } -Verifiable
            Mock Get-NetIPAddress -MockWith { return $null } -Verifiable
            Mock Get-NetAdapter -MockWith { return $null } -Verifiable
            Mock Get-NetworkAdaptor-Fallback -MockWith { return $null } -Verifiable
            Mock Start-Sleep -MockWith { } -Verifiable

            { Get-AKS-NetworkAdaptor } | Should -Throw "IPv4 address retrieval failed"

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            # Should not reach other calls due to exception
            Assert-MockCalled -CommandName "Get-NetIPAddress" -Exactly -Times 0
            Assert-MockCalled -CommandName "Get-NetAdapter" -Exactly -Times 0
            Assert-MockCalled -CommandName "Get-NetworkAdaptor-Fallback" -Exactly -Times 0
        }

        It "Should call fallback when Get-NetIPAddress throws errors" {
            $mockIPv4Address = "10.0.0.1"
            $mockFallbackAdapter = [PSCustomObject]@{ Name = "TestFallback"; ifIndex = 99 }

            Mock Get-Node-Ipv4-Address -MockWith { return $mockIPv4Address } -Verifiable
            Mock Get-NetIPAddress -MockWith { throw "Network interface query failed" } -Verifiable
            Mock Get-NetAdapter -MockWith { return $null } -Verifiable

            Mock Get-NetworkAdaptor-Fallback -MockWith { return $mockFallbackAdapter } -Verifiable
            Mock Start-Sleep -MockWith { } -Verifiable

            Get-AKS-NetworkAdaptor  | Should -Be $mockFallbackAdapter

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-NetIPAddress" -Exactly -Times 5
            # Should not reach these calls due to exception
            Assert-MockCalled -CommandName "Get-NetAdapter" -Exactly -Times 0
            Assert-MockCalled -CommandName "Get-NetworkAdaptor-Fallback" -Exactly -Times 1
        }

        It "Should retry when get-network-adapter fails and eventually call the fallback" {
            $mockIPv4Address = "10.0.0.1"
            $mockNetIP = [PSCustomObject]@{ ifIndex = 5; IPAddress = $mockIPv4Address }
            $mockFallback = [PSCustomObject]@{
                Name                 = "Complex Ethernet Adapter"
                ifIndex              = 12
                Status               = "Up"
                LinkSpeed            = "1000000000"
                FullDuplex           = $true
                MacAddress           = "00-11-22-33-44-55"
                InterfaceDescription = "Intel(R) 82574L Gigabit Network Connection"
            }

            Mock Start-Sleep -MockWith { } -Verifiable
            Mock Get-Node-Ipv4-Address -MockWith { return $mockIPv4Address } -Verifiable
            Mock Get-NetIPAddress -MockWith { return $mockNetIP } -Verifiable
            Mock Get-NetAdapter -MockWith { throw "Network adapter query failed" } -Verifiable
            Mock Get-NetworkAdaptor-Fallback -MockWith { return $mockFallback } -Verifiable

            $nas = Get-AKS-NetworkAdaptor

            $nas | Should -Be $mockFallback
            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-NetIPAddress" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-NetAdapter" -Exactly -Times 5
            Assert-MockCalled -CommandName "Get-NetworkAdaptor-Fallback" -Exactly -Times 1
        }

        It "Should propagate exceptions from Get-NetworkAdaptor-Fallback" {
            $mockIPv4Address = "10.0.0.1"

            Mock Get-Node-Ipv4-Address -MockWith { return $mockIPv4Address } -Verifiable
            Mock Get-NetIPAddress -MockWith { return $null } -Verifiable
            Mock Get-NetworkAdaptor-Fallback -MockWith { throw "Fallback method failed" } -Verifiable
            Mock Start-Sleep -MockWith { } -Verifiable

            { Get-AKS-NetworkAdaptor } | Should -Throw "Fallback method failed"

            Assert-MockCalled -CommandName "Get-Node-Ipv4-Address" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-NetIPAddress" -Exactly -Times 1
            Assert-MockCalled -CommandName "Get-NetworkAdaptor-Fallback" -Exactly -Times 1
        }
    }

    Context 'Logging behavior verification' {
        It "Should log error when IP address lookup fails" {
            $mockIPv4Address = "10.5.5.5"
            $mockFallbackAdapter = [PSCustomObject]@{ Name = "FallbackAdapterForIPError" }

            Mock Get-Node-Ipv4-Address -MockWith { return $mockIPv4Address } -Verifiable
            Mock Get-NetIPAddress -MockWith {
                throw @("Interface not found")
            } -Verifiable
            Mock Get-NetworkAdaptor-Fallback -MockWith { return $mockFallbackAdapter } -Verifiable
            Mock Logs-To-Event -MockWith { } -Verifiable

            Get-AKS-NetworkAdaptor | Should -Be $mockFallbackAdapter

            Assert-MockCalled -CommandName "Logs-To-Event" -ParameterFilter {
                $TaskName -eq "AKS.WindowsCSE.NewExternalHnsNetwork" -and
                $TaskMessage -like "*Failed to find IP address info for ip address ${mockIPv4Address}*"
            }
        }

        It "Should log error when network adapter lookup fails" {
            $mockIPv4Address = "192.168.50.100"
            $mockNetIP = [PSCustomObject]@{ ifIndex = 8; IPAddress = $mockIPv4Address }

            Mock Get-Node-Ipv4-Address -MockWith { return $mockIPv4Address } -Verifiable
            Mock Get-NetIPAddress -MockWith { return $mockNetIP } -Verifiable
            Mock Get-NetAdapter -MockWith { return $null } -Verifiable
            Mock Get-NetworkAdaptor-Fallback -MockWith { return [PSCustomObject]@{ Name = "Fallback" } } -Verifiable
            Mock Logs-To-Event -MockWith { } -Verifiable

            Get-AKS-NetworkAdaptor

            Assert-MockCalled -CommandName "Logs-To-Event" -ParameterFilter {
                $TaskName -eq "AKS.WindowsCSE.NewExternalHnsNetwork" -and
                $TaskMessage -like "*Failed to find network adapter info for ip address index 8 and ip address $mockIPv4Address*Reverting to old way to configure network*"
            }
        }
    }

    Context 'Edge cases and boundary conditions' {
        It "Should handle network adapter with complex properties" {
            $mockIPv4Address = "10.0.0.1"
            $mockNetIP = [PSCustomObject]@{
                ifIndex       = 12
                IPAddress     = $mockIPv4Address
                PrefixLength  = 24
                AddressFamily = "IPv4"
            }
            $mockComplexAdapter = [PSCustomObject]@{
                Name                 = "Complex Ethernet Adapter"
                ifIndex              = 12
                Status               = "Up"
                LinkSpeed            = "1000000000"
                FullDuplex           = $true
                MacAddress           = "00-11-22-33-44-55"
                InterfaceDescription = "Intel(R) 82574L Gigabit Network Connection"
            }

            Mock Get-Node-Ipv4-Address -MockWith { return $mockIPv4Address } -Verifiable
            Mock Get-NetIPAddress -MockWith { return $mockNetIP } -Verifiable
            Mock Get-NetAdapter -MockWith { return $mockComplexAdapter } -Verifiable

            $result = Get-AKS-NetworkAdaptor

            $result | Should -Be $mockComplexAdapter
            $result.Name | Should -Be "Complex Ethernet Adapter"
            $result.ifIndex | Should -Be 12
            $result.Status | Should -Be "Up"
            $result.MacAddress | Should -Be "00-11-22-33-44-55"
        }

        It "Should handle interface index edge values" {
            $edgeCases = @(
                @{ IfIndex = 1; Description = "Minimum interface index" },
                @{ IfIndex = 0; Description = "Zero interface index" },
                @{ IfIndex = 2147483647; Description = "Maximum integer interface index" }
            )

            foreach ($edgeCase in $edgeCases) {
                $mockIPv4Address = "10.0.0.1"
                $mockNetIP = [PSCustomObject]@{
                    ifIndex   = $edgeCase.IfIndex
                    IPAddress = $mockIPv4Address
                }
                $mockNetAdapter = [PSCustomObject]@{
                    Name    = "EdgeTestAdapter"
                    ifIndex = $edgeCase.IfIndex
                }

                Mock Get-Node-Ipv4-Address -MockWith { return $mockIPv4Address } -Verifiable
                Mock Get-NetIPAddress -MockWith { return $mockNetIP } -Verifiable
                Mock Get-NetAdapter -MockWith { return $mockNetAdapter } -Verifiable

                $result = Get-AKS-NetworkAdaptor

                $result.ifIndex | Should -Be $edgeCase.IfIndex
            }
        }

        It "Should handle empty or whitespace IPv4 addresses from Get-Node-Ipv4-Address" {
            $testCases = @(
                @{ IPv4 = ""; Description = "Empty string IPv4" },
                @{ IPv4 = "   "; Description = "Whitespace-only IPv4" },
                @{ IPv4 = "`t`n`r"; Description = "Tab and newline IPv4" }
            )

            foreach ($testCase in $testCases) {
                $mockFallbackAdapter = [PSCustomObject]@{ Name = "FallbackForEmptyIP" }

                Mock Get-Node-Ipv4-Address -MockWith { return $testCase.IPv4 } -Verifiable
                Mock Get-NetIPAddress -MockWith { return $null } -Verifiable
                Mock Get-NetworkAdaptor-Fallback -MockWith { return $mockFallbackAdapter } -Verifiable

                $result = Get-AKS-NetworkAdaptor

                $result | Should -Be $mockFallbackAdapter
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

            $result = GetMetadataContent

            $result | Should -Not -Be $null
            $result[0].ipv4.ipAddress[0].privateIpAddress | Should -Be "10.0.0.1"
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
                }
                else {
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
                }
                else {
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

            { GetMetadataContent } | Should -Throw "No IPv4 address found in metadata."

            # Should attempt all 120 retries
            Assert-MockCalled -CommandName "Invoke-WebRequest" -Exactly -Times 120
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
