#!/usr/bin/env python3
"""Prepare (never execute) the normal AL3 Packer build with local test artifacts."""

import argparse
import json
import re
from pathlib import Path

from validate import validate_bundle


def prepare(bundle, output, map_settings=None, build_id=None):
    bundle = bundle.resolve()
    validate_bundle(bundle)
    template = json.loads(Path("vhdbuilder/packer/vhd-image-builder-mariner.json").read_text())
    template["variables"].update({
        "feature_flags": "kata",
        "hyperv_generation": "V2",
        "openvmm_unsigned_test_only": "true",
    })
    if map_settings is not None:
        settings = json.loads(map_settings.read_text())
        subscription = "035db282-f1c8-4ce7-b78f-2a7265d5398c"
        if settings.get("subscription_id") != subscription:
            raise ValueError("OpenVMM evaluation requires the MAP test subscription")
        if settings.get("gallery_subscription_id", subscription) != subscription:
            raise ValueError("OpenVMM evaluation cannot publish outside the MAP test subscription")
        if not isinstance(build_id, str) or not re.fullmatch(r"[1-9][0-9]{0,9}", build_id):
            raise ValueError("a numeric MAP build ID is required")
        # Preserve MAP's builder, authentication, networking, SKU and capture settings.
        # Name only its temporary resources so the installer can verify exact IMDS identity.
        resource_group = f"pkr-openvmm-{build_id}"
        vm_name = f"openvmm-{build_id}"
        template["builders"][0].update({
            "temp_resource_group_name": resource_group,
            "temp_compute_name": vm_name,
        })
        template["variables"].update({
            "openvmm_install_host": "true",
            "openvmm_build_vm_name": vm_name,
            "openvmm_build_resource_group": resource_group,
            "openvmm_build_subscription": subscription,
        })
    else:
        if build_id is not None:
            raise ValueError("MAP settings are required with a build ID")
        template["variables"].update({
            "openvmm_install_host": "false", "openvmm_build_vm_name": "",
            "openvmm_build_resource_group": "", "openvmm_build_subscription": "",
        })
    provisioners = template["provisioners"]
    install_index = next(i for i, p in enumerate(provisioners)
                         if isinstance(p.get("inline"), list) and
                         any("/home/packer/install-dependencies.sh" in line
                             for line in p["inline"]))
    provisioners[install_index + 1:install_index + 1] = [
        {"type": "shell", "inline": [
            "mkdir /home/packer/openvmm-artifacts /home/packer/openvmm-integration"
        ]},
        {"type": "file", "source": str(bundle) + "/", "destination": "/home/packer/openvmm-artifacts"},
        {"type": "file", "source": "vhdbuilder/packer/openvmm/",
         "destination": "/home/packer/openvmm-integration"},
        {"type": "shell", "inline": [
            "sudo OPENVMM_UNSIGNED_TEST_ONLY={{user `openvmm_unsigned_test_only`}} "
            "OPENVMM_INSTALL_HOST={{user `openvmm_install_host`}} "
            "OPENVMM_BUILD_VM_NAME={{user `openvmm_build_vm_name`}} "
            "OPENVMM_BUILD_RESOURCE_GROUP={{user `openvmm_build_resource_group`}} "
            "OPENVMM_BUILD_SUBSCRIPTION={{user `openvmm_build_subscription`}} "
            "/bin/bash /home/packer/openvmm-integration/install.sh "
            "/home/packer/openvmm-artifacts"
        ]},
    ]
    # The normal dependency flow already reboots before post-install-dependencies.
    check_index = next(i for i, p in enumerate(provisioners)
                       if isinstance(p.get("inline"), list) and
                       any("/home/packer/post-install-dependencies.sh" in line
                           for line in p["inline"]))
    provisioners.insert(check_index, {"type": "shell", "inline": [
        "sudo /opt/kata-openvmm/integration/check-image.sh"
    ]})
    rendered = json.dumps(template, indent=2) + "\n"
    # MAP retries this task after transient Packer failures (including quota).
    # Reuse only identical output for this build; never overwrite an operator's
    # edits or trust stale output instead of validating the current inputs above.
    if map_settings is not None and output.exists():
        if output.is_symlink() or not output.is_file() or output.read_bytes() != rendered.encode():
            raise FileExistsError(f"refusing to reuse differing or non-regular template: {output}")
        print(f"Reusing identical MAP Packer template for build {build_id}")
        return
    # Standalone preparation still refuses any existing output.
    with output.open("x") as stream:
        stream.write(rendered)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifacts", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--allow-unsigned-test-artifacts", action="store_true", required=True)
    parser.add_argument("--map-settings", type=Path,
                        help="Existing MAP-generated Packer settings for the approved test build")
    parser.add_argument("--build-id", help="MAP build ID used to identify temporary build resources")
    args = parser.parse_args()
    prepare(args.artifacts, args.output, args.map_settings, args.build_id)
