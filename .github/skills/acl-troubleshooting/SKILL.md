---
name: acl-troubleshooting
description: >
  Use when diagnosing or resolving issues related to Azure Container Linux (ACL)
  nodes in AKS clusters. Triggers: "ACL node issue", "sysext failure",
  "ERR_ORAS_PULL_SYSEXT_FAIL", "immutable host", "no package manager on node",
  "Ignition provisioning failure", "SELinux denial on ACL", "GPU driver ACL",
  "coreos-cloudinit", "systemd-sysext", "ACL debug pod", "ACL package versions".
---

# Azure Container Linux (ACL) Troubleshooting for AKS Node SIG

ACL is fundamentally different from Ubuntu and traditional Azure Linux
(Mariner) — many debugging workflows that work on those distros will **not**
work on ACL.

## What is ACL?

Azure Container Linux is a container-host distro composed of Azure Linux 3.0
packages, assembled into a UKI-based (Unified Kernel Image) immutable image
using Flatcar Container Linux build scripts and composition tooling.

Key properties:
- **Immutable `/usr`** — `/usr` is mounted read-only and changes only via image updates or sysexts; `/etc`, `/opt`, and `/var` remain writable. No package manager (`dnf`, `rpm`, `tdnf`) is available on the host.
- **Image-based updates** — no in-place package updates; the entire OS image is replaced (currently via AKS node image upgrade; A/B Trident upgrades are in progress).
- **UKI boot** — ACL boots from a Unified Kernel Image. systemd-boot loads UKI addons from `/boot/EFI/Linux/<uki>.extra.d/`; a `firstboot.addon.efi` sets `flatcar.first_boot=detected` on the kernel command line, which triggers Ignition. The VHD build consumes this addon during packer provisioning and `cleanup-vhd.sh` restores it so every VM created from the VHD sees a genuine first boot.
- **Ignition-based provisioning** — first-boot config runs as systemd units in the initramfs, before switch-root (not cloud-init as primary).
- **SELinux enforcing by default**.
- **Sysexts for extensibility** — additional packages are layered via systemd system extensions rather than package installs.

## ACL in AgentBaker

### Relevant paths

| Purpose | Path |
|---------|------|
| ACL-specific CSE helpers | `parts/linux/cloud-init/artifacts/acl/cse_helpers_acl.sh` |
| ACL-specific install logic | `parts/linux/cloud-init/artifacts/acl/cse_install_acl.sh` |
| ACL cloud-init/Ignition config | `parts/linux/cloud-init/acl.yml` |
| ACL VHD packer customdata (Butane) | `vhdbuilder/packer/acl-customdata.yaml` |
| ACL VHD packer customdata (JSON) | `vhdbuilder/packer/acl-customdata.json` |
| OS detection helpers | `parts/linux/cloud-init/artifacts/cse_helpers.sh` — `isACL()` |
| Sysext merge logic | `parts/linux/cloud-init/artifacts/acl/cse_install_acl.sh` — `mergeSysexts()` |
| UKI firstboot addon restore | `vhdbuilder/packer/cleanup-vhd.sh` |

### OS detection

In shell scripts, use the helper functions:
```bash
isACL()                  # true on Azure Container Linux
isMarinerOrAzureLinux()  # true on Mariner/AzL — NOT on ACL
isUbuntu()               # true on Ubuntu
```

**Important:** `isMarinerOrAzureLinux()` explicitly excludes ACL. When adding
logic that should apply to ACL, you must handle the `isACL()` case separately.

### ACL stubs

Many package-management functions are stubbed out in `cse_helpers_acl.sh`
because ACL has no package manager. Functions like `apt_get_update()`,
`holdWALinuxAgent()`, etc. are no-ops on ACL. If you need to install software
on ACL, it must be done via sysexts or baked into the VHD image.

## Sysext-related error codes on ACL

These CSE exit codes are relevant when troubleshooting ACL; code 231 is also used by Flatcar:

| Code | Constant | Meaning |
|------|----------|---------|
| 231 | `ERR_ORAS_PULL_SYSEXT_FAIL` | Failed to pull a systemd sysext via ORAS from the registry |
| 232 | `ERR_SYSEXT_VERSION_ID_NOT_FOUND` | `VERSION_ID` not found in `/etc/os-release`; required for sysext tag resolution |

Related ORAS errors (not ACL-specific but frequently seen on ACL):

| Code | Constant | Meaning |
|------|----------|---------|
| 45 | `ERR_ORAS_DOWNLOAD_ERROR` | Unable to install/run ORAS |
| 207 | `ERR_ORAS_PULL_K8S_FAIL` | Failed to pull kube-node artifact via ORAS |
| 210 | `ERR_ORAS_IMDS_TIMEOUT` | Timeout waiting for IMDS response during ORAS auth |
| 211 | `ERR_ORAS_PULL_NETWORK_TIMEOUT` | Timeout pulling ORAS tokens for login |
| 212 | `ERR_ORAS_PULL_UNAUTHORIZED` | Authorization failure pulling artifact via ORAS |

## How components are delivered on ACL

Unlike Ubuntu/Mariner where binaries are installed via packages, ACL delivers
them as **sysexts pulled from OCI registries via ORAS**:

| Component | Registry path | Installed when | Notes |
|-----------|--------------|----------------|-------|
| kubelet | `mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext` | Node CSE | Tagged by k8s version |
| kubectl | `mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext` | Node CSE | Tagged by k8s version |
| azure-acr-credential-provider | `mcr.microsoft.com/oss/v2/kubernetes/azure-acr-credential-provider-sysext` | Node CSE | Tagged by version |
| aks-secure-tls-bootstrap-client | `mcr.microsoft.com/aks-secure-tls-bootstrap/v2/aks-secure-tls-bootstrap-client-sysext` | **VHD build only** | Pull failures surface in the VHD build pipeline, not in node provisioning logs |
| GPU drivers (NVIDIA) | `mcr.microsoft.com/azurelinux/<major.minor>/azure-container-linux/<sysext-name>` | Node CSE | Tagged by `VERSION_ID` from `/etc/os-release` |

After sysext merge, compatibility symlinks are created (e.g., `/opt/bin/kubelet` →
`/usr/bin/kubelet`) and `systemd-sysext --no-reload refresh` activates them.

**Network-isolated clusters:** When `BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER`
is set, ACR cache rules don't support ORAS repo tag listing. The code falls back
to a fixed tag format `v{version}-1-azlinux3-{arch}` instead of querying tags.

### GPU sysext selection, pull, and activation

GPU sysexts are selected based on the VM SKU and `NVIDIA_GPU_DRIVER_TYPE`
(set by AgentBaker from GPU SKU maps in `gpu_components.go`):

1. **Driver type resolution:**
   - `grid` (converged sizes like NVads_A10_v5) → `nvidia-driver-vgpu` sysext
   - `grid-v20` → **not supported on ACL** (Ubuntu-only); fails with `ERR_NVIDIA_DRIVER_INSTALL`
   - Default: `nvidia-driver-cuda` (proprietary) for legacy GPUs (T4, V100), or `nvidia-driver-cuda-open` (OpenRM) for A100+

2. **Version resolution:** `getACLVersionID()` reads `VERSION_ID` from `/etc/os-release` (e.g., `3.0.20260304`). GPU sysexts are tagged by the OS image version, not the driver version.

3. **Registry path:** `mcr.microsoft.com/azurelinux/<major.minor>/azure-container-linux/<sysext-name>:<VERSION_ID>`

4. **Pull and activation:** `installACLGPUSysext()` → `mergeSysexts()` → ORAS pull → symlink to `/etc/extensions/<name>.raw` → `systemd-sysext refresh`.

## Where to find logs

**CSE (Custom Script Extension) logs:**
```bash
/var/log/azure/aks/                              # main AKS log directory
/var/log/azure/cluster-provision.log             # provisioning output
/var/log/azure/cluster-provision-cse-output.log  # symlink to above
```

**Ignition logs** (first-boot only — check these first for provisioning failures):
```bash
sudo journalctl -u ignition-disks.service    # disk/partition provisioning
sudo journalctl -u ignition-files.service    # file writes, user creation
sudo journalctl -t ignition                  # all Ignition messages
```

**cloud-init (coreos-cloudinit) logs** — for AKS custom-data injection:
```bash
sudo journalctl -u oem-cloudinit.service
sudo journalctl -u coreos-cloudinit.service  # alternative unit name
```

> ACL uses Flatcar's `coreos-cloudinit`, not upstream `cloud-init`.
> There is no `/var/log/cloud-init.log`. All output goes through the journal.

**kubelet logs:**
```bash
sudo journalctl -u kubelet.service
```

## Troubleshooting common issues

### "Command not found" or "Package manager not available"

**Root cause:** ACL has no `dnf`, `rpm`, `tdnf`, or `apt`. The host is immutable.

**Resolution:** Run debug tools from a privileged container:
```bash
kubectl debug node/<node-name> -it \
  --image=mcr.microsoft.com/azurelinux/base/core:3.0 \
  --profile=sysadmin
```
Inside the debug pod (host PID/network/IPC namespaces and full capabilities;
host filesystem at `/host`):
```bash
tdnf install -y strace tcpdump
strace -p <pid>   # host PIDs visible via shared namespace
```
Delete the debug pod when done.

### Sysext issues

**Check active sysexts:**
```bash
systemd-sysext status
systemd-sysext list
```

**Sysext download failures:** Look for `ERR_ORAS_PULL_SYSEXT_FAIL` (exit 231)
in CSE logs. The `mergeSysexts()` function handles the flow:
1. `matchLocalSysext()` — checks `/opt/<name>/downloads/` for a local `.raw` file
2. `matchRemoteSysext()` — queries `oras repo tags` to find the best matching tag
3. `downloadSysextFromVersion()` — `oras pull` from the registry
4. Symlinks result to `/etc/extensions/<name>.raw`
5. `systemd-sysext --no-reload refresh` activates all extensions

### Provisioning failures (Ignition)

ACL uses **Ignition** for first-boot provisioning, running as systemd units
in the initramfs before switch-root. See the "Where to find logs" section
for log locations.

Ignition runs staged (`ignition-disks` → `ignition-files`). It is **not
transactional** — a mid-stage failure drops the node to `emergency.target`,
but writes from earlier stages are already applied. When triaging a failed
node, don't assume all-or-nothing; check for partially applied state.

Ignition is **strictly one-shot** — it never re-runs.

**When a node fails to provision:**
1. **Collect logs first** — the Ignition journal and `/var/log/azure/` live on
   the node and will be destroyed if you re-image without capturing them:
   ```bash
   # From a serial console:
   journalctl -t ignition > /tmp/ignition.log
   cp -r /var/log/azure/ /tmp/azure-logs/
   # From a debug pod (host filesystem is at /host):
   chroot /host sh -c 'journalctl -t ignition > /tmp/ignition.log; cp -r /var/log/azure/ /tmp/azure-logs/'
   ```
2. **Re-image safely** — in AKS, use `az aks nodepool upgrade --node-image-only`
   or use `az aks nodepool delete-machines` for a specific VMSS instance;
   `kubectl delete node` only removes the Kubernetes API object and does not
   reimage the VM. **Do not** use `az vmss reimage` against the `MC_*` resource
   group — it is unsupported on AKS-managed scale sets and can leave the pool
   unreconcilable.

**Check order:** Always check Ignition logs first. `coreos-cloudinit` handles
supplemental custom-data that runs after Ignition.

### GPU driver failures on ACL

GPU sysexts use `VERSION_ID` from `/etc/os-release` as the tag (not the driver
version). See "GPU sysext selection, pull, and activation" above for the full
flow.

**Common failure:** `ERR_SYSEXT_VERSION_ID_NOT_FOUND` (exit 232) — `/etc/os-release`
is missing or `VERSION_ID` is empty. This is a critical image defect.

**Debugging GPU sysext pulls:**
```bash
grep VERSION_ID /etc/os-release        # check VERSION_ID
ls /opt/<sysext-name>/downloads/       # check downloaded artifacts
ls -la /etc/extensions/                # check symlinks
systemd-sysext status                  # check activation
```

### SELinux denials

ACL ships with SELinux **enforcing**. Check for AVC denials:
```bash
sudo journalctl -t audit -g avc --since "1 hour ago"
```

**Diagnostic — temporarily set permissive to confirm SELinux is the cause:**
```bash
sudo setenforce 0   # permissive — for diagnosis only
# Reproduce the issue. If it succeeds, SELinux policy needs fixing.
sudo setenforce 1   # restore enforcing immediately
getenforce           # verify current mode
```

**Persistently change mode** (edit `/etc/selinux/config`):
```ini
SELINUX=permissive   # or enforcing
```
Requires reboot. This local `/etc` change is lost when AKS reimages or
replaces the node; use the node-pool tag below for durable configuration.

**Set SELinux mode at AKS nodepool level** — this is the recommended approach
as it is durable across image updates and node reimages:

```bash
# Create a new nodepool in permissive mode (recommended path):
az aks nodepool add \
  --resource-group myRG --cluster-name myCluster --name myPool \
  --node-count 1 \
  --tags acl-node-security-profile="selinux=permissive"
```

> **Warning for `nodepool update`:** `--tags` replaces the **entire** tag set,
> it is not additive. Read existing tags first with
> `az aks nodepool show ... --query tags` and re-supply all of them:
> ```bash
> az aks nodepool update \
>   --resource-group myRG --cluster-name myCluster --name myPool \
>   --tags existingKey=existingValue acl-node-security-profile="selinux=permissive"
> ```

The tag is consumed at node provisioning time. Existing nodes are **not**
affected until they are re-provisioned. To apply to an existing pool:
```bash
az aks nodepool upgrade \
  --resource-group myRG --cluster-name myCluster --name myPool \
  --node-image-only
```
Then verify on a **newly provisioned** node: `kubectl debug` → `chroot /host` → `sestatus`.

### Determining package versions on an ACL node

ACL nodes don't have `rpm -qa`. Instead:
1. Get `BUILD_ID` from `/etc/os-release` on the node.
2. Find the corresponding ACL PROD pipeline run (`Prod_BuildACL`) matching that `BUILD_ID` (search the ACL PROD pipelines in the `mariner-org` ADO organization).
3. In pipeline artifacts, find `drop_build_rpm_image_<arch>_build_azure` → `acl_production_image_packages.txt`.
4. Sysext package lists are also in artifacts (e.g., `nvidia-driver-cuda_packages.txt`).

### Ignition vs cloud-init: key differences

| | Ignition | cloud-init (coreos-cloudinit) |
|---|---|---|
| Format | JSON (or Butane YAML transpiled) | YAML cloud-config (Flatcar subset) |
| When | initramfs, as systemd units before switch-root | Post switch-root, as systemd service |
| Re-runs | Never — provision once | Can re-run per-boot modules |
| Failure mode | Fails fast into `emergency.target`; writes from earlier stages are already applied (not transactional) | Partial state possible |

### Writing binaries to the host

`/usr` is read-only. Options:
- Write to `/opt` (writable) — binaries can execute from there today.
- Use a sysext to layer into `/usr` at boot.
- **Future:** Once IPE (Integrity Policy Enforcement) is in enforcing mode, unsigned binaries will not execute regardless of location.

## ACL-specific considerations for AgentBaker changes

1. **No package installs in CSE:** Any `tdnf install` / `apt-get install` in provisioning scripts must be gated behind `! isACL()` or use a sysext alternative.
2. **Sysext-based delivery:** New components on ACL must be delivered as sysexts downloaded via ORAS and merged with `mergeSysexts()`.
3. **Butane/Ignition config:** VHD-level changes go in `vhdbuilder/packer/acl-customdata.yaml` (Butane format). Run transpilation to update the JSON.
4. **No update-ca-certificates.service:** ACL uses `update-ca-trust` (Azure Linux style), not Flatcar's `update-ca-certificates`.
5. **No `/usr/share/baselayout/`:** This Flatcar/Gentoo path does not exist on ACL.
6. **Certificate updates:** ACL has a dedicated `update_certs.service` in `parts/linux/cloud-init/artifacts/acl/`.
7. **UKI firstboot addon:** The `firstboot.addon.efi` must be restored in `cleanup-vhd.sh` after packer builds, otherwise VMs won't get Ignition triggered on first boot.

## References

- ACL TSG — internal wiki in the `mariner-org` ADO organization under `mariner.wiki` (search for "TSG Azure Container Linux")
- [Flatcar Ignition docs](https://www.flatcar.org/docs/latest/provisioning/ignition/) — ACL inherits Flatcar's Ignition implementation
- [Butane config transpiler](https://coreos.github.io/butane/)
- [Azure Container Linux Getting Started](https://aka.ms/azurecontainerlinux)
- [Ignition specification v3.4](https://coreos.github.io/ignition/configuration-v3_4/)
- ACL PROD pipelines — search the `mariner-org/ACL` ADO project for `Prod_BuildACL` pipeline definitions
