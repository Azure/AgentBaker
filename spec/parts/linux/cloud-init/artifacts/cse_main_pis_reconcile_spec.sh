#!/bin/bash

# PIS nodes skip basePrep. Per-cluster files must be written from provisioning CustomData in
# nodePrep and must not be captured in the image.

Describe 'cse_main.sh PIS-safe configuration'
    CSE_MAIN="./parts/linux/cloud-init/artifacts/cse_main.sh"

    phase_body() {
        awk -v name="${1}" '
            $0 ~ "^function " name "[[:space:]]*[{]" { inside = 1 }
            inside { print }
            inside && /^}/ { exit }
        ' "${CSE_MAIN}"
    }

    code_lines() {
        grep -v '^[[:space:]]*#'
    }

    phase_count() {
        phase_body "${1}" | code_lines | grep -c -- "${2}" || true
    }

    script_count() {
        code_lines < "${CSE_MAIN}" | grep -c -- "${1}" || true
    }

    node_prep_event_calls() {
        phase_body "nodePrep" | code_lines |
            sed -n 's/^[[:space:]]*logs_to_events "[^"]*" \([A-Za-z_][A-Za-z0-9_]*\).*/\1/p'
    }

    dispatch_calls() {
        sed -n '/^# The provisioning is split into two gated stages/,$p' "${CSE_MAIN}" |
            code_lines |
            grep -E '^if .*base_prep.complete|^[[:space:]]*(basePrep|nodePrep)$|^if .*PRE_PROVISION_ONLY'
    }

    Describe 'cloud provider config and cluster CA'
        It 'does not write per-cluster files in basePrep'
            base_prep_counts() {
                phase_count "basePrep" "configureAzureJson"
                phase_count "basePrep" "ensureKubeCACert"
            }
            When call base_prep_counts
            The line 1 of output should equal "0"
            The line 2 of output should equal "0"
        End

        It 'writes each per-cluster file exactly once in nodePrep'
            node_prep_counts() {
                phase_count "nodePrep" "configureAzureJson"
                phase_count "nodePrep" "ensureKubeCACert"
            }
            When call node_prep_counts
            The line 1 of output should equal "1"
            The line 2 of output should equal "1"
        End

        It 'has no duplicate writer elsewhere in the script'
            writer_totals() {
                script_count "configureAzureJson"
                script_count "ensureKubeCACert"
            }
            When call writer_totals
            The line 1 of output should equal "1"
            The line 2 of output should equal "1"
        End

        It 'writes azure.json and ca.crt before other nodePrep event calls'
            When call node_prep_event_calls
            The line 1 of output should equal "configureAzureJson"
            The line 2 of output should equal "ensureKubeCACert"
        End

        It 'writes azure.json before secure TLS bootstrap consumes it'
            node_prep_order() {
                phase_body "nodePrep" | code_lines | grep -n -E 'configureAzureJson|configureAndEnableSecureTLSBootstrapping'
            }
            When call node_prep_order
            The line 1 of output should include "configureAzureJson"
            The line 2 of output should include "configureAndEnableSecureTLSBootstrapping"
        End

        It 'writes ca.crt before the API server check consumes it'
            node_prep_order() {
                phase_body "nodePrep" | code_lines | grep -n -E 'ensureKubeCACert|cacert /etc/kubernetes/certs/ca.crt'
            }
            When call node_prep_order
            The line 1 of output should include "ensureKubeCACert"
            The line 2 of output should include "cacert /etc/kubernetes/certs/ca.crt"
        End

        It 'writes azure.json before kubelet starts'
            node_prep_order() {
                phase_body "nodePrep" | code_lines | grep -n -E 'configureAzureJson|ensureKubelet$'
            }
            When call node_prep_order
            The line 1 of output should include "configureAzureJson"
            The line 2 of output should include "ensureKubelet"
        End
    End

    Describe 'stage gate'
        It 'skips basePrep for cached images and nodePrep for image creation'
            When call dispatch_calls
            The line 1 of output should equal 'if [ ! -f /opt/azure/containers/base_prep.complete ]; then'
            The line 2 of output should equal '    basePrep'
            The line 3 of output should equal 'if [ "${PRE_PROVISION_ONLY}" != "true" ]; then'
            The line 4 of output should equal '    nodePrep'
            The lines of output should equal 4
        End
    End
End
