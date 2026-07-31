BeforeAll {
    function Write-Log {}

    function New-FakeRegistryKey {
        param (
            [hashtable] $ExistingValues = @{}
        )

        $registryKey = [PSCustomObject]@{
            ExistingValues = $ExistingValues
            SubKeys = [System.Collections.ArrayList]::new()
            Values = [System.Collections.ArrayList]::new()
        }
        $registryKey | Add-Member -MemberType ScriptMethod -Name CreateSubKey -Value {
            param($subKeyName)

            [void]$this.SubKeys.Add($subKeyName)
            $subKey = [PSCustomObject]@{
                ExistingValues = $this.ExistingValues
                Values = $this.Values
            }
            $subKey | Add-Member -MemberType ScriptMethod -Name GetValue -Value {
                param($name, $defaultValue)

                if ($this.ExistingValues.ContainsKey($name)) {
                    return $this.ExistingValues[$name]
                }
                return $defaultValue
            }
            $subKey | Add-Member -MemberType ScriptMethod -Name SetValue -Value {
                param($name, $value, $kind)

                [void]$this.Values.Add([PSCustomObject]@{
                    Name = $name
                    Value = $value
                    Kind = $kind
                })
            }
            $subKey | Add-Member -MemberType ScriptMethod -Name Dispose -Value {}
            return $subKey
        }
        return $registryKey
    }

    . $PSScriptRoot\provisioningscripts\windowssecuretls.ps1
}

Describe "Set-CryptoSetting" {
    It "preserves literal subkey names from an HKLM provider path" {
        $registryKey = New-FakeRegistryKey

        Set-CryptoSetting `
            "HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers\RC4 64/128" `
            "Enabled" `
            0 `
            "DWord" `
            $registryKey

        $registryKey.SubKeys | Should -HaveCount 1
        $registryKey.SubKeys[0] | Should -Be "SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers\RC4 64/128"
        $registryKey.Values | Should -HaveCount 1
        $registryKey.Values[0].Name | Should -Be "Enabled"
        $registryKey.Values[0].Value | Should -Be 0
        $registryKey.Values[0].Kind | Should -Be ([Microsoft.Win32.RegistryValueKind]::DWord)
    }

    It "does not rewrite a registry value that already has the desired data" {
        $registryKey = New-FakeRegistryKey -ExistingValues @{ Enabled = 1 }

        Set-CryptoSetting `
            "HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Protocols\TLS 1.2\Client" `
            "Enabled" `
            1 `
            "DWord" `
            $registryKey

        $registryKey.Values | Should -HaveCount 0
    }
}

Describe "DisableRC4" {
    It "disables every required RC4 cipher using literal registry subkey names" {
        $registryKey = New-FakeRegistryKey

        DisableRC4 -RegistryKey $registryKey

        $expectedSubKeys = @(
            "SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers\RC4 128/128",
            "SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers\RC4 64/128",
            "SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers\RC4 56/128",
            "SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers\RC4 40/128"
        )
        $registryKey.SubKeys | Should -HaveCount 4
        foreach ($subKey in $expectedSubKeys) {
            $registryKey.SubKeys | Should -Contain $subKey
        }

        $registryKey.Values | Should -HaveCount 4
        foreach ($value in $registryKey.Values) {
            $value.Name | Should -Be "Enabled"
            $value.Value | Should -Be 0
            $value.Kind | Should -Be ([Microsoft.Win32.RegistryValueKind]::DWord)
        }
    }
}
