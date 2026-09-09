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

## RCV1P refresh and node-health framework

The existing positive Linux scenarios `RCV1P_Ubuntu2204`, `RCV1P_Ubuntu2404`,
`RCV1P_Ubuntu2604Minimal`, `RCV1P_AzureLinuxV3`, and `RCV1P_ACL` call
`ValidateRCV1PRefreshHealth`. They retain `Tags{RCV1PCertMode: true}`, the
subscription feature guard, and the VMSS opt-in mutator (plus Trusted Launch
for ACL). Windows and negative opt-out validators are unchanged.

The framework runs these stages sequentially, returning the first error:

1. Verify provisioning selected RCV1P, the node opted in, certificates are
   present, and the refresh schedule is installed.
2. Compare SHA256 of the **installed** `/opt/azure/containers/init-aks-cloud.sh`
   with the checkout. A stale VHD script is a failure, not a skip or an
   opportunity to silently upload a substitute. Scripted CSE delivers branch
   code; ANC/ACL coverage needs the matching candidate VHD. No test fixture is
   embedded in production CustomData.
3. Establish a Ready node and active/running kubelet and containerd baseline,
   including PID, monotonic start timestamp and `NRestarts`. Schedule a uniquely
   named ordinary HTTP workload on **only this scenario node**, using the
   existing busybox image, `imagePullPolicy: Always`, and normal pod networking.
   Verify pod readiness, exec, Kubernetes service DNS and local HTTP.
4. Validate the installed cron command (Ubuntu/Azure Linux) or systemd
   `ExecStart` (ACL) contains the actual scenario VMSS ARM location. Execute
   that installed `ca-refresh` command, or start `azure-ca-refresh.service`.
   No certificate endpoint, roots, or production script is substituted.
5. Require successful exit and **fresh** output proving RCV1P mode, opt-in,
   root/intermediate acquisition and trust installation. Reject partial-download
   warnings. For systemd, require a new successful invocation and read only
   that invocation's journal, not historical provisioning output. Compare
   newly downloaded certificates with the installed anchors without printing
   their contents. Raw refresh tracing is not emitted into the scenario log.
6. Require unchanged service identities immediately after refresh. Check the
   node remains Ready and the original workload remains ready with the same
   pod UID, container ID and restart count; verify exec/DNS/HTTP again.
7. Reuse the fixture in **pull-only** mode for a demonstrably uncached real
   CRI network pull: a unique OCI manifest/config digest, observed registry
   requests, and a test-owned explicit per-registry CA. This phase does **not**
   change OS trust or invoke any refresh helper. It proves fresh CRI work, not
   acceptance of a newly distributed platform CA.
8. Schedule a second normal workload after refresh (including MCR resolution,
   readiness, exec/DNS/HTTP); recheck service identity and the original workload.

All waits are bounded. Service auto-recovery is not considered success: a
kubelet or containerd restart changes the baseline even if it recovers.
Scenario-owned pods are deleted with UID preconditions, and fixture-owned
registry configuration/images are cleaned up. Shared pools, nodes and cluster
configuration are not changed.

**This is real-refresh health/idempotence validation.** The real distribution
endpoint may return the same roots on consecutive runs. A passing test must
not be described as an observed platform CA rotation or as proof of removal
of roots cached by arbitrary services.

### Selection and subscription routing

Run against the **verified dedicated RCV1P E2E subscription** with
`Microsoft.Compute/PlatformSettingsOverride` registered; do not silently
fall back to a generic E2E subscription. The CLI flag is `--tags`, not
`--run-tags`; the local CLI subscription flag is `--subscription-id` (environment
`SUBSCRIPTION_ID`). Pipeline templates translate their E2E subscription
configuration and can use `SUBSCRIPTION_ID_OVERRIDE`.

From `e2e/`, with the existing approved E2E environment and gallery configuration:

```sh
go test ./...
go vet ./...
go build ./...

# Focused real-refresh health on branch-delivered scripted CSE.
# Set RCV1P_E2E_SUBSCRIPTION_ID to the verified dedicated test subscription.
RCV1P_TAGS_AUTO_INJECTED=true go run ./cmd/e2e run \
  --subscription-id "$RCV1P_E2E_SUBSCRIPTION_ID" \
  --tags rcv1pcertmode=true --parallel 3 --retries 0 --disable-scriptless \
  RCV1P_Ubuntu2204 RCV1P_Ubuntu2404 RCV1P_AzureLinuxV3
```

Only set `RCV1P_TAGS_AUTO_INJECTED=true` where the platform is known to inject
the tag. This variable controls **negative-case skipping only**: setting it
false does not exclude positive cases. The existing negative pipeline is
unchanged. Unfiltered CLI runs select all scenarios before applying guards;
the scenario tag alone is not a selector. Linux refresh and synthetic cases
require explicit `--tags rcv1pcertmode=true` / `TAGS_TO_RUN`, then check the
configured subscription's feature registration. Thus generic daily runs skip
this coverage even if their subscription also has the feature registered.
Pipeline filters need no changes. Selecting a case by name alone is insufficient.
The explicit filter is a suite opt-in, not proof of subscription identity:
callers must still use verified dedicated routing.

For the complete dedicated suite, select `--tags rcv1pcertmode=true` without
positional names. That also selects existing Windows/negative cases and the
separate synthetic cases below. Do not count a feature-query/authentication
skip as live coverage. For ANC, omit `--disable-scriptless` and use candidate
VHDs containing the branch refresh script. Ubuntu 26.04 minimal and ACL share
the framework, but their inclusion in code is not evidence of live validation.

Using an existing published VHD with `--disable-scriptless` validates that VHD's
runtime together with the branch's normal cloud-init/CSE-delivered refresh
script; it does **not** validate a newly baked/published repaired VHD. The
validator logs the installed path, observed and expected SHA256, and invocation
command, and fails on a mismatch without replacing the script. A successful PR
VHD build does not prove its images are present in the dedicated gallery.
Publishing/copying candidate images is a separate authorized operation, not
part of this test or implied by the framework migration.

## Separate synthetic CA-addition regression

`RCV1P_ContainerdSyntheticCARotation/{Ubuntu2204,Ubuntu2404,AzureLinuxV3}`
is the preserved isolated regression fixture, now RCV1P-tagged, guarded and
opted in. It runs on separate disposable scenario nodes, not between the
real-refresh health stages:

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

For a focused synthetic run in the same verified dedicated setup:

```sh
go test ./cmd/ca-rotation-fixture
go run ./cmd/e2e run --subscription-id "$RCV1P_E2E_SUBSCRIPTION_ID" \
  --tags rcv1pcertmode=true --parallel 3 --retries 0 --disable-scriptless \
  RCV1P_ContainerdSyntheticCARotation
```

Omit `--disable-scriptless` to exercise ANC as well; test results must record
which bootstrapping mode and image/runtime versions were actually used.
Flatcar/ACL/Mariner paths are not proven by the three scenarios above.

### Validation history and current limits

Before the RCV1P-framework migration, the original synthetic fixture reproduced
the stale-root failure on all three OS families. At `ec071b93b3`, its corrected
scripted delivery passed 3/3, none skipped: Ubuntu 22.04/containerd 1.7.34-2,
Ubuntu 24.04/2.3.3-2 and Azure Linux v3/2.2.4. Those tests sourced the trust
installer and staged synthetic certificates. They **did not** prove invocation
of the installed scheduled RCV1P acquisition/refresh path.

Migration-local E2E unit tests, fixture unit tests, `go vet ./...`, and harness
build pass. Tests cover positive/synthetic registration, tag selection, the
feature guard (including authentication errors), stage order/error propagation,
strict schedule/location checks and fresh acquisition evidence.

Dedicated daily routing has been verified, but live execution of the new health
framework is still pending. The local Azure account cache does not contain the
exact dedicated subscription, so no local live run was attempted. Hosted
validation must use that existing dedicated routing and the PR commit, with
matching candidate VHDs for paths that do not deliver the script via CSE. The prior generic
fixture successes are not relabeled as dedicated RCV1P integration passes.
Keep the PR draft until the required live matrix and broader gates are proven.

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
