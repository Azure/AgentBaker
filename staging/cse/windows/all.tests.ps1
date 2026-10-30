Describe 'Windows CSE bundle imports' {
    It 'does not import SSH helpers as a side effect of loading configfunc.ps1' {
        $config = Get-Content "$PSScriptRoot\configfunc.ps1" -Raw
        $config | Should -Not -Match '(?m)^\s*\.\s+.*sshfunc\.ps1'
    }

    It 'loads SSH helpers from all.ps1 without relying on configfunc.ps1' {
        $WINDOWS_SCRIPTS_DIRECTORY = Join-Path $TestDrive 'bundle'
        New-Item -Path $WINDOWS_SCRIPTS_DIRECTORY -ItemType Directory | Out-Null

        # Leave other helpers empty so configfunc cannot indirectly import SSH.
        foreach ($helper in @(
            'helpers', 'azurecnifunc', 'calicofunc', 'configfunc', 'containerdfunc',
            'kubeletfunc', 'kubernetesfunc', 'nvidiagpudriverfunc', 'securetlsbootstrapfunc',
            'windowsciliumnetworkingfunc', 'networkisolatedclusterfunc'
        )) {
            New-Item -Path "$WINDOWS_SCRIPTS_DIRECTORY\$helper.ps1" -ItemType File | Out-Null
        }
        Copy-Item "$PSScriptRoot\sshfunc.ps1" "$WINDOWS_SCRIPTS_DIRECTORY\sshfunc.ps1" -Force
        function Install-OpenSSH { param($SSHKeys) throw 'legacy implementation' }

        . "$PSScriptRoot\all.ps1"

        (Get-Command Install-OpenSSH -CommandType Function).Parameters.ContainsKey('Enabled') | Should -BeTrue
    }
}
