#!/bin/bash

# provision.complete is what `aks-node-controller provision-wait` waits for, so it must be written
# for a pre-provision run as well as a normal one. base_prep.complete is durable phase state and is
# written only by a bake, so a node created from the image skips basePrep.

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

# The egress check is telemetry only, so it runs after provision.complete instead of on the boot path.
Describe 'cse_start.sh egress endpoint check'
    CSE_START=./parts/linux/cloud-init/artifacts/cse_start.sh

    # Runs the egress block from cse_start.sh with a stub helpers file and prints its exit code and log.
    egress_check_with() {
        root="$(mktemp -d)"
        mkdir -p "${root}/var/log/azure"
        echo 'checkEgressEndpoints() { echo "checked cloud=$1"; }' > "${root}/helpers.sh"
        env CSE_HELPERS_FILEPATH="${root}/helpers.sh" EGRESS_AZ_ENVIRONMENT=AzurePublicCloud OUTBOUND_COMMAND="curl x" "$@" bash -c "$(
            awk '/^# Telemetry only: records the endpoints/{p=1} p{print} p && /^fi$/{exit}' "${CSE_START}" |
                sed "s|/var/log/azure|${root}/var/log/azure|g"
        )"
        echo "exit=$?"
        cat "${root}/var/log/azure/cluster-provision.log" 2>/dev/null
        rm -rf "${root}"
    }

    # Prints "ordered" when the cloud name is saved before the IMDS cache is deleted and the check
    # runs after provision.complete.
    egress_check_order() {
        save=$(grep -n '^EGRESS_AZ_ENVIRONMENT=' "${CSE_START}" | cut -d: -f1)
        cleanup=$(grep -n '^rm -f /opt/azure/containers/imds_instance_metadata_cache.json' "${CSE_START}" | cut -d: -f1)
        marker=$(grep -n '^touch /opt/azure/containers/provision.complete$' "${CSE_START}" | cut -d: -f1)
        check=$(grep -n 'checkEgressEndpoints "' "${CSE_START}" | cut -d: -f1)
        [ "${save}" -lt "${cleanup}" ] && [ "${marker}" -lt "${check}" ] && echo ordered
    }

    It 'checks egress with the saved cloud name on a normal provisioning run'
        When call egress_check_with PRE_PROVISION_ONLY=false
        The line 1 of output should equal "exit=0"
        The line 2 of output should equal "checked cloud=AzurePublicCloud"
    End

    It 'skips the check for a pre-provision bake'
        When call egress_check_with PRE_PROVISION_ONLY=true
        The output should equal "exit=0"
    End

    It 'skips the check for a bootstrap-profile cluster without outbound access'
        When call egress_check_with PRE_PROVISION_ONLY=false BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER=myacr.azurecr.io
        The output should equal "exit=0"
    End

    It 'skips the check when no outbound command is configured'
        When call egress_check_with PRE_PROVISION_ONLY=false OUTBOUND_COMMAND=
        The output should equal "exit=0"
    End

    It 'never fails provisioning when the helpers cannot be loaded'
        When call egress_check_with PRE_PROVISION_ONLY=false CSE_HELPERS_FILEPATH=/nonexistent/helpers.sh
        The line 1 of output should equal "exit=0"
    End

    It 'saves the cloud name before the IMDS cache is deleted and checks after provision.complete'
        When call egress_check_order
        The output should equal "ordered"
    End
End
