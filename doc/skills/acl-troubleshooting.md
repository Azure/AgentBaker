# Skill: Azure Container Linux (ACL) Troubleshooting for AKS Node SIG

Use this document when diagnosing or resolving issues related to Azure Container
Linux (ACL) nodes in AKS clusters. ACL is fundamentally different from Ubuntu
and traditional Azure Linux (Mariner) — many debugging workflows that work on
those distros will **not** work on ACL.

## What is ACL?

Azure Container Linux is a container-host distro composed of Azure Linux 3.0
packages, assembled into a UKI-based (Unified Kernel Image) immutable image
using Flatcar Container Linux build scripts and composition tooling.

Key properties:
- **Immutable `/usr`** — the root filesystem is read-only; no package manager (`dnf`, `rpm`, `tdnf`) is available on the host.
- **Image-based updates** — no in-place package updates; the entire OS image is replaced (currently via AKS node image upgrade; A/B Trident upgrades are in progress).
- **Ignition-based provisioning** — first-boot config uses Ignition (not cloud-init as primary), running in initramfs before switch-root.
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
| OS detection helpers | `parts/linux/cloud-init/artifacts/cse_helpers.sh` — use `isACL()` |
| Sysext merge logic | `parts/linux/cloud-init/artifacts/acl/cse_install_acl.sh` — `mergeSysexts()` |

### OS detection

In shell scripts, use the helper functions:
```bash
isACL()                  # returns true on Azure Container Linux
isMarinerOrAzureLinux()  # returns true on Mariner/AzL (but NOT ACL)
isUbuntu()               # returns true on Ubuntu
```

**Important:** `isMarinerOrAzureLinux()` does **not** match ACL. ACL has its own
`isACL()` check. When adding logic that should apply to ACL, you must explicitly
handle the `isACL()` case.

### ACL stubs

Many package-management functions are stubbed out in `cse_helpers_acl.sh`
because ACL has no package manager. Functions like `apt_get_update()`,
`holdWALinuxAgent()`, etc. are no-ops on ACL. If you need to install software
on ACL, it must be done via sysexts or baked into the VHD image.

## ACL-specific error codes

These exit codes in CSE output indicate ACL-specific failures:

| Code | Constant | Meaning |
|------|----------|---------|
| 231 | `ERR_ORAS_PULL_SYSEXT_FAIL` | Failed to pull a systemd sysext artifact via ORAS from the registry |
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

Unlike Ubuntu/Mariner where kubelet, kubectl, and other binaries are installed
via packages, ACL delivers them as **sysexts pulled from OCI registries via ORAS**:

| Component | Registry path | Notes |
|-----------|--------------|-------|
| kubelet | `mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext` | Tagged by k8s version |
| kubectl | `mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext` | Tagged by k8s version |
| azure-acr-credential-provider | `mcr.microsoft.com/oss/v2/kubernetes/azure-acr-credential-provider-sysext` | Tagged by version |
| aks-secure-tls-bootstrap-client | `mcr.microsoft.com/aks-secure-tls-bootstrap/v2/aks-secure-tls-bootstrap-client-sysext` | Tagged by version |
| GPU drivers (NVIDIA) | `mcr.microsoft.com/azurelinux/<major.minor>/azure-container-linux/<sysext-name>` | Tagged by `VERSION_ID` from `/etc/os-release` |

After sysext merge, symlinks are created (e.g., `/usr/bin/kubelet` → `/opt/bin/kubelet`).

**Network-isolated clusters:** When `BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER`
is set, ACR cache rules don't support ORAS repo tag listing. The code falls back
to a fixed tag format `v{version}-1-azlinux3-{arch}` instead of querying tags.

## Where to find logs

**CSE (Custom Script Extension) logs:**
```bash
/var/log/azure/aks/                          # main AKS log directory
/var/log/azure/cluster-provision.log         # provisioning output
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
Inside the debug pod:
```bash
tdnf install -y strace tcpdump  # install tools in the container
strace -p <pid>                  # host PIDs visible via shared namespace
```

The `--profile=sysadmin` flag provides host PID/network/IPC namespaces and full
capabilities. The host filesystem is mounted at `/host`.

### Sysext issues

**Check active sysexts:**
```bash
systemd-sysext status
systemd-sysext list
```

**Sysext download failures:** Look for `ERR_ORAS_PULL_SYSEXT_FAIL` in CSE logs.
The `mergeSysexts()` function in `cse_install_acl.sh` handles sysext download
and activation. It:
1. Checks for a local match in `/opt/<name>/downloads/`
2. Falls back to querying the remote registry via `oras repo tags`
3. Downloads and symlinks to `/etc/extensions/<name>.raw`

### Provisioning failures (Ignition)

ACL uses **Ignition** for first-boot provisioning (runs in initramfs, before
systemd). See the "Where to find logs" section above for log locations.

Ignition is **strictly one-shot** — it never re-runs. If provisioning failed,
the node must be re-imaged.

**Check order:** Always check Ignition logs first (it runs earlier and handles
the critical path). `coreos-cloudinit` handles supplemental custom-data that
runs after Ignition.

### GPU driver failures on ACL

GPU sysexts on ACL use the `VERSION_ID` from `/etc/os-release` as the tag
(not the driver version). The registry path pattern is:
```
mcr.microsoft.com/azurelinux/<major.minor>/azure-container-linux/<sysext-name>:<VERSION_ID>
```

Example: `mcr.microsoft.com/azurelinux/3.0/azure-container-linux/nvidia-driver-cuda:3.0.20260304`

**Common failure:** `ERR_SYSEXT_VERSION_ID_NOT_FOUND` (exit 232) — indicates
`/etc/os-release` is missing or `VERSION_ID` is empty. This is a critical
image defect.

**Debugging GPU sysext pulls:**
```bash
# Check what VERSION_ID the node has:
grep VERSION_ID /etc/os-release

# Check if the sysext was downloaded:
ls /opt/<sysext-name>/downloads/

# Check if it was linked:
ls -la /etc/extensions/
systemd-sysext status
```

### SELinux denials

ACL ships with SELinux **enforcing**. Check for AVC denials:
```bash
sudo journalctl | grep avc
```

**Temporarily set permissive (does not persist across reboots):**
```bash
sudo setenforce 0   # permissive
sudo setenforce 1   # back to enforcing
getenforce           # check current mode
```

**Persistently change mode** (edit `/etc/selinux/config`):
```ini
SELINUX=permissive   # or enforcing
```
Requires reboot. Note: `/etc` changes may not survive image-based updates.

**Set SELinux mode at AKS nodepool level:**
```bash
# Create nodepool in permissive mode:
az aks nodepool add \
  --resource-group myRG --cluster-name myCluster --name myPool \
  --node-count 1 \
  --tags acl-node-security-profile="selinux=permissive"

# Update existing nodepool:
az aks nodepool update \
  --resource-group myRG --cluster-name myCluster --name myPool \
  --tags acl-node-security-profile="selinux=permissive"
```
Verify: `kubectl debug` → `chroot /host` → `sestatus`.

### Determining package versions on an ACL node

ACL nodes don't have `rpm -qa`. Instead:
1. Get `BUILD_ID` from `/etc/os-release` on the node.
2. Find the corresponding [ACL PROD pipeline](https://dev.azure.com/mariner-org/ACL/_build?definitionScope=%5CACL%5CPROD) run (`Prod_BuildACL`) matching that `BUILD_ID`.
3. In pipeline artifacts, find `drop_build_rpm_image_<arch>_build_azure` → `acl_production_image_packages.txt`.
4. Sysext package lists are also in artifacts (e.g., `nvidia-driver-cuda_packages.txt`).

### Ignition vs cloud-init: key differences

| | Ignition | cloud-init |
|---|---|---|
| Format | JSON (or Butane YAML transpiled) | YAML cloud-config |
| When | initramfs — before switch-root | Post switch-root, as systemd service |
| Re-runs | Never — provision once | Can re-run per-boot modules |
| Failure mode | Atomic — fully provisions or fails clearly | Partial state possible |

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

## References

- [ACL TSG (internal)](https://dev.azure.com/mariner-org/mariner/_wiki/wikis/mariner.wiki/6648/TSG-Azure-Container-Linux)
- [Flatcar Ignition docs](https://www.flatcar.org/docs/latest/provisioning/ignition/) — ACL inherits Flatcar's Ignition implementation
- [Butane config transpiler](https://coreos.github.io/butane/)
- [Azure Container Linux Getting Started](https://aka.ms/azurecontainerlinux)
- [Ignition specification v3.4](https://coreos.github.io/ignition/configuration-v3_4/)
- [ACL PROD pipelines (ADO)](https://dev.azure.com/mariner-org/ACL/_build?definitionScope=%5CACL%5CPROD)
