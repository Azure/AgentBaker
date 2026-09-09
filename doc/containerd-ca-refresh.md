# Containerd and Linux CA refresh

Containerd's CRI registry resolver reads `certs.d` when resolving an image.
An explicit CA file is read for a new resolver, but the Go system-root pool in
the long-running daemon may retain the roots from its first use. Updating the
OS trust store alone therefore does not reliably enable pulls from a registry
whose CA was added later.

## Configuration

AgentBaker provides an `aks-system-ca.crt` symlink to the distribution's generated
system trust bundle:

* Ubuntu and Flatcar: `/etc/ssl/certs/ca-certificates.crt`
* Mariner, Azure Linux and ACL: `/etc/pki/tls/certs/ca-bundle.crt`

The link in `/etc/containerd/certs.d/_default` uses containerd's Docker-style
certificate-directory fallback. No wildcard registry, HTTP endpoint, or
`skip_verify` setting is needed. The link follows replacement of the bundle by
the distribution's trust-update command. It does not contain a copied snapshot
of the CA data.

New AKS-generated bootstrap and legacy mirror configurations reference the
native system bundle explicitly for **both** the mirror and the implicit upstream server.
Their existing capabilities, URL path handling and headers remain unchanged.
The standalone CA installer reconciles the fallback links after updating trust.
It also upgrades only exact, unmodified older AKS mirror templates to reference
the native bundle for both server and mirror, publishing the changed TOML
atomically with GNU `sed -i` while retaining permissions. This runs on the node actually
refreshing its CAs, without depending on base-preparation variables or changing
node preparation. New mirror configurations do not depend on a link or helper
from a newer VHD: they reference the existing OS bundle directly.
No containerd or kubelet restart is performed by the reconciliation.

## Deliberate boundaries

`_default` is a fallback, **not** an overlay on an existing per-registry
directory. Docker-style directories without a nonempty `hosts.toml` receive
the system-bundle link without replacing other `.crt`, `.cert`, or `.key`
files. An existing `aks-system-ca.crt` pointing somewhere else is an error,
not something to overwrite silently.

Custom `hosts.toml` files, including a custom `_default/hosts.toml`, retain
their owner's policy. Owners that want refreshed OS trust must add the bundle
path to the `ca` list for each configured host and the root/default server,
while retaining their custom CA paths. This also applies to configurations
installed by an external registry or artifact-streaming component. AgentBaker
does not parse/rewrite arbitrary custom TOML or alter authentication. Modified
older AKS templates are treated as custom rather than silently overwritten.
On older containerd versions, a root-only TOML configuration also needs an
empty `[host]` table.

This addresses **CA additions**. Explicit CA files augment the process's system
roots; they do not guarantee distrust/removal of roots already cached by Go.
Certificate revocation/removal and trust reload in other processes require a
separate assessment. Windows uses certificate stores rather than these PEM
bundle paths and is not changed here. Applying the change to source does not
retroactively update already-running nodes: the corresponding updated
provisioning/refresh artifacts must first be delivered.

## Reproducer

`ContainerdCARotation/{Ubuntu2204,Ubuntu2404,AzureLinuxV3}` runs in the
AgentBaker E2E harness on a disposable scenario node:

1. Generate ephemeral CAs A/B and matching TLS server certificates on the node.
2. Serve isolated OCI images on the node's private IP, **not localhost**
   (which has special containerd TLS behavior).
3. Pull an image through real CRI with a test-owned per-host CA A configuration.
4. Verify an unconfigured CA B endpoint fails TLS validation.
5. Stage B and invoke the real `install_certs_to_trust_store` function from
   the branch's `init-aks-cloud.sh`. Only certificate acquisition is replaced;
   the actual OS trust update and runtime integration execute.
6. Verify a fresh OS-trust client accepts B, then pull a new image through the
   still-running containerd. Tags have distinct config/manifest digests and
   successful pulls must make registry requests.
7. Pull through the real generated bootstrap mirror and an unchanged older
   mirror template migrated by refresh. Verify custom CA A still works, and
   wrong-SAN/untrusted-CA endpoints still fail. Compare containerd's PID and
   monotonic start timestamp before/after.
8. Remove only fixture-owned images, certificates, host configuration and
   temporary files. The harness tears down the scenario VM.

The fixture uses the harness's existing blob transport for its binary and
script, avoiding large SCP messages over Bastion. It never downloads customer
certificates and commits no private keys.

For a local focused run, from `e2e/`, use the documented E2E environment:

```sh
go test ./cmd/ca-rotation-fixture
go run ./cmd/e2e run --parallel 3 --retries 0 --disable-scriptless \
  ContainerdCARotation/Ubuntu2204 ContainerdCARotation/Ubuntu2404 \
  ContainerdCARotation/AzureLinuxV3
```

Omit `--disable-scriptless` to exercise ANC as well; test results must record
which bootstrapping mode and image/runtime versions were actually used.
Flatcar/ACL/Mariner paths are not proven by the three scenarios above.

## Scripted delivery regression guard

The initial repair produced 65,791 bytes / 87,724 encoded characters of scripted
Ubuntu CustomData, exceeding the existing 87,380-character compute API guard.
The reconciler now shares the installation/error path between acquisition modes
and uses native atomic file operations instead of duplicate template-building
code. The same regression configuration produces **65,525 bytes / 87,368 encoded
characters** for both Ubuntu 22.04 and 24.04: **12 encoded characters of headroom**.
This is a narrow margin, not a general guarantee for every configuration.

`should keep scripted Ubuntu CustomData within the compute API limit` asserts
the unchanged limit for both Ubuntu versions. Do not bypass it when extending
embedded scripts. No hotfix entry removal, unrelated script minification, or
production artifact-format change is used to fit the payload.

Passing the size guard, standalone-refresh tests, or baked-ANC scenarios does
not by itself prove scripted provisioning, a new ANC build, or a newly baked
VHD. Record actual E2E and hosted gate results separately before rollout.

References:
* [containerd hosts configuration](https://github.com/containerd/containerd/blob/v1.7.28/docs/hosts.md)
* [containerd resolver CA loading](https://github.com/containerd/containerd/blob/v1.7.28/remotes/docker/config/hosts.go)
* [CRI resolver configuration](https://github.com/containerd/containerd/blob/v1.7.28/pkg/cri/server/image_pull.go)
