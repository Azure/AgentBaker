#!/usr/bin/env python3
"""
Combined ANC hotfix generator.

Auto-detects what needs a hotfix and generates the version numbers for it:

1. If aks-node-controller/ (the Go module) has changes other than *_test.go or
   testdata files vs the base branch, bumps the patch of the current
   pkg/agent/datamodel/linux_sig_version.json version and uses it as `version`.

2. Detects which CSE provisioning scripts changed vs the base branch, selects their
   write_files entries from parts/linux/cloud-init/nodecustomdata.yml, and renders
   self-contained ANC payloads for each Linux platform with AgentBaker's canonical
   Go-template renderer.

3. Writes the resolved ANC `version` to
   parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.json when active.

Usage: python3 hotfix/hotfix_generate.py <base_ref>
  base_ref: git ref to diff against for changed-script/changed-code detection
            (e.g., origin/official/v20260219)

This script is called by the hotfix-generate GH Action.
"""

import argparse
import json
import os
import re
import shutil
import subprocess
import sys

TARGET_FILE = "parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.json"
TEMPLATE = "parts/linux/cloud-init/nodecustomdata.yml"
ARTIFACTS_DIR = "parts/linux/cloud-init/artifacts"
LINUX_SIG_VERSION_FILE = "pkg/agent/datamodel/linux_sig_version.json"
ANC_DIR = "aks-node-controller/"
GENERATED_DIR = os.path.join(ANC_DIR, "scripthotfix", "generated")

VERSION_RE = re.compile(r'^\d{6}\.\d{2}\.\d+$')

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

VARKEY_TO_SOURCE = {varkey: source for source, varkey in SOURCE_TO_VARKEY.items()}

HOTFIXABLE_SUFFIXES = (
    ".sh",
    ".py",
    ".service",
    ".timer",
    ".rules",
)
GENERATED_ARTIFACTS = {
    "aks-node-controller-hotfix.json",
}

class GenerationError(RuntimeError):
    """Raised when hotfix assets cannot be generated safely."""


def validate_source_mappings():
    """Validate the explicit hotfixable source allowlist."""
    if len(VARKEY_TO_SOURCE) != len(SOURCE_TO_VARKEY):
        raise GenerationError("source mappings contain duplicate variable keys")
    for varkey in VARKEY_TO_BLOCK_GROUP:
        if varkey not in VARKEY_TO_SOURCE:
            raise GenerationError(
                f"distro block variable key {varkey} has no source mapping"
            )


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


def path_changed(base_ref, *paths):
    """Return True if any selected path differs from the working tree and base_ref."""
    result = subprocess.run(["git", "diff", "--quiet", base_ref, "--", *paths])
    if result.returncode == 0:
        return False
    if result.returncode == 1:
        return True
    raise subprocess.CalledProcessError(result.returncode, result.args)


def write_hotfix_file(version):
    """Write the resolved ANC version to TARGET_FILE when active.

    When no hotfix applies, remove TARGET_FILE if present. An empty JSON object is
    still embedded as a real scriptless customData file, which changes payload
    shape even though there is no hotfix for the wrapper to consume.
    """
    payload = {}
    if version:
        payload["version"] = version

    if payload:
        with open(TARGET_FILE, "w") as f:
            json.dump(payload, f, indent=4)
            f.write("\n")
        print(f"Wrote {payload} to {TARGET_FILE}", file=sys.stderr)
        return

    try:
        os.remove(TARGET_FILE)
        print(f"No active hotfix; removed {TARGET_FILE}", file=sys.stderr)
    except FileNotFoundError:
        print(f"No active hotfix; {TARGET_FILE} already absent", file=sys.stderr)


def detect_changed_varkeys(base_ref, available_varkeys=None):
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
        if local_path in GENERATED_ARTIFACTS:
            continue
        if local_path in SOURCE_TO_VARKEY:
            source_path = os.path.join(ARTIFACTS_DIR, local_path)
            if not os.path.isfile(source_path):
                raise GenerationError(
                    f"changed hotfix source {local_path} does not exist at {source_path}"
                )
            varkey = SOURCE_TO_VARKEY[local_path]
            matched_varkeys.add(varkey)
            if varkey in VARKEY_TO_BLOCK_GROUP:
                matched_block_groups.add(VARKEY_TO_BLOCK_GROUP[varkey])
            print(f"  Matched: {local_path} → {varkey}")
        elif local_path.endswith(HOTFIXABLE_SUFFIXES) or local_path == "manifest.json":
            raise GenerationError(
                f"changed hotfixable artifact {local_path} has no source/runtime mapping"
            )
        else:
            print(f"  Warning: {local_path} has no mapping in SOURCE_TO_VARKEY (skipped)")

    if not matched_varkeys:
        print("No matched variable keys. Nothing to inject.")
        return set()

    # If a distro block group was matched, add all members of that group
    for varkey, group in VARKEY_TO_BLOCK_GROUP.items():
        if (
            group in matched_block_groups
            and (available_varkeys is None or varkey in available_varkeys)
        ):
            matched_varkeys.add(varkey)

    for varkey in matched_varkeys:
        source = VARKEY_TO_SOURCE.get(varkey)
        if not source:
            raise GenerationError(f"variable key {varkey} has no source mapping")
        source_path = os.path.join(ARTIFACTS_DIR, source)
        if not os.path.isfile(source_path):
            raise GenerationError(
                f"selected hotfix source {source} does not exist at {source_path}"
            )

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
            break

    if scriptless_start is None:
        return None, None, None

    depth = 0
    for i in range(scriptless_start, len(lines)):
        stripped = lines[i].strip()
        if re.match(r'\{\{-?\s*if(?:\s+|$)', stripped):
            depth += 1
            continue
        if (
            depth == 1
            and else_line is None
            and re.match(r'\{\{-?\s*else\s*-?\}\}$', stripped)
        ):
            else_line = i
            continue
        if re.match(r'\{\{-?\s*end\s*-?\}\}$', stripped):
            depth -= 1
            if depth == 0:
                end_line = i
                break

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


def build_hotfix_template(target_varkeys, traditional_lines):
    """Build a hotfix-only nodecustomdata template from canonical write_files blocks."""
    blocks = parse_write_files_blocks(traditional_lines)
    selected_blocks = []
    for varkeys, block_lines in blocks:
        if varkeys & target_varkeys:
            selected_blocks.append(block_lines)

    if target_varkeys and not selected_blocks:
        raise GenerationError("no matching write_files blocks found")
    if not selected_blocks:
        return "#cloud-config\nwrite_files: []\n"

    rendered = ["#cloud-config\n", "write_files:\n"]
    for block_lines in selected_blocks:
        rendered.extend(block_lines)
    return "".join(rendered)


def write_rendered_payload(target_varkeys, traditional_lines):
    """Render platform-specific YAML through AgentBaker's production template path."""
    hotfix_template = build_hotfix_template(target_varkeys, traditional_lines)
    shutil.rmtree(GENERATED_DIR, ignore_errors=True)
    os.makedirs(GENERATED_DIR, exist_ok=True)

    template_path = os.path.join(GENERATED_DIR, ".nodecustomdata-hotfix.template")
    with open(template_path, "w", newline="\n") as template_file:
        template_file.write(hotfix_template)
    try:
        subprocess.run(
            [
                "go",
                "run",
                "./hotfix/render-nodecustomdata",
                "--template",
                template_path,
                "--output-dir",
                GENERATED_DIR,
            ],
            check=True,
        )
    finally:
        try:
            os.remove(template_path)
        except FileNotFoundError:
            pass
    with open(os.path.join(GENERATED_DIR, "active"), "w", newline="\n") as active_file:
        active_file.write("true\n" if target_varkeys else "false\n")
    print(
        f"Rendered {len(target_varkeys)} hotfix variable keys into {GENERATED_DIR}",
        file=sys.stderr,
    )


def main():
    parser = argparse.ArgumentParser(description="Generate ANC hotfix assets")
    parser.add_argument("base_ref", help="git ref to diff against")
    args = parser.parse_args()
    base_ref = args.base_ref

    # Best-effort: make sure locally-known tags are up to date before checking for
    # collisions. Ignore failures (e.g. no network) and fall back to local tags.
    subprocess.run(["git", "fetch", "--tags"], capture_output=True)

    try:
        validate_source_mappings()
        with open(TEMPLATE, "r") as template_file:
            template_lines = template_file.readlines()
        _, else_line, end_line = find_block_boundaries(template_lines)
        if else_line is None or end_line is None:
            raise GenerationError(
                f"could not find traditional write_files section in {TEMPLATE}"
            )
        traditional_lines = template_lines[else_line + 1:end_line]
        available_varkeys = set()
        for varkeys, _ in parse_write_files_blocks(traditional_lines):
            available_varkeys.update(varkeys)
        changed_varkeys = detect_changed_varkeys(
            base_ref,
            available_varkeys=available_varkeys,
        )
        write_rendered_payload(changed_varkeys, traditional_lines)
    except GenerationError as err:
        print(f"ERROR: {err}", file=sys.stderr)
        sys.exit(1)

    base_version = read_base_version()

    version = ""
    if path_changed(
        base_ref,
        ANC_DIR,
        f":(exclude,glob){ANC_DIR}**/*_test.go",
        f":(exclude,glob){ANC_DIR}**/testdata/**",
    ):
        version = bump_version(base_version)
        print(f"aks-node-controller/ production files changed vs {base_ref}; "
              f"version={version}", file=sys.stderr)
    else:
        print(f"aks-node-controller/ has no production changes vs {base_ref}; "
              "version not set", file=sys.stderr)

    write_hotfix_file(version)


if __name__ == '__main__':
    main()
