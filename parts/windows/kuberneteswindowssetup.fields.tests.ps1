# Checks the runtime value and type of every Windows CSE input, independently of its encoding.
# pkg/agent/windows_cse_fields_test.go generates fields.tsv with all text cases and constrained inputs.
# Each shell runs the rendered statements in separate scopes; on Windows this includes PowerShell 5.1.

BeforeDiscovery {
    $fixturePath = [IO.Path]::GetFullPath([IO.Path]::Combine($PSScriptRoot, '..', '..', 'pkg', 'agent', 'testdata', 'windowscse', 'fields.tsv'))
    $types = @{
        string = 'System.String'
        first = 'System.Object[]'
        array = 'System.Object[]'
        boolean = 'System.Boolean'
        uint32 = 'System.UInt32'
    }
    $cases = @(
        $index = 0
        foreach ($row in (Get-Content -Path $fixturePath | Where-Object { $_ })) {
            $name, $kind, $expected, $line = $row -split "`t", 4
            if (-not $types.ContainsKey($kind)) { throw "Unknown field kind: $kind" }
            @{ Index = $index; Name = $name; Kind = $kind; ExpectedType = $types[$kind]; Expected = $expected; Line = $line }
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
            # One script runs every case in its own scope and prints "<index>`t<type>`t<base64 of the value>",
            # or "<index>`terror`t<message>" when the line fails.
            $script:Code = @(
                "`$ErrorActionPreference = 'Stop'"
                foreach ($case in $Cases) {
                    "try { & {"
                    "if (Test-Path -Path 'variable:global:$($case.Name)') { Remove-Variable -Name '$($case.Name)' -Scope Global }"
                    $case.Line
                    "`$v = Get-Variable -Name '$($case.Name)' -ValueOnly -ErrorAction Stop"
                    "`$type = if (`$null -eq `$v) { 'null' } else { `$v.GetType().FullName }"
                    if ($case.Kind -in @('first', 'array')) {
                        "if (@(`$v | Where-Object { `$_ -isnot [string] }).Count -gt 0) { throw 'Expected string array elements' }"
                    }
                    if ($case.Kind -eq 'first') {
                        "if (@(`$v).Count -eq 0) { throw 'Expected an array element' }"
                        "`$v = @(`$v)[0]"
                    }
                    elseif ($case.Kind -eq 'array') {
                        "`$v = `$v -join ""`n"""
                    }
                    "'{0}`t{1}`t{2}' -f $($case.Index), `$type, [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([string]`$v))"
                    "} } catch { '{0}`terror`t{1}' -f $($case.Index), (`$_.Exception.Message -replace '\s+', ' ') }"
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
            $script:Unexpected = @()
            foreach ($outputLine in $script:Output) {
                $parts = $outputLine -split "`t"
                if ($parts.Count -ge 2 -and ($parts[0] -eq 'canary' -or $parts[0] -match '^\d+$')) {
                    $script:Results[$parts[0]] = $parts[1..($parts.Count - 1)]
                }
                else {
                    $script:Unexpected += $outputLine
                }
            }
        }

        It 'runs without errors' {
            $script:ExitCode | Should -Be 0 -Because ($script:Output -join "`n")
            $script:Unexpected | Should -BeNullOrEmpty
        }

        It 'does not run any part of a value' {
            $script:Results['canary'] | Should -Be @('False')
        }

        It '<Name> #<Index> gets its expected value and type' -ForEach $Cases {
            $result = $script:Results["$Index"]
            $result | Should -Not -BeNullOrEmpty -Because ($script:Output -join "`n")
            $result[0] | Should -Be $ExpectedType -Because ($result -join ' ')
            $result[1] | Should -BeExactly $Expected
        }
    }
}
