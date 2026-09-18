#!/usr/bin/env python3
"""Validate `previousLatestVersion` entries in parts/common/components.json.

Many components in this file track two fields:

    "latestVersion":         the newest available package/image build
    "previousLatestVersion": the newest available build for a genuinely
                              PRIOR release (not just an older build of
                              the same release)

Renovate only knows how to copy the old `latestVersion` into
`previousLatestVersion` when it bumps `latestVersion`. That leads to two
distinct problems this script checks for:

  1. "collision" -- `previousLatestVersion` ends up pointing at the SAME
     release as the new `latestVersion` (only differing by package/image
     build revision), which happens when a package gets rebuilt (e.g. a
     Debian revision bump) without an actual version bump.
  2. "stale" -- `previousLatestVersion` does reference a genuinely prior
     release, but not the newest build known upstream for that release
     (e.g. it was copied forward from an old `latestVersion` that was
     itself already several builds behind upstream by the time it became
     `previousLatestVersion`).

For both, this script:
  1. Diffs parts/common/components.json against the PR's base ref.
  2. For every changed entry that has both `latestVersion` and
     `previousLatestVersion` (a `k8sVersion` field is not required -- this
     applies to any component tracking these two fields), parses their
     releases/build numbers.
  3. Queries the same upstream source the entry's `renovateTag` points at
     (mirroring .github/renovate.json's customManagers / customDatasources)
     to compute a best-effort recommendation: for a collision, the highest
     build found upstream for a genuinely prior release; for staleness,
     the highest build found upstream for `previousLatestVersion`'s own
     release.

Version comparison currently only understands one narrow, unambiguous shape:
a prefix of `v?MAJOR.MINOR.PATCH-BUILD` where BUILD is purely numeric
(e.g. "v0.1.16-16" or "0.1.16-16", optionally followed by an arbitrary
suffix like ".azl3" or "-azlinux3"). That's the one case where both "same
release" and "build number" can be identified with certainty. Entries whose
latestVersion/previousLatestVersion don't match this shape -- e.g.
"1.35.7-ubuntu24.04u2" (the character after the dash isn't a digit),
"10.0.20348.5622" (no dash at all), or a bare "1.35.7" (no build suffix) --
are skipped rather than guessed at; broadening to those formats is left for
a follow-up once we're confident how to compare them unambiguously.

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


RELEASE_RE = re.compile(r"^(?P<prefix>v?\d+\.\d+\.\d+)-(?P<build>\d+)(?P<suffix>.*)$")


@dataclass
class ParsedVersion:
    release: tuple  # (major, minor, patch)
    build: int
    prefix: str  # e.g. "v0.1.16" -- everything before "-BUILD", kept verbatim (with leading "v" if present)
    suffix: str  # e.g. "" or ".azl3" or "-azlinux3" -- everything after BUILD, kept verbatim


def parse_version(version: str) -> Optional[ParsedVersion]:
    """Parse a version string, but only for the narrow, unambiguous shape
    this check targets: a prefix of `v?MAJOR.MINOR.PATCH-BUILD` where BUILD
    is a purely numeric run immediately after the dash (an arbitrary
    string, such as ".azl3" or "-azlinux3", may follow). This is the one
    shape where "same release" and "build number" can both be identified
    with certainty -- the dash cleanly separates the semver release from a
    package/image build/revision counter, and the digits right after it
    are unambiguously the build number.

    Anything else (e.g. "1.35.7-ubuntu24.04u2", where the character right
    after the dash is not a digit, "10.0.20348.5622", which has no dash at
    all, or a bare "1.35.7" with no build suffix) returns None and is
    skipped rather than guessed at: those formats mix in OS/distro
    identifiers or extra version segments that make "same release"/"build
    number" ambiguous to determine (see module docstring)."""
    m = RELEASE_RE.match(version)
    if not m:
        return None
    release = tuple(int(part) for part in m.group("prefix").lstrip("v").split("."))
    return ParsedVersion(release=release, build=int(m.group("build")), prefix=m.group("prefix"), suffix=m.group("suffix"))


def extract_release(version: str) -> Optional[tuple]:
    parsed = parse_version(version)
    return parsed.release if parsed else None


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


def find_previous_version_lines(path: str) -> list:
    """Return the 1-indexed line number and raw text of every literal
    `"previousLatestVersion":` line in the file, in top-to-bottom document
    order. Every ComponentEntry from collect_entries() has exactly one such
    line (an entry requires both latestVersion and previousLatestVersion to
    be present), and dict/list traversal order in collect_entries() mirrors
    the file's physical order (json.load preserves key/list order). So
    zipping this list against collect_entries()'s output gives each entry
    its exact line number in the file without needing a line-tracking JSON
    parser -- used to post GitHub suggested-change review comments."""
    lines = []
    with open(path, "r", encoding="utf-8") as f:
        for i, line in enumerate(f, start=1):
            if '"previousLatestVersion"' in line:
                lines.append((i, line.rstrip("\n")))
    return lines


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


def entry_key(entry: ComponentEntry) -> str:
    # JSON path (array index / dict key chain) uniquely and stably identifies
    # an entry across base/head diffs for this kind of PR: Renovate only
    # replaces the latestVersion/previousLatestVersion *values* in place, it
    # never reorders or adds/removes array entries. renovateTag looks like a
    # natural identity key, but it is NOT unique -- multiple rows for
    # different k8sVersion minors commonly share the exact same
    # `name=..., os=..., release=...` tag (same package/OS/release), which
    # would otherwise collide different entries onto the same dict key.
    return entry.path


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


def find_highest_build(available_versions: list, target_release: tuple) -> Optional[ParsedVersion]:
    """Among available upstream version strings, return the ParsedVersion
    with the highest build number for exactly `target_release`, or None if
    no upstream version matches that release at all."""
    best: Optional[ParsedVersion] = None
    for v in available_versions:
        parsed = parse_version(v)
        if parsed is None or parsed.release != target_release:
            continue
        if best is None or parsed.build > best.build:
            best = parsed
    return best


def find_prior_release_highest_build(available_versions: list, latest_release: tuple) -> Optional[ParsedVersion]:
    """Among available upstream version strings, find the highest release
    that is still strictly lower than `latest_release`, then return the
    ParsedVersion with the highest build number within that release."""
    candidates: dict = {}
    for v in available_versions:
        parsed = parse_version(v)
        if parsed is None or parsed.release >= latest_release:
            continue
        candidates.setdefault(parsed.release, []).append(parsed)
    if not candidates:
        return None
    prior_release = max(candidates.keys())
    return max(candidates[prior_release], key=lambda p: p.build)


def format_recommendation(parsed: ParsedVersion, like: ParsedVersion) -> str:
    """Render `parsed` (a release + build found upstream) using the same
    textual style (leading "v" or not, trailing suffix) as `like` (the
    entry's current previousLatestVersion) -- upstream tags/packages may
    include extra formatting (e.g. CPU-arch suffixes on OCI tags) that
    components.json doesn't use, so we don't just echo the upstream string
    verbatim."""
    prefix = ("v" if like.prefix.startswith("v") else "") + ".".join(map(str, parsed.release))
    return f"{prefix}-{parsed.build}{like.suffix}"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", required=True, help="git ref to diff against, e.g. origin/main")
    parser.add_argument("--file", required=True, help="path to components.json")
    parser.add_argument(
        "--report",
        help="optional path to write a Markdown report for PR-commenting when issues are found",
    )
    parser.add_argument(
        "--suggestions",
        help="optional path to write a JSON list of GitHub suggested-change review "
        "comments (path/line/body) for each finding that has a recommendation, so a "
        "developer can apply the fix with GitHub's native 'Commit suggestion' button",
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

    prev_version_lines = find_previous_version_lines(args.file)
    if len(prev_version_lines) == len(head_entries):
        entry_lines = dict(zip((e.path for e in head_entries), prev_version_lines))
    else:
        # Should not happen (every ComponentEntry has exactly one
        # "previousLatestVersion" line), but degrade gracefully: skip
        # suggestion generation rather than risk mis-attributing a line.
        print(
            "::warning::previousLatestVersion line count does not match entry count; "
            "skipping suggested-change generation for this run."
        )
        entry_lines = {}

    # Each row: (entry, kind, recommendation_or_None, note)
    # kind is "collision" (previousLatestVersion == latestVersion's release)
    # or "stale" (previousLatestVersion is a real prior release, but not the
    # highest build known upstream for that release).
    findings = []
    # Each row: (path, line_number, raw_line, previous_latest_version, recommendation)
    suggestions = []

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

        previous = parse_version(entry.previous_latest_version)
        if previous is None:
            continue  # unrecognized version shape -- skip rather than guess (see module docstring)
        latest = parse_version(entry.latest_version)

        if latest is not None and latest.release == previous.release:
            kind = "collision"
            print(
                f"::error::{entry.path}: latestVersion ({entry.latest_version}) and "
                f"previousLatestVersion ({entry.previous_latest_version}) are both "
                f"release {'.'.join(map(str, latest.release))}. "
                f"previousLatestVersion must reference a genuinely prior release."
            )
        else:
            kind = "stale"

        try:
            available = lookup_available_versions(entry.renovate_tag)
        except NotImplementedError as e:
            note = f"cannot auto-check this entry ({e}); please verify manually."
            print(f"::warning::{entry.path}: {note}")
            if kind == "collision":
                findings.append((entry, kind, None, note))
            continue
        except (urllib.error.URLError, urllib.error.HTTPError, RuntimeError) as e:
            note = f"failed to query upstream source ({e}); please verify manually."
            print(f"::warning::{entry.path}: {note}")
            if kind == "collision":
                findings.append((entry, kind, None, note))
            continue

        if kind == "collision":
            # Renovate's autoReplaceStringTemplate always copies the OLD
            # latestVersion string into previousLatestVersion on every bump
            # (see .github/renovate.json). When the bump is revision-only
            # (same release, higher build), that copy clobbers whatever
            # genuinely prior release the base branch had retained, creating
            # the collision. Prefer restoring/refreshing THAT retained
            # release (a revision refresh) over re-selecting the highest
            # upstream release below latestVersion (a release-selection
            # change) -- otherwise we can silently swap the tracked prior
            # release to one that was never actually shipped.
            rec = None
            base_latest = (
                parse_version(base_entry.latest_version)
                if base_entry and base_entry.latest_version
                else None
            )
            base_previous = (
                parse_version(base_entry.previous_latest_version)
                if base_entry and base_entry.previous_latest_version
                else None
            )
            if (
                base_latest is not None
                and base_previous is not None
                and base_latest.release == latest.release  # revision-only update
                and base_previous.release < latest.release  # base retained a genuinely prior release
            ):
                rec = find_highest_build(available, base_previous.release) or base_previous
            if rec is None:
                rec = find_prior_release_highest_build(available, latest.release)
            if rec:
                recommendation = format_recommendation(rec, previous)
                print(
                    f"::error::{entry.path}: best-effort recommendation: {recommendation} "
                    f"(highest build found upstream for the prior release)"
                )
                findings.append((entry, kind, recommendation, None))
                record_suggestion(suggestions, entry_lines, entry, recommendation)
            else:
                note = "no build found upstream for a prior release; set this manually."
                print(f"::error::{entry.path}: {note}")
                findings.append((entry, kind, None, note))
        else:
            rec = find_highest_build(available, previous.release)
            if rec is not None and rec.build > previous.build:
                recommendation = format_recommendation(rec, previous)
                print(
                    f"::error::{entry.path}: previousLatestVersion ({entry.previous_latest_version}) "
                    f"is not the newest known build for release {'.'.join(map(str, previous.release))}: "
                    f"upstream has build {rec.build}. best-effort recommendation: {recommendation}"
                )
                findings.append((entry, kind, recommendation, None))
                record_suggestion(suggestions, entry_lines, entry, recommendation)
            # else: previousLatestVersion is already the highest known build (or upstream has
            # nothing newer that we could find) -- no finding.

    if args.report:
        write_report(args.report, findings)
    if args.suggestions:
        write_suggestions(args.suggestions, suggestions)

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


def record_suggestion(suggestions: list, entry_lines: dict, entry: ComponentEntry, recommendation: str) -> None:
    """Record a GitHub suggested-change candidate for `entry`, if we know
    which line its previousLatestVersion lives on. The suggestion body is
    built by replacing the current value in-place within the *original raw
    line* (so indentation, quoting, and any trailing comma are preserved
    exactly), rather than re-rendering the JSON ourselves."""
    line_info = entry_lines.get(entry.path)
    if line_info is None:
        return
    line_no, raw_line = line_info
    if f'"{entry.previous_latest_version}"' not in raw_line:
        # Shouldn't happen given find_previous_version_lines()'s 1:1 zip with
        # collect_entries(), but don't guess at a line we can't confirm.
        return
    new_line = raw_line.replace(f'"{entry.previous_latest_version}"', f'"{recommendation}"', 1)
    suggestions.append((line_no, new_line))


def write_suggestions(path: str, suggestions: list) -> None:
    payload = [{"line": line_no, "new_line": new_line} for line_no, new_line in suggestions]
    with open(path, "w", encoding="utf-8") as f:
        json.dump(payload, f, indent=2)


def write_report(path: str, findings: list) -> None:
    if not findings:
        return
    lines = [
        "### `previousLatestVersion` needs a fix",
        "",
        "One or more changed entries in `parts/common/components.json` have a "
        "`previousLatestVersion` problem: either it points at the same release as "
        "`latestVersion` (must reference a genuinely prior release), or it's a real "
        "prior release but not the newest build known upstream for that release. "
        "Where a recommendation was found, this check also posted it as an inline "
        "GitHub suggested-change review comment on the affected line below -- use its "
        "\"Add suggestion to batch\" / \"Commit suggestion\" button to apply the fix "
        "directly to this PR (or set a different value if you know a better one).",
        "",
        "| Entry | Issue | latestVersion | previousLatestVersion (current) | Suggested previousLatestVersion |",
        "|---|---|---|---|---|",
    ]
    issue_label = {"collision": "same release as latestVersion", "stale": "newer build available upstream"}
    for entry, kind, recommendation, note in findings:
        suggestion = f"`{recommendation}`" if recommendation else f"_{note}_"
        lines.append(
            f"| `{entry.path}` | {issue_label.get(kind, kind)} | `{entry.latest_version}` | "
            f"`{entry.previous_latest_version}` | {suggestion} |"
        )
    lines.append("")
    lines.append(
        "_Suggestions are best-effort: the highest build this check found upstream for the "
        "relevant release. If a suggested value doesn't actually exist, a later "
        "CI step (schema/build validation) will catch it._"
    )
    with open(path, "w", encoding="utf-8") as f:
        f.write("\n".join(lines) + "\n")



if __name__ == "__main__":
    sys.exit(main())
