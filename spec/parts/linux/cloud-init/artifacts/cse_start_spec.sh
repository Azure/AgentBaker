#!/bin/bash

# A pre-provision (image bake) run must report its result without leaving a durable
# provision.complete behind: /run is tmpfs, so the volatile marker cannot be captured into the
# image, while a captured provision.complete would make cse_main.sh exit before nodePrep.

Describe 'cse_start.sh completion markers'
    markers_for() {
        root="$(mktemp -d)"
        mkdir -p "${root}/var/log/azure"
        PRE_PROVISION_ONLY="${1}" bash -c "$(
            awk '/^# Create the marker for the completed provisioning stage\.$/,/^fi$/' \
                ./parts/linux/cloud-init/artifacts/cse_start.sh |
                sed "s|/opt/azure/containers|${root}/opt/azure/containers|g; s|/run/azure|${root}/run/azure|g; s|/var/log/azure|${root}/var/log/azure|g"
        )"
        find "${root}/opt/azure/containers" "${root}/run/azure" -type f 2>/dev/null | sed "s|^${root}||" | sort
        rm -rf "${root}"
    }

    It 'writes durable phase state and a volatile result marker for a pre-provision run'
        When call markers_for "true"
        The line 1 of output should equal "/opt/azure/containers/base_prep.complete"
        The line 2 of output should equal "/run/azure/pre-provision.complete"
        The lines of output should equal 2
    End

    It 'writes only provision.complete for a normal provisioning run'
        When call markers_for "false"
        The output should equal "/opt/azure/containers/provision.complete"
    End
End
