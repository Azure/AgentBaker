#!/usr/bin/env python3
"""
Detects changed provisioning scripts and injects corresponding write_files
entries into the EnableScriptlessCSECmd section of nodecustomdata.yml.

Usage: python3 hack/hotfix_generate.py <base_ref>
  base_ref: the git ref to diff against (e.g., official/v20260219)

This script is called by the hotfix-generate GH Action.

Note: once we move to the final stage of scriptless, we no longer need to hotfix
the scripts into the template and can remove this script entirely.
"""

import json
import subprocess
import sys
import re

TEMPLATE = "parts/linux/cloud-init/nodecustomdata.yml"
ARTIFACTS_DIR = "parts/linux/cloud-init/artifacts"
SIG_VERSION_PATH = "pkg/agent/datamodel/linux_sig_version.json"
HOTFIX_STAGING_DIR = "/opt/azure/hotfix/scripts"
HOTFIX_MANIFEST_PATH = f"{HOTFIX_STAGING_DIR}/manifest.json"

# Map from source file paths (relative to artifacts/) to the GetVariableProperty
# keys used in nodecustomdata.yml. Only scripts that appear as write_files entries
# in the traditional section are included.
SOURCE_TO_VARKEY = {
    # CSE helpers — base (non-distro)
    "cse_helpers.sh": "provisionSource",
    # CSE helpers — distro variants (all map to the same conditional block)
    "ubuntu/cse_helpers_ubuntu.sh": "provisionSourceUbuntu",
    "mariner/cse_helpers_mariner.sh": "provisionSourceMariner",
    "azlosguard/cse_helpers_osguard.sh": "provisionSourceAzlOSGuard",
    "flatcar/cse_helpers_flatcar.sh": "provisionSourceFlatcar",
    "acl/cse_helpers_acl.sh": "provisionSourceACL",
    # CSE install — base
    "cse_install.sh": "provisionInstalls",
    # CSE install — distro variants
    "ubuntu/cse_install_ubuntu.sh": "provisionInstallsUbuntu",
    "mariner/cse_install_mariner.sh": "provisionInstallsMariner",
    "azlosguard/cse_install_osguard.sh": "provisionInstallsAzlOSGuard",
    "flatcar/cse_install_flatcar.sh": "provisionInstallsFlatcar",
    "acl/cse_install_acl.sh": "provisionInstallsACL",
    # CSE config
    "cse_config.sh": "provisionConfigs",
    # CSE main / start
    "cse_main.sh": "provisionScript",
    "cse_start.sh": "provisionStartScript",
    # Python scripts
    "cse_redact_cloud_config.py": "provisionRedactCloudConfig",
    "cse_send_logs.py": "provisionSendLogs",
    # Other scripts
    "reconcile-private-hosts.sh": "reconcilePrivateHostsScript",
    "bind-mount.sh": "bindMountScript",
    "mig-partition.sh": "migPartitionScript",
    "enable-dhcpv6.sh": "dhcpv6ConfigurationScript",
    "ensure_imds_restriction.sh": "ensureIMDSRestrictionScript",
    "ensure-no-dup.sh": "ensureNoDupEbtablesScript",
    "cloud-init-status-check.sh": "cloudInitStatusCheckScript",
    "measure-tls-bootstrapping-latency.sh": "measureTLSBootstrappingLatencyScript",
    "validate-kubelet-credentials.sh": "validateKubeletCredentialsScript",
    "setup-custom-search-domains.sh": "customSearchDomainsScript",
    "configure-azure-network.sh": "configureAzureNetworkScript",
    "init-aks-custom-cloud.sh": "initAKSCustomCloud",
    "init-aks-custom-cloud-mariner.sh": "initAKSCustomCloud",
    "init-aks-custom-cloud-operation-requests.sh": "initAKSCustomCloud",
    "init-aks-custom-cloud-operation-requests-mariner.sh": "initAKSCustomCloud",
    # Distro-specific scripts
    "ubuntu/ubuntu-snapshot-update.sh": "snapshotUpdateScript",
    "mariner/mariner-package-update.sh": "packageUpdateScriptMariner",
    # Systemd services
    "kubelet.service": "kubeletSystemdService",
    "reconcile-private-hosts.service": "reconcilePrivateHostsService",
    "bind-mount.service": "bindMountSystemdService",
    "dhcpv6.service": "dhcpv6SystemdService",
    "mig-partition.service": "migPartitionSystemdService",
    "secure-tls-bootstrap.service": "secureTLSBootstrapService",
    "ensure-no-dup.service": "ensureNoDupEbtablesService",
    "measure-tls-bootstrapping-latency.service": "measureTLSBootstrappingLatencyService",
    "ubuntu/snapshot-update.service": "snapshotUpdateService",
    "ubuntu/snapshot-update.timer": "snapshotUpdateTimer",
    "mariner/package-update.service": "packageUpdateServiceMariner",
    "mariner/package-update.timer": "packageUpdateTimerMariner",
    "99-azure-network.rules": "azureNetworkUdevRule",
    # Component manifest
    "manifest.json": "componentManifestFile",
}

# Distro-variant variable keys that share a single conditional write_files block.
# When any variant in a group changes, the entire block (with all conditionals) is injected.
VARKEY_TO_BLOCK_GROUP = {
    "provisionSourceUbuntu": "helpers_distro",
    "provisionSourceMariner": "helpers_distro",
    "provisionSourceAzlOSGuard": "helpers_distro",
    "provisionSourceFlatcar": "helpers_distro",
    "provisionSourceACL": "helpers_distro",
    "provisionInstallsUbuntu": "install_distro",
    "provisionInstallsMariner": "install_distro",
    "provisionInstallsAzlOSGuard": "install_distro",
    "provisionInstallsFlatcar": "install_distro",
    "provisionInstallsACL": "install_distro",
}

# Map from varkey to the real destination path on the node.
# These must match the constants in pkg/agent/const.go.
# For distro-variant scripts, all variants share the same destination path
# (the distro-specific content is selected at template render time).
VARKEY_TO_DEST_PATH = {
    "provisionSource": "/opt/azure/containers/provision_source.sh",
    "provisionSourceUbuntu": "/opt/azure/containers/provision_source_distro.sh",
    "provisionSourceMariner": "/opt/azure/containers/provision_source_distro.sh",
    "provisionSourceAzlOSGuard": "/opt/azure/containers/provision_source_distro.sh",
    "provisionSourceFlatcar": "/opt/azure/containers/provision_source_distro.sh",
    "provisionSourceACL": "/opt/azure/containers/provision_source_distro.sh",
    "provisionInstalls": "/opt/azure/containers/provision_installs.sh",
    "provisionInstallsUbuntu": "/opt/azure/containers/provision_installs_distro.sh",
    "provisionInstallsMariner": "/opt/azure/containers/provision_installs_distro.sh",
    "provisionInstallsAzlOSGuard": "/opt/azure/containers/provision_installs_distro.sh",
    "provisionInstallsFlatcar": "/opt/azure/containers/provision_installs_distro.sh",
    "provisionInstallsACL": "/opt/azure/containers/provision_installs_distro.sh",
    "provisionConfigs": "/opt/azure/containers/provision_configs.sh",
    "provisionScript": "/opt/azure/containers/provision.sh",
    "provisionStartScript": "/opt/azure/containers/provision_start.sh",
    "provisionRedactCloudConfig": "/opt/azure/containers/provision_redact_cloud_config.py",
    "provisionSendLogs": "/opt/azure/containers/provision_send_logs.py",
    "reconcilePrivateHostsScript": "/opt/azure/containers/reconcilePrivateHosts.sh",
    "bindMountScript": "/opt/azure/containers/bind-mount.sh",
    "migPartitionScript": "/opt/azure/containers/mig-partition.sh",
    "dhcpv6ConfigurationScript": "/opt/azure/containers/enable-dhcpv6.sh",
    "ensureIMDSRestrictionScript": "/opt/azure/containers/ensure_imds_restriction.sh",
    "ensureNoDupEbtablesScript": "/opt/azure/containers/ensure-no-dup.sh",
    "cloudInitStatusCheckScript": "/opt/azure/containers/cloud-init-status-check.sh",
    "measureTLSBootstrappingLatencyScript": "/opt/azure/containers/measure-tls-bootstrapping-latency.sh",
    "validateKubeletCredentialsScript": "/opt/azure/containers/validate-kubelet-credentials.sh",
    "customSearchDomainsScript": "/opt/azure/containers/setup-custom-search-domains.sh",
    "configureAzureNetworkScript": "/opt/azure-network/configure-azure-network.sh",
    "initAKSCustomCloud": "/opt/azure/containers/init-aks-custom-cloud.sh",
    "snapshotUpdateScript": "/opt/azure/containers/ubuntu-snapshot-update.sh",
    "packageUpdateScriptMariner": "/opt/azure/containers/mariner-package-update.sh",
    "kubeletSystemdService": "/etc/systemd/system/kubelet.service",
    "reconcilePrivateHostsService": "/etc/systemd/system/reconcile-private-hosts.service",
    "bindMountSystemdService": "/etc/systemd/system/bind-mount.service",
    "dhcpv6SystemdService": "/etc/systemd/system/dhcpv6.service",
    "migPartitionSystemdService": "/etc/systemd/system/mig-partition.service",
    "secureTLSBootstrapService": "/etc/systemd/system/secure-tls-bootstrap.service",
    "ensureNoDupEbtablesService": "/etc/systemd/system/ensure-no-dup.service",
    "measureTLSBootstrappingLatencyService": "/etc/systemd/system/measure-tls-bootstrapping-latency.service",
    "snapshotUpdateService": "/etc/systemd/system/snapshot-update.service",
    "snapshotUpdateTimer": "/etc/systemd/system/snapshot-update.timer",
    "packageUpdateServiceMariner": "/etc/systemd/system/snapshot-update.service",
    "packageUpdateTimerMariner": "/etc/systemd/system/snapshot-update.timer",
    "azureNetworkUdevRule": "/etc/udev/rules.d/99-azure-network.rules",
    "componentManifestFile": "/opt/azure/manifest.json",
}


def read_target_version():
    """Read the VHD image version from linux_sig_version.json."""
    with open(SIG_VERSION_PATH) as f:
        return json.load(f)["version"]


def detect_changed_varkeys(base_ref):
    """Detect changed scripts via git diff and return the set of varkeys to inject."""
    result = subprocess.run(
        ["git", "diff", "--name-only", base_ref, "--", f"{ARTIFACTS_DIR}/"],
        capture_output=True, text=True, check=True,
    )
    changed_files = result.stdout.strip()
    if not changed_files:
        print("No changed scripts detected. Nothing to do.")
        return set()

    print("Changed files:")
    print(changed_files)
    print()

    matched_varkeys = set()
    matched_block_groups = set()

    for filepath in changed_files.splitlines():
        local_path = filepath.removeprefix(f"{ARTIFACTS_DIR}/")
        if local_path in SOURCE_TO_VARKEY:
            varkey = SOURCE_TO_VARKEY[local_path]
            matched_varkeys.add(varkey)
            if varkey in VARKEY_TO_BLOCK_GROUP:
                matched_block_groups.add(VARKEY_TO_BLOCK_GROUP[varkey])
            print(f"  Matched: {local_path} → {varkey}")
        else:
            print(f"  Warning: {local_path} has no mapping in SOURCE_TO_VARKEY (skipped)")

    if not matched_varkeys:
        print("No matched variable keys. Nothing to inject.")
        return set()

    # If a distro block group was matched, add all members of that group
    for varkey, group in VARKEY_TO_BLOCK_GROUP.items():
        if group in matched_block_groups:
            matched_varkeys.add(varkey)

    print(f"\nVariable keys to inject: {' '.join(sorted(matched_varkeys))}")
    return matched_varkeys


def find_block_boundaries(lines):
    """Find the EnableScriptlessCSECmd / else / end block boundaries."""
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
        if re.match(r'\{\{-?\s*end\s*-?\}\}$', stripped):
            end_line = i
            break

    if else_line is not None and end_line is not None and end_line <= else_line:
        end_line = None

    return scriptless_start, else_line, end_line


def parse_write_files_blocks(traditional_lines):
    """Parse write_files blocks from the traditional section.

    Each block is either a simple '- path:' entry or an entire conditional
    block (e.g., {{if IsAzlOSGuard}}...{{end}}) treated as a single unit.

    Returns a list of (varkeys_set, lines_list) tuples.
    """
    blocks = []
    current_block = []
    current_varkeys = set()
    in_block = False
    conditional_depth = 0

    for line in traditional_lines:
        stripped = line.strip()

        # Track conditional nesting depth
        if re.match(r'\{\{-?\s*if\s+', stripped):
            conditional_depth += 1
        if re.match(r'\{\{-?\s*end\s*-?\}\}', stripped):
            conditional_depth -= 1

        # Detect start of a new top-level write_files entry
        is_path_line = stripped.startswith('- path:')
        # Distro conditionals in the template are unindented, while nested
        # conditionals inside write_files entries are indented.
        is_unindented = not line[0:1].isspace() if line else False
        is_conditional_start = (conditional_depth == 1 and is_unindented and re.match(r'\{\{-?\s*if\s+', stripped))

        start_new = False
        if conditional_depth == 0 and is_path_line:
            start_new = True
        elif is_conditional_start:
            start_new = True

        if start_new:
            if current_block and current_varkeys:
                blocks.append((current_varkeys.copy(), list(current_block)))
            current_block = []
            current_varkeys = set()
            in_block = True

        if in_block:
            current_block.append(line)
            match = re.search(r'GetVariableProperty\s+"cloudInitData"\s+"(\w+)"', stripped)
            if match:
                current_varkeys.add(match.group(1))

    if current_block and current_varkeys:
        blocks.append((current_varkeys.copy(), list(current_block)))

    return blocks


def rewrite_block_to_staging(block_lines, varkeys):
    """Rewrite a write_files block to use staging paths instead of real destination paths.

    For each '- path:' line, replace the path (which may be a Go template function call)
    with a staging path under HOTFIX_STAGING_DIR. The staging filename is derived from
    the destination path basename.

    Returns (rewritten_lines, staging_to_dest_mapping) where the mapping is a list of
    (staging_path, dest_path) tuples for the manifest.
    """
    rewritten = []
    manifest_entries = []

    # Collect unique destination paths from the varkeys in this block
    dest_paths = set()
    for vk in varkeys:
        if vk in VARKEY_TO_DEST_PATH:
            dest_paths.add(VARKEY_TO_DEST_PATH[vk])

    for line in block_lines:
        stripped = line.strip()
        if stripped.startswith('- path:'):
            # Extract the path value (may be a Go template expression or literal)
            path_value = stripped[len('- path:'):].strip()

            # Find which dest path this corresponds to by matching template functions
            matched_dest = None
            for vk in varkeys:
                if vk in VARKEY_TO_DEST_PATH:
                    dest = VARKEY_TO_DEST_PATH[vk]
                    # Check if this path line could resolve to this dest
                    # Template functions like {{GetCSEInstallScriptFilepath}} resolve to the dest
                    basename = dest.rsplit('/', 1)[-1]
                    staging = f"{HOTFIX_STAGING_DIR}/{basename}"
                    if dest not in [e[1] for e in manifest_entries]:
                        matched_dest = dest
                        manifest_entries.append((staging, dest))
                        break

            if matched_dest:
                basename = matched_dest.rsplit('/', 1)[-1]
                staging_path = f"{HOTFIX_STAGING_DIR}/{basename}"
                rewritten.append(f"- path: {staging_path}\n")
            else:
                rewritten.append(line)
        else:
            rewritten.append(line)

    return rewritten, manifest_entries


def inject_hotfix(target_varkeys):
    """Extract matching write_files blocks from traditional section and inject into scriptless section.

    Scripts are written to staging paths under /opt/azure/hotfix/scripts/ along with a
    manifest.json that maps staging paths to real destinations. ANC reads the manifest
    at boot, checks the target version against its own VHD version, and copies files
    only when the version matches.
    """
    target_version = read_target_version()
    print(f"Target VHD version for hotfix: {target_version}", file=sys.stderr)

    with open(TEMPLATE, 'r') as f:
        content = f.read()

    # Remove any previous hotfix entries (idempotent)
    content = re.sub(
        r'\n# ---- hotfix: auto-generated by hotfix-generate GH Action ----\n.*?# ---- end hotfix ----\n',
        '',
        content,
        flags=re.DOTALL,
    )

    lines = content.splitlines(keepends=True)

    scriptless_start, else_line, end_line = find_block_boundaries(lines)
    if scriptless_start is None or else_line is None or end_line is None:
        print(f"ERROR: Could not find EnableScriptlessCSECmd block boundaries", file=sys.stderr)
        print(f"  scriptless_start={scriptless_start}, else_line={else_line}, end_line={end_line}", file=sys.stderr)
        sys.exit(1)

    print(f"\nTemplate structure:", file=sys.stderr)
    print(f"  EnableScriptlessCSECmd block: lines {scriptless_start+1}-{else_line+1}", file=sys.stderr)
    print(f"  Traditional block: lines {else_line+2}-{end_line+1}", file=sys.stderr)

    traditional_lines = lines[else_line+1:end_line]
    blocks = parse_write_files_blocks(traditional_lines)
    print(f"Found {len(blocks)} write_files blocks in traditional section", file=sys.stderr)

    selected_blocks = []
    for varkeys, block_lines in blocks:
        if varkeys & target_varkeys:
            selected_blocks.append((varkeys, block_lines))
            print(f"  Selected block with varkeys: {varkeys}", file=sys.stderr)

    if not selected_blocks:
        print("No matching write_files blocks found for the target varkeys.", file=sys.stderr)
        return False

    # Build staging blocks and manifest entries
    all_manifest_entries = []
    staged_blocks = []
    for varkeys, block_lines in selected_blocks:
        rewritten, entries = rewrite_block_to_staging(block_lines, varkeys)
        staged_blocks.append(rewritten)
        all_manifest_entries.extend(entries)

    # Deduplicate manifest entries (distro variants share the same dest path)
    seen_dests = set()
    unique_entries = []
    for staging, dest in all_manifest_entries:
        if dest not in seen_dests:
            seen_dests.add(dest)
            unique_entries.append({"staging": staging, "destination": dest})

    manifest = {"targetVersion": target_version, "files": unique_entries}
    manifest_json = json.dumps(manifest, separators=(',', ':'))

    hotfix_lines = [
        "\n",
        f"# ---- hotfix: auto-generated by hotfix-generate GH Action (target={target_version}) ----\n",
        f"- path: {HOTFIX_MANIFEST_PATH}\n",
        '  permissions: "0644"\n',
        "  owner: root\n",
        f"  content: |\n",
        f"    {manifest_json}\n",
        "\n",
    ]
    for block_lines in staged_blocks:
        hotfix_lines.extend(block_lines)
    hotfix_lines.append(f"# ---- end hotfix ----\n")

    final_lines = lines[:else_line] + hotfix_lines + lines[else_line:]

    with open(TEMPLATE, 'w') as f:
        f.writelines(final_lines)

    print(f"\nInjected {len(staged_blocks)} write_files block(s) to staging paths", file=sys.stderr)
    print(f"Manifest: {manifest_json}", file=sys.stderr)
    print(f"Updated {TEMPLATE}", file=sys.stderr)
    return True


def main():
    if len(sys.argv) < 2:
        print("Usage: python3 hack/hotfix_generate.py <base_ref>", file=sys.stderr)
        sys.exit(1)

    base_ref = sys.argv[1]
    print(f"Diffing against {base_ref} for changed scripts in {ARTIFACTS_DIR}/...")

    target_varkeys = detect_changed_varkeys(base_ref)
    if not target_varkeys:
        sys.exit(0)

    changed = inject_hotfix(target_varkeys)
    if changed:
        print("\nDone. Template updated.")
    else:
        print("\nNo template changes needed.")


if __name__ == '__main__':
    main()
