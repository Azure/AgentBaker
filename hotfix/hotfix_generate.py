#!/usr/bin/env python3
"""
Combined ANC hotfix generator.

Auto-detects what needs a hotfix and generates the version numbers for it:

1. If aks-node-controller/ (the Go module) has changes vs the base branch, bumps the
   patch of the current pkg/agent/datamodel/linux_sig_version.json version to the
   first patch number that isn't already tagged in the repo (e.g. 202607.02.0 ->
   202607.02.1, or .2/.3/... if those tags already exist), and uses that as `version`.

2. Detects which CSE provisioning scripts changed vs the base branch and injects their
   write_files entries into the EnableScriptlessCSECmd section of
   parts/linux/cloud-init/nodecustomdata.yml. If that injection (or a direct edit)
   leaves nodecustomdata.yml different from the base branch, `scripts_version` is
   bumped using the same base-version + tag-collision algorithm as `version`.

3. Writes the resulting {"version", "scripts_version"} (omitting fields that don't
   apply) to parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.json, the
   file pkg/agent/baker.go embeds directly into every scriptless customData render
   (see hotfixJSONFile in pkg/agent/const.go).

Usage: python3 hotfix/hotfix_generate.py <base_ref>
  base_ref: git ref to diff against for changed-script/changed-code detection
            (e.g., origin/official/v20260219)

This script is called by the hotfix-generate GH Action.
"""

import json
import re
import subprocess
import sys

TARGET_FILE = "parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.json"
TEMPLATE = "parts/linux/cloud-init/nodecustomdata.yml"
ARTIFACTS_DIR = "parts/linux/cloud-init/artifacts"
LINUX_SIG_VERSION_FILE = "pkg/agent/datamodel/linux_sig_version.json"
ANC_DIR = "aks-node-controller/"

VERSION_RE = re.compile(r'^\d{6}\.\d{2}\.\d+$')

# Marker comments for idempotent injection of the raw changed-script blocks.
SCRIPTS_BEGIN = "# ---- hotfix-scripts: auto-generated ----"
SCRIPTS_END = "# ---- end hotfix-scripts ----"

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
    "init-aks-cloud.sh": "initAKSCloud",
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


def read_base_version():
    """Read the current released VHD image version, e.g. '202607.02.0'."""
    with open(LINUX_SIG_VERSION_FILE) as f:
        data = json.load(f)
    version = (data.get("version") or "").strip()
    if not VERSION_RE.match(version):
        print(f"ERROR: {LINUX_SIG_VERSION_FILE} has invalid version '{version}', "
              f"expected YYYYMM.DD.PATCH", file=sys.stderr)
        sys.exit(1)
    return version


def tag_exists(tag):
    """Check whether a git tag already exists (locally)."""
    result = subprocess.run(
        ["git", "rev-parse", "-q", "--verify", f"refs/tags/{tag}"],
        capture_output=True,
    )
    return result.returncode == 0


def bump_version(base_version):
    """Bump base_version's patch to the first patch number not already tagged.

    base_version is 'YYYYMM.DD.PATCH' (e.g. '202607.02.0'). Tags are
    'v0.YYYYMMDD.PATCH' (e.g. 'v0.20260702.1'). Returns the new
    'YYYYMM.DD.PATCH' string.
    """
    match = re.match(r'^(\d{6})\.(\d{2})\.\d+$', base_version)
    yyyymm, dd = match.group(1), match.group(2)
    patch = 1
    while True:
        tag = f"v0.{yyyymm}{dd}.{patch}"
        if not tag_exists(tag):
            return f"{yyyymm}.{dd}.{patch}"
        patch += 1


def path_changed(base_ref, path):
    """Return True if path differs between the working tree and base_ref."""
    result = subprocess.run(["git", "diff", "--quiet", base_ref, "--", path])
    return result.returncode != 0


def write_hotfix_file(version, scripts_version):
    """Write the resolved {version, scripts_version} to TARGET_FILE, the file
    pkg/agent/baker.go embeds directly at customData-render time."""
    payload = {}
    if version:
        payload["version"] = version
    if scripts_version:
        payload["scripts_version"] = scripts_version

    with open(TARGET_FILE, "w") as f:
        json.dump(payload, f, indent=4)
        f.write("\n")

    if payload:
        print(f"Wrote {payload} to {TARGET_FILE}", file=sys.stderr)
    else:
        print(f"No active hotfix; reset {TARGET_FILE} to {{}}", file=sys.stderr)


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


def remove_scripts_block():
    """Remove any previously injected hotfix-scripts block (idempotent cleanup)."""
    with open(TEMPLATE) as f:
        content = f.read()

    new_content = re.sub(
        rf'\n?{re.escape(SCRIPTS_BEGIN)}\n.*?{re.escape(SCRIPTS_END)}\n',
        '', content, flags=re.DOTALL,
    )

    if new_content != content:
        with open(TEMPLATE, 'w') as f:
            f.write(new_content)
        print(f"Removed previous hotfix-scripts block from {TEMPLATE}", file=sys.stderr)
        return True
    return False


def inject_scripts(target_varkeys):
    """Extract matching write_files blocks from the traditional section and inject
    them into the scriptless section, replacing any previously injected block."""
    with open(TEMPLATE, 'r') as f:
        content = f.read()

    content = re.sub(
        rf'\n?{re.escape(SCRIPTS_BEGIN)}\n.*?{re.escape(SCRIPTS_END)}\n',
        '', content, flags=re.DOTALL,
    )

    lines = content.splitlines(keepends=True)

    scriptless_start, else_line, end_line = find_block_boundaries(lines)
    if scriptless_start is None or else_line is None or end_line is None:
        print("ERROR: Could not find EnableScriptlessCSECmd block boundaries", file=sys.stderr)
        print(f"  scriptless_start={scriptless_start}, else_line={else_line}, end_line={end_line}", file=sys.stderr)
        sys.exit(1)

    print("\nTemplate structure:", file=sys.stderr)
    print(f"  EnableScriptlessCSECmd block: lines {scriptless_start+1}-{else_line+1}", file=sys.stderr)
    print(f"  Traditional block: lines {else_line+2}-{end_line+1}", file=sys.stderr)

    traditional_lines = lines[else_line+1:end_line]
    blocks = parse_write_files_blocks(traditional_lines)
    print(f"Found {len(blocks)} write_files blocks in traditional section", file=sys.stderr)

    selected_blocks = []
    for varkeys, block_lines in blocks:
        if varkeys & target_varkeys:
            selected_blocks.append(block_lines)
            print(f"  Selected block with varkeys: {varkeys}", file=sys.stderr)

    if not selected_blocks:
        print("No matching write_files blocks found for the target varkeys.", file=sys.stderr)
        return False

    scripts_lines = [
        "\n",
        f"{SCRIPTS_BEGIN}\n",
    ]
    for block_lines in selected_blocks:
        scripts_lines.extend(block_lines)
    scripts_lines.append(f"{SCRIPTS_END}\n")

    final_lines = lines[:else_line] + scripts_lines + lines[else_line:]

    with open(TEMPLATE, 'w') as f:
        f.writelines(final_lines)

    print(f"\nInjected {len(selected_blocks)} write_files block(s) into EnableScriptlessCSECmd section", file=sys.stderr)
    print(f"Updated {TEMPLATE}", file=sys.stderr)
    return True


def main():
    if len(sys.argv) < 2:
        print("Usage: python3 hotfix/hotfix_generate.py <base_ref>", file=sys.stderr)
        sys.exit(1)
    base_ref = sys.argv[1]

    # Best-effort: make sure locally-known tags are up to date before checking for
    # collisions. Ignore failures (e.g. no network) and fall back to local tags.
    subprocess.run(["git", "fetch", "--tags"], capture_output=True)

    # Detect & inject changed CSE scripts into nodecustomdata.yml first, since whether
    # that leaves the template modified is itself the signal used below to decide
    # scripts_version.
    target_varkeys = detect_changed_varkeys(base_ref)
    if target_varkeys:
        inject_scripts(target_varkeys)
    else:
        remove_scripts_block()

    base_version = read_base_version()

    version = ""
    if path_changed(base_ref, ANC_DIR):
        version = bump_version(base_version)
        print(f"aks-node-controller/ changed vs {base_ref}; version={version}", file=sys.stderr)
    else:
        print(f"aks-node-controller/ unchanged vs {base_ref}; version not set", file=sys.stderr)

    scripts_version = ""
    if path_changed(base_ref, TEMPLATE):
        scripts_version = bump_version(base_version)
        print(f"{TEMPLATE} changed vs {base_ref}; scripts_version={scripts_version}", file=sys.stderr)
    else:
        print(f"{TEMPLATE} unchanged vs {base_ref}; scripts_version not set", file=sys.stderr)

    write_hotfix_file(version, scripts_version)


if __name__ == '__main__':
    main()
