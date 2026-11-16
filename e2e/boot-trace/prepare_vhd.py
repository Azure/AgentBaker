"""Generate an opt-in Packer template without changing the standard image."""

import argparse
import copy
import json
import os
from pathlib import Path
import re
import shlex


IMAGE_NAME = "2604minimalgen2containerd-boot-trace-"
AZURELINUX_IMAGE_NAME = "AzureLinuxV3gen2-boot-trace-"


def instrument_template(template, backend, base_version, image_label):
    if backend not in ("ftrace", "lttng"):
        raise ValueError("Expected ftrace or lttng")
    if not re.fullmatch(r"\d+\.\d+\.\d+", base_version):
        raise ValueError("An exact Marketplace image version is required, not latest")
    if not re.fullmatch(r"[a-zA-Z0-9._-]+", image_label):
        raise ValueError("Invalid image label")
    result = copy.deepcopy(template)
    provisioners = result["provisioners"]
    installs = [
        i for i, step in enumerate(provisioners)
        if isinstance(step.get("inline"), list)
        and any("/home/packer/post-install-dependencies.sh" in line for line in step["inline"])
    ]
    if len(installs) != 1:
        raise ValueError("Expected exactly one post-install provisioner")
    arguments = " ".join(map(shlex.quote, (backend, base_version, image_label)))
    provisioners[installs[0]:installs[0]] = [
        {"type": "shell", "inline": ["mkdir -m 0755 /home/packer/boot-trace"]},
        {"type": "file", "source": "e2e/boot-trace/guest/",
         "destination": "/home/packer/boot-trace"},
        {"type": "shell", "inline": [
            "sudo /bin/bash -eu /home/packer/boot-trace/install.sh prepare " + arguments,
        ]},
    ]
    cleanup = [
        step for step in provisioners
        if isinstance(step.get("inline"), list)
        and any("/opt/azure/containers/cleanup-vhd.sh" in line for line in step["inline"])
    ]
    if len(cleanup) != 1:
        raise ValueError("Expected exactly one final cleanup provisioner")
    commands = cleanup[0]["inline"]
    index = next(i for i, line in enumerate(commands) if "/opt/azure/containers/cleanup-vhd.sh" in line)
    if not any("waagent" in line and "deprovision" in line for line in commands[index + 1:]):
        raise ValueError("Final cleanup must still precede deprovisioning")
    if cleanup[0] is not provisioners[-1]:
        raise ValueError("Arming must be in the final provisioner, with no later reboot or cleanup")
    commands.insert(index + 1, "sudo /bin/bash -eu /opt/aks-node-boot-trace/install.sh arm " + arguments)
    return result


def validate_environment(environment, settings):
    backend = environment.get("BOOT_TRACE_BACKEND")
    if backend not in ("ftrace", "lttng"):
        raise ValueError("BOOT_TRACE_BACKEND must be ftrace or lttng")
    required = {
        "MODE": "linuxVhdMode", "ARCHITECTURE": "X86_64", "HYPERV_GENERATION": "V2",
    }
    if environment.get("OS_SKU") == "AzureLinux":
        if backend != "ftrace":
            raise ValueError("Azure Linux 3 boot tracing only supports ftrace")
        required.update({
            "OS_SKU": "AzureLinux", "OS_VERSION": "V3", "FEATURE_FLAGS": "None",
            "IMG_PUBLISHER": "MicrosoftCBLMariner", "IMG_OFFER": "azure-linux-3",
            "IMG_SKU": "azure-linux-3-gen2",
            "SIG_IMAGE_NAME": AZURELINUX_IMAGE_NAME + backend, "ENABLE_CGROUPV2": "True",
        })
    else:
        required.update({
            "OS_SKU": "Ubuntu", "OS_VERSION": "26.04", "FEATURE_FLAGS": "minimal",
            "IMG_PUBLISHER": "Canonical", "IMG_OFFER": "ubuntu-26_04-lts", "IMG_SKU": "minimal",
            "SIG_IMAGE_NAME": IMAGE_NAME + backend,
        })
    for key, value in required.items():
        if environment.get(key) != value:
            raise ValueError(f"{key} must be {value} for a boot-trace build")
    for key in ("GENERATE_PUBLISHING_INFO", "RUN_PREFETCH_OPTIMIZATION", "ENABLE_FIPS",
                "ENABLE_TRUSTED_LAUNCH"):
        if environment.get(key, "").lower() != "false":
            raise ValueError(f"{key} must explicitly be false")
    version = environment.get("IMG_VERSION", "")
    if not re.fullmatch(r"\d+\.\d+\.\d+", version) or settings.get("img_version") != version:
        raise ValueError("Packer must use the requested exact Marketplace base image version")
    if settings.get("sig_image_name") != required["SIG_IMAGE_NAME"]:
        raise ValueError("Packer image definition must be the isolated boot-trace definition")
    return backend, version


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--template", type=Path)
    parser.add_argument("--settings", type=Path, default=Path("vhdbuilder/packer/settings.json"))
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    backend, version = validate_environment(os.environ, json.loads(args.settings.read_text()))
    template = args.template or Path(
        "vhdbuilder/packer/vhd-image-builder-mariner.json" if os.environ["OS_SKU"] == "AzureLinux"
        else "vhdbuilder/packer/vhd-image-builder-base.json"
    )
    label = os.environ["SIG_IMAGE_NAME"] + "-" + version
    result = instrument_template(json.loads(template.read_text()), backend, version, label)
    args.output.write_text(json.dumps(result, indent=2) + "\n")


if __name__ == "__main__":
    main()
