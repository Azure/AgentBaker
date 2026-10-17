#!/usr/bin/env python3
"""Record ACL image provenance and compare explicitly selected published snapshots."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
from datetime import datetime
import json
import os
from pathlib import Path
import re
import shlex
import sys
import urllib.error
import urllib.parse
import urllib.request

UPSTREAM = "https://github.com/microsoft/azure-container-linux"
API = "https://api.github.com/repos/microsoft/azure-container-linux"
PROVENANCE_SECTION = "ACL Base Image"


def release_version(tag: str) -> tuple[int, ...] | None:
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]{8}", tag):
        return None
    try:
        datetime.strptime(tag.rsplit(".", 1)[1], "%Y%m%d")
    except ValueError:
        return None
    return tuple(int(part) for part in tag.split("."))


def release_url(tag: str) -> str:
    return f"{UPSTREAM}/releases/tag/{urllib.parse.quote(tag, safe='')}"


class GitHub:
    def __init__(self) -> None:
        self.responses: dict[str, object] = {}

    def get(self, endpoint: str) -> object:
        if endpoint not in self.responses:
            headers = {
                "Accept": "application/vnd.github+json",
                "X-GitHub-Api-Version": "2022-11-28",
                "User-Agent": "AgentBaker-ACL-release-notes",
            }
            token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
            if token:
                headers["Authorization"] = f"Bearer {token}"
            request = urllib.request.Request(f"{API}/{endpoint}", headers=headers)
            try:
                with urllib.request.urlopen(request, timeout=30) as response:
                    self.responses[endpoint] = json.load(response)
            except urllib.error.HTTPError as error:
                if error.code != 404:
                    raise
                self.responses[endpoint] = None
        return self.responses[endpoint]

    def published(self, tag: str) -> bool:
        data = self.get(f"releases/tags/{urllib.parse.quote(tag, safe='')}")
        if data is None:
            return False
        if not isinstance(data, dict) or data.get("tag_name") != tag:
            raise ValueError(f"Invalid GitHub release response for {tag}")
        if not isinstance(data.get("draft"), bool) or not isinstance(data.get("prerelease"), bool):
            raise ValueError(f"Missing publication status for ACL release {tag}")
        return not data["draft"] and not data["prerelease"]

    def compare(self, previous: str, current: str) -> str:
        data = self.get(f"compare/{previous}...{current}")
        if not isinstance(data, dict) or data.get("status") not in {
            "ahead", "behind", "identical", "diverged"
        }:
            raise ValueError(f"Invalid GitHub comparison for {previous}...{current}")
        return data["status"]

    def intervening(self, previous: str, current: str) -> list[str]:
        previous_version = release_version(previous)
        current_version = release_version(current)
        if previous_version is None or current_version is None:
            raise ValueError("ACL release ranges require stable release tags")
        tags = {current}
        page = 1
        while True:
            releases = self.get(f"releases?per_page=100&page={page}")
            if not isinstance(releases, list) or any(
                not isinstance(item, dict)
                or not isinstance(item.get("tag_name"), str)
                or not isinstance(item.get("draft"), bool)
                or not isinstance(item.get("prerelease"), bool)
                for item in releases
            ):
                raise ValueError(f"Invalid GitHub release list on page {page}")
            for item in releases:
                tag = item.get("tag_name")
                version = release_version(tag) if isinstance(tag, str) else None
                if (
                    version is not None
                    and version[:2] == current_version[:2]
                    and previous_version < version <= current_version
                    and item.get("draft") is False
                    and item.get("prerelease") is False
                ):
                    tags.add(tag)
            if len(releases) < 100:
                break
            page += 1
        return [
            tag for tag in sorted(tags, key=release_version)
            if tag == current or self.compare(tag, current) in {"ahead", "identical"}
        ]


def section(text: str, name: str, required: bool = True) -> dict[str, str]:
    begin, end = f"=== {name} Begin", f"=== {name} End"
    blocks = re.findall(rf"(?ms)^{re.escape(begin)}\n(.*?)^{re.escape(end)}$", text)
    if len(blocks) != 1 or text.count(begin) != 1 or text.count(end) != 1:
        if not required and not blocks and begin not in text and end not in text:
            return {}
        raise ValueError(f"Expected exactly one complete {name} section")
    values = {}
    for line in blocks[0].splitlines():
        if not line or line.startswith("#"):
            continue
        key, separator, value = line.partition("=")
        words = shlex.split(value)
        if not separator or key in values or len(words) > 1:
            raise ValueError(f"Invalid or duplicate {name} key: {key}")
        values[key] = words[0] if words else ""
    return values


def validate_base_image(source: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/[0-9]+\.[0-9]+\.[0-9]+", source):
        raise ValueError("ACL provenance requires a gallery, image definition, and pinned ACG version")
    return source


@dataclass(frozen=True)
class Image:
    tag: str
    base_image: str | None


def read_image(text: str) -> Image:
    os_release = section(text, "os-release")
    if (
        os_release.get("ID") != "azurecontainerlinux"
        and os_release.get("VARIANT_ID") != "azurecontainerlinux"
    ):
        raise ValueError("Release notes do not describe an ACL image")
    tag = os_release.get("VERSION_ID", "")
    if not re.fullmatch(r"[A-Za-z0-9_.+-]+", tag):
        raise ValueError("Missing or invalid ACL VERSION_ID in image release notes")
    provenance = section(text, PROVENANCE_SECTION, required=False)
    if not provenance:
        return Image(tag, None)
    if provenance.get("ACL_RELEASE_TAG") != tag:
        raise ValueError("Recorded ACL provenance disagrees with the image VERSION_ID")
    url = provenance.get("ACL_RELEASE_URL")
    if url not in {"", release_url(tag)}:
        raise ValueError("Missing or mismatched ACL release URL in provenance")
    return Image(tag, validate_base_image(provenance.get("ACL_ACG_SOURCE", "")))


def enrich_image(notes: Path) -> None:
    text = notes.read_text(encoding="utf-8")
    image = read_image(text)
    source = validate_base_image("/".join(
        os.environ.get(key, "") for key in (
            "SIG_SOURCE_GALLERY_UNIQUE_NAME", "SIG_SOURCE_IMAGE_NAME", "SIG_SOURCE_IMAGE_VERSION"
        )
    ))
    if image.base_image is not None and image.base_image != source:
        raise ValueError("Existing ACL provenance disagrees with the pinned ACG source")
    url = ""
    if release_version(image.tag) is not None:
        url = release_url(image.tag)
    else:
        print(
            f"WARNING: {image.tag} is not a stable ACL release tag; "
            "recording provenance without a release URL",
            file=sys.stderr,
        )
    block = (
        f"=== {PROVENANCE_SECTION} Begin\n"
        f"ACL_RELEASE_TAG={image.tag}\n"
        f"ACL_RELEASE_URL={url}\n"
        f"ACL_RELEASE_URL_STATUS={'unverified' if url else 'not-a-release-tag'}\n"
        f"ACL_ACG_SOURCE={source}\n"
        f"=== {PROVENANCE_SECTION} End\n"
    )
    text = re.sub(
        rf"(?ms)^=== {PROVENANCE_SECTION} Begin\n.*?^=== {PROVENANCE_SECTION} End\n?",
        "", text,
    )
    notes.write_text(f"{text.rstrip()}\n\n{block}", encoding="utf-8")


def require_published(image: Image, github: GitHub) -> tuple[int, ...]:
    version = release_version(image.tag)
    if version is None or not github.published(image.tag):
        raise ValueError(f"Cannot generate a weekly rollup for unpublished ACL release {image.tag}")
    return version


def weekly_rollup(previous: Path, current: Path, github: GitHub) -> str:
    if not previous.is_dir() or not current.is_dir():
        raise ValueError("Both published-image snapshot directories must exist")
    paths = sorted(current.rglob("latest.txt"))
    if not paths:
        raise ValueError("Current snapshot contains no per-SKU latest.txt release notes")
    lines = ["## ACL base OS releases", ""]
    for path in paths:
        relative = path.relative_to(current)
        sku = relative.parent.as_posix()
        if not re.fullmatch(r"[A-Za-z0-9_./-]+", sku):
            raise ValueError(f"Invalid SKU path: {sku}")
        image = read_image(path.read_text(encoding="utf-8"))
        image_version = require_published(image, github)
        lines.extend([
            f"### {sku}", "",
            f"- Shipped ACL release: [{image.tag}]({release_url(image.tag)})",
            f"- ACG source: `{image.base_image}`" if image.base_image else "- ACG source: not recorded in historical notes.",
        ])
        baseline = previous / relative
        if not baseline.is_file():
            lines.extend(["", "No previous published-image baseline for this SKU.", ""])
            continue
        old = read_image(baseline.read_text(encoding="utf-8"))
        old_version = require_published(old, github)
        lines.append(f"- Previous ACL release: [{old.tag}]({release_url(old.tag)})")
        if old.base_image:
            lines.append(f"- Previous ACG source: `{old.base_image}`")
        lines.append("")
        if old.tag == image.tag:
            lines.append("Upstream release unchanged.")
            if old.base_image and image.base_image and old.base_image != image.base_image:
                lines.append("ACG source build changed; this is not a new upstream ACL release.")
        else:
            status = github.compare(old.tag, image.tag)
            if status == "diverged":
                raise ValueError(f"{sku}: ACL releases diverged; review the upstream baseline manually")
            lines.append(f"[Upstream comparison]({UPSTREAM}/compare/{old.tag}...{image.tag})")
            if status == "behind" or image_version < old_version:
                lines.append("Upstream source/tag rollback; no forward release rollup.")
            else:
                if old_version[:2] != image_version[:2]:
                    raise ValueError(f"{sku}: ACL release line changed; review the baseline manually")
                lines.extend(["", "Included upstream release notes:", ""])
                lines.extend(f"- [{tag}]({release_url(tag)})" for tag in github.intervening(old.tag, image.tag))
        lines.append("")
    return "\n".join(lines) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    image = commands.add_parser("image", help="Enrich exported ACL image notes using the pinned ACG source environment")
    image.add_argument("notes", type=Path)
    weekly = commands.add_parser("weekly", help="Compare previous/current published per-SKU latest.txt snapshots")
    weekly.add_argument("--previous-notes", type=Path, required=True)
    weekly.add_argument("--current-notes", type=Path, required=True)
    weekly.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        if args.command == "image":
            enrich_image(args.notes)
        else:
            result = weekly_rollup(args.previous_notes, args.current_notes, GitHub())
            args.output.write_text(result, encoding="utf-8")
    except (OSError, ValueError, urllib.error.URLError) as error:
        parser.exit(1, f"ERROR: {error}\n")


if __name__ == "__main__":
    main()
