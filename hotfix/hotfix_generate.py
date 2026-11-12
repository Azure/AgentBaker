#!/usr/bin/env python3
"""Publish one ANC version pointer for changes to code or its embedded filesystem."""

import argparse
import json
import re
import subprocess
from pathlib import Path

TARGET_FILE = Path("parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.json")
VERSION_FILE = Path("pkg/agent/datamodel/linux_sig_version.json")


def path_changed(base_ref):
    result = subprocess.run([
        "git", "diff", "--quiet", base_ref, "--",
        "aks-node-controller/", "parts/root/", "parts/parts.go",
        ":(exclude,glob)aks-node-controller/**/*_test.go",
        ":(exclude,glob)aks-node-controller/**/testdata/**",
        ":(exclude)aks-node-controller/README.md",
    ])
    if result.returncode not in (0, 1):
        raise subprocess.CalledProcessError(result.returncode, result.args)
    return result.returncode == 1


def bump_version(version):
    match = re.fullmatch(r"(\d{6})\.(\d{2})\.(\d+)", version)
    if not match:
        raise ValueError(f"invalid image version {version!r}")
    month, day, current_patch = match.groups()
    patch = int(current_patch) + 1
    while True:
        result = subprocess.run(
            ["git", "show-ref", "--verify", "--quiet", f"refs/tags/v0.{month}{day}.{patch}"]
        )
        if result.returncode == 1:
            return f"{month}.{day}.{patch}"
        if result.returncode != 0:
            raise subprocess.CalledProcessError(result.returncode, result.args)
        patch += 1


def generate(base_ref):
    if not path_changed(base_ref):
        print("ANC and embedded files unchanged; keeping the existing pointer.")
        return
    version = bump_version(json.loads(VERSION_FILE.read_text())["version"])
    payload = json.loads(TARGET_FILE.read_text()) if TARGET_FILE.exists() else {}
    payload.pop("scripts_version", None)
    payload.setdefault("hotfixes", {})[".".join(version.split(".")[:2])] = version
    TARGET_FILE.write_text(json.dumps(payload, indent=4) + "\n")
    print(f"ANC and all embedded files: {version}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("base_ref", help="PR base branch; the ANC payload always contains the complete tree")
    args = parser.parse_args()
    generate(args.base_ref)
