# AzureLinux ARM64 Dual-Boot Kernel (Grace/GB200 Support)

Implement Ubuntu 24.04-style dual-boot kernel selection for AzureLinux ARM64, enabling a single VHD to boot the correct kernel on both standard ARM64 VMs and NVIDIA Grace (GB200/GB300) VMs.

## Background

Ubuntu 24.04 ARM64 ships two kernels in one VHD:
- `linux-image-azure-lts-24.04` (6.8.x) — standard ARM64
- `linux-azure-nvidia` (6.14.x) — NVIDIA Grace optimized

A GRUB script (`/etc/grub.d/10_azure_nvidia`) uses SMBIOS to detect the CPU manufacturer at boot and selects the appropriate kernel + command-line args.

AzureLinux has `kernel-hwe` (6.12.x LTS, Hardware Enablement) as the newer kernel variant alongside the standard `kernel` (6.6.x LTS). Currently `kernel.spec` declares `Conflicts: kernel-hwe`, preventing co-installation.

### Why kernel-hwe?

| Property | `kernel` | `kernel-hwe` |
|---|---|---|
| Linux version | 6.6.138.x LTS | 6.12.87.x LTS |
| Source branch | `rolling-lts/mariner-3/` | `rolling-lts/hwe/` |
| Architecture | x64 + arm64 | x64 + arm64 |
| uname_r | `6.6.138.1-1.azl3` | `6.12.87.1-1.azl3` |
| Tool subpackages | `bpftool`, `python3-perf` | `bpftool-hwe`, `python3-perf-hwe` |

The newer 6.12 kernel has better hardware enablement for Grace CPUs. The different major versions mean `uname_r` values are **naturally distinct** — no file path collisions on `/boot/vmlinuz-*` or `/lib/modules/*/`.

## Reference Implementation (Ubuntu 24.04 ARM64)

Files in AgentBaker that implement the Ubuntu dual-boot:
- `parts/linux/cloud-init/artifacts/10_azure_nvidia` — GRUB boot script with SMBIOS detection
- `parts/linux/cloud-init/artifacts/51-azure-nvidia.cfg` — GRUB env config
- `vhdbuilder/packer/pre-install-dependencies.sh` (lines 290-310) — dual kernel install
- `vhdbuilder/packer/packer_source.sh` (lines 465-472) — GRUB file placement
- `vhdbuilder/packer/vhd-image-builder-arm64-gen2.json` (lines 732-738) — packer file upload

## TODO

### 1. AzureLinux Repo: Remove kernel↔kernel-hwe Conflicts (upstream PR)

**Repo:** `microsoft/azurelinux` branch `3.0`

#### The blocker

`SPECS/kernel/kernel.spec` declares:
```spec
Conflicts:      kernel-hwe
```
This single line prevents co-installation. RPM `Conflicts` is bidirectional — with this in place, `dnf install kernel-hwe` fails when `kernel` is present, and vice versa.

#### Why it was added

The `Conflicts` was added June 10, 2025 (Harshit Gupta, `6.6.92.2-3`) as a blanket fix for a subpackage collision bug dating back to Mariner 2.0: `bpftool` and `python3-perf` binaries installed to the same file paths from different kernel packages. Rather than fix the file paths, `Conflicts` was added between all kernel variants.

#### Why co-installation is safe for kernel + kernel-hwe

Unlike `kernel-64k` (which shares the same 6.6.x version and would collide on `uname_r` paths), `kernel-hwe` has a **completely different version** (6.12.x). The main packages install to non-overlapping paths:

| File | kernel | kernel-hwe |
|------|--------|------------|
| vmlinuz | `/boot/vmlinuz-6.6.138.1-1.azl3` | `/boot/vmlinuz-6.12.87.1-1.azl3` |
| modules | `/lib/modules/6.6.138.1-1.azl3/` | `/lib/modules/6.12.87.1-1.azl3/` |
| config | `/boot/config-6.6.138.1-1.azl3` | `/boot/config-6.12.87.1-1.azl3` |
| headers | `/usr/src/linux-headers-6.6.138.1-1.azl3/` | `/usr/src/linux-headers-6.12.87.1-1.azl3/` |

The **tools subpackages** (`kernel-tools` vs `kernel-hwe-tools`, `bpftool` vs `bpftool-hwe`) do install to the same binary paths (`/usr/sbin/bpftool`, perf binaries, etc.), but these are separate subpackages. For the AKS VHD we only need one set of tools — install `kernel-tools`/`bpftool`/`python3-perf` from the standard kernel and skip the `-hwe` tools entirely.

Additionally, `kernel-hwe` does **NOT** declare `Provides: python3-perf` or `Provides: bpftool` on its subpackages (they're named `python3-perf-hwe` and `bpftool-hwe` with no generic Provides), so there's no virtual provides collision.

#### Fix

**File:** `SPECS/kernel/kernel.spec`

Remove this line:
```spec
Conflicts:      kernel-hwe
```

**Acceptance criteria:**
- `dnf install kernel kernel-hwe` succeeds on AzureLinux 3.0 ARM64 (without installing `-hwe` tools)
- Both `/boot/vmlinuz-6.6.*` and `/boot/vmlinuz-6.12.*` exist
- `/lib/modules/` contains two separate module directories
- `grub2-mkconfig` generates menu entries for both kernels

---

### 2. Verify GRUB2 smbios Module on AzureLinux ARM64

**On an AzureLinux 3.0 ARM64 VM**, verify the smbios GRUB module exists:

```bash
# Check module directory
ls /usr/lib/grub/arm64-efi/smbios.mod
# or
ls /boot/grub2/arm64-efi/smbios.mod

# Test building with smbios
grub2-mkimage --directory /usr/lib/grub/arm64-efi -O arm64-efi -o /tmp/test.efi smbios
```

The AzureLinux `grub2.spec` builds `grubaa64.efi` with a fixed module list that does NOT include `smbios`. However, `smbios` is a standard GRUB 2.06 module and should be built as `smbios.mod` in the module directory. The `insmod smbios` command in a `/etc/grub.d/` script will load it dynamically.

If `smbios.mod` is **not** present, the `grub2.spec` needs a one-line fix to include it in the module install.

**Also verify SMBIOS Type 4 data is accessible on ARM64 Azure VMs:**
```bash
dmidecode -t 4 | grep Manufacturer
```

On a Grace VM this should return `NVIDIA`. On a standard ARM64 VM (Ampere Altra) it should return `Ampere(R)` or similar.

---

### 3. Create GRUB Boot Script for AzureLinux

Create the AzureLinux equivalent of `10_azure_nvidia`. Key differences from the Ubuntu version:

| Aspect | Ubuntu | AzureLinux |
|--------|--------|------------|
| GRUB config path | `/boot/grub/grub.cfg` | `/boot/grub2/grub.cfg` |
| Regen command | `update-grub` | `grub2-mkconfig -o /boot/grub2/grub.cfg` |
| Kernel naming | `vmlinuz-*-azure-nvidia` vs `vmlinuz-*-azure` | `vmlinuz-6.12.*` (hwe) vs `vmlinuz-6.6.*` (standard) — distinguished by major version |
| Menu entry format | `gnulinux-${version}-advanced-${boot_device_id}` | Depends on AzureLinux GRUB2 `10_linux` output — needs investigation |

**New files to create:**
- `parts/linux/cloud-init/artifacts/10_azure_nvidia_azurelinux` — AzureLinux GRUB script
- `parts/linux/cloud-init/artifacts/51-azure-nvidia-azurelinux.cfg` — AzureLinux GRUB env

The script must:
1. Source `/etc/grub.d/10_linux` to get the kernel list (same as Ubuntu version)
2. Identify the `kernel-hwe` kernel (6.12.x) vs standard `kernel` (6.6.x) by version string in `uname_r`
3. `insmod smbios` and read SMBIOS Type 4, string 7
4. If `cpu_manufacturer == NVIDIA`: set default to hwe kernel, set `nvidia_args="iommu.passthrough=1 irqchip.gicv3_nolpi=y arm_smmu_v3.disable_msipolling=1"`
5. Otherwise: set default to standard kernel

**Key risk:** The GRUB menu entry naming format differs between Ubuntu and AzureLinux. The Ubuntu script uses `gnulinux-${version}-advanced-${boot_device_id}` — verify what AzureLinux's `10_linux` produces by running `grub2-mkconfig` on an AzureLinux ARM64 VM and inspecting the output.

---

### 4. Modify AgentBaker VHD Build for AzureLinux ARM64

#### 4a. Add dual kernel install to pre-install-dependencies.sh

After the existing AzureLinux lockdown removal (`disableKernelLockdownCmdline`), add logic to install `kernel-hwe` alongside the standard kernel:

```bash
# In pre-install-dependencies.sh, AzureLinux ARM64 section:
if isMarinerOrAzureLinux "$OS" && [[ "${CPU_ARCH}" == "arm64" ]]; then
    if dnf list available kernel-hwe &>/dev/null; then
        echo "ARM64 AzureLinux: installing kernel-hwe alongside standard kernel"
        dnf_install 30 1 600 kernel-hwe
        echo "After dual kernel install:"
        rpm -qa | grep kernel | sort
    else
        echo "kernel-hwe not available, skipping dual kernel install"
    fi
fi
```

Note: only install `kernel-hwe` main package — do NOT install `kernel-hwe-tools`, `bpftool-hwe`, or `python3-perf-hwe` to avoid file collisions with the standard kernel's tools.

#### 4b. Add GRUB script file uploads to mariner ARM64 packer template

**File:** `vhdbuilder/packer/vhd-image-builder-mariner-arm64.json`

Add file provisioners for the new GRUB scripts:
```json
{
    "type": "file",
    "source": "parts/linux/cloud-init/artifacts/10_azure_nvidia_azurelinux",
    "destination": "/home/packer/10_azure_nvidia_azurelinux"
},
{
    "type": "file",
    "source": "parts/linux/cloud-init/artifacts/51-azure-nvidia-azurelinux.cfg",
    "destination": "/home/packer/51-azure-nvidia-azurelinux.cfg"
}
```

#### 4c. Install GRUB scripts in packer_source.sh

Add AzureLinux ARM64 handling in `copyPackerFiles()`:

```bash
if isMarinerOrAzureLinux "$OS" && [ "$CPU_ARCH" = "arm64" ]; then
    GRUB_AZ_NV_SCRIPT_SRC=/home/packer/10_azure_nvidia_azurelinux
    GRUB_AZ_NV_SCRIPT_DEST=/etc/grub.d/10_azure_nvidia
    cpAndMode $GRUB_AZ_NV_SCRIPT_SRC $GRUB_AZ_NV_SCRIPT_DEST 755

    GRUB_AZ_NV_ENV_SRC=/home/packer/51-azure-nvidia-azurelinux.cfg
    GRUB_AZ_NV_ENV_DEST=/etc/default/grub.d/51-azure-nvidia.cfg
    cpAndMode $GRUB_AZ_NV_ENV_SRC $GRUB_AZ_NV_ENV_DEST 644
fi
```

#### 4d. Regenerate GRUB config

After GRUB script placement, run:
```bash
grub2-mkconfig -o /boot/grub2/grub.cfg
```

This is already called by `disableKernelLockdownCmdline()` and the `%grub2_post` RPM macro on kernel install, but verify the ordering ensures the GRUB scripts are in place before regeneration.

---

### 5. Update mariner-aks-pipelines

#### 5a. No new pipeline job needed

The dual-boot approach means the existing `build_vhd_azurelinux_arm64` job produces a VHD that works on both standard and Grace ARM64 VMs. The separate `build_vhd_azurelinux_arm64_kernel_hwe` job (which builds a VHD with ONLY kernel-hwe) can remain as-is for non-Grace HWE use cases.

#### 5b. Add Grace-specific test stage

Add a new test stage in `.pipelines/templates/tests/pre-release-tests/main.yaml`:

```yaml
- stage: run_azurelinux_arm64_grace_tests
  displayName: "AzureLinux ARM64 Grace Tests"
  condition: and(succeeded(), eq(variables['run_AzureLinux_ARM64'], 'true'))
  jobs:
    - template: ../common/jobs/e2e-test.yaml
      parameters:
        AZURE_VM_SIZE: Standard_D16ps_v5  # Or Grace-specific SKU when available
        MARINER_VERSION_STRING: "azurelinux-arm64"
```

---

### 6. Runtime CSE Considerations

No CSE changes needed for the kernel selection — GRUB handles it at boot time before the OS starts.

However, verify these existing behaviors are compatible:
- `ensureGPUDrivers()` already returns early on ARM64 — this is correct for Grace
- `uname -r` will return the hwe kernel's version (`6.12.x`) on Grace — any CSE logic that parses `uname -r` should be tested
- NVIDIA CUDA driver matching uses `uname -r` for kernel module path — verify compatibility with 6.12 kernel
- The `disableKernelLockdownCmdline()` function in `tool_installs_mariner.sh` modifies `/etc/default/grub` — ensure it doesn't conflict with `51-azure-nvidia.cfg`

---

### 7. Validation Checklist

- [ ] `dnf install kernel kernel-hwe` succeeds on AzureLinux 3.0 ARM64
- [ ] `smbios.mod` exists in GRUB2 module directory on AzureLinux ARM64
- [ ] `insmod smbios` works in GRUB2 on AzureLinux ARM64
- [ ] SMBIOS Type 4 returns correct manufacturer on Grace vs non-Grace
- [ ] `grub2-mkconfig` generates entries for both kernels (6.6.x and 6.12.x)
- [ ] GRUB boot script selects correct kernel based on CPU
- [ ] Standard ARM64 VM boots standard kernel (6.6.x)
- [ ] Grace ARM64 VM boots kernel-hwe (6.12.x) with `iommu.passthrough=1 irqchip.gicv3_nolpi=y arm_smmu_v3.disable_msipolling=1`
- [ ] CSE completes successfully on both kernel variants
- [ ] `nvidia-smi` works on Grace VMs (if GPU drivers are applicable)
- [ ] VHD size increase from dual kernel is acceptable (expect ~200-400MB)

---

### Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| `smbios.mod` not shipped in AzureLinux GRUB2 | Blocks entire approach | PR to add smbios to grub2.spec install; or embed it in grubaa64.efi via `grub2-mkimage` |
| AzureLinux kernel team rejects Conflicts removal | Blocks co-installation | The fix is a single-line removal in kernel.spec; main packages have no file collisions; tools subpackages are not co-installed; fallback is Approach B (separate VHDs) |
| GRUB menu entry format differs from Ubuntu | Script doesn't select correct kernel | Investigate on real VM; adapt script to AzureLinux menu format |
| VHD size increase unacceptable | May not fit in disk budget | kernel-hwe modules are ~70-100MB; total VHD increase ~200MB |
| NVIDIA drivers not available for ARM64 AzureLinux | No GPU functionality on Grace | Currently a known gap — no ARM64 nvidia repo exists for any AzureLinux version |

---

### Dependency Order

```
[1] Verify smbios.mod on AzureLinux ARM64 VM
 │
 ├──→ If missing: PR to microsoft/azurelinux grub2.spec
 │
[2] PR to microsoft/azurelinux: remove Conflicts: kernel-hwe from kernel.spec
 │
[3] Create GRUB boot scripts (10_azure_nvidia_azurelinux, 51-azure-nvidia-azurelinux.cfg)
 │   └── Requires [1] confirmed
 │
[4] AgentBaker changes (packer template, pre-install-deps, packer_source.sh)
 │   └── Requires [2] merged and [3] ready
 │
[5] mariner-aks-pipelines test stages
 │   └── Requires [4] merged
 │
[6] End-to-end validation on Grace hardware
     └── Requires [5] and Grace VM access
```
