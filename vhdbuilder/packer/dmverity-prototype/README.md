# dm-verity prototype (Phase-1 scaffolding — temporary)

This directory carries the AgentBaker side of the dm-verity prototype for
[notaryproject/notation #1337][issue]. It hot-installs two pre-release
RPMs into the AzureLinux 3 VHD during the Packer build so the patched
containerd handles every image pull from that point on.

**This whole directory is temporary.** Once the patched RPMs ship in the
AzureLinux base repository, the only change needed in AgentBaker is a
one-line bump of the `containerd2` pin in
[`parts/common/components.json`](../../../parts/common/components.json)
(`azurelinux.v3.0.versionsV2[0].latestVersion`). At that point the
entire `dmverity-prototype/` directory, the hook in
[`install-dependencies.sh`](../install-dependencies.sh) at the
`extractAndCacheCoreDnsBinary` site, and the upload step in
[`vhd-image-builder-mariner.json`](../vhd-image-builder-mariner.json)
should all be deleted.

## What it does

`install.sh` runs once during the Packer build, right after
`installStandaloneContainerd`. It is a hard no-op when the URLs in
`dmverity-prototype.env` are empty — a build without the prototype
enabled is byte-identical to a stock VHD build.

When enabled, it:

1. **(Optional)** `tdnf install`s the patched kernel RPM
   (`kernel-6.6.139.1-99.osguard1.azl3`) by file path. The osguard
   kernel bakes the OS Guard signer CA into
   `CONFIG_SYSTEM_TRUSTED_KEYS`, so on first boot the CA is already in
   `.builtin_trusted_keys` and dm-verity verifies layer signatures with
   no userspace enrollment. Normally the kernel RPM is delivered
   through the buddy-build channel (`proprietaryRpmBuddyBuildId` on the
   `buildAzureLinuxV3gen2` pipeline) and `DMVERITY_KERNEL_RPM_URL`
   stays empty here — re-installing via `tdnf` from kataccstorage was
   observed to fail mid-build when the blob container's public-access
   flag flipped off.
2. `tdnf install`s the patched containerd2 RPM
   (`containerd2-2.2.0-4000.cb15e731a.azl3` or newer) by file path,
   replacing the stock version that `installStandaloneContainerd`
   pulled per `components.json`. The RPM ships its own
   `/etc/containerd/config.toml` (erofs snapshotter + dm-verity),
   `/etc/modules-load.d/aks-dmverity.conf`, and a systemd drop-in at
   `/etc/systemd/system/containerd.service.d/dmverity-overlay.conf`
   that re-overlays the config on every containerd start (defeats AKS
   CSE's bootstrap clobber). It pulls `erofs-utils` + `veritysetup` from
   the AzureLinux base repo as transitive deps.
3. Drops the OS Guard signer CA at
   `/etc/aks/dmverity/osguard-signer-ca.pem`. With the osguard kernel
   this PEM is redundant for dm-verity verification (the kernel keyring
   already has the CA); it is still useful for userspace verifiers
   (`notation verify`) and is the input file the keyring-loader script
   reads.
4. Installs [`dmverity-keyring-load.sh`](dmverity-keyring-load.sh) and
   enables [`aks-dmverity-keyring.service`](dmverity-keyring.service)
   (ordered before `containerd.service`). On the osguard kernel the
   loader script short-circuits to a single `keyctl list | grep` and
   exits — a confirmed no-op. It exists only as defense in depth for a
   VHD that accidentally boots a non-osguard kernel; in that case it
   enrolls the CA into `.machine`.
5. Restarts containerd so the rest of `install-dependencies.sh` runs
   against the patched binary + erofs snapshotter.

## Why it has to run late in `install-dependencies.sh`

The hook fires *after* `extractAndCacheCoreDnsBinary`, not alongside
`installStandaloneContainerd`. The patched containerd routes
`ctr -n k8s.io images mount` through a new mount-manager subsystem that
returns `err: no such device` for regular (non-dm-verity-signed) images;
`extractAndCacheCoreDnsBinary` relies on that command and must run on
the stock binary. The image pre-pull loop uses the content API and is
unaffected.

## What lands on the VHD

| Source (this dir) | Destination on VHD |
|---|---|
| `install.sh` | (build VM only; not installed on the VHD) |
| `dmverity-prototype.env` | (build VM only; not installed on the VHD) |
| `dmverity-keyring-load.sh` | `/opt/azure/containers/dmverity-keyring-load.sh` |
| `dmverity-keyring.service` | `/etc/systemd/system/aks-dmverity-keyring.service` (enabled) |
| Downloaded osguard kernel RPM | `/boot/vmlinuz-6.6.139.1-99.osguard1.azl3` etc. |
| Downloaded patched `containerd2` RPM | `/usr/bin/containerd`, `/etc/containerd/config.toml`, `/etc/modules-load.d/aks-dmverity.conf`, `/etc/systemd/system/containerd.service.d/dmverity-overlay.conf` |
| Downloaded OS Guard CA (PEM) | `/etc/aks/dmverity/osguard-signer-ca.pem` |

## Storage toggle (kataccstorage)

The RPMs and CA cert live in the `aks-rpms` container of the
`kataccstorage` storage account, which has account-level
`allow-blob-public-access` set to `false` by default. Enable for the
build, then disable:

```bash
# Before queuing the AgentBaker Packer build
az storage account update \
    --name kataccstorage \
    --resource-group dadelanaksmshvtest \
    --allow-blob-public-access true

# ... run the AgentBaker Packer build ...

# After the build (always, even on failure)
az storage account update \
    --name kataccstorage \
    --resource-group dadelanaksmshvtest \
    --allow-blob-public-access false
```

The container itself is configured with `publicAccess=blob`, so the
account-level toggle is the only switch.

## Validating on a provisioned node

Once a VHD built with the prototype enabled is wired into an AKS cluster
(see the dm-verity prototype design doc for cluster creation flags), the
node-side smoke check is:

```bash
# Patched kernel with baked-in CA
uname -r                                                # 6.6.139.1-99.osguard1.azl3
sudo keyctl list %:.builtin_trusted_keys | grep -i "OS Guard CA"

# Patched containerd + RPM-shipped config + auto-loaded modules
rpm -q containerd2                                      # containerd2-2.2.0-4000.cb15e731a.azl3
grep -q 'dmverity_mode = "auto"' /etc/containerd/config.toml
lsmod | grep -E '^(erofs|dm_verity)\b'
systemctl is-active containerd
systemctl is-active aks-dmverity-keyring                # active (exited; no-op on osguard kernel)

# Active dm-verity devices and per-layer kernel verifications
sudo dmsetup ls --target verity
sudo journalctl -k | grep -c 'dm-verity sha256-ni'
```

## Disabling the prototype

Set `DMVERITY_RPM_URL=` empty in
[`dmverity-prototype.env`](dmverity-prototype.env). `install.sh` exits
as a no-op and the resulting VHD is byte-identical to a stock build.

[issue]: https://github.com/notaryproject/notation/issues/1337
