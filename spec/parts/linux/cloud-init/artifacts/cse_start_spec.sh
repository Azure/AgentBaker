#!/bin/bash

# Completion markers must let `aks-node-controller provision-wait` report a result for a normal node
# and for a pre-provision (image bake) run, without a bake ever producing durable state that a node
# created from the captured image could mistake for its own result.
#
# - provision.complete  : durable, normal provisioning only.
# - base_prep.complete  : durable phase state, pre-provision only, captured into the image.
# - /run/azure/pre-provision.complete : volatile (tmpfs), pre-provision only, cannot be captured.

Describe 'cse_start.sh completion markers'
    CSE_START="./parts/linux/cloud-init/artifacts/cse_start.sh"

    marker_block() {
        awk '
            /^# Create the marker for the completed provisioning stage\.$/ { inside = 1 }
            inside { print }
            inside && /^fi$/ { exit }
        ' "${CSE_START}"
    }

    markers_for() {
        root="$(mktemp -d)"
        mkdir -p "${root}/var/log/azure"
        PRE_PROVISION_ONLY="${1}" bash -c "$(marker_block | sed "s|/opt/azure/containers|${root}/opt/azure/containers|g; s|/run/azure|${root}/run/azure|g; s|/var/log/azure|${root}/var/log/azure|g")"
        find "${root}/opt/azure/containers" "${root}/run/azure" -type f 2>/dev/null |
            sed "s|^${root}||" | sort
        rm -rf "${root}"
    }

    It 'writes durable phase state and a volatile result marker, and never provision.complete, for a pre-provision run'
        When call markers_for "true"
        The line 1 of output should equal "/opt/azure/containers/base_prep.complete"
        The line 2 of output should equal "/run/azure/pre-provision.complete"
        The lines of output should equal 2
    End

    It 'writes only provision.complete, and no volatile marker, for a normal provisioning run'
        When call markers_for "false"
        The output should equal "/opt/azure/containers/provision.complete"
    End

    It 'writes only provision.complete when PRE_PROVISION_ONLY is unset'
        When call markers_for ""
        The output should equal "/opt/azure/containers/provision.complete"
    End
End
