BeforeAll {
    function Write-Log {}

    function New-FakeRegistryKey {
        $registryKey = [PSCustomObject]@{
            SubKeys = [System.Collections.ArrayList]::new()
            Values = [System.Collections.ArrayList]::new()
        }
        $registryKey | Add-Member -MemberType ScriptMethod -Name CreateSubKey -Value {
            param($subKeyName)

            [void]$this.SubKeys.Add($subKeyName)
            $subKey = [PSCustomObject]@{
                Values = $this.Values
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
