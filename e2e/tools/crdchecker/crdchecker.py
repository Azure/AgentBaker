import argparse
import json
from packaging.version import Version

def extract_installed_packages(relnotes_file):
    installed_packages = {}
    while True:
        line = relnotes_file.readline()
        if not line:
            raise ValueError("release notes file ended early")
        if line.startswith("=== Installed Packages Begin"):
            next_line = relnotes_file.readline()
            if not next_line.startswith("Listing..."):
                raise ValueError("Unexpected format in release notes")
            break
    # Now we keep reading until we see the end marker
    while True:
        line = relnotes_file.readline()
        if not line:
            raise ValueError("release notes file ended early")
        if line.startswith("=== Installed Packages End"):
            break
        parts = line.split()
        if len(parts) < 2:
            continue
        package_name_and_repo = parts[0]
        package_version = parts[1]
        package_name = package_name_and_repo.split("/")[0]
        installed_packages[package_name] = package_version
    return installed_packages

CRD_MAPPING_LITERALS = {
    "DOCA": "doca-ofed",
    "IMEX": "nvidia-imex",
}

CRD_MAPPING_REGEXES = {
    "NVIDIA": "libnvidia-common-.*",
    "CUDA": "cuda-driver-dev-.*",
    "DCGM": "datacenter-gpu-manager-.*",
}

def compare(hpc_crd, installed_packages):
    for crd_key, package_name in CRD_MAPPING_LITERALS.items():
        if crd_key not in hpc_crd:
            raise ValueError(f"CRD missing key: {crd_key}")
        if package_name not in installed_packages:
            raise ValueError(f"Installed packages missing expected package: {package_name}")
        crd_version = Version(hpc_crd[crd_key])
        package_version = installed_packages[package_name]
        installed_version = Version(package_version)
        if crd_version != installed_version:
            if crd_version > installed_version:
                raise ValueError(f"CRD version for {package_name} is newer than installed version: CRD has {crd_version}, installed has {installed_version}")
            print(f"Installed version for {package_name} is fresher than CRD: installed has {package_version}, CRD has {hpc_crd[crd_key]}")

    import re
    for crd_key, pattern in CRD_MAPPING_REGEXES.items():
        if crd_key not in hpc_crd:
            raise ValueError(f"CRD missing key: {crd_key}")
        matched = False
        for package_name, package_version in installed_packages.items():
            if re.fullmatch(pattern, package_name):
                matched = True
                crd_version = Version(hpc_crd[crd_key])
                # NVIDIA driver is a bit weird. Crop the -0ubuntu1 suffix from installed version.
                if package_version.endswith("-0ubuntu1"):
                    package_version = package_version.removesuffix("-0ubuntu1")
                # DCGM is also a bit weird. It has a 1: prefix.
                if package_name.startswith("datacenter-gpu-manager-"):
                    if package_version.startswith("1:"):
                        package_version = package_version.removeprefix("1:")
                installed_version = Version(package_version)
                if crd_version != installed_version:
                    if crd_version > installed_version:
                        raise ValueError(f"Version mismatch for {package_name}: CRD has {crd_version}, installed has {installed_version}")
                    print(f"Installed version for {crd_key} aka {package_name} is fresher than CRD: installed has {package_version}, CRD has {hpc_crd[crd_key]}")
        if not matched:
            raise ValueError(f"No installed package matches pattern for {crd_key}: {pattern}")
    
    # Kernel is a bit weird. Crop the -azure-nvidia suffix from CRD.
    if "KERNEL" not in hpc_crd:
        raise ValueError("CRD missing key: KERNEL")
    kernel_crd_version_str = hpc_crd["KERNEL"].removesuffix("-azure-nvidia")
    kernel_crd_version = Version(kernel_crd_version_str)
    package_version = installed_packages["linux-azure-nvidia"]
    # If there are any dashes in the middle of the version, convert them to dots.
    package_version = package_version.replace("-", ".")
    kernel_installed_version = Version(package_version)
    if kernel_installed_version is None:
        raise ValueError("Installed packages missing expected package: linux-azure-nvidia")
    if kernel_crd_version != kernel_installed_version:
        if kernel_crd_version > kernel_installed_version:
            raise ValueError(f"Version mismatch for linux-azure-nvidia: CRD has {kernel_crd_version}, installed has {kernel_installed_version}")
        print(f"Installed version for linux-azure-nvidia is fresher than CRD: installed has {installed_packages['linux-azure-nvidia']}, CRD has {kernel_crd_version_str}")

    # OFED is super weird. Parse it by hand.
    if "OFED" not in hpc_crd:
        raise ValueError("CRD missing key: OFED")
    ofed_crd_version_str = hpc_crd["OFED"]
    ofed_installed_version = installed_packages.get("mlnx-ofed-kernel-dkms", None)
    if ofed_installed_version is None:
        raise ValueError("Installed packages missing expected package: mlnx-ofed-kernel-dkms")
    # First, convert BOM OFED dashes to dots.
    ofed_crd_version = Version(ofed_crd_version_str.replace("-", "."))
    # For the installed version, strip off everything up to ".OFED." embedded in the version.
    installed_version_str = ofed_installed_version.split(".OFED.")[-1]
    installed_version = Version(installed_version_str)
    if installed_version != ofed_crd_version:
        if ofed_crd_version > installed_version:
            raise ValueError(f"Version mismatch for mlnx-ofed-kernel-dkms: CRD has {ofed_crd_version_str}, installed has {installed_version_str}")
        print(f"Installed version for mlnx-ofed-kernel-dkms is fresher than CRD: installed has {installed_version_str}, CRD has {ofed_crd_version_str}")

def main(args):
    parser = argparse.ArgumentParser(description="Check CRD consistency")
    parser.add_argument("--hpc-crd-file", required=True, help="Path to the HPC CRD file")
    parser.add_argument("--vhd-relnotes-file", required=True, help="Path to the VHD release notes file")
    flags = parser.parse_args()
    hpc_crd_file = open(flags.hpc_crd_file)
    vhd_relnotes_file = open(flags.vhd_relnotes_file)

    hpc_crd = json.load(hpc_crd_file)
    installed_packages = extract_installed_packages(vhd_relnotes_file)
    compare(hpc_crd, installed_packages)
    print("CRD check passed!")

if __name__ == "__main__":
    import sys
    main(sys.argv)