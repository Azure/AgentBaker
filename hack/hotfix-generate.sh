#!/usr/bin/env bash
# hotfix-generate.sh
#
# Detects changed provisioning scripts and injects corresponding write_files
# entries into the EnableScriptlessCSECmd section of nodecustomdata.yml.
#
# Usage: hack/hotfix-generate.sh <base_ref>
#   base_ref: the git ref to diff against (e.g., official/v20260219)
#
# This script is called by the hotfix-generate GH Action.
#
# Note: once we move to the final stage of scriptless, we no longer need to hotfix
# the scripts into the template and can remove this script entirely.

set -euo pipefail

TEMPLATE="parts/linux/cloud-init/nodecustomdata.yml"
ARTIFACTS_DIR="parts/linux/cloud-init/artifacts"
BASE_REF="${1:?Usage: $0 <base_ref>}"

# Map from source file paths (relative to artifacts/) to the GetVariableProperty
# keys used in nodecustomdata.yml. Only scripts that appear as write_files entries
# in the traditional section are included.
#
# Format: "source_path:variable_property_key"
# For distro-variant scripts, we need to map to the block that contains the
# conditional (IsMariner/IsFlatcar/etc.) since they share the same write_files entry.
declare -A SOURCE_TO_VARKEY=(
    # CSE helpers — base (non-distro)
    ["cse_helpers.sh"]="provisionSource"
    # CSE helpers — distro variants (all map to the same conditional block)
    ["ubuntu/cse_helpers_ubuntu.sh"]="provisionSourceUbuntu"
    ["mariner/cse_helpers_mariner.sh"]="provisionSourceMariner"
    ["azlosguard/cse_helpers_osguard.sh"]="provisionSourceAzlOSGuard"
    ["flatcar/cse_helpers_flatcar.sh"]="provisionSourceFlatcar"
    ["acl/cse_helpers_acl.sh"]="provisionSourceACL"
    # CSE install — base
    ["cse_install.sh"]="provisionInstalls"
    # CSE install — distro variants
    ["ubuntu/cse_install_ubuntu.sh"]="provisionInstallsUbuntu"
    ["mariner/cse_install_mariner.sh"]="provisionInstallsMariner"
    ["azlosguard/cse_install_osguard.sh"]="provisionInstallsAzlOSGuard"
    ["flatcar/cse_install_flatcar.sh"]="provisionInstallsFlatcar"
    ["acl/cse_install_acl.sh"]="provisionInstallsACL"
    # CSE config
    ["cse_config.sh"]="provisionConfigs"
    # CSE main / start
    ["cse_main.sh"]="provisionScript"
    ["cse_start.sh"]="provisionStartScript"
    # Python scripts
    ["cse_redact_cloud_config.py"]="provisionRedactCloudConfig"
    ["cse_send_logs.py"]="provisionSendLogs"
    # Other scripts
    ["reconcile-private-hosts.sh"]="reconcilePrivateHostsScript"
    ["bind-mount.sh"]="bindMountScript"
    ["mig-partition.sh"]="migPartitionScript"
    ["enable-dhcpv6.sh"]="dhcpv6ConfigurationScript"
    ["ensure_imds_restriction.sh"]="ensureIMDSRestrictionScript"
    ["ensure-no-dup.sh"]="ensureNoDupEbtablesScript"
    ["cloud-init-status-check.sh"]="cloudInitStatusCheckScript"
    ["measure-tls-bootstrapping-latency.sh"]="measureTLSBootstrappingLatencyScript"
    ["validate-kubelet-credentials.sh"]="validateKubeletCredentialsScript"
    ["setup-custom-search-domains.sh"]="customSearchDomainsScript"
    ["configure-azure-network.sh"]="configureAzureNetworkScript"
    ["init-aks-custom-cloud.sh"]="initAKSCustomCloud"
    # Distro-specific scripts
    ["ubuntu/ubuntu-snapshot-update.sh"]="snapshotUpdateScript"
    ["mariner/mariner-package-update.sh"]="packageUpdateScriptMariner"
    # Systemd services
    ["kubelet.service"]="kubeletSystemdService"
    ["reconcile-private-hosts.service"]="reconcilePrivateHostsService"
    ["bind-mount.service"]="bindMountSystemdService"
    ["dhcpv6.service"]="dhcpv6SystemdService"
    ["mig-partition.service"]="migPartitionSystemdService"
    ["secure-tls-bootstrap.service"]="secureTLSBootstrapService"
    ["ensure-no-dup.service"]="ensureNoDupEbtablesService"
    ["measure-tls-bootstrapping-latency.service"]="measureTLSBootstrappingLatencyService"
    ["ubuntu/snapshot-update.service"]="snapshotUpdateService"
    ["ubuntu/snapshot-update.timer"]="snapshotUpdateTimer"
    ["mariner/package-update.service"]="packageUpdateServiceMariner"
    ["mariner/package-update.timer"]="packageUpdateTimerMariner"
    ["99-azure-network.rules"]="azureNetworkUdevRule"
)

# Distro-variant variable keys that share a single conditional write_files block.
# When any variant in a group changes, the entire block (with all conditionals) is injected.
declare -A VARKEY_TO_BLOCK_GROUP=(
    ["provisionSourceUbuntu"]="helpers_distro"
    ["provisionSourceMariner"]="helpers_distro"
    ["provisionSourceAzlOSGuard"]="helpers_distro"
    ["provisionSourceFlatcar"]="helpers_distro"
    ["provisionSourceACL"]="helpers_distro"
    ["provisionInstallsUbuntu"]="install_distro"
    ["provisionInstallsMariner"]="install_distro"
    ["provisionInstallsAzlOSGuard"]="install_distro"
    ["provisionInstallsFlatcar"]="install_distro"
    ["provisionInstallsACL"]="install_distro"
)

echo "Diffing against ${BASE_REF} for changed scripts in ${ARTIFACTS_DIR}/..."

changed_files=$(git diff --name-only "${BASE_REF}" -- "${ARTIFACTS_DIR}/")
if [[ -z "${changed_files}" ]]; then
    echo "No changed scripts detected in ${ARTIFACTS_DIR}/. Nothing to do."
    exit 0
fi

echo "Changed files:"
echo "${changed_files}"
echo ""

# Collect unique variable property keys for changed files
declare -A matched_varkeys=()
declare -A matched_block_groups=()

while IFS= read -r filepath; do
    local_path="${filepath#"${ARTIFACTS_DIR}"/}"
    if [[ -n "${SOURCE_TO_VARKEY[${local_path}]+x}" ]]; then
        varkey="${SOURCE_TO_VARKEY[${local_path}]}"
        matched_varkeys["${varkey}"]=1
        # Check if this varkey belongs to a block group
        if [[ -n "${VARKEY_TO_BLOCK_GROUP[${varkey}]+x}" ]]; then
            matched_block_groups["${VARKEY_TO_BLOCK_GROUP[${varkey}]}"]=1
        fi
        echo "  Matched: ${local_path} → ${varkey}"
    else
        echo "  Warning: ${local_path} has no mapping in SOURCE_TO_VARKEY (skipped)"
    fi
done <<< "${changed_files}"

if [[ ${#matched_varkeys[@]} -eq 0 ]]; then
    echo "No matched variable keys. Nothing to inject."
    exit 0
fi

# If a distro block group was matched, add all members of that group
for group in "${!matched_block_groups[@]}"; do
    for varkey in "${!VARKEY_TO_BLOCK_GROUP[@]}"; do
        if [[ "${VARKEY_TO_BLOCK_GROUP[${varkey}]}" == "${group}" ]]; then
            matched_varkeys["${varkey}"]=1
        fi
    done
done

echo ""
echo "Variable keys to inject: ${!matched_varkeys[*]}"

# Extract the write_files blocks from the traditional section of nodecustomdata.yml
# and inject matching ones into the EnableScriptlessCSECmd section.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
python3 "${SCRIPT_DIR}/hotfix_inject.py" "${TEMPLATE}" ${!matched_varkeys[*]}

echo ""
echo "Done. Run 'make generate' to regenerate test data."
