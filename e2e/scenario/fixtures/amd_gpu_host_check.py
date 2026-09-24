"""Read-only validation of the baked driver, AMD SMI, and MI300X devices."""

import base64
import json
from pathlib import Path
import os
import platform
import re
import subprocess
import sys


def command(*args):
    return subprocess.check_output(args, text=True).strip()


manifest = json.loads(base64.b64decode(sys.argv[1]))
expected = manifest["AMDGPUDriver"]
diagnostics = manifest["AMDGPUDiagnostics"]
marker = json.loads(Path("/opt/azure/amd-gpu/driver.json").read_text())
assert marker["schema_version"] == 1, marker
for marker_key, manifest_key in (("package_version", "packageVersion"),
                                 ("firmware_package_version", "firmwarePackageVersion"),
                                 ("module_version", "moduleVersion")):
    assert marker[marker_key] == expected[manifest_key], (marker_key, marker)
for package, key in (("amdgpu-dkms", "packageVersion"),
                     ("amdgpu-dkms-firmware", "firmwarePackageVersion")):
    assert command("dpkg-query", "-W", "-f=${Status} ${Version}", package) == f"install ok installed {expected[key]}", package
for package_key, version_key in (("amdsmiPackage", "amdsmiVersion"),
                                 ("sysdepsPackage", "sysdepsVersion")):
    package = diagnostics[package_key]
    assert command("dpkg-query", "-W", "-f=${Status} ${Version}", package) == f"install ok installed {diagnostics[version_key]}", package
cli = Path("/usr/local/bin/amd-smi")
assert cli.is_symlink() and cli.resolve() == Path(diagnostics["cliPath"]).resolve(), cli
assert os.access(cli, os.X_OK), cli
kernel = platform.release()
module_path = command("modinfo", "-F", "filename", "amdgpu")
assert module_path.startswith(f"/lib/modules/{kernel}/updates/dkms/"), module_path
assert command("modinfo", "-F", "version", "amdgpu") == expected["moduleVersion"]
assert Path("/sys/module/amdgpu/version").read_text().strip() == expected["moduleVersion"]
assert command("modinfo", "-F", "vermagic", "amdgpu").split()[0] == kernel
assert Path("/sys/module/amdgpu/srcversion").read_text().strip() == command("modinfo", "-F", "srcversion", "amdgpu")
assert f"amdgpu/{expected['dkmsVersion']}, {kernel}, x86_64: installed" in command("dkms", "status")
assert Path("/dev/kfd").is_char_device(), "/dev/kfd missing"
devices = []
for device in Path("/sys/bus/pci/devices").iterdir():
    if (device / "vendor").read_text().strip() == "0x1002" and (device / "device").read_text().strip() == "0x74b5":
        assert (device / "driver").resolve().name == "amdgpu", device
        devices.append(device.name)
assert len(devices) == 8, devices
amdsmi_version = json.loads(command(str(cli), "version", "--json"))
assert isinstance(amdsmi_version, list) and len(amdsmi_version) == 1, amdsmi_version
version = amdsmi_version[0]
assert version["tool"] == "AMDSMI Tool" and version["version"] and version["amdsmi_library_version"], version
assert version["amdgpu_version"] == expected["moduleVersion"], version
amdsmi_devices = json.loads(command(str(cli), "list", "--json"))
assert isinstance(amdsmi_devices, list) and len(amdsmi_devices) == 8, amdsmi_devices
assert {device["gpu"] for device in amdsmi_devices} == set(range(8)), amdsmi_devices
assert {device["bdf"].lower() for device in amdsmi_devices} == set(devices), amdsmi_devices
packages = command("dpkg-query", "-W", "-f=${Package}\\t${Status}\\n")
diagnostics_packages = {diagnostics["amdsmiPackage"], diagnostics["sysdepsPackage"]}
unexpected_packages = []
for line in packages.splitlines():
    package, status = line.split("\t", 1)
    package = package.split(":", 1)[0]
    if status == "install ok installed" and package not in diagnostics_packages and re.match(
            r"^(amdrocm|rocm|hip|hsa-rocr|rocblas|rocfft|rocrand|rocsolver|rocsparse|miopen|migraphx|"
            r"amdgpu-(core|lib|install|pro)|lib.*-amdgpu-|nvidia-|libnvidia-|cuda-|"
            r"datacenter-gpu-manager-|dcgm-exporter)", package):
        unexpected_packages.append(package)
assert not unexpected_packages, f"Unexpected GPU userspace packages: {unexpected_packages}"
print(json.dumps({"kernel": kernel, "module": expected["moduleVersion"], "gpu_functions": devices,
                  "baked_kernel": marker["kernel_version"], "host_rocm_sdk": False,
                  "amdsmi_version": version, "amdsmi_devices": amdsmi_devices}))
