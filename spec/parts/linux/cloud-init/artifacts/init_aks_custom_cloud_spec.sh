#!/bin/bash

Describe 'init-aks-custom-cloud.sh refresh mode wiring'
    script_path='./parts/linux/cloud-init/artifacts/init-aks-custom-cloud.sh'

    It 'parses action argument after deriving location, with init default'
        When run grep -Eq '^action=\$\{1:-init\}$' "$script_path"
        The status should eq 0
    End

    It 'uses arg2 as location fallback when invoked as ca-refresh'
        # refresh_location falls back to LOCATION env var when arg2 is absent
        When run grep -Eq '^refresh_location="\$\{2:-\$\{LOCATION\}\}"$' "$script_path"
        The status should eq 0
    End

    It 'always derives cert endpoint mode from refresh_location'
        When run grep -Eq '^location_normalized="\$\{refresh_location,,\}"$' "$script_path"
        The status should eq 0

        When run grep -Eq 'ussec\*\|usnat\*\) cert_endpoint_mode="legacy"' "$script_path"
        The status should eq 0
    End

    It 'exits early in ca-refresh mode after certificate refresh logic'
        When run grep -Eq '^if \[ "\$action" = "ca-refresh" \]; then$' "$script_path"
        The status should eq 0

        When run grep -Eq '^\s*exit$' "$script_path"
        The status should eq 0
    End

    It 'passes LOCATION directly into cron refresh command'
        When run grep -Eq 'ca-refresh \\\\"\$LOCATION\\\\"' "$script_path"
        The status should eq 0
    End

    It 'passes LOCATION directly into systemd refresh command'
        When run grep -Eq '^ExecStart=\$script_path ca-refresh \$LOCATION$' "$script_path"
        The status should eq 0
    End
End
