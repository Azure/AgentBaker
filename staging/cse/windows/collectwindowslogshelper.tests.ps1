BeforeAll {
  Import-Module "$PSScriptRoot\debug\collect-windows-logs-helper.psm1" -Force

  function New-TestHandleCommand {
    param(
      [Parameter(Mandatory = $true)]
      [string]$Path,
      [Parameter(Mandatory = $true)]
      [bool]$WriteOutput
    )

    if ($env:OS -eq "Windows_NT") {
      $commandPath = "$Path.cmd"
      $command = @(
        "@echo off",
        "if not ""%1""==""-accepteula"" exit /b 2",
        "if not ""%2""==""rsa-key-container"" exit /b 2"
      )
      if ($WriteOutput) {
        $command += "echo rsa-key-container handle"
      }
      $command += "exit /b 1"
      Set-Content -Path $commandPath -Value $command
      return $commandPath
    }

    $commandPath = $Path
    $command = @(
      "#!/bin/sh",
      "[ ""`$1"" = ""-accepteula"" ] || exit 2",
      "[ ""`$2"" = ""rsa-key-container"" ] || exit 2"
    )
    if ($WriteOutput) {
      $command += "echo rsa-key-container handle"
    }
    $command += "exit 1"
    Set-Content -Path $commandPath -Value $command
    chmod +x $commandPath
    return $commandPath
  }
}

Describe "Get-RSAKeyContainerHandleLog" {
  It "returns no output when Handle is unavailable" {
    $outputPath = Join-Path $TestDrive "missing-handle-output.txt"

    $result = Get-RSAKeyContainerHandleLog `
      -HandlePath (Join-Path $TestDrive "missing-handle") `
      -OutputPath $outputPath

    $result | Should -BeNullOrEmpty
    Test-Path $outputPath | Should -BeFalse
  }

  It "does not return or retain an empty output file" {
    $handlePath = New-TestHandleCommand -Path (Join-Path $TestDrive "empty-handle") -WriteOutput $false
    $outputPath = Join-Path $TestDrive "empty-output.txt"

    $result = Get-RSAKeyContainerHandleLog -HandlePath $handlePath -OutputPath $outputPath

    $result | Should -BeNullOrEmpty
    Test-Path $outputPath | Should -BeFalse
  }

  It "returns non-empty output even when Handle exits nonzero" {
    $handlePath = New-TestHandleCommand -Path (Join-Path $TestDrive "output-handle") -WriteOutput $true
    $outputPath = Join-Path $TestDrive "handle-output.txt"

    $result = Get-RSAKeyContainerHandleLog -HandlePath $handlePath -OutputPath $outputPath

    $result | Should -Be $outputPath
    (Get-Item $outputPath).Length | Should -BeGreaterThan 0
  }
}
