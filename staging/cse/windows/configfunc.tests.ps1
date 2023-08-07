BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1','.ps1')
}

Describe 'Adjust-DynamicPortRange' {
    BeforeEach {
        Mock Invoke-Executable
    }

    Context '$global:EnableIncreaseDynamicPortRange is true' {
        It "Should call Invoke-Executable 4 times" {
            $global:EnableIncreaseDynamicPortRange = $true

            Adjust-DynamicPortRange
            Assert-MockCalled -CommandName "Invoke-Executable" -Exactly -Times 4
        }
    }

    Context '$global:EnableIncreaseDynamicPortRange is false' {
        It "Should call Invoke-Executable 1 times" {
            $global:EnableIncreaseDynamicPortRange = $false

            Adjust-DynamicPortRange
            Assert-MockCalled -CommandName "Invoke-Executable" -Exactly -Times 1
        }
    }
}

Describe 'Resize-OSDrive' {
    BeforeAll{
        Mock Resize-Partition -MockWith {
            Param(
              $DriveLetter,
              $Size
            )
            Write-Host "Resize-Partition $DriveLetter $Size $ErrorAction"
        } -Verifiable

        Mock Get-Partition -MockWith {
            Param(
              $DriveLetter
            )
                Write-Host "Get-Partition $DriveLetter $ErrorAction"
                $valueObj = [PSCustomObject]@{
                    Size = 1024*1024
                }
                return $valueObj
        } -Verifiable

        Mock Set-ExitCode -MockWith {
            Param(
              $ExitCode,
              $ErrorMessage
            )
            Write-Host "Set-ExitCode $ExitCode $ErrorMessage"
        } -Verifiable
    }
    
    Context 'success' {
        It "Should call Resize-Partition once" {
            Mock Get-PartitionSupportedSize -MockWith {
                Param(
                  $DriveLetter
                )
                Write-Host "Get-PartitionSupportedSize $DriveLetter $ErrorAction"
                $valueObj = [PSCustomObject]@{
                    SizeMax = 4*1024*1024
                }
                return $valueObj
            } -Verifiable

            Resize-OSDrive
            Assert-MockCalled -CommandName "Resize-Partition" -Exactly -Times 1
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 0
        }

        It "Should not call Resize-Partition" {
            Mock Get-PartitionSupportedSize -MockWith {
                Param(
                  $DriveLetter
                )
                Write-Host "Get-PartitionSupportedSize $DriveLetter $ErrorAction"
                $valueObj = [PSCustomObject]@{
                    SizeMax = 1024*1024
                }
                return $valueObj
            } -Verifiable

            Resize-OSDrive
            Assert-MockCalled -CommandName "Resize-Partition" -Exactly -Times 0
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 0
        }
    }

    Context 'fail' {
        BeforeEach {
            Mock Get-Partition -MockWith {
                Param(
                  $DriveLetter
                )
                throw "Get-Partition $DriveLetter $ErrorAction"
            } -Verifiable
        }

        It "Should not call Resize-Partition" {
            Resize-OSDrive
            Assert-MockCalled -CommandName "Resize-Partition" -Exactly -Times 0
            Assert-MockCalled -CommandName "Set-ExitCode" -Exactly -Times 1 -ParameterFilter { $ExitCode -eq $global:WINDOWS_CSE_ERROR_RESIZE_OS_DRIVE }
        }
    }
}
