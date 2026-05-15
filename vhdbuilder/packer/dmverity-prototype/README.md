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

1. `tdnf install -y` the patched containerd2 RPM, replacing the AKS-pinned
   stock package. `containerd --version` should report 2.0.1+dmverity.
2. Drops [`containerd-dmverity.toml`](containerd-dmverity.toml) at
   `/etc/containerd/config.toml` (the RPM ships none, and built-in defaults
   use overlayfs which would skip the erofs differ).
3. Drops the OS Guard signer CA at `/etc/aks/dmverity/osguard-signer-ca.pem`.
4. Installs [`dmverity-keyring-load.sh`](dmverity-keyring-load.sh) plus
   [`aks-dmverity-keyring.service`](dmverity-keyring.service) (enabled,
   ordered before `containerd.service`).
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
| `containerd-dmverity.toml` | `/etc/containerd/config.toml` |
| (downloaded) patched `containerd2-2.0.1-...azl3` RPM | `/usr/bin/containerd` etc. |
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

- `containerd2-2.0.1-3000.azl3.x86_64.rpm` — patched containerd
- `osguard-signer-ca.pem` — OS Guard signer CA (public material; safe to
  expose intentionally)

Other blobs in the same container (private keys, VHDX images) are listed
in [`dmverity-prototype.env`](dmverity-prototype.env) comments — keep an
eye on them when toggling.

## Validating on a provisioned node (optional Tier 2)

The VHD build is the primary test. To additionally verify on a provisioned
AKS node, note that `cse_config.sh` overwrites `/etc/containerd/config.toml`
at provisioning time, so the erofs config must be re-applied:

```bash
# Re-install our config + restart containerd.
sudo install -m 0644 \
    /opt/azure/containers/dmverity-prototype/containerd-dmverity.toml \
    /etc/containerd/config.toml
sudo systemctl restart containerd

# Confirm the CA is in the kernel keyring.
sudo /opt/azure/containers/dmverity-keyring-load.sh
sudo keyctl show %keyring:.machine | grep -i osguard

# Pull the test image and inspect for .dmverity sig files.
sudo crictl pull notarycontainerregistry.azurecr.io/notary-demo@sha256:1f2972bc2f1e4f7e3c2bef9cb382859a0cdd5458465a72c0c9568083b8007f3c
sudo find /var/lib/containerd -name '*.dmverity' -ls 2>/dev/null
```

Productizing this for customer-provisioned nodes requires wiring the erofs
+ dm-verity settings into the apiserver-generated CSE template — out of
scope for Tier 1.

## Removing the prototype from a VHD

Set `DMVERITY_RPM_URL=` and `DMVERITY_CERT_URL=` empty in
[`dmverity-prototype.env`](dmverity-prototype.env). `install.sh` exits as a
no-op and the resulting VHD is identical to a stock build.

[sig]: https://github.com/containerd/containerd/blob/main/pkg/snapshotters/signatures.go
[issue]: https://github.com/notaryproject/notation/issues/1337
