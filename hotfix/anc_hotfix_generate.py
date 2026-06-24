#!/usr/bin/env python3
"""
Reads the ANC hotfix version from hotfix/anc-hotfix-version.json and writes
it to parts/hotfix/aks-node-controller-hotfix.json so baker.go can embed it into the
boothook, which writes hotfix.json directly to disk at provisioning time.

Usage: python3 hotfix/anc_hotfix_generate.py

This script is called by the anc-hotfix-generate GH Action.
"""

import json
import re
import sys

SOURCE_FILE = "hotfix/anc-hotfix-version.json"
EMBED_FILE = "parts/hotfix/aks-node-controller-hotfix.json"


def _validate_version(value, key, allow_base=False):
    """Validate version formats used by ANC hotfix config."""
    if not value:
        return
    pattern = r'^\d{6}\.\d{2}\.\d+$'
    expected = "YYYYMM.DD.PATCH (e.g., 202604.01.1)"
    if allow_base:
        pattern = r'^\d{6}\.\d{2}(?:\.\d+)?$'
        expected = "YYYYMM.DD or YYYYMM.DD.PATCH (e.g., 202604.01 or 202604.01.1)"
    if not re.match(pattern, value):
        print(f"ERROR: invalid {key} format '{value}', expected {expected}", file=sys.stderr)
        sys.exit(1)


def read_hotfix_config():
    """Read and validate the hotfix config from the source version file."""
    try:
        with open(SOURCE_FILE) as f:
            data = json.load(f)
    except FileNotFoundError:
        print(f"ERROR: {SOURCE_FILE} not found", file=sys.stderr)
        sys.exit(1)
    except json.JSONDecodeError as e:
        print(f"ERROR: {SOURCE_FILE} contains invalid JSON: {e}", file=sys.stderr)
        sys.exit(1)

    version = data.get("version", "").strip()
    scripts_version = data.get("scripts_version", "").strip()
    if not version and not scripts_version:
        print(f"{SOURCE_FILE} has no version/scripts_version set. Nothing to do.")
        return None

    _validate_version(version, "version")
    _validate_version(scripts_version, "scripts_version", allow_base=True)

    payload = {}
    if version:
        payload["version"] = version
    if scripts_version:
        payload["scripts_version"] = scripts_version
    return payload


def main():
    payload = read_hotfix_config()
    if payload:
        with open(EMBED_FILE, 'w') as f:
            json.dump(payload, f, separators=(',', ':'))
        print(f"\nDone. Wrote hotfix config {payload} to {EMBED_FILE}.")
    else:
        # No version/scripts_version set — clear the embed file
        with open(EMBED_FILE, 'w') as f:
            f.write("{}")
        print(f"\nDone. Cleared {EMBED_FILE}.")


if __name__ == '__main__':
    main()
