#!/bin/bash

# provision.complete is the single signal that a CSE run finished. It must be written for a
# pre-provision (image bake) run and for a normal node provisioning run, so that
# `aks-node-controller provision-wait` reports the result of both.
# base_prep.complete is durable phase state and is written only by the pre-provision run.

Describe 'cse_start.sh completion markers'
    CSE_START="./parts/linux/cloud-init/artifacts/cse_start.sh"

    marker_block() {
        awk '
            /^# Create the markers for the completed provisioning stage\.$/ { inside = 1 }
            inside { print }
            inside && /^touch .*\/provision\.complete$/ { exit }
        ' "${CSE_START}"
    }

    markers_for() {
        root="$(mktemp -d)"
        mkdir -p "${root}/var/log/azure"
        PRE_PROVISION_ONLY="${1}" bash -c "$(marker_block | sed "s|/opt/azure/containers|${root}/opt/azure/containers|g; s|/var/log/azure|${root}/var/log/azure|g")"
        ls "${root}/opt/azure/containers" | sort
        rm -rf "${root}"
    }

    It 'writes both markers for a pre-provision run'
        When call markers_for "true"
        The line 1 of output should equal "base_prep.complete"
        The line 2 of output should equal "provision.complete"
        The lines of output should equal 2
    End

    It 'writes only provision.complete for a normal provisioning run'
        When call markers_for "false"
        The output should equal "provision.complete"
    End

    It 'writes only provision.complete when PRE_PROVISION_ONLY is unset'
        When call markers_for ""
        The output should equal "provision.complete"
    End
End
