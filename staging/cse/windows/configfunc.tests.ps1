BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1','.ps1')
}

Describe 'Adjust-DynamicPortRange' {
    BeforeEach {
        Mock Invoke-Executable
    }

    Context '$global:IsIncreaseDynamicPortRange is true' {
        It "Should call Invoke-Executable 4 times" {
            $global:IsIncreaseDynamicPortRange = $true

            Adjust-DynamicPortRange
            Assert-MockCalled -CommandName "Invoke-Executable" -Exactly -Times 4
        }
    }

    Context '$global:IsIncreaseDynamicPortRange is false' {
        It "Should call Invoke-Executable 1 times" {
            $global:IsIncreaseDynamicPortRange = $false

            Adjust-DynamicPortRange
            Assert-MockCalled -CommandName "Invoke-Executable" -Exactly -Times 1
        }
    }
}
