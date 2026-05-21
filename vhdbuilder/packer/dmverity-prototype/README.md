# dm-verity prototype

Tier 1 prototype that lands a containerd build with the [snapshotter
signature handler][sig] (containerd `pkg/snapshotters/signatures.go`) plus
the OS Guard signer CA onto an AgentBaker AzureLinux 3 VHD. Lets us prove
the full [notaryproject/notation #1337][issue] flow end-to-end **during
the VHD build itself** — the build is the test.

```
notation sign --dm-verity → ORAS referrer (PKCS#7) → ACR
                                                     │
                                                     ▼
                                  containerd pull (CRI/transfer)
                                  ├─ AppendSignatureHandlerWrapper
                                  │  fetches referrer, stamps annotations
                                  │
                                  ▼
                                  erofs differ Apply()
                                  ├─ formats layer w/ dm-verity
                                  ├─ verifies root hash matches
                                  └─ writes <layer>.dmverity sig file
                                                     │
                                                     ▼
                                  erofs snapshotter Mount()
                                  └─ verity.Open(... signature_file)
                                                     │
                                                     ▼
                                  Linux kernel dm-verity target
                                  └─ verifies PKCS#7 against keys in
                                     .machine kernel keyring
```

## How it's wired

Single-step: [`install.sh`](install.sh) runs from
[`vhdbuilder/packer/install-dependencies.sh`](../install-dependencies.sh)
right after `installStandaloneContainerd` (AzureLinux/Mariner only). On
empty URLs in [`dmverity-prototype.env`](dmverity-prototype.env) it is a
hard no-op — the resulting VHD is identical to a stock build.

When enabled it:

1. (Optional) `tdnf install` the patched kernel RPM
   (`kernel-6.6.139.1-99.osguard1.azl3`) by file path. The kernel bakes
   the OS Guard CA into `CONFIG_SYSTEM_TRUSTED_KEYS`, so dm-verity
   accepts layer signatures out of `.builtin_trusted_keys` on first boot
   with no userspace enrollment. The packer build VM stays on the stock
   kernel for the rest of the build; the captured VHD boots the new
   kernel on first start.
2. `tdnf install` the patched containerd2 RPM (`>=2.0.1-3001`) by file
   path, replacing the AKS-pinned stock package. The RPM itself ships:
   - `/etc/containerd/config.toml` (erofs snapshotter + dm-verity)
   - `/etc/modules-load.d/aks-dmverity.conf` (auto-load `erofs` +
     `dm_verity` at boot via `systemd-modules-load.service`)
   - `Requires:` `erofs-utils`, `veritysetup` (pulled from the base repo)
3. Drops the OS Guard signer CA at
   `/etc/aks/dmverity/osguard-signer-ca.pem` (defensive — used by
   userspace verifiers and as the keyring fallback for non-osguard
   kernels).
4. Installs [`dmverity-keyring-load.sh`](dmverity-keyring-load.sh) plus
   [`aks-dmverity-keyring.service`](dmverity-keyring.service) (enabled,
   ordered before `containerd.service`). The loader short-circuits to
   a single `keyctl list` + `grep` on osguard kernels, so it is a
   confirmed no-op on the patched kernel path.
5. Restarts containerd so the next stage of `install-dependencies.sh` —
   the container-image pre-pull loop — uses the patched binary + erofs
   snapshotter for every image listed in
   [`parts/common/components.json`](../../../parts/common/components.json),
   including the dm-verity test image
   `notarycontainerregistry.azurecr.io/notary-demo@sha256:1f29...3c`.

If the test image's signed referrer is fetched, the layer-root-hash matches,
and the `.dmverity` sig file lands in the snapshotter content store, the
prototype works. If any of those fail the VHD build fails loudly.

## Files this directory ships into the VHD

| Source (in this dir) | Destination on VHD |
|---|---|
| `install.sh` | (runs in build VM only; not installed) |
| `dmverity-prototype.env` | (runs in build VM only; not installed) |
| `dmverity-keyring-load.sh` | `/opt/azure/containers/dmverity-keyring-load.sh` |
| `dmverity-keyring.service` | `/etc/systemd/system/aks-dmverity-keyring.service` (enabled) |
| (downloaded) patched `kernel-6.6.139.1-99.osguard1.azl3` RPM | `/boot/vmlinuz-6.6.139.1-99.osguard1.azl3` etc. |
| (downloaded) patched `containerd2-2.0.1-3001.azl3` RPM | `/usr/bin/containerd`, `/etc/containerd/config.toml`, `/etc/modules-load.d/aks-dmverity.conf` |
| (downloaded) OS Guard CA (PEM) | `/etc/aks/dmverity/osguard-signer-ca.pem` |

## Pipeline workflow (anonymous-blob toggle)

The blob storage account that hosts the artifacts (`kataccstorage`) does not
allow anonymous public access by default. Enable, build, disable:

```bash
# Before the AgentBaker Packer build kicks off
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

The container `aks-rpms` is already configured with `publicAccess=blob`, so
the account-level toggle is the only switch required. Files exposed during
the toggle window:

- `containerd2-2.0.1-3001.azl3.x86_64.rpm` — patched containerd
  (ships config + modules-load + Requires erofs-utils/veritysetup)
- `kernel-6.6.139.1-99.osguard1.azl3.x86_64.rpm` — patched kernel with
  OS Guard CA in `CONFIG_SYSTEM_TRUSTED_KEYS`
- `osguard-signer-ca.pem` — OS Guard signer CA (public material; safe to
  expose intentionally)

Other blobs in the same container (private keys, VHDX images) are listed
in [`dmverity-prototype.env`](dmverity-prototype.env) comments — keep an
eye on them when toggling.

## Validating on a provisioned node (optional Tier 2)

The VHD build is the primary test. On a provisioned AKS node booted from
the new VHD, sanity-check that everything came up correctly:

```bash
# Patched kernel with baked-in CA.
uname -r                                                # 6.6.139.1-99.osguard1.azl3
sudo keyctl list %:.builtin_trusted_keys | grep -i "OS Guard CA"

# Patched containerd + shipped config + auto-loaded modules.
rpm -q containerd2                                      # containerd2-2.0.1-3001.azl3
grep -q 'dmverity_mode = "auto"' /etc/containerd/config.toml
lsmod | grep -E '^(erofs|dm_verity)\b'
systemctl is-active containerd
systemctl is-active aks-dmverity-keyring                # active (exited)

# Pull the test image and inspect for .dmverity sig files.
sudo crictl pull notarycontainerregistry.azurecr.io/notary-demo@sha256:1f2972bc2f1e4f7e3c2bef9cb382859a0cdd5458465a72c0c9568083b8007f3c
sudo find /var/lib/containerd -name '*.dmverity' -ls 2>/dev/null
```

Note: if AKS CSE overwrites `/etc/containerd/config.toml` at provisioning
time, re-apply the RPM-shipped config: `sudo tdnf reinstall -y containerd2`
(or copy from `/usr/share/factory/etc/containerd/config.toml` if you've
added a `tmpfiles.d` rule).

Productizing this for customer-provisioned nodes requires wiring the erofs
+ dm-verity settings into the apiserver-generated CSE template — out of
scope for Tier 1.

## Removing the prototype from a VHD

Set `DMVERITY_RPM_URL=` empty in
[`dmverity-prototype.env`](dmverity-prototype.env). `install.sh` exits as a
no-op and the resulting VHD is identical to a stock build.

[sig]: https://github.com/containerd/containerd/blob/main/pkg/snapshotters/signatures.go
[issue]: https://github.com/notaryproject/notation/issues/1337
