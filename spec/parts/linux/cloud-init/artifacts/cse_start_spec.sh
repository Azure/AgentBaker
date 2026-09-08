#!/bin/bash

# provision.complete is what `aks-node-controller provision-wait` waits for, so it must be written
# for a pre-provision (image bake) run as well as a normal one. base_prep.complete is durable phase
# state and is written only by a bake, so a node created from the image skips basePrep.

Describe 'cse_start.sh completion markers'
    markers_for() {
        root="$(mktemp -d)"
        mkdir -p "${root}/var/log/azure"
        PRE_PROVISION_ONLY="${1}" bash -c "$(
            awk '/^# Create the marker for the completed provisioning stage\.$/,/^touch .*provision\.complete$/' \
                ./parts/linux/cloud-init/artifacts/cse_start.sh |
                sed "s|/opt/azure/containers|${root}/opt/azure/containers|g; s|/var/log/azure|${root}/var/log/azure|g"
        )"
        ls "${root}/opt/azure/containers" | sort
        rm -rf "${root}"
    }

    It 'writes base_prep.complete and provision.complete for a pre-provision run'
        When call markers_for "true"
        The line 1 of output should equal "base_prep.complete"
        The line 2 of output should equal "provision.complete"
        The lines of output should equal 2
    End

    It 'writes only provision.complete for a normal provisioning run'
        When call markers_for "false"
        The output should equal "provision.complete"
    End
End
