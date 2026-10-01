#!/usr/bin/env python3
"""Skip Windows PR builds only for proven Linux-only components.json changes."""

import json
import os
from pathlib import Path
import re
import subprocess
import sys


COMPONENTS_PATH = "parts/common/components.json"
LINUX_DISTROS = (
    "ubuntu", "mariner", "marinerkata", "azurelinux", "azurelinuxkata", "flatcar"
)


def unique_object(pairs):
    result = {}
    names = set()
    for key, value in pairs:
        # PowerShell property lookup is case-insensitive.
        if key.casefold() in names:
            raise ValueError(f"Duplicate JSON property: {key!r}")
        names.add(key.casefold())
        result[key] = value
    return result


def invalid_constant(value):
    raise ValueError(f"Invalid JSON constant: {value}")


def object_array(document, name):
    value = document.get(name)
    if not isinstance(value, list) or not all(isinstance(item, dict) for item in value):
        raise ValueError(f"{name} must be an array of objects")
    return value


def windows_inputs(text):
    """Subtract only known Linux-only fields; preserve unknown fields and order."""
    document = json.loads(
        text, object_pairs_hook=unique_object, parse_constant=invalid_constant
    )
    if not isinstance(document, dict):
        raise ValueError("Components must be a JSON object")
    images = object_array(document, "ContainerImages")
    packages = object_array(document, "Packages")
    if "GPUContainerImages" in document:
        object_array(document, "GPUContainerImages")
    if "OCIArtifacts" in document:
        object_array(document, "OCIArtifacts")

    # Windows helpers never consume GPUContainerImages or amd64OnlyVersions.
    document.pop("GPUContainerImages", None)
    for image in images:
        image.pop("amd64OnlyVersions", None)
    for package in packages:
        package.pop("downloadLocation", None)
        uris = package.get("downloadURIs")
        if not isinstance(uris, dict):
            raise ValueError("Package downloadURIs must be an object")
        for distro in LINUX_DISTROS:
            uris.pop(distro, None)

    # Keep all Windows/default fallback fields, OCI artifacts, and array order:
    # sorted URL inventories lose authentication flags and the default containerd.
    return json.dumps(document, sort_keys=True, allow_nan=False)


def git(repo, *args):
    return subprocess.run(
        ["git", "-C", str(repo), *args],
        check=True, capture_output=True, text=True,
    ).stdout


def skip_windows_vhd(repo, reason, source_branch, source_version):
    if reason != "PullRequest":
        return False, "Not a PR validation build."
    if not re.fullmatch(r"refs/pull/[0-9]+/merge", source_branch):
        return False, "Not a GitHub PR merge ref."

    head = git(repo, "rev-parse", "--verify", "HEAD").strip()
    if head != source_version:
        raise ValueError("Checkout does not match Build.SourceVersion")
    headers = git(repo, "cat-file", "-p", head).split("\n\n", 1)[0]
    parents = [
        line[len("parent "):]
        for line in headers.splitlines() if line.startswith("parent ")
    ]
    if len(parents) != 2:
        return False, "PR validation commit does not have exactly two parents."

    # GitHub's merge commit has the target as its first parent. Compare the
    # actual tested trees, not HEAD~1 on the PR branch or a moving origin/main.
    base = parents[0]
    changes = git(repo, "diff", "--no-ext-diff", "--name-status", "--no-renames",
                  "-z", base, head, "--")
    if changes != f"M\0{COMPONENTS_PATH}\0":
        return False, "Changes are not limited to modifying components.json."
    for ref in (base, head):
        if not git(repo, "ls-tree", ref, "--", COMPONENTS_PATH).startswith("100644 blob "):
            return False, "components.json is not a regular, non-executable file."

    before = windows_inputs(git(repo, "show", f"{base}:{COMPONENTS_PATH}"))
    after = windows_inputs(git(repo, "show", f"{head}:{COMPONENTS_PATH}"))
    if before != after:
        return False, "Windows, shared, or unrecognized component inputs changed."
    return True, "Only Linux-only component fields changed; no Windows VHD impact."


def main():
    try:
        skip, explanation = skip_windows_vhd(
            Path.cwd(), os.environ.get("BUILD_REASON", ""),
            os.environ.get("BUILD_SOURCEBRANCH", ""),
            os.environ.get("BUILD_SOURCEVERSION", ""),
        )
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        # Never emit permission to skip when the comparison is incomplete.
        print(f"##vso[task.logissue type=error]Windows impact detection failed: {error!r}")
        return 1
    print(explanation)
    print(f"##vso[task.setvariable variable=skipWindowsVhd;isOutput=true]{str(skip).lower()}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
