BeforeAll {
    . (Join-Path $PSScriptRoot 'kubeletconfig.ps1')
    . (Join-Path $PSScriptRoot 'kubernetesfunc.ps1')
    . (Join-Path $PSScriptRoot 'kubeletfunc.ps1')

    function Logs-To-Event { param($TaskName, $TaskMessage) }
    function Write-Log { param($Message) }
    function netsh { }
    function Stop-Service { param($Name) }
    function Restart-Service { param($Name, [switch] $Force) }
    function mkdir {
        param($Path, [switch] $Force)
        New-Item -Path $Path -ItemType Directory -Force:$Force | Out-Null
    }

    Add-Type -TypeDefinition @'
namespace RunProcess {
    public static class exec {
        public static string LastArguments;
        public static void RunProcess(string executable, string arguments, System.Diagnostics.ProcessPriorityClass priority) {
            LastArguments = arguments;
        }
    }
}
'@

    function ConvertTo-TestBase64 {
        param([string] $Content)
        return [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($Content))
    }

    function Write-TestClusterConfiguration {
        Write-KubeClusterConfig -MasterIP 'test.invalid' -KubeDnsServiceIp '192.0.2.53'
        $cluster = Get-Content -LiteralPath $script:ClusterPath -Raw | ConvertFrom-Json
        $cluster.Install.Destination = $script:KubeDirectory
        $cluster | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $script:ClusterPath -Encoding Unicode
    }

    function Install-TestKubeletConfiguration {
        param([string] $Request = '', [bool] $RotationEnabled = $false)
        Set-WindowsKubeletConfiguration -ContentBase64 (ConvertTo-TestBase64 ($script:Configuration | ConvertTo-Json -Depth 10)) `
            -FlagsToOmitBase64 $Request -KubeDir $script:KubeDirectory -ClusterConfigPath $script:ClusterPath `
            -BootstrapDirectory $script:BootstrapDirectory -ServingCertificateRotationEnabled $RotationEnabled
    }
}

Describe 'Windows kubelet omission decoding' {
    It 'accepts empty, raw and padded requests without evaluating unknown strings' {
        @(ConvertFrom-WindowsKubeletOmissionRequest '').Count | Should -Be 0
        foreach ($request in @('W10', 'W10=')) {
            @(ConvertFrom-WindowsKubeletOmissionRequest $request).Count | Should -Be 0
        }
        $request = ConvertTo-TestBase64 '["--volume-plugin-dir","--container-runtime-endpoint","$(throw)","--hairpin-mode"]'
        foreach ($encoding in @($request, $request.TrimEnd('='))) {
            @(ConvertFrom-WindowsKubeletOmissionRequest $encoding) -join ',' | Should -Be '--volume-plugin-dir,--container-runtime-endpoint'
        }
    }

    It 'rejects malformed requests and limits before returning names' -ForEach @(
        @{ Request = '!' }, @{ Request = 'W11=' }, @{ Request = 'W10==' }, @{ Request = "W10=`n" },
        @{ Request = 'bnVsbA' }, @{ Request = 'e30' }, @{ Request = 'WzFd' }, @{ Request = 'W251bGxd' },
        @{ Request = 'W1tdXQ' }, @{ Request = ('W' * 1025) }
    ) {
        { ConvertFrom-WindowsKubeletOmissionRequest $Request } | Should -Throw
    }

    It 'enforces the name limit even for unknown names' {
        $request = ConvertTo-TestBase64 ('[' + ('"unknown",' * 16) + '"unknown"]')
        { ConvertFrom-WindowsKubeletOmissionRequest $request } | Should -Throw '*Too many*'
    }

    It 'accepts exactly 16 names and 1024 encoded bytes' {
        $request = ConvertTo-TestBase64 ('[' + ('"unknown",' * 15) + '"unknown"]')
        @(ConvertFrom-WindowsKubeletOmissionRequest $request).Count | Should -Be 0
        $request = ConvertTo-TestBase64 ('["' + ('x' * 764) + '"]')
        $request.Length | Should -Be 1024
        @(ConvertFrom-WindowsKubeletOmissionRequest $request).Count | Should -Be 0
    }
}

Describe 'Windows kubelet materialization and startup' {
    BeforeEach {
        $caseDirectory = Join-Path $TestDrive ([Guid]::NewGuid().ToString())
        $script:KubeDirectory = Join-Path $caseDirectory 'k'
        $script:ClusterPath = Join-Path $script:KubeDirectory 'kubeclusterconfig.json'
        $script:BootstrapDirectory = Join-Path $caseDirectory 'bootstrap'
        New-Item -ItemType Directory -Path $script:KubeDirectory, $script:BootstrapDirectory -Force | Out-Null
        Copy-Item (Join-Path $PSScriptRoot 'kubeletconfig.ps1') $script:BootstrapDirectory
        Copy-Item (Join-Path $PSScriptRoot 'provisioningscripts/kubeletstart.ps1') $script:BootstrapDirectory
        $script:StartupPath = Join-Path $script:KubeDirectory 'kubeletstart.ps1'
        Set-Content -LiteralPath $script:StartupPath -Value 'cached-startup' -NoNewline
        $script:Configuration = @{
            kind = 'KubeletConfiguration'
            apiVersion = 'kubelet.config.k8s.io/v1beta1'
            volumePluginDir = [IO.Path]::Combine($script:KubeDirectory, 'volumeplugins')
            containerRuntimeEndpoint = 'npipe://./pipe/containerd-containerd'
            enableServer = $false
            containerLogMaxFiles = 0
            staticPodPath = 'c:\pods\节点'
        }
        $global:KubeClusterConfigPath = $script:ClusterPath
        $global:KubeBinariesVersion = '1.38.0'
        $global:KubeletConfigArgs = @('--anonymous-auth=false', '--hairpin-mode=promiscuous-bridge', '--register-with-taints=example.com/init=:NoExecute')
        $global:KubeletNodeLabels = 'example.com/node=windows'
        $global:NetworkPlugin = 'test-no-network'
        $global:IsSkipCleanupNetwork = $true
        $global:EnableSecureTLSBootstrapping = $false
        $global:EnableKubeletServingCertificateRotation = $false
        Write-TestClusterConfiguration
        Mock Import-Module -ParameterFilter { $Name -eq 'c:\k\hns.v2.psm1' }
        Mock Add-Type
        Mock netsh
        Mock Stop-Service
        Mock Restart-Service
        Mock Get-TagValue { 'false' }
        Mock Get-Content -ParameterFilter { $Path -eq 'c:\k\kubeclusterconfig.json' } {
            Get-Content -LiteralPath $script:ClusterPath
        }
        [RunProcess.exec]::LastArguments = $null
    }

    It 'delivers exact UTF8 bytes and attaches once without any omission request' {
        $content = $script:Configuration | ConvertTo-Json -Depth 10
        Install-TestKubeletConfiguration
        Install-TestKubeletConfiguration
        $path = Join-Path $script:KubeDirectory 'kubelet-config-rp.json'
        [Convert]::ToBase64String([IO.File]::ReadAllBytes($path)) | Should -Be (ConvertTo-TestBase64 $content)
        $cluster = Get-Content -LiteralPath $script:ClusterPath -Raw | ConvertFrom-Json
        @($cluster.Kubernetes.Kubelet.ConfigArgs | Where-Object { $_ -like '--config=*' }).Count | Should -Be 1
        @($cluster.Kubernetes.Kubelet.ConfigFile.FlagsToOmit).Count | Should -Be 0
        (Get-WindowsKubeletConfigurationForStartup -ConfigFile $cluster.Kubernetes.Kubelet.ConfigFile -ConfigArgs $cluster.Kubernetes.Kubelet.ConfigArgs -KubeDir $script:KubeDirectory).staticPodPath | Should -Be $script:Configuration.staticPodPath
        . $script:StartupPath
        $expected = ($global:KubeletConfigArgs + @("--config=$path", '--node-labels=example.com/node=windows', "--volume-plugin-dir=$($script:Configuration.volumePluginDir)", '--windows-priorityclass=ABOVE_NORMAL_PRIORITY_CLASS', '--container-runtime-endpoint=npipe://./pipe/containerd-containerd')) -join ' '
        [RunProcess.exec]::LastArguments | Should -BeExactly $expected
        Test-Path $script:Configuration.volumePluginDir -PathType Container | Should -BeTrue
    }

    It 'omits only explicitly requested exact hardcoded matches' {
        Install-TestKubeletConfiguration -Request (ConvertTo-TestBase64 '["--volume-plugin-dir","--container-runtime-endpoint","--hairpin-mode"]')
        . $script:StartupPath
        $path = Join-Path $script:KubeDirectory 'kubelet-config-rp.json'
        $expected = ($global:KubeletConfigArgs + @("--config=$path", '--node-labels=example.com/node=windows', '--windows-priorityclass=ABOVE_NORMAL_PRIORITY_CLASS')) -join ' '
        [RunProcess.exec]::LastArguments | Should -BeExactly $expected
        Test-Path $script:Configuration.volumePluginDir -PathType Container | Should -BeTrue
    }

    It 'retains mismatching, absent or wrongly typed replacements' -ForEach @(
        @{ Value = 'different' }, @{ Value = $null }, @{ Value = $false }, @{ Value = 0 }, @{ Value = '' }
    ) {
        $script:Configuration.volumePluginDir = $Value
        $script:Configuration.containerRuntimeEndpoint = $Value
        Install-TestKubeletConfiguration -Request (ConvertTo-TestBase64 '["--volume-plugin-dir","--container-runtime-endpoint"]')
        . $script:StartupPath
        [RunProcess.exec]::LastArguments | Should -Match '--volume-plugin-dir='
        [RunProcess.exec]::LastArguments | Should -Match '--container-runtime-endpoint=npipe://./pipe/containerd-containerd'
    }

    It 'retains custom duplicate precedence and priority override' {
        $global:KubeletConfigArgs += @('--volume-plugin-dir=d:\custom', '--container-runtime-endpoint=npipe://custom', '--windows-priorityclass=HIGH_PRIORITY_CLASS')
        Write-TestClusterConfiguration
        Install-TestKubeletConfiguration -Request (ConvertTo-TestBase64 '["--volume-plugin-dir","--container-runtime-endpoint"]')
        . $script:StartupPath
        $path = Join-Path $script:KubeDirectory 'kubelet-config-rp.json'
        $expected = ($global:KubeletConfigArgs + @("--config=$path", '--node-labels=example.com/node=windows', "--volume-plugin-dir=$($script:Configuration.volumePluginDir)", '--container-runtime-endpoint=npipe://./pipe/containerd-containerd')) -join ' '
        [RunProcess.exec]::LastArguments | Should -BeExactly $expected
    }

    It 'preserves the serving certificate tag override and labels' -ForEach @(@{ Disabled = 'true' }, @{ Disabled = 'false' }) {
        Mock Get-TagValue { $Disabled }
        $script:Configuration.serverTLSBootstrap = $true
        $global:KubeletConfigArgs += '--rotate-server-certificates=true'
        $global:EnableKubeletServingCertificateRotation = $true
        Configure-KubeletServingCertificateRotation
        Write-TestClusterConfiguration
        Install-TestKubeletConfiguration -RotationEnabled $true
        . $script:StartupPath
        $expectedRotation = if ($Disabled -eq 'true') { 'false' } else { 'true' }
        [RunProcess.exec]::LastArguments | Should -Match "--rotate-server-certificates=$expectedRotation"
        if ($Disabled -eq 'true') {
            [RunProcess.exec]::LastArguments | Should -Not -Match 'kubernetes.azure.com/kubelet-serving-ca=cluster'
        } else {
            [RunProcess.exec]::LastArguments | Should -Match 'kubernetes.azure.com/kubelet-serving-ca=cluster'
        }
    }

    It 'rejects conflicting custom configuration before writing feature files' -ForEach @(
        @{ Argument = '--config=c:\custom.json' }, @{ Argument = '--config-dir=c:\custom' }, @{ Argument = '--config' }
    ) {
        $global:KubeletConfigArgs += $Argument
        Write-TestClusterConfiguration
        $originalCluster = (Get-FileHash $script:ClusterPath).Hash
        { Install-TestKubeletConfiguration } | Should -Throw '*conflicts*'
        (Get-FileHash $script:ClusterPath).Hash | Should -Be $originalCluster
        Get-Content -LiteralPath $script:StartupPath -Raw | Should -Be 'cached-startup'
        Test-Path (Join-Path $script:KubeDirectory 'kubelet-config-rp.json') | Should -BeFalse
    }

    It 'rejects unsafe server TLS before writing' {
        $script:Configuration.serverTLSBootstrap = $true
        { Install-TestKubeletConfiguration } | Should -Throw '*serving-certificate*'
        Test-Path (Join-Path $script:KubeDirectory 'kubelet-config-rp.json') | Should -BeFalse
        Get-Content -LiteralPath $script:StartupPath -Raw | Should -Be 'cached-startup'
    }

    It 'rejects malformed active requests and missing or corrupt bootstrap scripts before writing' -ForEach @(
        @{ Failure = 'request' }, @{ Failure = 'missing' }, @{ Failure = 'corrupt' }, @{ Failure = 'config' }
    ) {
        $request = ''
        if ($Failure -eq 'request') { $request = '!' }
        if ($Failure -eq 'missing') { Remove-Item (Join-Path $script:BootstrapDirectory 'kubeletstart.ps1') }
        if ($Failure -eq 'corrupt') { Set-Content (Join-Path $script:BootstrapDirectory 'kubeletstart.ps1') 'function {' }
        if ($Failure -eq 'config') { $script:Configuration.kind = 'Other' }
        $originalCluster = (Get-FileHash $script:ClusterPath).Hash
        { Install-TestKubeletConfiguration -Request $request } | Should -Throw
        (Get-FileHash $script:ClusterPath).Hash | Should -Be $originalCluster
        Test-Path (Join-Path $script:KubeDirectory 'kubelet-config-rp.json') | Should -BeFalse
        Get-Content -LiteralPath $script:StartupPath -Raw | Should -Be 'cached-startup'
    }

    It 'fails before service or process launch when an attached file is missing or corrupt' -ForEach @(@{ Missing = $true }, @{ Missing = $false }) {
        Install-TestKubeletConfiguration -Request (ConvertTo-TestBase64 '["--volume-plugin-dir"]')
        $path = Join-Path $script:KubeDirectory 'kubelet-config-rp.json'
        if ($Missing) { Remove-Item $path } else { Set-Content $path 'not JSON' }
        { . $script:StartupPath } | Should -Throw
        Should -Invoke Stop-Service -Times 0 -Exactly
        Should -Invoke netsh -Times 0 -Exactly
        [RunProcess.exec]::LastArguments | Should -BeNullOrEmpty
    }

    It 'rejects corrupt full payloads before writing' -ForEach @(
        @{ EncodedContent = '!' }, @{ EncodedContent = 'ew==' }, @{ EncodedContent = 'bnVsbA==' }, @{ EncodedContent = 'W10=' }
    ) {
        $originalCluster = (Get-FileHash $script:ClusterPath).Hash
        { Set-WindowsKubeletConfiguration -ContentBase64 $EncodedContent -KubeDir $script:KubeDirectory `
            -ClusterConfigPath $script:ClusterPath -BootstrapDirectory $script:BootstrapDirectory } | Should -Throw
        (Get-FileHash $script:ClusterPath).Hash | Should -Be $originalCluster
        Test-Path (Join-Path $script:KubeDirectory 'kubelet-config-rp.json') | Should -BeFalse
    }

    It 'refreshes cached scripts and baked values from live node input without BasePrep' {
        Install-TestKubeletConfiguration -Request (ConvertTo-TestBase64 '["--volume-plugin-dir"]')
        $script:Configuration.staticPodPath = 'c:\pods\live'
        $global:KubeletConfigArgs += '--container-log-max-files=9'
        Write-TestClusterConfiguration
        Set-Content -LiteralPath $script:StartupPath -Value 'stale-startup'
        Install-TestKubeletConfiguration -Request (ConvertTo-TestBase64 '["--container-runtime-endpoint"]')
        (Get-FileHash $script:StartupPath).Hash | Should -Be (Get-FileHash (Join-Path $script:BootstrapDirectory 'kubeletstart.ps1')).Hash
        . $script:StartupPath
        [RunProcess.exec]::LastArguments | Should -Match '--container-log-max-files=9'
        [RunProcess.exec]::LastArguments | Should -Match '--volume-plugin-dir='
        [RunProcess.exec]::LastArguments | Should -Not -Match '--container-runtime-endpoint='
        $configuration = Get-Content (Join-Path $script:KubeDirectory 'kubelet-config-rp.json') -Raw -Encoding UTF8 | ConvertFrom-Json
        $configuration.staticPodPath | Should -Be 'c:\pods\live'
    }

    It 'rejects competing loaded configuration and mismatched metadata before service startup' -ForEach @(
        @{ Conflict = '--config=c:\custom.json' }, @{ Conflict = '--config-dir=c:\dropins' }, @{ Conflict = 'metadata' }
    ) {
        Install-TestKubeletConfiguration -Request (ConvertTo-TestBase64 '["--volume-plugin-dir"]')
        $cluster = Get-Content -LiteralPath $script:ClusterPath -Raw | ConvertFrom-Json
        if ($Conflict -eq 'metadata') {
            $cluster.Kubernetes.Kubelet.ConfigFile.Path = 'c:\custom.json'
        } else {
            $cluster.Kubernetes.Kubelet.ConfigArgs += $Conflict
        }
        $cluster | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $script:ClusterPath -Encoding Unicode
        { . $script:StartupPath } | Should -Throw
        Should -Invoke Stop-Service -Times 0 -Exactly
        [RunProcess.exec]::LastArguments | Should -BeNullOrEmpty
    }

    It 'ignores and preserves stale feature files when the fresh cluster writer has no opt-in metadata' {
        Install-TestKubeletConfiguration -Request (ConvertTo-TestBase64 '["--volume-plugin-dir","--container-runtime-endpoint"]')
        $path = Join-Path $script:KubeDirectory 'kubelet-config-rp.json'
        Set-Content $path 'stale-invalid-json'
        $helperPath = Join-Path $script:KubeDirectory 'kubeletconfig.ps1'
        Set-Content $helperPath 'throw "stale helper must not run"'
        Write-TestClusterConfiguration
        . $script:StartupPath
        [RunProcess.exec]::LastArguments | Should -Not -Match '--config='
        [RunProcess.exec]::LastArguments | Should -Match '--volume-plugin-dir='
        [RunProcess.exec]::LastArguments | Should -Match '--container-runtime-endpoint='
        Get-Content $path | Should -Be 'stale-invalid-json'
        Get-Content $helperPath | Should -Be 'throw "stale helper must not run"'
    }

    It 'preserves legacy custom config arguments and version boundaries without new metadata' -ForEach @(
        @{ Version = '1.26.15' }, @{ Version = '1.31.0' }, @{ Version = '1.37.99' },
        @{ Version = '1.38.0-beta.0' }, @{ Version = '1.38.0' }
    ) {
        $global:KubeBinariesVersion = $Version
        $global:KubeletConfigArgs += @('--config=c:\custom.json', '--config-dir=c:\custom-directory')
        Write-TestClusterConfiguration
        . (Join-Path $script:BootstrapDirectory 'kubeletstart.ps1')
        $expected = $global:KubeletConfigArgs + @('--node-labels=example.com/node=windows', "--volume-plugin-dir=$($script:Configuration.volumePluginDir)", '--windows-priorityclass=ABOVE_NORMAL_PRIORITY_CLASS', '--container-runtime-endpoint=npipe://./pipe/containerd-containerd')
        if ($Version -lt '1.27.0') { $expected += '--container-runtime=remote' }
        [RunProcess.exec]::LastArguments | Should -BeExactly ($expected -join ' ')
        Get-Content -LiteralPath $script:StartupPath -Raw | Should -Be 'cached-startup'
        Test-Path (Join-Path $script:KubeDirectory 'kubelet-config-rp.json') | Should -BeFalse
    }
}
