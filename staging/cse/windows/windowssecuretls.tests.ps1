BeforeAll {
    function Write-Log {}
    . $PSScriptRoot\provisioningscripts\windowssecuretls.ps1
}

Describe "DisableRC4" {
    BeforeEach {
        Mock Set-CryptoSetting
    }

    It "independently disables every required RC4 cipher registry key" {
        DisableRC4

        $expectedPaths = @(
            "HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers\RC4 128/128",
            "HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers\RC4 64/128",
            "HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers\RC4 56/128",
            "HKLM:\SYSTEM\CurrentControlSet\Control\SecurityProviders\SCHANNEL\Ciphers\RC4 40/128"
        )
        foreach ($path in $expectedPaths) {
            Should -Invoke Set-CryptoSetting -Exactly -Times 1 -ParameterFilter {
                $regKeyName -eq $path -and
                $value -eq "Enabled" -and
                $valuedata -eq 0 -and
                $valuetype -eq "DWord"
            }
        }
        Should -Invoke Set-CryptoSetting -Exactly -Times 4
    }
}
