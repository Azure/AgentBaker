# Checks that PowerShell reads each value in the Windows CSE script as the plain string AgentBaker meant.
# pkg/agent/windows_cse_fields_test.go renders the script line of each text field with test values, including
# PowerShell code and non-ASCII characters, into pkg/agent/testdata/windowscse/fields.tsv
# (make generate-testdata). Each line is run in a new PowerShell process, as the CSE runs, and on Windows
# also in Windows PowerShell 5.1, which runs the CSE on nodes.

BeforeDiscovery {
    $fixturePath = [IO.Path]::GetFullPath([IO.Path]::Combine($PSScriptRoot, '..', '..', 'pkg', 'agent', 'testdata', 'windowscse', 'fields.tsv'))
    $cases = @(
        $index = 0
        foreach ($row in (Get-Content -Path $fixturePath | Where-Object { $_ })) {
            $name, $kind, $expected, $line = $row -split "`t", 4
            @{ Index = $index; Name = $name; Kind = $kind; Expected = $expected; Line = $line }
            $index++
        }
    )
    $shells = @('pwsh')
    if ($IsWindows -or $PSVersionTable.PSEdition -eq 'Desktop') {
        $shells += 'powershell'
    }
    $script:ShellCases = @(foreach ($shell in $shells) { @{ Shell = $shell; Cases = $cases } })
}

Describe 'Windows CSE field values' {
    Context 'in <Shell>' -ForEach $ShellCases {
        BeforeAll {
            # One script runs every case in its own scope and prints "<index>`t<type>`t<base64 of the value>".
            $script:Code = @(
                foreach ($case in $Cases) {
                    $select = if ($case.Kind -eq 'first') { '@($v)[0]' } else { '$v' }
                    "& {"
                    $case.Line
                    "`$v = Get-Variable -Name '$($case.Name)' -ValueOnly"
                    "`$v = $select"
                    "`$type = if (`$null -eq `$v) { 'null' } else { `$v.GetType().FullName }"
                    "'{0}`t{1}`t{2}' -f $($case.Index), `$type, [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([string]`$v))"
                    "}"
                }
                "'canary`t' + (Test-Path -Path 'variable:global:AKSInjectionCanary')"
            ) -join "`n"

            $directory = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
            New-Item -ItemType Directory -Path $directory | Out-Null
            try {
                $scriptPath = Join-Path $directory 'fields.ps1'
                # ASCII without a byte order mark, like CustomDataSetupScript.ps1.
                [IO.File]::WriteAllText($scriptPath, $script:Code, [Text.Encoding]::ASCII)
                $exe = if ($Shell -eq 'powershell') { 'powershell.exe' } else { [Diagnostics.Process]::GetCurrentProcess().MainModule.FileName }
                $arguments = @('-NoProfile', '-NonInteractive')
                if ($IsWindows -or $PSVersionTable.PSEdition -eq 'Desktop') {
                    $arguments += @('-ExecutionPolicy', 'Bypass')
                }
                $script:Output = @(& $exe @arguments -File $scriptPath 2>&1 | ForEach-Object { "$_" })
                $script:ExitCode = $LASTEXITCODE
            }
            finally {
                Remove-Item -Path $directory -Recurse -Force
            }

            $script:Results = @{}
            foreach ($outputLine in $script:Output) {
                $parts = $outputLine -split "`t"
                if ($parts.Count -ge 2) {
                    $script:Results[$parts[0]] = $parts[1..($parts.Count - 1)]
                }
            }
        }

        It 'runs without errors' {
            $script:ExitCode | Should -Be 0 -Because ($script:Output -join "`n")
        }

        It 'does not run any part of a value' {
            $script:Results['canary'] | Should -Be @('False')
        }

        It '<Name> #<Index> gets the value as a string' -ForEach $Cases {
            $result = $script:Results["$Index"]
            $result | Should -Not -BeNullOrEmpty -Because ($script:Output -join "`n")
            $result[0] | Should -Be 'System.String'
            $result[1] | Should -BeExactly $Expected
        }
    }
}
