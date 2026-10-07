function ConvertFrom-WindowsKubeletOmissionRequest {
    param([AllowEmptyString()][string] $EncodedRequest)

    if ([string]::IsNullOrEmpty($EncodedRequest)) { return @() }
    if ($EncodedRequest.Length -gt 1024 -or $EncodedRequest -cnotmatch '^[A-Za-z0-9+/]*={0,2}$') {
        throw 'Invalid Windows kubelet omission request encoding'
    }
    if ($EncodedRequest.Contains('=')) {
        if ($EncodedRequest.Length % 4 -ne 0) { throw 'Invalid Windows kubelet omission request padding' }
    } else {
        $EncodedRequest = $EncodedRequest.PadRight($EncodedRequest.Length + ((4 - $EncodedRequest.Length % 4) % 4), '=')
    }
    $requestBytes = [Convert]::FromBase64String($EncodedRequest)
    if ([Convert]::ToBase64String($requestBytes) -cne $EncodedRequest) { throw 'Invalid Windows kubelet omission request padding bits' }
    $requestJson = (New-Object System.Text.UTF8Encoding($false, $true)).GetString($requestBytes)
    if (-not $requestJson.TrimStart().StartsWith('[')) { throw 'Windows kubelet omission request must be an array' }
    $requestedFlags = @($requestJson | ConvertFrom-Json -ErrorAction Stop)
    if ($requestedFlags.Count -gt 16) { throw 'Too many Windows kubelet omission names' }
    foreach ($flagName in $requestedFlags) {
        if ($flagName -isnot [string]) { throw 'Windows kubelet omission names must be strings' }
        if ($flagName -cin @('--volume-plugin-dir', '--container-runtime-endpoint')) { $flagName }
    }
}

function ConvertFrom-WindowsKubeletConfiguration {
    param([Parameter(Mandatory = $true)][string] $Content)

    if (-not $Content.TrimStart().StartsWith('{')) { throw 'Windows kubelet configuration must be an object' }
    $configuration = $Content | ConvertFrom-Json -ErrorAction Stop
    if ($configuration.kind -cne 'KubeletConfiguration' -or $configuration.apiVersion -cne 'kubelet.config.k8s.io/v1beta1') {
        throw 'Unsupported Windows kubelet configuration kind or apiVersion'
    }
    return $configuration
}

function Set-WindowsKubeletConfiguration {
    param(
        [Parameter(Mandatory = $true)][string] $ContentBase64,
        [AllowEmptyString()][string] $FlagsToOmitBase64,
        [Parameter(Mandatory = $true)][string] $KubeDir,
        [Parameter(Mandatory = $true)][string] $ClusterConfigPath,
        [Parameter(Mandatory = $true)][string] $BootstrapDirectory,
        [bool] $ServingCertificateRotationEnabled = $false
    )

    $contentBytes = [Convert]::FromBase64String($ContentBase64)
    $content = (New-Object System.Text.UTF8Encoding($false, $true)).GetString($contentBytes)
    $configuration = ConvertFrom-WindowsKubeletConfiguration -Content $content
    $flagsToOmit = @(ConvertFrom-WindowsKubeletOmissionRequest -EncodedRequest $FlagsToOmitBase64)
    $clusterConfiguration = Get-Content -Path $ClusterConfigPath -Raw -ErrorAction Stop | ConvertFrom-Json -ErrorAction Stop
    $kubeletArguments = @($clusterConfiguration.Kubernetes.Kubelet.ConfigArgs)
    $configurationPath = Join-Path $KubeDir 'kubelet-config-rp.json'
    $configurationArgument = "--config=$configurationPath"
    foreach ($argument in $kubeletArguments) {
        if ($argument -match '^--config(?:-dir)?(?:=|$)') {
            if ($argument -cne $configurationArgument -or $clusterConfiguration.Kubernetes.Kubelet.ConfigFile.Path -cne $configurationPath) {
                throw 'Explicit Windows kubelet configuration conflicts with existing --config or --config-dir'
            }
        }
    }
    if ($configuration.serverTLSBootstrap -eq $true) {
        if (-not $ServingCertificateRotationEnabled -or -not ($kubeletArguments -cmatch '^--rotate-server-certificates=(true|false)$')) {
            throw 'Windows serverTLSBootstrap requires the retained serving-certificate rotation CLI and tag pipeline'
        }
    }
    foreach ($scriptName in @('kubeletconfig.ps1', 'kubeletstart.ps1')) {
        $scriptPath = Join-Path $BootstrapDirectory $scriptName
        if (-not (Test-Path -Path $scriptPath -PathType Leaf)) { throw "Missing Windows kubelet bootstrap script: $scriptName" }
        $parseTokens = $null
        $parseErrors = $null
        $null = [System.Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref] $parseTokens, [ref] $parseErrors)
        if ($parseErrors.Count -ne 0) { throw "Invalid Windows kubelet bootstrap script: $scriptName" }
    }

    $clusterConfiguration.Kubernetes.Kubelet.ConfigArgs = @($kubeletArguments | Where-Object { $_ -cne $configurationArgument }) + @($configurationArgument)
    $clusterConfiguration.Kubernetes.Kubelet | Add-Member -MemberType NoteProperty -Name ConfigFile -Value @{
        Path = $configurationPath
        FlagsToOmit = $flagsToOmit
    } -Force
    [IO.File]::WriteAllBytes("$configurationPath.tmp", $contentBytes)
    Move-Item -Path "$configurationPath.tmp" -Destination $configurationPath -Force -ErrorAction Stop
    foreach ($scriptName in @('kubeletconfig.ps1', 'kubeletstart.ps1')) {
        Copy-Item -Path (Join-Path $BootstrapDirectory $scriptName) -Destination (Join-Path $KubeDir $scriptName) -Force -ErrorAction Stop
    }
    $clusterJson = $clusterConfiguration | ConvertTo-Json -Depth 10
    [IO.File]::WriteAllText("$ClusterConfigPath.tmp", $clusterJson, [Text.Encoding]::Unicode)
    Move-Item -Path "$ClusterConfigPath.tmp" -Destination $ClusterConfigPath -Force -ErrorAction Stop
}

function Get-WindowsKubeletConfigurationForStartup {
    param(
        [Parameter(Mandatory = $true)] $ConfigFile,
        [string[]] $ConfigArgs,
        [Parameter(Mandatory = $true)][string] $KubeDir
    )

    $configurationPath = Join-Path $KubeDir 'kubelet-config-rp.json'
    if ($ConfigFile.Path -cne $configurationPath -or "--config=$configurationPath" -cnotin $ConfigArgs) {
        throw 'Windows kubelet configuration metadata does not match the attached file'
    }
    if (@($ConfigArgs | Where-Object { $_ -match '^--config(?:-dir)?(?:=|$)' }).Count -ne 1) {
        throw 'Windows kubelet configuration conflicts with another --config or --config-dir'
    }
    $content = Get-Content -Path $configurationPath -Encoding UTF8 -Raw -ErrorAction Stop
    return ConvertFrom-WindowsKubeletConfiguration -Content $content
}

function Test-WindowsKubeletFlagInConfiguration {
    param(
        [Parameter(Mandatory = $true)][string] $FlagName,
        [Parameter(Mandatory = $true)][string] $ExpectedValue,
        [Parameter(Mandatory = $true)] $Configuration,
        [string[]] $RequestedFlags,
        [string[]] $ConfigArgs
    )

    if ($FlagName -cnotin $RequestedFlags) { return $false }
    if (@($ConfigArgs | Where-Object { $_ -ceq $FlagName -or $_.StartsWith("$FlagName=", [StringComparison]::Ordinal) }).Count -gt 0) { return $false }
    switch -CaseSensitive ($FlagName) {
        '--volume-plugin-dir' { $configuredValue = $Configuration.volumePluginDir }
        '--container-runtime-endpoint' { $configuredValue = $Configuration.containerRuntimeEndpoint }
        default { return $false }
    }
    return $configuredValue -is [string] -and $configuredValue -ceq $ExpectedValue
}
