# The Windows CSE variables block has two branches. The legacy branch renders values into PowerShell
# code. The structured branch parses them from gzip-compressed JSON (EnableWindowsStructuredBootstrapConfig).
# These tests run the rendered blocks that pkg/agent/windows_bootstrap_template_test.go writes to
# pkg/agent/testdata/windowsbootstrap (make generate-testdata) and check that:
# - both branches set exactly the variables and values that AgentBaker expects (<name>.expected.txt);
# - hostile values stay data in the structured branch;
# - a bad config fails with WINDOWS_CSE_ERROR_LOAD_BOOTSTRAP_CONFIG.
# On Windows, the blocks also run in Windows PowerShell 5.1, which runs the CSE on nodes.

BeforeDiscovery {
    $fixtureDir = [IO.Path]::GetFullPath([IO.Path]::Combine($PSScriptRoot, '..', '..', 'pkg', 'agent', 'testdata', 'windowsbootstrap'))
    $shells = @('pwsh')
    if ($IsWindows -or $PSVersionTable.PSEdition -eq 'Desktop') {
        $shells += 'powershell'
    }

    $script:BlockCases = @(
        foreach ($expected in Get-ChildItem -Path $fixtureDir -Filter '*.expected.txt') {
            $name = $expected.Name -replace '\.expected\.txt$', ''
            foreach ($branch in 'legacy', 'structured') {
                $block = Join-Path $fixtureDir "$name.$branch.ps1"
                if (Test-Path $block) {
                    foreach ($shell in $shells) {
                        @{ Name = $name; Branch = $branch; Block = $block; Expected = $expected.FullName; Shell = $shell }
                    }
                }
            }
        }
    )
    $script:ShellCases = @(foreach ($shell in $shells) { @{ Shell = $shell; Block = (Join-Path $fixtureDir 'default.structured.ps1') } })
}

BeforeAll {
    $script:DumpTemplate = @'
$CSEResultFilePath = '__RESULT_FILE__'
$__before = @(Get-Variable | ForEach-Object { $_.Name })
__BLOCK__
function __Format($value) {
    if ($null -eq $value) { return 'null' }
    if ($value -is [array]) { return 'array:' + ((@($value) | ForEach-Object { __Format $_ }) -join ',') }
    if ($value -is [string]) { return 'string:' + [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($value)) }
    if ($value -is [bool]) { return 'bool:' + $value }
    if ($value -is [uint32]) { return 'uint32:' + $value }
    return $value.GetType().FullName + ':' + [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([string]$value))
}
foreach ($__variable in Get-Variable) {
    if ($__before -notcontains $__variable.Name -and -not $__variable.Name.StartsWith('__')) {
        "{0}`t{1}" -f $__variable.Name, (__Format $__variable.Value)
    }
}
'@

    # Invoke-VariablesBlock runs a variables block in a new PowerShell process, like the CSE does, and
    # returns the variables it created as "<name>`t<value>" lines.
    function Invoke-VariablesBlock {
        param(
            [Parameter(Mandatory = $true)][string] $BlockText,
            [Parameter(Mandatory = $true)][string] $Shell
        )

        $directory = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
        New-Item -ItemType Directory -Path $directory | Out-Null
        try {
            $resultFile = Join-Path $directory 'provision.complete'
            $scriptPath = Join-Path $directory 'variables.ps1'
            $scriptText = $script:DumpTemplate.Replace('__RESULT_FILE__', $resultFile).Replace('__BLOCK__', $BlockText)
            Set-Content -Path $scriptPath -Value $scriptText -Encoding ascii

            $exe = if ($Shell -eq 'powershell') { 'powershell.exe' } else { [Diagnostics.Process]::GetCurrentProcess().MainModule.FileName }
            $arguments = @('-NoProfile', '-NonInteractive')
            if ($IsWindows -or $PSVersionTable.PSEdition -eq 'Desktop') {
                $arguments += @('-ExecutionPolicy', 'Bypass')
            }
            $output = & $exe @arguments -File $scriptPath 2>&1
            return [pscustomobject]@{
                ExitCode = $LASTEXITCODE
                Lines    = @($output | ForEach-Object { "$_" } | Where-Object { $_.Contains("`t") })
                Output   = ($output | ForEach-Object { "$_" }) -join "`n"
                Result   = if (Test-Path $resultFile) { Get-Content -Raw -Path $resultFile } else { $null }
            }
        }
        finally {
            Remove-Item -Path $directory -Recurse -Force
        }
    }

    function ConvertTo-VariableTable {
        param([string[]] $Lines)
        $table = @{}
        foreach ($line in $Lines) {
            if ($line) {
                $name, $value = $line -split "`t", 2
                $table[$name.ToLowerInvariant()] = $value
            }
        }
        return $table
    }

    function Set-ConfigBlob {
        param(
            [Parameter(Mandatory = $true)][string] $BlockText,
            [Parameter(Mandatory = $true)][string] $Blob
        )
        $pattern = "FromBase64String\('[A-Za-z0-9+/=]+'\)"
        [regex]::Matches($BlockText, $pattern).Count | Should -Be 1
        return [regex]::Replace($BlockText, $pattern, "FromBase64String('$Blob')")
    }

    function ConvertTo-ConfigBlob {
        param([Parameter(Mandatory = $true)][string] $Json)
        $memory = New-Object IO.MemoryStream
        $gzip = New-Object IO.Compression.GZipStream($memory, [IO.Compression.CompressionMode]::Compress)
        $bytes = [Text.Encoding]::UTF8.GetBytes($Json)
        $gzip.Write($bytes, 0, $bytes.Length)
        $gzip.Dispose()
        return [Convert]::ToBase64String($memory.ToArray())
    }
}

Describe 'Windows CSE variables block' {
    It '<Branch> block of <Name> sets the expected variables in <Shell>' -ForEach $BlockCases {
        $result = Invoke-VariablesBlock -BlockText (Get-Content -Raw -Path $Block) -Shell $Shell
        $result.ExitCode | Should -Be 0 -Because $result.Output
        $result.Lines | Should -Not -Match '^AKSInjectionCanary\t'

        $actual = ConvertTo-VariableTable $result.Lines
        $expected = ConvertTo-VariableTable (Get-Content -Path $Expected)
        @($actual.Keys | Sort-Object) | Should -Be @($expected.Keys | Sort-Object)
        foreach ($key in $expected.Keys) {
            $actual[$key] | Should -Be $expected[$key] -Because "variable $key"
        }
    }

    Context 'invalid structured config in <Shell>' -ForEach $ShellCases {
        BeforeAll {
            $script:Structured = Get-Content -Raw -Path $Block
        }

        It 'fails with WINDOWS_CSE_ERROR_LOAD_BOOTSTRAP_CONFIG when the config is not gzip' {
            $result = Invoke-VariablesBlock -BlockText (Set-ConfigBlob -BlockText $script:Structured -Blob 'AAAA') -Shell $Shell
            $result.ExitCode | Should -Be 85 -Because $result.Output
            $result.Result | Should -Match '^ExitCode: \|85\|, Output: \|WINDOWS_CSE_ERROR_LOAD_BOOTSTRAP_CONFIG\|, Error: \|Failed to load the AKS bootstrap configuration: System\.[A-Za-z.]+\|'
            $result.Output | Should -Match 'WINDOWS_CSE_ERROR_LOAD_BOOTSTRAP_CONFIG'
            $result.Lines | Should -BeNullOrEmpty
        }

        It 'fails with WINDOWS_CSE_ERROR_LOAD_BOOTSTRAP_CONFIG for an unknown schema version' {
            $blob = ConvertTo-ConfigBlob -Json '{"SchemaVersion":2,"MasterIP":"secret-value"}'
            $result = Invoke-VariablesBlock -BlockText (Set-ConfigBlob -BlockText $script:Structured -Blob $blob) -Shell $Shell
            $result.ExitCode | Should -Be 85 -Because $result.Output
            $result.Result | Should -Match '^ExitCode: \|85\|, Output: \|WINDOWS_CSE_ERROR_LOAD_BOOTSTRAP_CONFIG\|'
            $result.Result | Should -Not -Match 'secret-value'
        }
    }

    It 'uses the exit code that windowscsehelper.ps1 defines' {
        . (Join-Path $PSScriptRoot 'windowscsehelper.ps1')
        $global:WINDOWS_CSE_ERROR_LOAD_BOOTSTRAP_CONFIG | Should -Be 85
        $global:ErrorCodeNames[85] | Should -Be 'WINDOWS_CSE_ERROR_LOAD_BOOTSTRAP_CONFIG'

        $template = Get-Content -Raw -Path (Join-Path $PSScriptRoot 'kuberneteswindowssetup.ps1.template')
        $template | Should -Match ([regex]::Escape('ExitCode: |85|, Output: |WINDOWS_CSE_ERROR_LOAD_BOOTSTRAP_CONFIG|'))
        $template | Should -Match '(?m)^\s*exit 85\s*$'
    }
}
