# Containerd and Linux CA refresh

Linux containerd already uses OS trust without an explicit `ca =` override.
Go initializes and caches its system-root pool on first use. A later OS CA
installation does not replace that pool in a long-running daemon, so new
registry CAs can fail with `x509` errors until containerd starts again.

## Design decision: conditional restart, not registry migration

This revision replaces the earlier restart-free design with a scheduled,
trust-change-triggered containerd restart. It removes `update_containerd_ca`,
the `aks-system-ca.crt` links, and the added `ca =` lines in the AKS mirror
generators. No registry configuration is reconciled or rewritten.

Explicit CA references can let newly constructed resolvers read updated files,
but applying that design to existing nodes requires handling fallback directories,
per-host and upstream TOML scopes, customized policies and symlinks. Restarting
the daemon instead addresses the cached system-root pool directly and leaves
registry policy ownership unchanged. This also removes the migration code that
could replace a symlinked `hosts.toml` with a regular file.

The trade-off is a temporary interruption to CRI operations and image pulls.
The checked-in containerd unit uses `KillMode=process`; runtime shims are expected
to preserve existing containers while the daemon restarts. This expectation
requires live verification for each supported runtime/unit combination, including
ACL/Flatcar units delivered through system extensions. The code does not restart
kubelet, kill containers, drain nodes, reboot or modify unit policy.

## Scheduled refresh and recovery

Only `init-aks-cloud.sh ca-refresh <location>` invokes
`refresh_certs_and_containerd`. Initial provisioning still acquires and installs
certificates without runtime lifecycle changes, jitter or recovery state.

Each scheduled invocation sleeps once for a random 0-300 seconds, then holds
a node-local lock across acquisition, installation and recovery. The systemd
timer has `RandomizedDelaySec=0` to avoid a second delay; cron and systemd retain
their installed location argument. Lock acquisition is bounded at 60 seconds.

The coordinator compares SHA256 of the generated native system-bundle bytes,
not download count or timestamps. It uses `/etc/ssl/certs/ca-certificates.crt`,
or `/etc/pki/tls/certs/ca-bundle.crt` when the former is absent/empty, matching
the normal Ubuntu/Flatcar and Mariner/Azure Linux/ACL paths. A missing/empty
bundle is an error. Reordered bundle bytes can conservatively trigger a restart;
the implementation does not normalize certificate sets.

The baseline is saved before acquisition so a partially failed installation
cannot make a later retry mistake changed roots for already-loaded roots.
Root-owned `/run/aks-ca-refresh/pending` is written atomically with restrictive
permissions under the lock. Before a restart it records the attempted bundle
digest and the prior systemd InvocationID. It is data, never executable shell.
Boot-local state disappears on reboot and is not relied on across PIS baking.

Unchanged trust with no pending recovery does not restart containerd. Changed
trust uses `systemctl try-restart` for an active daemon, avoiding starting a
service stopped between the state check and request. A failed daemon is retried
with `restart` only when this coordinator previously attempted recovery.
An unrelated inactive or failed daemon is never started. Recovery requests are
bounded at 90 seconds, followed by bounded active-service and CRI `RuntimeReady`
checks. A new systemd InvocationID is required; `NRestarts` alone is insufficient.

Failed acquisition, installation or recovery returns nonzero and retains pending
state. A later refresh retries even if it downloads identical certificates.
If the earlier restart completed, its digest still matches, and the changed
invocation is healthy, recovery is acknowledged without another restart.
Pending state is removed only after success. Telemetry and a final
`CA_REFRESH_RESULT=unchanged|restarted|recovered|inactive` marker identify the
outcome; failures have no success marker.

## Deliberate boundaries

Custom registry policies, authentication, client keys, mirrors and symlinks are
left untouched. No wildcard registry, HTTP endpoint or `skip_verify` is added.
A restart does not override an owner's explicit trust policy. This PR has not
shipped the superseded experimental links/configurations: use clean scenario
nodes rather than inventing a production cleanup migration.

This addresses **CA additions**, not general revocation/removal correctness or
trust reload in other processes. Windows is unchanged. Changing this source
does not update existing customer nodes; delivery of the repaired artifact and
release tracking are separate from this implementation.

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
   with the exact script bytes in AgentBaker's production-rendered CustomData,
   recorded before test-only injections. This honors normal comment removal,
   templating, compression, and ACL/Flatcar Ignition tar packaging without
   duplicating the renderer. Only the expected digest and delivery origin are
   retained. When a recognized payload does not deliver that file (normally
   ANC/scriptless), require the exact current VHD source instead; such coverage
   needs the matching candidate image. Missing provenance, malformed/ambiguous
   payloads, and mismatched hashes fail closed. The test never uploads a
   substitute or adds a fixture to production CustomData.
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
6. Compare independent before/after bundle fingerprints and the coordinator
   outcome. Unchanged trust requires containerd continuity; changed trust
   requires a new healthy invocation. Kubelet identity must remain unchanged.
   Check node readiness and the survivor workload's pod UID, container ID and
   restart count, then verify exec/DNS/HTTP again. Repeat the installed refresh
   for idempotence, applying the same change-aware expectations if the platform
   changes trust again.
7. Reuse the fixture in **pull-only** mode for a demonstrably uncached real
   CRI network pull: a unique OCI manifest/config digest, observed registry
   requests, and a test-owned explicit per-registry CA. This phase does **not**
   change OS trust or invoke any refresh helper. It proves fresh CRI work, not
   acceptance of a newly distributed platform CA.
8. Schedule a second normal workload after refresh (including MCR resolution,
   readiness, exec/DNS/HTTP); recheck service identity and the original workload.

All waits are bounded and include the scheduled jitter and recovery budget.
A kubelet restart, unexpected containerd restart on unchanged trust, or changed
workload identity is not considered success merely because it later recovers.
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
TEST_TIMEOUT=90m RCV1P_TAGS_AUTO_INJECTED=true go run . run \
  --subscription-id "$RCV1P_E2E_SUBSCRIPTION_ID" \
  --tags rcv1pcertmode=true --parallel 3 --retries 0 --disable-scriptless \
  RCV1P_Ubuntu2204 RCV1P_Ubuntu2404 RCV1P_AzureLinuxV3
```

Use `TEST_TIMEOUT=90m` for dedicated restart validation. The unchanged global
50-minute scenario default can expire during provisioning plus the 55-minute
refresh-health framework budget. Individual scheduled invocations are bounded
at 20 minutes and the two-refresh synthetic fixture at 43 minutes; these budgets
include acquisition retries, jitter and runtime recovery.

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
5. Stage B through fixture acquisition and invoke the production
   `refresh_certs_and_containerd` coordinator from `init-aks-cloud.sh`.
   The actual installer, jitter and recovery execute; repeat the refresh to
   require a no-op with identical trust.
6. Verify a fresh OS-trust client accepts B, then pull a new image through the
   recovered containerd. Tags have distinct config/manifest digests and
   successful pulls must make registry requests.
7. Pull through the real generated bootstrap mirror and an unchanged older
   mirror template, without migration. Verify custom CA A still works and
   wrong-SAN/untrusted-CA endpoints still fail. Require runtime recovery, stable
   kubelet identity and survival of the harness's existing workload.
8. Remove only fixture-owned images, certificates, host configuration and
   temporary files. The harness tears down the scenario VM.

The fixture uses the harness's existing blob transport for its binary and
branch scripts, including the sibling modules sourced by `cse_config.sh`,
avoiding large SCP messages over Bastion. Scripts come from the embedded
repository artifacts, and the fixture build resolves the E2E module independently
of the caller's working directory. It never downloads customer certificates
and commits no private keys.

For a focused synthetic run in the same verified dedicated setup:

```sh
go test ./cmd/ca-rotation-fixture
TEST_TIMEOUT=90m go run . run --subscription-id "$RCV1P_E2E_SUBSCRIPTION_ID" \
  --tags rcv1pcertmode=true --parallel 3 --retries 0 --disable-scriptless \
  RCV1P_ContainerdSyntheticCARotation
```

Omit `--disable-scriptless` to exercise ANC as well; test results must record
which bootstrapping mode and image/runtime versions were actually used.
Flatcar/ACL/Mariner paths are not proven by the three scenarios above.

### Local restart-revision validation

Before integrating main, at `5c7eea23415df2fee4a56ce29ecbf616b8753bab`,
the 56 focused shell cases, root Go suite and vet, E2E unit suite and vet,
harness build and Linux fixture cross-build passed locally. The fixture and shared
evidence package also passed race tests. The two scripted Ubuntu size guards passed,
as detailed below. Standard gzip decoding, deterministic encoding, artifact
round trips and production-rendered provenance remain covered. ANC parser
regeneration passes without snapshot changes. macOS tests and cross-compilation
do not establish live Linux/systemd restart safety, workload continuity or
cross-OS coverage.

### Main integration: payload-size blocker

Integrating main at `2944b6dce46ce9721aadb099a2adf94b4ac498e7` preserves the
conditional-restart production script, ports the CA validators to the new
scenario/runner packages, and stages the registry generator's split modules
together. The E2E unit suite, vet and builds pass, as do root vet, gzip
compatibility, fixture/shared-evidence race tests, the Linux fixture cross-build
and 70 focused shell cases.

**The root Go suite is not passing:** both scripted Ubuntu 22.04 and 24.04
CustomData guards now measure **68,190 bytes / 90,920 encoded characters**,
exceeding the unchanged **87,380-character limit by 3,540 characters**.
The earlier passing payload measurements below predate this integration.
Conflict resolution is published with this explicit blocker; additional
payload/compression work is deferred. No guard is weakened, provisioning
safeguard removed, or production encoding changed to hide the failure.
This branch must not be treated as merge- or rollout-ready.

Repository-wide `make validate-shell` also fails its POSIX-only pass
(`SC3010`/`SC3014`) on Bash-specific syntax across existing scripts, including
the unchanged refresh coordinator's digest check. The normal shell-dialect
pass succeeds. This lint failure is recorded rather than waived or addressed
through unrelated production-script edits in the conflict-resolution commit.

### Historical validation of the superseded design

**None of the hosted results below validates the conditional-restart revision.**
They are preserved for traceability, not counted as runtime-restart,
workload-survival or release evidence for the new design.

Before the RCV1P-framework migration, the original synthetic fixture reproduced
the stale-root failure on all three OS families. At `ec071b93b3`, its corrected
scripted delivery passed 3/3, none skipped: Ubuntu 22.04/containerd 1.7.34-2,
Ubuntu 24.04/2.3.3-2 and Azure Linux v3/2.2.4. Those tests sourced the trust
installer and staged synthetic certificates. They **did not** prove invocation
of the installed scheduled RCV1P acquisition/refresh path.

The earlier framework migration's local E2E unit tests, fixture unit tests,
`go vet ./...`, and harness build passed. Those tests covered registration, tag selection, the
feature guard (including authentication errors), stage order/error propagation,
strict schedule/location checks and fresh acquisition evidence. Provenance
regression tests use the real AgentBaker renderer for all five Linux distros
in scripted, scriptless and scriptless-NBC modes, including Ignition packaging.
Malformed payloads, missing provenance, altered bytes and incorrect paths fail.

The first dedicated hosted attempt (180373926) failed before live execution:
an existing configuration test assumed the region environment variable was
unset. Test-isolation commit `f9851f37a5` unblocked that preflight. The rerun
(180376228) executed exactly the three selected Ubuntu 22.04/24.04 and Azure
Linux v3 cases, none skipped; all original attempts and two retries per case
failed at the same provenance assertion. The installed script matched the
normal production-rendered bytes, but the validator incorrectly compared raw
source before CSE comment removal. The corrected validator derives its expected
digest from the actual production payload instead. These failed runs provide
**no** real-refresh health pass.

Dedicated hosted run **180383015**, at implementation commit
`ed992a3a0668c8dc162ab7b8c8fc988b1d47275b`, passed on 2026-09-09 at
20:19:52 UTC. Configuration preflight passed with the required region override.
Exactly the three selected real-health cases executed, each on its first attempt:

| Case | Observed containerd | First attempt | Retries used |
|---|---|---|---|
| `RCV1P_Ubuntu2204` | 1.7.34-2 | Passed (5m35s) | 0 |
| `RCV1P_Ubuntu2404` | 2.3.3-2 | Passed (5m31s) | 0 |
| `RCV1P_AzureLinuxV3` | 2.2.4 | Passed (4m44s) | 0 |

The runner allowed two retries but used none. Its overall accounting was
3 passed, 0 flaky, 194 filtered/skipped, 0 failed; none of the three intended
cases was skipped. All ten sequential refresh-health stages completed.
On each node, the installed script SHA256 matched production-rendered CustomData:
`9c5b4828394a92f85ece1929ee62f05e9844ea1e3dc3ec9ab0d5e36a676c85bf`.
The test verified the installed cron schedule and invoked its script with the
actual VMSS location, then required fresh RCV1P mode/opt-in, root/intermediate
acquisition and matching installed anchors. Kubelet/containerd identity,
monotonic start and restart counters remained unchanged across refresh.
Persistent workload identity/readiness and zero container restarts, new
scheduled workload readiness, exec/DNS/HTTP and uncached CRI pulls all passed.
Each fixture pull observed three registry requests without changing OS trust.
Scenario cleanup completed and disposable VMSS deletion was initiated.

This run used verified dedicated routing, `southcentralus`,
`Standard_D2ds_v6`, scripted delivery and existing published VHD build
180236096. It proves **real-refresh health/idempotence with the branch-delivered
production script**, not forced platform CA rotation, a newly baked repaired
image, fresh ANC or PIS. Ubuntu 26.04 minimal, ACL/systemd, Windows, negative
and separate synthetic cases were explicitly outside this focused run.

Dedicated daily routing is verified. The local Azure account cache does not
contain the exact dedicated subscription, so no local live run was attempted. Hosted
validation must use that existing dedicated routing and the PR commit, with
matching candidate VHDs for paths that do not deliver the script via CSE. The prior generic
fixture successes are not relabeled as dedicated RCV1P integration passes.
Review readiness is not merge or rollout readiness. The pre-integration pipeline
snapshot had merge-conflict-blocked gates; earlier general/GPU failures remain
unresolved. New design changes do not waive those gates or replace live coverage.

## Scripted delivery regression guard

The superseded implementation at `f837e10adf541654eb3475aa0d0fa9c90900fd6b`
produces **65,525 bytes / 87,368 encoded characters** for both Ubuntu 22.04 and
24.04, leaving just **12 encoded characters** below the 87,380 limit. This
baseline was reproduced with the current Go toolchain using an isolated overlay.

With the standard-library encoder, the restart revision produced **66,003 bytes /
88,004 encoded characters**, exceeding the limit by **624 characters**.
Source deduplication and literal/quoted cloud-init embedding did not solve the
compressed-size problem and were reverted rather than dropping recovery checks.

The producer now uses `github.com/klauspost/compress/gzip` **v1.18.5**, already
used by the E2E module, at best compression. The **wire format remains standard
gzip/base64 and decompressed artifact bytes are unchanged by this encoder
switch**. There is no new node-side dependency, no alternate cloud-init encoding,
and no per-file format exception. The standard-library gzip reader remains in
the decoder and compatibility tests.

Before main integration, both Ubuntu cases produced **65,488 bytes / 87,320 encoded characters**,
leaving **60 encoded characters** below the unchanged 87,380 limit. This is
a historical measurement, superseded by the failing merged measurements above,
not a guarantee for arbitrary CustomData. Keep the guard when extending embedded
scripts.

Local three-run compression benchmarks on Apple M4 Pro measured the init script
at 0.58-0.62 ms versus 0.70-0.71 ms with the standard encoder, and `cse_config.sh`
at 1.94-1.98 ms versus 2.10-2.20 ms. The trade-off is approximately **329 KB more
allocation per compression call** (with two fewer allocations). These are
producer microbenchmarks, not API load, VHD build or node provisioning tests.

`should keep scripted Ubuntu CustomData within the compute API limit` has an
independent case for each Ubuntu version so regressions in either remain visible.
No hotfix entry removal, unrelated script minification or production
artifact-format change is used to fit the payload.

Passing the size guard, standalone-refresh tests, or baked-ANC scenarios does
not by itself prove scripted provisioning, a new ANC build, or a newly baked
VHD. Record actual E2E and hosted gate results separately before rollout.

References:
* [containerd hosts configuration](https://github.com/containerd/containerd/blob/v1.7.28/docs/hosts.md)
* [containerd resolver CA loading](https://github.com/containerd/containerd/blob/v1.7.28/remotes/docker/config/hosts.go)
* [CRI resolver configuration](https://github.com/containerd/containerd/blob/v1.7.28/pkg/cri/server/image_pull.go)

🤖 Generated by GitHub Copilot
