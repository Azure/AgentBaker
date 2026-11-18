"""Prepare reproducible CA matrix inputs from checked-in sources and a CI-built binary."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import sys

from guest.diagnostics import instrument_cloud


VARIANTS = {
    "Ubuntu2604Minimal": ("original", "skip_ca", "rust", "parallel", "parallel_rust"),
    "AzureLinuxV3": ("original", "parallel"),
}
IMAGES = {
    "Ubuntu2604Minimal": "2604minimalgen2containerd-boot-trace-ftrace",
    "AzureLinuxV3": "AzureLinuxV3gen2-boot-trace-ftrace",
}


def scenario_names():
    return [f"{os_name}_BootTrace_CA_{variant}_{repeat}"
            for repeat in (1, 2, 3) for os_name, variants in VARIANTS.items()
            for variant in variants]


def prepare(source, binary, metadata_path, output):
    images = json.loads(metadata_path.read_text())
    for name in IMAGES.values():
        if not re.search(r"/versions/\d+\.\d+\.\d+$", images.get(name, {}).get("resourceId", "")):
            raise ValueError(f"Missing exact tracing VHD metadata: {name}")
    if not binary.is_file():
        raise ValueError(f"Missing CI-built Rust fixture: {binary}")
    output.mkdir(parents=True, mode=0o700, exist_ok=False)
    shutil.copyfile(binary, output / "update-ca-certificates-rs")
    digest = hashlib.sha256(binary.read_bytes()).hexdigest()
    fixtures = Path(__file__).with_name("fixtures")
    serial = (fixtures / "serial-certificate-requests.sh").read_text()
    original = source.read_text()
    for name in ("make_request_with_retry", "process_cert_operations"):
        pattern = rf"^function {name} \{{\n.*?^\}}\n"
        match = re.search(pattern, serial, re.M | re.S)
        if match is None or len(re.findall(pattern, original, re.M | re.S)) != 1:
            raise ValueError(f"Unexpected serial certificate control function: {name}")
        original = re.sub(pattern, lambda _: match[0], original, flags=re.M | re.S)
    batch = r"^download_cert_batch\(\) \(\n.*?^\)\n"
    if len(re.findall(batch, original, re.M | re.S)) != 1:
        raise ValueError("Unexpected parallel certificate batch function")
    original = re.sub(batch, "", original, flags=re.M | re.S)
    for mode, text in (("original", original), ("parallel", source.read_text())):
        if text.count("set +x\n") != 1:
            raise ValueError("Unexpected certificate script entry")
        text = text.replace("set +x\n", "set +x\n/bin/bash /opt/aks-node-boot-trace/prepare-fixture.sh || exit $?\n", 1)
        path = output / f"init-{mode}.sh"
        path.write_text(text)
        instrument_cloud(path)
    preparation = (fixtures / "prepare-fixture.sh").read_text().replace("@RUST_SHA256@", digest)
    (output / "prepare-fixture.sh").write_text(preparation)
    (output / "fixture.json").write_text(json.dumps({
        "rust_sha256": digest, "images": images, "scenario_names": scenario_names(),
        "init_sha256": {mode: hashlib.sha256((output / f"init-{mode}.sh").read_bytes()).hexdigest()
                        for mode in ("original", "parallel")},
    }, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--metadata", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        prepare(Path(__file__).resolve().parents[2] / "parts/linux/cloud-init/artifacts/init-aks-cloud.sh",
                args.binary, args.metadata, args.output)
    except (OSError, ValueError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
