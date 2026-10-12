#!/usr/bin/env python3
"""Fail closed on incomplete local OpenVMM test bundles; no downloads or signing."""

import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tomllib

PREFIX = "/opt/kata-openvmm"
CONFIG = "configuration-openvmm-azure-snp-runtime-rs"
HOST_PACKAGES = ("kernel-mshv", "mshv", "mshv-bootloader-lx", "mshv-kd", "openvmm")
REQUIRED = (
    "containerd-shim-kata-v2", "kata-openvmm-snp.bin",
    "kata-openvmm-snp-debug.bin", "kata-containers.img",
    f"{CONFIG}.toml", f"{CONFIG}-debug.toml",
    "containerd", "ctr", "mkfs.erofs", "bundle.json",
)


def sha256(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def validate_packages(manifest):
    packages = manifest.get("packages", {})
    if not isinstance(packages, dict) or set(packages) != set(HOST_PACKAGES):
        raise ValueError(f"packages must pin exactly {HOST_PACKAGES}")
    for name, version in packages.items():
        if not isinstance(version, str) or not re.fullmatch(r"\d+:[A-Za-z0-9.+_~-]+\.x86_64", version):
            raise ValueError(f"{name}: expected epoch:version-release.x86_64")
    kernel = packages["kernel-mshv"].split(":", 1)[1].removesuffix(".x86_64")
    if manifest.get("kernel_release") != kernel:
        raise ValueError("kernel_release must match the kernel-mshv package")
    if len({packages[name] for name in ("mshv", "mshv-bootloader-lx", "mshv-kd")}) != 1:
        raise ValueError("MSHV packages must come from the same version")
    hyperv = manifest.get("hyperv_running_version")
    if not isinstance(hyperv, str) or not re.fullmatch(r"10\.0\.\d+\.\d+-\d+-\d+", hyperv):
        raise ValueError("hyperv_running_version must identify the expected booted Hyper-V")
    return packages


def validate_installed_host(path):
    manifest = json.loads(path.read_text())
    packages = validate_packages(manifest)
    for name, expected in packages.items():
        actual = subprocess.check_output(
            ["rpm", "-q", "--qf", "%{EPOCHNUM}:%{VERSION}-%{RELEASE}.%{ARCH}\\n",
             f"{name}-{expected}"], text=True).strip()
        if actual != expected:
            raise ValueError(f"{name}: installed package {actual!r} does not match {expected!r}")
    if subprocess.check_output(["rpm", "-qf", "--qf", "%{NAME}", "/usr/bin/openvmm"],
                               text=True).strip() != "openvmm":
        raise ValueError("/usr/bin/openvmm must be provided by the openvmm RPM")
    verification = subprocess.run(
        ["rpm", "-V", "--nomode", "--nomtime",
         *(f"{name}-{version}" for name, version in packages.items())],
        capture_output=True, text=True,
    )
    if verification.returncode not in (0, 1) or verification.stderr:
        raise ValueError(f"RPM verification failed: {verification.stderr}")
    # depmod regenerates these indexes during normal kernel package installation.
    indexes = {"modules.alias", "modules.alias.bin", "modules.builtin.alias.bin",
               "modules.builtin.bin", "modules.dep", "modules.dep.bin", "modules.devname",
               "modules.softdep", "modules.symbols", "modules.symbols.bin"}
    for line in verification.stdout.splitlines():
        fields = line.split()
        if (len(fields) != 2 or fields[0] not in ("S.5......", "..5......", "S........")
                or Path(fields[1]).parent != Path("/lib/modules") / manifest["kernel_release"]
                or Path(fields[1]).name not in indexes):
            raise ValueError(f"RPM payload verification failed: {line}")
        print(f"RPM regenerated module index: {fields[1]}", file=sys.stderr)


def validate_boot_evidence(host, kernel_release, boot_id, previous_boot_id, kernel_log):
    if not boot_id.strip() or not previous_boot_id.strip() or boot_id.strip() == previous_boot_id.strip():
        raise ValueError("a changed boot ID after the separately approved reboot is required")
    if kernel_release != host["kernel_release"]:
        raise ValueError("running kernel must match the kernel-mshv package")
    if "Hyper-V: running as root partition" not in kernel_log:
        raise ValueError("Hyper-V root partition evidence is missing")
    version = re.escape(host["hyperv_running_version"])
    if not re.search(r"Hyper-V: (?:Hypervisor|Host) Build " + version + r"(?:\s|$)", kernel_log):
        raise ValueError("dmesg does not report the pinned Hyper-V host build")
    if "misc mshv: SEV-SNP support status: available" not in kernel_log:
        raise ValueError("MSHV SEV-SNP availability evidence is missing")


def strings(value):
    if isinstance(value, dict):
        for child in value.values():
            yield from strings(child)
    elif isinstance(value, list):
        for child in value:
            yield from strings(child)
    elif isinstance(value, str):
        yield value


def debug_enabled(value):
    if isinstance(value, dict):
        return any(("debug" in key and child is True) or debug_enabled(child)
                   for key, child in value.items())
    if isinstance(value, list):
        return any(debug_enabled(child) for child in value)
    return False


def validate_config(path, debug):
    config = tomllib.loads(path.read_text())
    hypervisor = config["hypervisor"]["openvmm"]
    expected = {
        "path": f"{PREFIX}/bin/openvmm",
        "image": f"{PREFIX}/share/kata-containers/kata-containers.img",
        "default_vcpus": 1,
        "default_memory": 1024,
        "overhead_vcpus": 0,
        "overhead_memory": 0,
        "confidential_guest": True,
        "sev_snp_guest": True,
    }
    for key, value in expected.items():
        if hypervisor.get(key) != value:
            raise ValueError(f"{path.name}: expected hypervisor.openvmm.{key}={value!r}")
    igvm = f"{PREFIX}/share/kata-containers/kata-openvmm-snp{'-debug' if debug else ''}.bin"
    if hypervisor.get("igvm") != igvm:
        raise ValueError(f"{path.name}: must select {igvm}")
    if hypervisor.get("enable_debug", False) != debug:
        raise ValueError(f"{path.name}: enable_debug must be {debug}")
    if config.get("runtime", {}).get("enable_debug", False) != debug:
        raise ValueError(f"{path.name}: runtime.enable_debug must be {debug}")
    if config.get("runtime", {}).get("hypervisor_name") != "openvmm":
        raise ValueError(f"{path.name}: runtime.hypervisor_name must be openvmm")
    if config.get("runtime", {}).get("static_sandbox_resource_mgmt") is not True:
        raise ValueError(f"{path.name}: runtime.static_sandbox_resource_mgmt must be true")
    for key, value in (("static_sandbox_default_workload_vcpus", 1),
                       ("static_sandbox_default_workload_mem", 1024)):
        if config.get("runtime", {}).get(key) != value:
            raise ValueError(f"{path.name}: expected runtime.{key}={value!r}")
    if not debug and debug_enabled(config):
        raise ValueError(f"{path.name}: production configuration enables debugging")


def validate_bundle(bundle):
    manifest = json.loads((bundle / "bundle.json").read_text())
    for name in ("openvmm", "host-components.json", "host", "hypervisor", "kdcom.dll", "kdnet.dll"):
        if (bundle / name).exists():
            raise ValueError(f"remove obsolete package-provided bundle artifact: {name}")
    if manifest.get("unsigned_test_only") is not True:
        raise ValueError("bundle.json must explicitly declare unsigned_test_only=true")
    validate_packages(manifest)
    for key in ("package_source", "guest_kernel_package", "containerd_source", "containerd_version",
                "erofs_utils_source", "kata_source"):
        if not isinstance(manifest.get(key), str) or not manifest[key].strip():
            raise ValueError(f"bundle.json requires provenance: {key}")
    if not re.search(r"\bv2\.", manifest["containerd_version"]):
        raise ValueError("a compatible containerd 2.x version is required")
    if manifest.get("erofs_default_size_zero") != "directory-backed-no-fsmerge":
        raise ValueError("explicit audited containerd default_size=0 semantics required")
    measurements = manifest.get("measurements", {})
    for variant in ("production", "debug"):
        for kind in ("json", "cbor", "idblocks"):
            value = measurements.get(variant, {}).get(kind)
            if not isinstance(value, str) or not value.startswith("measurements/"):
                raise ValueError(f"measurements.{variant}.{kind} must name a native measurements/ file")
    checksums = {}
    if any(path.is_symlink() for path in bundle.rglob("*")):
        raise ValueError("not a regular bundle: symlinks are not allowed")
    for line in (bundle / "SHA256SUMS").read_text().splitlines():
        match = re.fullmatch(r"([0-9a-f]{64}) [ *](.+)", line)
        if not match:
            raise ValueError("SHA256SUMS must use sha256sum format")
        digest, name = match.groups()
        path = Path(name)
        if path.is_absolute() or ".." in path.parts or name in checksums:
            raise ValueError(f"unsafe or duplicate checksum path: {name}")
        checksums[name] = digest
    required = set(REQUIRED)
    required.update(strings(measurements))
    files = {str(p.relative_to(bundle)) for p in bundle.rglob("*") if p.is_file()}
    if files - {"SHA256SUMS"} != set(checksums):
        raise ValueError("SHA256SUMS must cover every bundle file (except itself), without extras")
    for name in required:
        if name not in checksums:
            raise ValueError(f"missing required checksummed artifact: {name}")
    for name, digest in checksums.items():
        path = bundle / name
        if not path.is_file() or path.is_symlink() or bundle.resolve() not in path.resolve().parents:
            raise ValueError(f"not a regular bundle file: {name}")
        actual = sha256(path)
        if actual != digest or path.stat().st_size == 0:
            raise ValueError(f"empty or checksum mismatch: {name}")
    for debug in (False, True):
        validate_config(bundle / f"{CONFIG}{'-debug' if debug else ''}.toml", debug)
    return manifest


if __name__ == "__main__":
    try:
        if sys.argv[1] == "--installed-host":
            validate_installed_host(Path(sys.argv[2]))
        elif sys.argv[1] == "--boot-evidence":
            host = json.loads(Path(sys.argv[2]).read_text())
            validate_packages(host)
            kernel_log = subprocess.run(["dmesg", "--color=never"], check=True,
                                        capture_output=True, text=True).stdout
            validate_boot_evidence(host, os.uname().release,
                                   Path("/proc/sys/kernel/random/boot_id").read_text(),
                                   Path(sys.argv[3]).read_text(), kernel_log)
            descriptor = os.open("/dev/mshv", os.O_RDWR | os.O_CLOEXEC)
            os.close(descriptor)
        else:
            validate_bundle(Path(sys.argv[1]))
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        sys.exit(f"OpenVMM test bundle rejected: {error}")
