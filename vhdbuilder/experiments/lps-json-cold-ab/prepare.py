#!/usr/bin/env python3
"""Generate this isolated branch's test-only cache input and immutable registry metadata."""
import hashlib
import json
from pathlib import Path
import re
import urllib.request

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
REFERENCES = [
    "mcr.microsoft.com/containernetworking/cilium/cilium-distroless-init:v1.19.10-261107",
    "mcr.microsoft.com/containernetworking/cilium/cilium-distroless:v1.19.10-261107",
    "mcr.microsoft.com/containernetworking/v2/azure-cns:v1.8.15-4",
    "mcr.microsoft.com/containernetworking/v2/azure-ipam:v0.4.4-3",
    "mcr.microsoft.com/containernetworking/v2/azure-iptables-monitor:v0.0.8-4",
    "mcr.microsoft.com/containernetworking/v2/cilium-log-collector:v0.0.6-3",
    "mcr.microsoft.com/oss/v2/azure/ip-masq-agent-v2:v0.1.16-24",
]
ACCEPT = ", ".join(["application/vnd.oci.image.index.v1+json",
                     "application/vnd.docker.distribution.manifest.list.v2+json",
                     "application/vnd.oci.image.manifest.v1+json",
                     "application/vnd.docker.distribution.manifest.v2+json"])


def write(path, data):
    path.write_text(json.dumps(data, indent=2) + "\n")


def filter_components(original, excluded):
    text = original.decode()
    decoder = json.JSONDecoder()
    removed = []
    for category in ("ContainerImages", "GPUContainerImages", "OCIArtifacts"):
        start = re.search(r'"' + category + r'":\s*(\[)', text).start(1)
        entries, length = decoder.raw_decode(text[start:])
        if not any(item.get("downloadURL", "").rsplit(":", 1)[0] in excluded for item in entries):
            continue
        cursor = start + 1
        keep = []
        for item in entries:
            while text[cursor].isspace() or text[cursor] == ",":
                cursor += 1
            _, size = decoder.raw_decode(text[cursor:])
            if item.get("downloadURL", "").rsplit(":", 1)[0] in excluded:
                removed.append(dict(category=category, entry=item))
            else:
                keep.append(text[cursor:cursor + size])
            cursor += size
        text = text[:start] + "[\n    " + ",\n    ".join(keep) + "\n  ]" + text[start + length:]
    return text.encode(), removed


def graph(reference):
    repository, tag = reference.removeprefix("mcr.microsoft.com/").rsplit(":", 1)

    def get(kind, identity):
        req = urllib.request.Request(
            f"https://mcr.microsoft.com/v2/{repository}/{kind}/{identity}",
            headers={"Accept": ACCEPT})
        with urllib.request.urlopen(req, timeout=30) as response:
            data = response.read()
        digest = "sha256:" + hashlib.sha256(data).hexdigest()
        if identity.startswith("sha256:") and digest != identity:
            raise ValueError("registry metadata digest mismatch")
        return json.loads(data), dict(digest=digest, size=len(data))

    index, index_descriptor = get("manifests", tag)
    if "manifests" in index:
        selected = [item for item in index["manifests"]
                    if item.get("platform", {}).get("os") == "linux"
                    and item.get("platform", {}).get("architecture") == "amd64"]
        if len(selected) != 1:
            raise ValueError("ambiguous platform")
        manifest, manifest_descriptor = get("manifests", selected[0]["digest"])
    else:
        manifest, manifest_descriptor = index, index_descriptor
    config, config_descriptor = get("blobs", manifest["config"]["digest"])
    if config["architecture"] != "amd64" or config["os"] != "linux":
        raise ValueError("wrong image platform")
    chain_ids = []
    for diff_id in config["rootfs"]["diff_ids"]:
        chain_ids.append(diff_id if not chain_ids else "sha256:" + hashlib.sha256(
            (chain_ids[-1] + " " + diff_id).encode()).hexdigest())
    return dict(reference=reference, index=index_descriptor, manifest=manifest_descriptor,
                config=config_descriptor, layers=manifest["layers"],
                diff_ids=config["rootfs"]["diff_ids"], chain_ids=chain_ids)


def main():
    # Freeze registry metadata BEFORE mutating the cache recipe.
    inventory = dict(schemaVersion=1, platform="linux/amd64", images=[graph(ref) for ref in REFERENCES])
    write(HERE / "expected-inventory.json", inventory)
    components_path = ROOT / "parts/common/components.json"
    original = components_path.read_bytes()
    selected_repos = {ref.rsplit(":", 1)[0] for ref in REFERENCES}
    # Native/legacy repository aliases can share identical image graphs.
    excluded = selected_repos | {repo.replace("/v2/", "/") for repo in selected_repos}
    filtered, removed = filter_components(original, excluded)
    components_path.write_bytes(filtered)
    write(HERE / "cache-input-record.json", dict(
        schemaVersion=1, purpose="TEST-ONLY selected-networking-image-cold A/B",
        original_components_sha256=hashlib.sha256(original).hexdigest(),
        excluded_repositories=sorted(excluded), removed=removed,
        output_components_sha256=hashlib.sha256(components_path.read_bytes()).hexdigest()))


if __name__ == "__main__":
    main()
