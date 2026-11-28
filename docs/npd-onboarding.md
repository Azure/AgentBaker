# NPD in Ubuntu and Azure Linux 3 VHDs and generic live patching

This work builds on the generic Ubuntu snapshot updater (#8952) and the generic
Azure Linux updater (#9567). The AKS resource provider publishes the `npdConfig`
and `npdBinary` components through the Live Patching Controller.

Ubuntu and Azure Linux 3 share one handler (`parts/linux/cloud-init/artifacts/npd-update.sh`)
and one lifecycle. Only the package primitives differ: dpkg/apt on Ubuntu and
rpm/tdnf on Azure Linux.

## Lifecycle

- `components.json` pins the NPD binary and config packages; see
  [VHD package pins](#vhd-package-pins-componentsjson). Both distributions ship the
  same config payload (monitors, scripts, `/opt/bin/node-problem-detector-startup.sh`,
  and the systemd unit).
- The VHD installs both packages, holds them, and caches the config package for
  recovery. Ubuntu holds them with `dpkg --set-selections` (like `walinuxagent`, avoiding
  apt-mark's slow apt initialization). Azure Linux uses
  `/etc/tdnf/locks.d/node-problem-detector.conf`; this matters because Azure Linux
  security patching runs `tdnf update` over the ms-oss repository that carries the
  config package. A locked package is skipped without failing that update.
- NPD is disabled for capture. `/etc/node-problem-detector.d/skip_vhd_npd` tells
  the VM extension to leave it alone, matching the exporter/IG ownership pattern.
- CSE starts the baked service in `nodePrep`, after kubelet configuration and GPU
  setup. This also runs on PIS-cached images. No public-settings file is generated;
  monitor defaults and hardware selection come from the config package.
- Pins exist for Ubuntu 20.04, 22.04, 24.04 and 26.04 and Azure Linux 3.0 (including
  Kata), amd64 and arm64. The config package content is the same on every release, so
  Ubuntu 26.04, which has no config build of its own, uses the 24.04 package (see below).
  Azure Linux OSGuard, Azure Container Linux, Mariner 2 and other OSes remain
  extension-managed.
- Azure Linux 3 Kata uses the same packages, tdnf lock and handler as Azure Linux 3.
  NPD runs on the Kata host, not inside Kata guest VMs. The Kata host kernel lacks PSI,
  so pressure monitors fall back to load and iowait. LPC config updates change only
  the held NPD config package; Kata's OS, kernel and Kata packages still update only
  through images (unattended upgrades and securityPatch stay excluded for Kata).

## VHD package pins (`components.json`)

For the IG pressure diagnostics integration, see
[Continuous IG diagnostics](#continuous-ig-diagnostics).

`node-problem-detector-kubernetes` and `node-problem-detector-aks-config` follow the
same convention as other package-backed components (see
`parts/linux/cloud-init/artifacts/README-COMPONENTS.md`): entries are revisionless
upstream versions, for example `0.8.25` and `1.0.0`.

- During the VHD build, `install-npd.sh` resolves each pin to the newest matching
  package revision in PMC (`getLatestDebPackageVersion` on Ubuntu,
  `getLatestRPMPackageVersion` on Azure Linux), for example `0.8.25` ->
  `0.8.25-ubuntu24.04u22` or `0.8.25-22.azl3`. The resolution is logged and written
  to the VHD release notes.
- That exact resolved version is installed, held, and, for the config package,
  cached for recovery. A VHD therefore carries one fixed revision even though the
  pin is revisionless.
- Renovate tracks upstream versions only; new package revisions are picked up by the
  next VHD build without a `components.json` change.
- The build fails if no published revision matches the pin.

This governs only what a new VHD bakes. It goes through the normal VHD build,
validation, and release process.

## Continuous IG diagnostics

Continuous diagnostics starts by default on Ubuntu and standard Azure Linux 3
images carrying capability version `1`, amd64/arm64, on cgroup v2. Older images,
unsupported variants and cgroup v1 retain their current behavior. No opt-in
marker, RP rollout field or in-place IG updater is introduced.

AgentBaker owns `ig-diagnostics.service`, `/etc/ig.d/diagnostics.yaml` and
`/usr/share/inspektor-gadget/diagnostics.sh`, plus the recovery script and
`ig-diagnostics-recovery.service`/`.timer`. Packer installs these alongside the
existing IG baseline. `install-ig.sh` stops diagnostics/recovery and disables the
daemon and timer for capture, then writes
`/etc/ig.d/diagnostics-version` containing `1`. No live reports or collector
definitions may be captured. The import service remains separate;
stopping it is not part of the diagnostics lifecycle.

`nodePrep` arms the recovery timer independently, before checking daemon
prerequisites, then enables/starts diagnostics when they are present. Activation
is bounded and nonfatal to provisioning; NPD and kubelet do not depend on these
optional units. Enabled units return after reboot without another CSE run.
systemd retries failed daemon starts after ten seconds, at most three starts
within ten minutes; every start reconciles the three owned collectors.

The timer checks every five minutes. On an active daemon, it invokes the
packaged reader against `collectors.json`. Unavailable reports alone do not
cause a restart: bounded `gadgetctl show` calls must confirm a missing collector
or an explicit `Error` state. Running collectors with stale reports, unknown
states, client timeouts and ongoing service transitions are left unchanged with
an explicit diagnostic message.

For confirmed failures or inactive/failed services, recovery rechecks
prerequisites and waits fifteen minutes from the first observed eligible failure.
It allows at most two additional recovery batches per boot, at least fifteen
minutes apart. Each batch reserves its budget before `reset-failed` and a
nonblocking start/restart that refuses conflicting systemd jobs. Recovery also
defers if the service changed state in the last ten minutes or changed generation
during the check. The timer may request only two additional starts/restarts;
the daemon's existing automatic restart policy still applies. The root-only
budget in `/run/ig-diagnostics-recovery` survives daemon and recovery-unit
restarts and resets on reboot. Healthy reports clear a pending failure's
cooldown but never replenish the consumed batch count. Budget exhaustion or
ambiguous failures require operator investigation. These are conservative
starting limits, not validated pressure-performance guarantees.

`Running` alone is not readiness: IG can report it before gadget initialization
finishes. After creating the collectors, the bootstrap invokes the packaged
reader with `--check-startup` and a private candidate manifest. It waits up to
30 seconds for fresh, non-initial reports from all three current instances
(including an empty throttle report). Only then is `collectors.json` published.
The reader command has a 35-second timeout, within the unit's existing overall
120-second startup limit. Missing/incompatible readers or unavailable reports
fail `ExecStartPost`, so systemd stops the daemon and applies its restart policy.
Every recovery start must pass this same startup gate. The periodic recovery
check does not add a cold-start fallback to NPD.

The root-only socket is `/run/ig-diagnostics/ig.socket`. The daemon uses its own
persistent definitions directory, bind-mounted over IG's fixed
`/var/lib/ig/gadgets` path inside the service mount namespace. Imported images
remain shared. Reconciliation replaces only `aks-npd-cpu-v1`,
`aks-npd-memory-v1` and `aks-npd-throttle-v1`, with no registry pulls. The initial
settings are five-second reports and fifteen server-limited entries per
collector. CPU and memory use separate process sorts; throttle uses its own
cgroup datasource. These settings and the combined resource budget still need
representative AKS validation with the pinned IG 0.57.0 package.

Reports are root-restricted at `/run/ig-diagnostics/reports.ndjson`, with a 1 MiB
active file and one uncompressed backup, maximum age one day. The buffer uses
the host's `/run` tmpfs and is removed when the service stops or the node reboots;
it is not persistent troubleshooting history. Rotation limits retention, not
the size of the shared tmpfs, and cleanup can temporarily retain an extra backup.
The reader also supports older images' `/var/log/ig-diagnostics/gadget.log` when
no runtime report files exist. It reads at most 8 MiB across the selected active
file and up to two rotations, matching the file's suffix.
`/run/ig-diagnostics/collectors.json` atomically identifies
the boot, reconciliation generation and exact instance IDs. Initial, incomplete,
stale (over ten seconds), future or mismatched reports are rejected. Fresh empty
throttle data stays empty. Successful telemetry is explicitly described as
recent pre-trigger history; it may not include a newly started workload.

### Companion NPD package and compatibility

The companion source changes are in `aks-vm-extension`:
`config/node-problem-detector/plugin/pressure_common.sh` and
`ig_report_reader.py`. The reader uses Python 3's standard library; activation
checks Python 3 and jq rather than installing dependencies at first boot.
The package must deliver the reader beside the pressure helper under
`/etc/node-problem-detector.d/plugin`. AgentBaker must not overlay package-owned
plugins. The daemon's startup check also requires this reader, including its
`--check-startup` mode and runtime `.ndjson` path/rotation support; publish and bake
the compatible package before shipping the image that starts the daemon by default.

**Publication gate:** local companion changes do not update the PMC
`node-problem-detector-aks-config` package. The extension's checked-in
`dependencies/node-problem-detector/npd.mk` packages its `baseline/`, not the
`config/` plugin tree. Coordinate the actual PMC config-package source/revision
with its release owner and include both changed plugins before releasing images
with this feature. No package publication or revision change is made
here. The existing `npdConfig` updater can deliver the compatible NPD reader, but
cannot deliver the IG daemon, binary and images as an atomic feature update.

Absent capability, NPD intentionally uses the one-shot IG path (with its minimum
version check for CPU throttling). Legacy process collection selects the CPU and
memory sort fields for the installed IG version: `cpuUsageRelative` and
`memoryRelative` before 0.57.0, `cpuRelative` and `memRelative` from 0.57.0 onward.
Failed or unrecognized version queries warn and skip legacy IG collection rather
than guessing a schema. On supported, capable images, an unavailable
or stale collector produces
an explicit diagnostic warning, **not** a hidden cold-IG fallback. Pressure
thresholds, event names, redaction, chunking and diagnostic-only exit semantics
are unchanged.

### Runtime verification

On a provisioned supported node, verify capability `1`, the compatible reader,
Python 3/jq and the automatically started unit:

```sh
test "$(cat /etc/ig.d/diagnostics-version)" = 1
test -f /etc/node-problem-detector.d/plugin/ig_report_reader.py
command -v python3
command -v jq
sudo systemctl status ig-diagnostics.service --no-pager
sudo cat /run/ig-diagnostics/collectors.json
```

The common Linux E2E validators include `ValidateIGDiagnostics`. On supported
capability-1, cgroup-v2 nodes it requires the service to be enabled, waits for it
to become active, and uses the packaged reader to verify fresh reports from all
three current collectors. Older images and unsupported environments are skipped.
The validator does not start or repair the service.

NPD rechecks capability at each diagnostic invocation. Stopping or failing the
daemon does not silently select cold collection on a capable image; it produces
an explicit unavailable warning. Normal VHD releases and node-image upgrades
deliver the baseline. Reboot/PIS/offline behavior, service mount isolation,
collector failures,
package update/recovery, end-to-end guest-agent telemetry and the aggregate
three-collector overhead must be validated before merging and releasing.

## Releases that share a config package

The config package is generic: its content is identical across releases. The binary
package is built per release. A release without its own config build uses another
release's config package; today Ubuntu 26.04 uses the 24.04 package. One mapping in
`npd-update.sh` (`npd_ubuntu_config_source`) covers both paths:

- The VHD build resolves the `1.0.0` pin against the 24.04 Microsoft repository and
  bakes, holds and caches that package.
- The LPC handler on a 26.04 node reads the `24.04` payload entry and downloads from
  the 24.04 repository. 26.04 nodes therefore follow the 24.04 config selection; no RP
  change or 26.04 key is needed.
- Both read that repository through a private apt configuration (own sources list,
  lists and cache). System apt never sees the 24.04 repository, and packages are still
  verified against the Microsoft keys the VHD already trusts.

## Live configuration (LPC version selection)

Each generic updater dispatches `npdConfig` with the RP payload. Each node reads
only its own distribution's map:

```json
{
  "ubuntuPackageVersions": {"22.04": "1.0.0-ubuntu22.04u1", "24.04": "1.0.0-ubuntu24.04u1"},
  "azureLinuxPackageVersions": {"3.0": "1.0.0-1.azl3"}
}
```

Unlike the VHD pins, these are exact package versions including the revision. The
release embedded in the version must match the map key (`ubuntu24.04` for `24.04`,
`azl3` for `3.0`).

The payload is the rollout control for running nodes. The RP toggle selects exactly
which revision each cluster moves to, through its staged rollout. Nodes never
resolve "the newest revision" themselves, so publishing a new revision to PMC does
not change running nodes until the toggle selects it. Every node given the same
selection converges to the same package.

Selection is forward-only (fix forward):

- An installed version equal to or newer than the selection is successful
  no-action. This includes VHDs that baked a newer revision than the one selected.
- Selecting an older version, or clearing the toggle, does not revert nodes.
- To recover from a config that installed successfully but behaves badly, publish
  the known-good content to PMC under a higher version and select it. For example,
  if `1.0.0-ubuntu22.04u3` is bad, publish the fix as `u4` and select `u4`; selecting
  `u2` is a no-op on `u3` nodes.
- Local rollback is separate: a package that fails to install or start is replaced
  by the cached prior package automatically (see below).

The node compares the target against its installed config version with the native
package ordering (`dpkg --compare-versions` or `rpm.vercmp`) before accessing
repositories. No target, an equal/newer installed version, or an extension-managed
node is successful no-action.

For an update, the handler downloads and checks the desired package before
installing it. A durable pending marker records the prior cached version before
mutation. It installs the held package, cleans obsolete package-owned config
files, reloads systemd, restarts NPD, and checks the local health endpoint. A
failure reinstalls the cached prior package to restore both package-manager and
file state. Interrupted transactions are recovered on the next invocation, first
finishing an interrupted dpkg run (`dpkg --configure -a`) or dropping a duplicate
rpm database entry left by an interrupted upgrade.
`/healthz` establishes process startup, not successful reporting of every monitor
or node condition.

Package-manager differences handled by the shared transaction:

- Locally modified config files take the package copy (`--force-confnew` on
  Ubuntu; `.rpmnew` replacement on Azure Linux). Leftover `.dpkg-old`, `.dpkg-dist`
  and `.rpmsave` copies are removed.
- Azure Linux downloads use a copy of `tdnf.conf` without `locks.d`, so the held
  package can be fetched with normal repository and package signature checks.
  Installation and rollback use `rpm -U --oldpackage --replacepkgs`, which does not
  read tdnf locks.

The generic updater owns successful-goal checkpoints and node status. NPD uses
the shared event transport with task prefix `AKS.LivePatching.npdConfig`.
Direct PMC servicing requires egress; RP currently excludes network-isolated
clusters from NPD publication. Their baked baseline still starts without a pull.

## Live binary updates (`npdBinary`)

`npdBinary` updates the binary package `node-problem-detector-kubernetes` and is a
separate component from `npdConfig`, with its own RP toggle
(`npd-binary-package-versions`, not expedited), payload, status and events
(`AKS.LivePatching.npdBinary.*`). The payload has the same shape, for example
`{"ubuntuPackageVersions":{"24.04":"0.8.25-ubuntu24.04u24"},"azureLinuxPackageVersions":{"3.0":"0.8.25-24.azl3"}}`.

Both components run the same transaction in `npd-update.sh` (forward-only version
selection, download, validation, durable marker, held install, restart, health
check, rollback to the cached prior package, interrupted-transaction recovery), but
each uses only its own package, marker (`pending` for config, `pending-binary` for
the binary) and cached rollback package. A config update never moves the binary,
and the reverse. Every release has its own binary build, so the 26.04 config
mapping does not apply to the binary.

Binary-specific checks:

- The binary is updated only when the config package is also installed: it owns the unit and startup script,
  so without it NPD can't be restarted or rolled back. This matches the provisioning check.

- Before installing, the staged package must contain the NPD executables, and the
  new `node-problem-detector --version` must run on the node.
- After restarting NPD, `/healthz` is not enough: a binary can stay active and
  healthy while every apiserver call fails. The handler stops NPD before the
  restart and requires a node condition heartbeat (`KernelDeadlock` or
  `ReadonlyFilesystem`) newer than the stop, so a heartbeat from the old process
  cannot count. Otherwise it rolls back to the cached binary. Rollback itself
  only requires local health, so an apiserver outage cannot block recovery.

Both components share one transaction in `npd-update.sh`. Each entry point calls
`npd_select_component`, which sets the package, the transaction marker and the
hooks where the components differ (release mapping, package validation,
post-install cleanup and the post-restart check). An unknown component fails
before any package operation.

The VHD caches both packages in `/opt/node-problem-detector/downloads` for offline
rollback (about 20 MB for the binary). After a successful update the installed version is the only rollback
target, so the handler removes that component's other cached versions.

## Validation

ShellSpec covers package selection, ownership, CSE activation, binary/config
component isolation, the binary reporting check, version/no-action
logic, dispatch isolation on both updaters, failed downloads, activation failure,
transaction recovery and Azure Linux lock/config handling. VHD content checks
require the marker, executables, monitors, cached package, holds, disabled service
and generic handler. The E2E validator checks baked service startup.

`spec/parts/linux/cloud-init/artifacts/npd-package-integration.sh` runs only in a
disposable Ubuntu or Azure Linux 3 container. It uses the published package and
the real package manager, creates a local test revision, and tests held upgrade,
local-edit replacement, removed-config cleanup, marker preservation and downgrade
recovery. Only the service stop and restart are stubbed.
`spec/parts/linux/cloud-init/artifacts/npd-binary-integration.sh` does the same for
the binary with two published revisions: real held upgrade, interrupted recovery
and rollback when NPD stops reporting. Only the service stop, restart and the
reporting check are stubbed.

Like the generic updaters and securityPatch handlers, `npd-update.sh` is delivered
only by the VHD. It is not written by provisioning CustomData or hotfix delivery,
which avoids CustomData growth. Nodes gain the handler through new VHDs.

Config package content comes from the AKS VM extension's NPD configuration. The
`1.0.0` revision 2 packages (`1.0.0-ubuntu<release>u2`, `1.0.0-2.azl3`) carry current
extension content, including the node-exporter, intrusive-probe, CPU-throttle IG and
GPU ECC monitors. Because the pins are revisionless, VHD builds bake the newest
published revision, so new VHDs start on current config and running nodes only need
LPC for changes published after their image was built.
