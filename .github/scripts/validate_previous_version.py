#!/usr/bin/env python3
"""Validate `previousLatestVersion` entries in parts/common/components.json.

Many components in this file track two fields:

    "latestVersion":         the newest available package/image build
    "previousLatestVersion": the newest available build for a genuinely
                              PRIOR release (not just an older build of
                              the same release)

Renovate only knows how to copy the old `latestVersion` into
`previousLatestVersion` when it bumps `latestVersion`. It has no logic to
detect that both fields ended up pointing at the SAME underlying release
(only differing by package/image revision), which happens when a package
gets rebuilt (e.g. a Debian revision bump) without an actual version bump.

This script:
  1. Diffs parts/common/components.json against the PR's base ref.
  2. For every changed entry that has both `latestVersion` and
     `previousLatestVersion`, checks whether they share the same
     major.minor.patch release (a `k8sVersion` field is not required --
     this applies to any component tracking these two fields).
  3. If they do, queries the same upstream source the entry's `renovateTag`
     points at (mirroring .github/renovate.json's customManagers /
     customDatasources) to compute a best-effort recommendation: the
     highest build found upstream for the PRIOR release.

Version comparison currently only understands a `major.minor.patch` style
release embedded in the version string (the format used by every entry in
components.json today, e.g. "1.35.7-ubuntu24.04u2" or "v1.35.7-2-azlinux3").
Entries whose version strings don't contain a recognizable major.minor.patch
are skipped rather than guessed at -- broadening to other formats (date-based
tags, bare commit SHAs, etc.) is left for a follow-up once real examples of
those formats show up in this file.

This script only reports; it never rewrites components.json. Recommendations
are best-effort and not independently re-verified here -- if a suggested
value doesn't actually exist, a later CI step (schema/build validation)
will fail and catch it. A human applies the fix (using the recommendation
as a starting point, not a guarantee).
"""
from __future__ import annotations

import argparse
import gzip
import io
import json
import re
import subprocess
import sys
import urllib.error
import urllib.request
import xml.etree.ElementTree as ET
from dataclasses import dataclass
from typing import Optional

REQUEST_TIMEOUT_SECONDS = 20

# Mirrors .github/renovate.json's "customDatasources" table: for each
# (os, release) pair used in a `name=..., repository=production, os=...,
# release=...` renovateTag, this is the Packages index Renovate itself reads.
DEB_PACKAGE_INDEXES = {
    ("ubuntu", "20.04"): "https://packages.microsoft.com/ubuntu/20.04/prod/dists/focal/main/binary-amd64/Packages",
    ("ubuntu", "22.04"): "https://packages.microsoft.com/ubuntu/22.04/prod/dists/jammy/main/binary-amd64/Packages",
    ("ubuntu", "24.04"): "https://packages.microsoft.com/ubuntu/24.04/prod/dists/noble/main/binary-amd64/Packages",
    ("ubuntu", "26.04"): "https://packages.microsoft.com/ubuntu/26.04/prod/dists/resolute/main/binary-amd64/Packages",
}


@dataclass
class ComponentEntry:
    path: str  # human-readable JSON path, for messages
    renovate_tag: Optional[str]
    latest_version: Optional[str]
    previous_latest_version: Optional[str]


RELEASE_RE = re.compile(r"v?(?P<nums>\d+(?:\.\d+)+)")


def extract_release(version: str) -> Optional[tuple]:
    """Extract the full dot-separated numeric release embedded in a
    build/version string, e.g. "1.35.7" from "1.35.7-ubuntu24.04u2", or
    "10.0.20348.5622" from a 4-part Windows build number. Returns None if
    no such pattern is found -- entries in that shape are skipped rather
    than guessed at (see module docstring).

    The whole numeric run is captured (not hardcoded to 3 parts) so that
    schemes with more segments than major.minor.patch, such as Windows's
    major.minor.build.revision, aren't falsely treated as "the same
    release" just because their first three segments match."""
    m = RELEASE_RE.search(version)
    if not m:
        return None
    return tuple(int(part) for part in m.group("nums").split("."))


def natural_sort_key(revision: str):
    """Split a revision/suffix string into alternating text/number chunks so
    that e.g. "2" < "10" and "ubuntu20.04u3" compares sanely. This is a
    best-effort stand-in for `dpkg --compare-versions` / rpm version compare;
    it is not a full implementation of either ecosystem's version rules."""
    return [int(chunk) if chunk.isdigit() else chunk for chunk in re.split(r"(\d+)", revision)]


def load_json_at_ref(ref: str, path: str) -> dict:
    result = subprocess.run(
        ["git", "show", f"{ref}:{path}"],
        capture_output=True,
        text=True,
        check=True,
    )
    return json.loads(result.stdout)


def load_json_file(path: str) -> dict:
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def collect_entries(node, path="root") -> list:
    """Recursively find every dict in components.json that tracks both
    `latestVersion` and `previousLatestVersion`, regardless of component
    type (kubectl/kubelet, containerd, CNI plugins, GPU drivers, etc.)."""
    entries = []
    if isinstance(node, dict):
        if "latestVersion" in node and "previousLatestVersion" in node:
            entries.append(
                ComponentEntry(
                    path=path,
                    renovate_tag=node.get("renovateTag"),
                    latest_version=node.get("latestVersion"),
                    previous_latest_version=node.get("previousLatestVersion"),
                )
            )
        for key, value in node.items():
            entries.extend(collect_entries(value, f"{path}.{key}"))
    elif isinstance(node, list):
        for i, value in enumerate(node):
            entries.extend(collect_entries(value, f"{path}[{i}]"))
    return entries


def entry_key(entry: ComponentEntry) -> tuple:
    # renovateTag uniquely identifies an entry across versions of the file
    # even if its surrounding array index shifts. Entries without a
    # renovateTag (uncommon) fall back to their JSON path, which is only
    # stable if the file's array ordering doesn't change around them.
    return entry.renovate_tag if entry.renovate_tag else ("path", entry.path)


# ---------------------------------------------------------------------------
# Upstream source lookups, one per renovateTag family. Each returns the full
# list of available version strings (in the same textual form used in
# components.json) for the given package name.
# ---------------------------------------------------------------------------

def fetch_url(url: str) -> bytes:
    req = urllib.request.Request(url, headers={"User-Agent": "agentbaker-validate-components"})
    with urllib.request.urlopen(req, timeout=REQUEST_TIMEOUT_SECONDS) as resp:
        return resp.read()


def lookup_oci_versions(registry_url: str, package_name: str) -> list:
    # e.g. registry_url=https://mcr.microsoft.com, package_name=oss/v2/kubernetes/kubectl-sysext
    host = registry_url.rstrip("/").split("://", 1)[-1]
    url = f"https://{host}/v2/{package_name}/tags/list"
    data = json.loads(fetch_url(url))
    return data.get("tags", [])


def lookup_deb_versions(os_name: str, release: str, repository: str, package_name: str) -> list:
    if repository == "nvidia":
        # Nvidia CUDA repos aren't part of DEB_PACKAGE_INDEXES; skip rather
        # than guess at a URL.
        raise NotImplementedError(f"no known package index for repository={repository}")
    index_url = DEB_PACKAGE_INDEXES.get((os_name, release))
    if not index_url:
        raise NotImplementedError(f"no known package index for os={os_name}, release={release}")
    text = fetch_url(index_url).decode("utf-8", errors="replace")
    lines = text.splitlines()
    target = f"Package: {package_name}"
    versions = []
    for i, line in enumerate(lines):
        if line.strip() != target:
            continue
        for lookahead in lines[i + 1 : i + 6]:
            if lookahead.startswith("Version:"):
                versions.append(lookahead.split(":", 1)[1].strip())
                break
    return versions


def lookup_rpm_versions(registry_url: str, package_name: str) -> list:
    # registry_url looks like ".../azurelinux/3.0/prod/cloud-native/x86_64/repodata"
    repomd_url = registry_url.rstrip("/") + "/repomd.xml"
    baseurl = registry_url.rstrip("/").rsplit("/repodata", 1)[0]
    ns = {"r": "http://linux.duke.edu/metadata/repo"}
    repomd = ET.fromstring(fetch_url(repomd_url))
    primary_href = None
    for data_el in repomd.findall("r:data", ns):
        if data_el.get("type") == "primary":
            primary_href = data_el.find("r:location", ns).get("href")
            break
    if not primary_href:
        raise RuntimeError("primary.xml.gz location not found in repomd.xml")
    primary_url = f"{baseurl}/{primary_href}"
    raw = fetch_url(primary_url)
    if primary_href.endswith(".gz"):
        raw = gzip.GzipFile(fileobj=io.BytesIO(raw)).read()
    common_ns = {"c": "http://linux.duke.edu/metadata/common"}
    root = ET.fromstring(raw)
    versions = []
    for pkg in root.findall("c:package", common_ns):
        name_el = pkg.find("c:name", common_ns)
        if name_el is None or name_el.text != package_name:
            continue
        version_el = pkg.find("c:version", common_ns)
        if version_el is None:
            continue
        ver = version_el.get("ver")
        rel = version_el.get("rel")
        versions.append(f"{ver}-{rel}" if rel else ver)
    return versions


RENOVATE_TAG_RE = re.compile(
    r"^(?:(?P<oci>OCI_registry)|(?P<docker>registry)|(?P<rpm>RPM_registry))=(?P<registry>[^,]+),\s*"
    r"name=(?P<name>[^,]+)(?:,\s*repository=(?P<repository>[^,]+))?(?:,\s*os=(?P<os>[^,]+))?"
    r"(?:,\s*release=(?P<release>[^,]+))?$"
)
DEB_TAG_RE = re.compile(
    r"^name=(?P<name>[^,]+),\s*repository=(?P<repository>[^,]+),\s*os=(?P<os>[^,]+),\s*release=(?P<release>[^,]+)$"
)


def lookup_available_versions(renovate_tag: Optional[str]) -> list:
    """Returns the raw list of available upstream version strings for the
    package described by `renovate_tag`. Raises NotImplementedError for
    sources this script does not yet know how to query (including entries
    with no renovateTag at all)."""
    if not renovate_tag:
        raise NotImplementedError("entry has no renovateTag to identify its upstream source")
    m = RENOVATE_TAG_RE.match(renovate_tag)
    if m:
        if m.group("oci") or m.group("docker"):
            return lookup_oci_versions(m.group("registry"), m.group("name"))
        if m.group("rpm"):
            return lookup_rpm_versions(m.group("registry"), m.group("name"))
    m = DEB_TAG_RE.match(renovate_tag)
    if m:
        return lookup_deb_versions(m.group("os"), m.group("release"), m.group("repository"), m.group("name"))
    raise NotImplementedError(f"unrecognized renovateTag format: {renovate_tag!r}")


def recommend_previous_version(latest_version: str, available_versions: list) -> Optional[str]:
    latest_release = extract_release(latest_version)
    if latest_release is None:
        return None
    candidates_by_release: dict = {}
    for v in available_versions:
        release = extract_release(v)
        if release is None or release >= latest_release:
            continue
        candidates_by_release.setdefault(release, []).append(v)
    if not candidates_by_release:
        return None
    prior_release = max(candidates_by_release.keys())
    return max(candidates_by_release[prior_release], key=natural_sort_key)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True, help="git ref to diff against, e.g. origin/main")
    parser.add_argument("--file", required=True, help="path to components.json")
    parser.add_argument(
        "--report",
        help="optional path to write a Markdown report for PR-commenting when issues are found",
    )
    args = parser.parse_args()

    try:
        base_json = load_json_at_ref(args.base, args.file)
    except subprocess.CalledProcessError as e:
        print(f"::warning::could not read {args.file} at {args.base}: {e}")
        base_json = {}
    head_json = load_json_file(args.file)

    base_entries = {entry_key(e): e for e in collect_entries(base_json)}
    head_entries = collect_entries(head_json)

    # Each row: (entry, recommendation_or_None, note)
    findings = []

    for entry in head_entries:
        base_entry = base_entries.get(entry_key(entry))
        changed = (
            base_entry is None
            or base_entry.latest_version != entry.latest_version
            or base_entry.previous_latest_version != entry.previous_latest_version
        )
        if not changed:
            continue
        if not entry.latest_version or not entry.previous_latest_version:
            continue

        latest_release = extract_release(entry.latest_version)
        previous_release = extract_release(entry.previous_latest_version)
        if latest_release is None or previous_release is None:
            continue  # no recognizable major.minor.patch in one of the fields -- skip
        if latest_release != previous_release:
            continue  # different releases -> already correct shape

        print(
            f"::error::{entry.path}: latestVersion ({entry.latest_version}) and "
            f"previousLatestVersion ({entry.previous_latest_version}) are both "
            f"version {'.'.join(map(str, latest_release))}. "
            f"previousLatestVersion must reference a genuinely prior release."
        )
        try:
            available = lookup_available_versions(entry.renovate_tag)
            recommendation = recommend_previous_version(entry.latest_version, available)
            if recommendation:
                print(
                    f"::error::{entry.path}: best-effort recommendation: {recommendation} "
                    f"(highest build found upstream for the prior release)"
                )
                findings.append((entry, recommendation, None))
            else:
                note = "no build found upstream for a prior release; set this manually."
                print(f"::error::{entry.path}: {note}")
                findings.append((entry, None, note))
        except NotImplementedError as e:
            note = f"cannot auto-recommend a value ({e}); set this manually."
            print(f"::warning::{entry.path}: {note}")
            findings.append((entry, None, note))
        except (urllib.error.URLError, urllib.error.HTTPError, RuntimeError) as e:
            note = f"failed to query upstream source ({e}); set this manually."
            print(f"::warning::{entry.path}: {note}")
            findings.append((entry, None, note))

    if args.report:
        write_report(args.report, findings)

    if findings:
        print(
            "::error::One or more previousLatestVersion entries need a fix. "
            "This check never edits components.json automatically; recommendations are "
            "best-effort (highest version found upstream) and are not independently "
            "re-verified here — a subsequent CI step (schema/build validation) will fail "
            "if an applied value turns out not to exist."
        )
        return 1

    print("previousLatestVersion validation passed.")
    return 0


def write_report(path: str, findings: list) -> None:
    if not findings:
        return
    lines = [
        "### `previousLatestVersion` needs a fix",
        "",
        "One or more changed entries in `parts/common/components.json` have "
        "`latestVersion` and `previousLatestVersion` pointing at the same release. "
        "`previousLatestVersion` must reference a genuinely prior release. "
        "This check does not edit the file for you — please apply a value below "
        "(or replace it with a better one if you know it).",
        "",
        "| Entry | latestVersion | previousLatestVersion (current) | Suggested previousLatestVersion |",
        "|---|---|---|---|",
    ]
    for entry, recommendation, note in findings:
        suggestion = f"`{recommendation}`" if recommendation else f"_{note}_"
        lines.append(
            f"| `{entry.path}` | `{entry.latest_version}` | `{entry.previous_latest_version}` | {suggestion} |"
        )
    lines.append("")
    lines.append(
        "_Suggestions are best-effort: the highest version this check found upstream for "
        "the prior release. If a suggested value doesn't actually exist, a later "
        "CI step (schema/build validation) will catch it._"
    )
    with open(path, "w", encoding="utf-8") as f:
        f.write("\n".join(lines) + "\n")


if __name__ == "__main__":
    sys.exit(main())
