# A node booted from a PIS-cached VHD skips BasePrep, because base_prep.complete is already in the
# image. NodePrep must therefore apply the live cluster configuration and credentials, even when
# BasePrep also configured a component such as SSH during the bake.
# These checks read the template as text so they pin the phase a call is made from.

BeforeAll {
    $script:TemplatePath = Join-Path $PSScriptRoot 'kuberneteswindowssetup.ps1.template'
    $script:TemplateText = Get-Content -Path $script:TemplatePath -Raw

    function Get-FunctionBody {
        param(
            [Parameter(Mandatory = $true)][string] $Text,
            [Parameter(Mandatory = $true)][string] $Name
        )

        $lines = $Text -split "`r?`n"
        $start = -1
        for ($i = 0; $i -lt $lines.Count; $i++) {
            if ($lines[$i] -match "^\s*function\s+$Name\s*\{") {
                $start = $i
                break
            }
        }
        if ($start -lt 0) {
            throw "function $Name not found in template"
        }

        $depth = 0
        $body = New-Object System.Collections.Generic.List[string]
        for ($i = $start; $i -lt $lines.Count; $i++) {
            $line = $lines[$i]
            $depth += ([regex]::Matches($line, '\{')).Count
            $depth -= ([regex]::Matches($line, '\}')).Count
            $body.Add($line)
            if ($depth -le 0 -and $i -gt $start) {
                break
            }
        }
        return ($body -join "`n")
    }

    $script:BasePrepBody = Get-FunctionBody -Text $script:TemplateText -Name 'BasePrep'
    $script:NodePrepBody = Get-FunctionBody -Text $script:TemplateText -Name 'NodePrep'
    $script:SSHPolicyBody = $script:NodePrepBody.Substring($script:NodePrepBody.IndexOf('{') + 1)
    $script:SSHPolicyBody = $script:SSHPolicyBody.Substring(0, $script:SSHPolicyBody.IndexOf('    # Write the kubeconfigs'))
    function Write-Log { param($Message) }
}

Describe 'Windows CSE PIS phase placement' {
    # Get-FunctionBody tracks brace depth across all text, including braces inside strings and
    # comments, so a miscount could truncate a body early and make the negative assertions below
    # pass for the wrong reason. Anchor on the closing log line of each phase so a truncated
    # extraction fails loudly here first.
    Context 'extraction integrity' {
        It 'extracts the whole BasePrep body' {
            $script:BasePrepBody | Should -Match 'function BasePrep'
            $script:BasePrepBody | Should -Match 'BasePrep completed successfully'
        }

        It 'extracts the whole NodePrep body' {
            $script:NodePrepBody | Should -Match 'function NodePrep'
            $script:NodePrepBody | Should -Match 'NodePrep completed successfully'
        }

        It 'does not bleed one phase into the other' {
            $script:BasePrepBody | Should -Not -Match 'function NodePrep'
            $script:NodePrepBody | Should -Not -Match 'function BasePrep'
        }
    }

    Context 'phase gate' {
        It 'skips BasePrep when the image already carries the marker' {
            $script:TemplateText | Should -Match 'base_prep\.complete'
            $script:TemplateText | Should -Match 'if\s*\(-not\s*\(Test-Path\s*"C:\\AzureData\\base_prep\.complete"\)\)'
        }

        It 'runs NodePrep on every node that is not a pre-provision bake' {
            $script:TemplateText | Should -Match 'if\s*\(-not\s*\$PreProvisionOnly\)'
        }

        It 'writes the marker only for a pre-provision bake' {
            $script:TemplateText | Should -Match '\$PreProvisionOnly.*base_prep\.complete'
        }
    }

    Context 'cloud provider config' {
        # azure.json carries the service principal secret, the user assigned identity and the
        # VMSS, subnet, NSG, VNet and route table this node belongs to. Writing it in BasePrep
        # would bake one cluster's identity and network into an image reused by other nodes.
        It 'writes azure.json in NodePrep' {
            $script:NodePrepBody | Should -Match 'Write-AzureConfig'
        }

        It 'does not write azure.json in BasePrep' {
            $script:BasePrepBody | Should -Not -Match 'Write-AzureConfig'
        }

        It 'writes azure.json before kubelet is installed and started' {
            $configIndex = $script:NodePrepBody.IndexOf('Write-AzureConfig')
            $kubeletIndex = $script:NodePrepBody.IndexOf('Install-KubernetesServices')
            $configIndex | Should -BeGreaterThan -1
            $kubeletIndex | Should -BeGreaterThan -1
            $configIndex | Should -BeLessThan $kubeletIndex
        }
    }

    Context 'credentials and cluster identity' {
        It 'writes the bootstrap kubeconfig in NodePrep only' {
            $script:NodePrepBody | Should -Match 'Write-BootstrapKubeConfig'
            $script:BasePrepBody | Should -Not -Match 'Write-BootstrapKubeConfig'
        }

        It 'writes the client kubeconfig in NodePrep only' {
            $script:NodePrepBody | Should -Match 'Write-KubeConfig'
            $script:BasePrepBody | Should -Not -Match 'Write-KubeConfig\b'
        }

        It 'writes the cluster CA certificate in NodePrep only' {
            $script:NodePrepBody | Should -Match 'Write-CACert'
            $script:BasePrepBody | Should -Not -Match 'Write-CACert'
        }
    }

    Context 'SSH access policy' {
        It 'applies the live SSH policy and keys in NodePrep' {
            $script:NodePrepBody | Should -Match '\$sshEnabled=\[System.Convert\]::ToBoolean\("\{\{WindowsSSHEnabled\}\}"\)'
            $script:NodePrepBody | Should -Match 'Install-OpenSSH -Enabled \$sshEnabled -SSHKeys \$global:SSHKeys'
        }

        It 'preserves SSH configuration in BasePrep only when enabled' {
            $script:BasePrepBody | Should -Match '\$sshEnabled=\[System.Convert\]::ToBoolean\("\{\{WindowsSSHEnabled\}\}"\)'
            $script:BasePrepBody | Should -Match 'if\s*\(\s*\$sshEnabled\s*\)\s*\{\s*Install-OpenSSH -SSHKeys \$SSHKeys\s*\}'
        }

        It 'applies SSH policy before starting Kubernetes services' {
            $sshIndex = $script:NodePrepBody.IndexOf('Install-OpenSSH')
            $kubeletIndex = $script:NodePrepBody.IndexOf('Install-KubernetesServices')
            $sshIndex | Should -BeGreaterThan -1
            $kubeletIndex | Should -BeGreaterThan -1
            $sshIndex | Should -BeLessThan $kubeletIndex
        }

        It 'loads SSH helpers through the optional scripts in Get-HelperScripts' {
            $helpers = Get-FunctionBody -Text $script:TemplateText -Name 'Get-HelperScripts'
            $helpers | Should -Match "'c:\\AzureData\\windows\\sshfunc\.ps1'"
            $helpers | Should -Match 'if \(Test-Path -Path \$script\)\s*\{\s*\. \$script'
            $script:TemplateText | Should -Not -Match 'windowsssh\.ps1'
        }

        Context 'bundle compatibility' {
            BeforeEach {
                $script:SavedSSHKeys = $global:SSHKeys
                $global:SSHKeys = @('ssh-rsa live-node-key')
                Mock Write-Log
            }

            AfterEach {
                $global:SSHKeys = $script:SavedSSHKeys
            }

            It 'applies policy <Enabled> with live keys when the bundle supports it' -TestCases @(
                @{ Enabled = 'True' }
                @{ Enabled = 'False' }
            ) {
                param($Enabled)
                $script:SSHPolicyCall = $null
                function Install-OpenSSH {
                    param([bool]$Enabled, [string[]]$SSHKeys)
                    $script:SSHPolicyCall = @{ Enabled = $Enabled; Keys = $SSHKeys }
                }

                & ([scriptblock]::Create($script:SSHPolicyBody.Replace('{{WindowsSSHEnabled}}', $Enabled)))

                $script:SSHPolicyCall.Enabled | Should -Be ([System.Convert]::ToBoolean($Enabled))
                $script:SSHPolicyCall.Keys | Should -Be @('ssh-rsa live-node-key')
                Assert-MockCalled Write-Log -Times 0 -Exactly
            }

            It 'preserves an enabled-only legacy implementation for policy <Enabled>' -TestCases @(
                @{ Enabled = 'True' }
                @{ Enabled = 'False' }
            ) {
                param($Enabled)
                function Install-OpenSSH {
                    [CmdletBinding()]
                    param([string[]]$SSHKeys)
                    throw 'NodePrep must not call the legacy implementation'
                }

                { & ([scriptblock]::Create($script:SSHPolicyBody.Replace('{{WindowsSSHEnabled}}', $Enabled))) } | Should -Not -Throw
                Assert-MockCalled Write-Log -Times 1 -Exactly -ParameterFilter { $Message -like '*preserving legacy behavior*' }
            }

            It 'skips enforcement when the bundle has no SSH function' {
                Mock Get-Command { $null } -ParameterFilter { $Name -eq 'Install-OpenSSH' }

                { & ([scriptblock]::Create($script:SSHPolicyBody.Replace('{{WindowsSSHEnabled}}', 'False'))) } | Should -Not -Throw
                Assert-MockCalled Write-Log -Times 1 -Exactly
            }

            It 'does not hide enforcement failures from a supported bundle' {
                function Install-OpenSSH {
                    param([bool]$Enabled, [string[]]$SSHKeys)
                    throw 'SSH policy enforcement failed'
                }

                { & ([scriptblock]::Create($script:SSHPolicyBody.Replace('{{WindowsSSHEnabled}}', 'True'))) } | Should -Throw '*SSH policy enforcement failed*'
                Assert-MockCalled Write-Log -Times 0 -Exactly
            }
        }
    }

    Context 'cluster network config' {
        It 'writes the Azure CNI config in NodePrep only' {
            $script:NodePrepBody | Should -Match 'Set-AzureCNIConfig'
            $script:BasePrepBody | Should -Not -Match 'Set-AzureCNIConfig'
        }

        # BasePrep writes placeholders so the cached CSE scripts can read the pause image while
        # configuring containerd during the bake. NodePrep must rewrite the file so the values a
        # cached image carries are replaced with this cluster's.
        It 'rewrites the kube cluster config in NodePrep after writing placeholders in BasePrep' {
            $script:BasePrepBody | Should -Match 'Write-KubeClusterConfig'
            $script:NodePrepBody | Should -Match 'Write-KubeClusterConfig'
        }

        It 'refreshes the kubelet serving certificate config before the kube cluster config is written' {
            $rotationIndex = $script:NodePrepBody.IndexOf('Configure-KubeletServingCertificateRotation')
            $clusterConfigIndex = $script:NodePrepBody.IndexOf('Write-KubeClusterConfig')
            $rotationIndex | Should -BeGreaterThan -1
            $clusterConfigIndex | Should -BeGreaterThan -1
            $rotationIndex | Should -BeLessThan $clusterConfigIndex
        }
    }
}
