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
    local_path="${filepath#${ARTIFACTS_DIR}/}"
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

# Extract the write_files blocks from the traditional section of nodecustomdata.yml.
# The traditional section is between {{- else }} (line after EnableScriptlessCSECmd block)
# and {{- end}} (last line).
#
# Strategy: use Python for reliable template parsing since the YAML contains Go template
# directives that make it hard to parse with standard YAML tools.
python3 - "${TEMPLATE}" "${!matched_varkeys[*]}" << 'PYTHON_SCRIPT'
import sys
import re

template_path = sys.argv[1]
target_varkeys = set(sys.argv[2].split())

with open(template_path, 'r') as f:
    content = f.read()

# Step 1: Remove any previous hotfix entries (idempotent)
content = re.sub(
    r'\n# ---- hotfix: auto-generated by hotfix-generate GH Action ----\n.*?# ---- end hotfix ----\n',
    '',
    content,
    flags=re.DOTALL
)

lines = content.splitlines(keepends=True)

# Step 2: Find the EnableScriptlessCSECmd block boundaries
scriptless_start = None
else_line = None
end_line = None

for i, line in enumerate(lines):
    stripped = line.strip()
    if '{{if EnableScriptlessCSECmd}}' in stripped or '{{ if EnableScriptlessCSECmd }}' in stripped:
        scriptless_start = i
    elif scriptless_start is not None and else_line is None and stripped.startswith('{{- else'):
        else_line = i

for i in range(len(lines) - 1, -1, -1):
    stripped = lines[i].strip()
    if stripped in ('{{- end}}', '{{end}}', '{{ end }}', '{{- end }}'):
        end_line = i
        break

if scriptless_start is None or else_line is None or end_line is None:
    print(f"ERROR: Could not find EnableScriptlessCSECmd block boundaries", file=sys.stderr)
    print(f"  scriptless_start={scriptless_start}, else_line={else_line}, end_line={end_line}", file=sys.stderr)
    sys.exit(1)

print(f"Template structure:", file=sys.stderr)
print(f"  EnableScriptlessCSECmd block: lines {scriptless_start+1}-{else_line+1}", file=sys.stderr)
print(f"  Traditional block: lines {else_line+2}-{end_line+1}", file=sys.stderr)

# Step 3: Extract write_files blocks from the traditional section
traditional_lines = lines[else_line+1:end_line]

# Parse individual write_files entries. Each entry starts with "- path:" and ends
# before the next "- path:" or "{{" conditional block.
# We need to capture entire conditional blocks (e.g., {{if IsAzlOSGuard}}...{{end}})
# as single units since they represent one logical write_files entry with distro variants.

blocks = []
current_block = []
current_varkeys = set()
in_block = False
conditional_depth = 0

for line in traditional_lines:
    stripped = line.strip()

    # Track conditional nesting depth
    # Opening: {{if ...}} or {{ if ...}}
    if re.match(r'\{\{-?\s*if\s+', stripped):
        conditional_depth += 1
    # Closing: {{end}} or {{- end}} (but NOT {{- else if ...}})
    if re.match(r'\{\{-?\s*end\s*\}\}', stripped):
        conditional_depth -= 1

    # Detect start of a new top-level write_files entry
    is_path_line = stripped.startswith('- path:')
    # Detect start of a top-level conditional block (only at depth 0 before increment)
    is_conditional_start = (conditional_depth == 1 and re.match(r'\{\{-?\s*if\s+Is', stripped))
    # Other conditionals ({{ if not ...) at top level
    is_other_conditional_start = (conditional_depth == 1 and re.match(r'\{\{-?\s*if\s+', stripped) and not re.match(r'\{\{-?\s*if\s+Is', stripped))

    # Start a new block only at top level (not inside a conditional)
    start_new = False
    if conditional_depth == 0 and is_path_line:
        start_new = True
    elif is_conditional_start:
        start_new = True
    elif is_other_conditional_start:
        start_new = True

    if start_new:
        # Save previous block if any
        if current_block and current_varkeys:
            blocks.append((current_varkeys.copy(), list(current_block)))
        current_block = []
        current_varkeys = set()
        in_block = True

    if in_block:
        current_block.append(line)
        # Detect GetVariableProperty references
        match = re.search(r'GetVariableProperty\s+"cloudInitData"\s+"(\w+)"', stripped)
        if match:
            current_varkeys.add(match.group(1))

# Don't forget the last block
if current_block and current_varkeys:
    blocks.append((current_varkeys.copy(), list(current_block)))

print(f"Found {len(blocks)} write_files blocks in traditional section", file=sys.stderr)

# Select blocks that contain any of the target varkeys
selected_blocks = []
for varkeys, block_lines in blocks:
    if varkeys & target_varkeys:
        selected_blocks.append(block_lines)
        print(f"  Selected block with varkeys: {varkeys}", file=sys.stderr)

if not selected_blocks:
    print("No matching write_files blocks found for the target varkeys.", file=sys.stderr)
    sys.exit(0)

# Step 5: Build the hotfix insertion text
hotfix_lines = []
hotfix_lines.append("\n")
hotfix_lines.append("# ---- hotfix: auto-generated by hotfix-generate GH Action ----\n")
for block_lines in selected_blocks:
    hotfix_lines.extend(block_lines)
hotfix_lines.append("# ---- end hotfix ----\n")

# Step 6: Insert before {{- else }} line
final_lines = lines[:else_line] + hotfix_lines + lines[else_line:]

with open(template_path, 'w') as f:
    f.writelines(final_lines)

print(f"\nInjected {len(selected_blocks)} write_files block(s) into EnableScriptlessCSECmd section", file=sys.stderr)
print(f"Updated {template_path}", file=sys.stderr)
PYTHON_SCRIPT

echo ""
echo "Done. Run 'make generate' to regenerate test data."
