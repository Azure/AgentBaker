"""Test-branch-only boot comparison; never builds or changes gallery images."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlsplit
from urllib.request import urlopen


SUBSCRIPTION = "c4c3550e-a965-4993-a50c-628fd38cd3e1"
REGION = "westus3"
API = "2024-07-01"
ARM = "https://management.azure.com"
SOURCE = (
    "/SharedGalleries/035db282-f1c8-4ce7-b78f-2a7265d5398c-ACLDEVEL"
    "/Images/acl-3.0-amd64/Versions/20260923.1227393.1"
)
CAPTURE = (
    f"/subscriptions/{SUBSCRIPTION}/resourceGroups/aksvhdtestbuildrg"
    "/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS"
    "/images/aclgen2TL/versions/1.1792202959.29660"
)
SUBNET = (
    f"/subscriptions/{SUBSCRIPTION}"
    "/resourceGroups/nodesig-test-westus3-packer-vnet-rg"
    "/providers/Microsoft.Network/virtualNetworks/nodesig-packer-vnet-westus3"
    "/subnets/scanning"
)


def az(*args):
    result = subprocess.run(
        ["az", *args, "--only-show-errors", "-o", "json"],
        capture_output=True, text=True, timeout=180,
    )
    if result.returncode:
        raise RuntimeError(f"Azure CLI {args[0]} failed: {result.stderr}")
    return json.loads(result.stdout) if result.stdout.strip() else None


def rest(method, resource, body=None, query=None):
    args = ["rest", "--method", method, "--url",
            f"{ARM}{resource}?api-version={API}" + (f"&{query}" if query else "")]
    if body is None:
        return az(*args)
    with tempfile.TemporaryDirectory() as tmp:
        path = Path(tmp) / "request.json"
        path.write_text(json.dumps(body), encoding="utf-8")
        return az(*args, "--body", f"@{path}")


def context():
    build = os.environ["BUILD_BUILDID"]
    attempt = os.environ["SYSTEM_JOBATTEMPT"]
    if not build.isdigit() or not attempt.isdigit():
        raise ValueError("Numeric pipeline build ID and job attempt are required")
    if os.environ["BUILD_SOURCEBRANCH"] != (
        "refs/heads/aadagarwal/diagnose-acl-capture-boot-20261017"
    ):
        raise ValueError("This diagnostic is restricted to its test branch")
    if az("account", "show")["id"] != SUBSCRIPTION:
        raise ValueError("Unexpected subscription; refusing resource changes")
    group = f"acl-boot-diag-{build}-{attempt}"
    tags = {
        "source": "AgentBaker",
        "qualification": "acl-capture-boot-diagnostic",
        "build": build,
        "attempt": attempt,
        "project": os.environ["SYSTEM_TEAMPROJECTID"],
        "branch": os.environ["BUILD_SOURCEBRANCH"],
    }
    output = Path(os.environ["DIAGNOSTIC_OUTPUT"])
    output.mkdir(parents=True, exist_ok=True)
    return group, tags, output


def save(output, name, value):
    (output / name).write_text(json.dumps(value, indent=2), encoding="utf-8")


def vm_body(name, image, nic, public_key, tags):
    return {
        "location": REGION,
        "tags": tags,
        "properties": {
            "hardwareProfile": {"vmSize": "Standard_D2ds_v5"},
            "storageProfile": {
                "imageReference": image,
                "osDisk": {
                    "createOption": "FromImage", "diskSizeGB": 60,
                    "managedDisk": {"storageAccountType": "Premium_LRS"},
                    "deleteOption": "Delete",
                },
            },
            "osProfile": {
                "computerName": name, "adminUsername": "azureuser",
                "linuxConfiguration": {
                    "disablePasswordAuthentication": True,
                    "provisionVMAgent": True,
                    "ssh": {"publicKeys": [{
                        "path": "/home/azureuser/.ssh/authorized_keys",
                        "keyData": public_key,
                    }]},
                },
            },
            "networkProfile": {"networkInterfaces": [{"id": nic}]},
            "securityProfile": {
                "securityType": "TrustedLaunch",
                "uefiSettings": {"secureBootEnabled": True, "vTpmEnabled": True},
            },
            "diagnosticsProfile": {"bootDiagnostics": {"enabled": True}},
        },
    }


def capture_diagnostics(output, name, vm_id, suffix):
    evidence = {}
    for kind, resource, query in (
        ("vm", vm_id, None),
        ("instance-view", vm_id + "/instanceView", None),
    ):
        try:
            evidence[kind] = rest("get", resource, query=query)
            save(output, f"{name}-{suffix}-{kind}.json", evidence[kind])
        except (RuntimeError, subprocess.TimeoutExpired) as error:
            evidence[kind] = {"error": str(error)}
            save(output, f"{name}-{suffix}-{kind}-error.json", evidence[kind])
            print(f"{name}: {kind} collection failed: {error}", flush=True)
    try:
        links = rest("post", vm_id + "/retrieveBootDiagnosticsData",
                     query="sasUriExpirationTimeInMinutes=10")
        for key, extension in (("serialConsoleLogBlobUri", "serial.log"),
                               ("consoleScreenshotBlobUri", "screenshot.bmp")):
            uri = links.get(key)
            if not uri:
                raise RuntimeError(f"Boot diagnostics did not return {key}")
            parsed = urlsplit(uri)
            if (parsed.scheme != "https" or not parsed.hostname
                    or not parsed.hostname.endswith(".blob.core.windows.net")):
                raise ValueError("Unexpected boot diagnostics download host")
            with urlopen(uri, timeout=60) as response:
                data = response.read()
            (output / f"{name}-{suffix}-{extension}").write_bytes(data)
            print(f"{name}: saved {extension}, {len(data)} bytes", flush=True)
    except (RuntimeError, ValueError, HTTPError, URLError, TimeoutError,
            subprocess.TimeoutExpired) as error:
        # Never persist diagnostic SAS URLs in logs or artifacts.
        message = f"Boot diagnostics download failed ({type(error).__name__})"
        if isinstance(error, HTTPError):
            message += f": HTTP {error.code}"
        save(output, f"{name}-{suffix}-download-error.json", {"error": message})
        print(f"{name}: {message}", flush=True)
    return evidence


def diagnose():
    group, tags, output = context()
    if az("group", "exists", "--name", group):
        raise RuntimeError("Diagnostic resource group already exists; refusing reuse")
    az("group", "create", "--name", group, "--location", REGION,
       "--tags", *[f"{key}={value}" for key, value in tags.items()])
    save(output, "capture-image.json",
         rest("get", CAPTURE, query="$expand=UefiSettings"))
    images = {"original": {"sharedGalleryImageId": SOURCE}, "captured": {"id": CAPTURE}}
    vm_ids = {}
    with tempfile.TemporaryDirectory() as tmp:
        key = Path(tmp) / "key"
        subprocess.run(["ssh-keygen", "-q", "-t", "rsa", "-b", "3072",
                        "-N", "", "-f", str(key)], check=True, timeout=30)
        public_key = key.with_suffix(".pub").read_text()
        for name, image in images.items():
            nic = az("network", "nic", "create", "--resource-group", group,
                     "--name", name, "--subnet", SUBNET)["NewNIC"]["id"]
            vm_id = (f"/subscriptions/{SUBSCRIPTION}/resourceGroups/{group}"
                     f"/providers/Microsoft.Compute/virtualMachines/{name}")
            body = vm_body(name, image, nic, public_key, tags)
            save(output, f"{name}-request.json", body)
            save(output, f"{name}-create.json", rest("put", vm_id, body))
            vm_ids[name] = vm_id
            print(f"Created {name} VM request with Secure Boot enabled", flush=True)
    deadline = time.monotonic() + 12 * 60
    states = {}
    iteration = 0
    while time.monotonic() < deadline:
        for name, vm_id in vm_ids.items():
            vm = rest("get", vm_id)
            states[name] = vm["properties"]["provisioningState"]
        print(f"Provisioning states: {json.dumps(states)}", flush=True)
        if all(state in {"Succeeded", "Failed"} for state in states.values()):
            break
        if iteration in {3, 8}:
            for name, vm_id in vm_ids.items():
                capture_diagnostics(output, name, vm_id, f"sample-{iteration}")
        iteration += 1
        time.sleep(60)
    for name, vm_id in vm_ids.items():
        evidence = capture_diagnostics(output, name, vm_id, "final")
        if states.get(name) == "Succeeded":
            result = az("vm", "run-command", "invoke", "--ids", vm_id,
                        "--command-id", "RunShellScript", "--scripts",
                        "uname -a; systemctl is-active walinuxagent; "
                        "bootctl status --no-pager; "
                        "cat /sys/kernel/security/ipe/enforce 2>/dev/null; "
                        "cat /sys/kernel/security/ipe/policies/acl_ipe_boot_policy/active 2>/dev/null")
            save(output, f"{name}-guest.json", result)
        print(f"{name} final state: {states.get(name)}; "
              f"instance view collected: {'statuses' in evidence.get('instance-view', {})}",
              flush=True)
    save(output, "summary.json", {
        "states": states, "images": images,
        "note": "Diagnostic only, not scan/E2E qualification. No trust settings changed.",
    })
    if any(state != "Succeeded" for state in states.values()):
        raise RuntimeError("One or more diagnostic VMs did not provision; inspect retained boot evidence")


def cleanup():
    group, tags, output = context()
    if not az("group", "exists", "--name", group):
        save(output, "cleanup.json", {"group": group, "exists": False})
        return
    actual = az("group", "show", "--name", group)
    if any(actual.get("tags", {}).get(key) != value for key, value in tags.items()):
        raise RuntimeError("Ownership tags differ; refusing resource-group deletion")
    az("group", "delete", "--name", group, "--yes", "--no-wait")
    for _ in range(50):
        if not az("group", "exists", "--name", group):
            save(output, "cleanup.json", {"group": group, "exists": False})
            print(f"Verified deletion of {group}", flush=True)
            return
        time.sleep(10)
    raise RuntimeError("Diagnostic resource group deletion has not completed")


if __name__ == "__main__":
    if sys.argv[1:] == ["diagnose"]:
        diagnose()
    elif sys.argv[1:] == ["cleanup"]:
        cleanup()
    else:
        raise SystemExit("Expected diagnose or cleanup")
