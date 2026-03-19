#!/bin/bash

Describe 'init-aks-custom-cloud.sh refresh mode wiring'
    script_path='./parts/linux/cloud-init/artifacts/init-aks-custom-cloud.sh'

    It 'parses action and optional requested cert endpoint mode arguments'
        When run grep -Eq '^action=\$\{1:-init\}$' "$script_path"
        The status should eq 0

        When run grep -Eq '^requested_cert_endpoint_mode="\$\{2:-\}"$' "$script_path"
        The status should eq 0
    End

    It 'uses requested mode during ca-refresh when provided'
        When run grep -Eq '^if \[ "\$action" = "ca-refresh" \] && \[ -n "\$requested_cert_endpoint_mode" \]; then$' "$script_path"
        The status should eq 0

        When run grep -Eq '^\s*cert_endpoint_mode="\$\{requested_cert_endpoint_mode,,\}"$' "$script_path"
        The status should eq 0
    End

    It 'exits early in ca-refresh mode after certificate refresh logic'
        When run grep -Eq '^if \[ "\$action" = "ca-refresh" \]; then$' "$script_path"
        The status should eq 0

        When run grep -Eq '^\s*exit$' "$script_path"
        The status should eq 0
    End

    It 'passes cert endpoint mode into cron refresh command'
        When run grep -Eq 'ca-refresh "\$cert_endpoint_mode"' "$script_path"
        The status should eq 0
    End

    It 'passes cert endpoint mode into systemd refresh command'
        When run grep -Eq '^ExecStart=\$script_path ca-refresh \$cert_endpoint_mode$' "$script_path"
        The status should eq 0
    End
End
