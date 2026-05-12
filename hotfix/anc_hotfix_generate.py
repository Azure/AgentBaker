#!/usr/bin/env python3
"""
Reads the ANC hotfix version from hotfix/anc-hotfix-version.json and injects
(or updates) the aks-node-controller-hotfix.json write_files entry into the
EnableScriptlessCSECmd section of nodecustomdata.yml.

Usage: python3 hotfix/anc_hotfix_generate.py

This script is called by the anc-hotfix-generate GH Action.
"""

import json
import re
import sys

TEMPLATE = "parts/linux/cloud-init/nodecustomdata.yml"
VERSION_FILE = "hotfix/anc-hotfix-version.json"
HOTFIX_PATH = "/opt/azure/containers/aks-node-controller-hotfix.json"

# Marker comments for idempotent injection
BEGIN_MARKER = "# ---- anc-hotfix: auto-generated ----"
END_MARKER = "# ---- end anc-hotfix ----"


def read_hotfix_version():
    """Read and validate the hotfix version from the version file."""
    try:
        with open(VERSION_FILE) as f:
            data = json.load(f)
    except FileNotFoundError:
        print(f"{VERSION_FILE} not found. Nothing to do.")
        return None
    except json.JSONDecodeError as e:
        print(f"ERROR: {VERSION_FILE} contains invalid JSON: {e}", file=sys.stderr)
        sys.exit(1)

    version = data.get("version", "").strip()
    if not version:
        print(f"{VERSION_FILE} has no version set. Nothing to do.")
        return None

    # Validate YYYYMM.DD.PATCH format
    if not re.match(r'^\d{6}\.\d{2}\.\d+$', version):
        print(f"ERROR: invalid version format '{version}', "
              f"expected YYYYMM.DD.PATCH (e.g., 202604.01.1)", file=sys.stderr)
        sys.exit(1)

    return version


def build_hotfix_entry(version):
    """Build the write_files YAML lines for the hotfix JSON config."""
    hotfix_json = json.dumps({"version": version}, separators=(',', ':'))
    return [
        f"\n",
        f"{BEGIN_MARKER}\n",
        f"- path: {HOTFIX_PATH}\n",
        f"  permissions: \"0644\"\n",
        f"  owner: root\n",
        f"  content: |\n",
        f"    {hotfix_json}\n",
        f"{END_MARKER}\n",
    ]


def inject(version):
    """Inject or update the ANC hotfix entry in the scriptless section of nodecustomdata.yml."""
    with open(TEMPLATE) as f:
        content = f.read()

    # Remove any previous ANC hotfix entry (idempotent)
    content = re.sub(
        rf'\n?{re.escape(BEGIN_MARKER)}\n.*?{re.escape(END_MARKER)}\n',
        '', content, flags=re.DOTALL,
    )

    lines = content.splitlines(keepends=True)

    # Find the EnableScriptlessCSECmd block and its {{- else}} boundary
    scriptless_start = None
    else_idx = None
    for i, line in enumerate(lines):
        if '{{if EnableScriptlessCSECmd}}' in line:
            scriptless_start = i
        if scriptless_start is not None and else_idx is None:
            if line.strip().startswith('{{- else'):
                else_idx = i

    if scriptless_start is None or else_idx is None:
        print("ERROR: could not find EnableScriptlessCSECmd / {{- else}} boundary "
              "in template", file=sys.stderr)
        sys.exit(1)

    entry_lines = build_hotfix_entry(version)

    # Insert just before the {{- else}} line
    final = lines[:else_idx] + entry_lines + lines[else_idx:]

    with open(TEMPLATE, 'w') as f:
        f.writelines(final)

    print(f"Injected ANC hotfix version {version} into {TEMPLATE}", file=sys.stderr)
    return True


def remove_hotfix():
    """Remove any existing ANC hotfix entry from the template."""
    with open(TEMPLATE) as f:
        content = f.read()

    new_content = re.sub(
        rf'\n?{re.escape(BEGIN_MARKER)}\n.*?{re.escape(END_MARKER)}\n',
        '', content, flags=re.DOTALL,
    )

    if new_content != content:
        with open(TEMPLATE, 'w') as f:
            f.write(new_content)
        print(f"Removed previous ANC hotfix entry from {TEMPLATE}", file=sys.stderr)
        return True
    return False


def main():
    version = read_hotfix_version()
    if version:
        inject(version)
        print(f"\nDone. Injected ANC hotfix version {version}.")
    else:
        # No version set — remove any stale hotfix entry
        if remove_hotfix():
            print("\nDone. Removed stale ANC hotfix entry.")
        else:
            print("\nNothing to do.")


if __name__ == '__main__':
    main()
