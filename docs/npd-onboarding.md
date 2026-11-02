# NPD in Ubuntu and Azure Linux 3 VHDs and generic live patching

This work builds on the generic Ubuntu snapshot updater (#8952) and the generic
Azure Linux updater (#9567). The AKS resource provider publishes the `npdConfig`
component through the Live Patching Controller.

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
`/usr/share/inspektor-gadget/diagnostics.sh`. Packer installs these alongside the
existing IG baseline. `install-ig.sh` disables the service for capture and writes
`/etc/ig.d/diagnostics-version` containing `1`. No live reports or collector
definitions may be captured. The import service remains separate;
stopping it is not part of the diagnostics lifecycle.

`nodePrep` enables and starts diagnostics whenever the image capability matches
and prerequisites are present. Activation is bounded and nonfatal to
provisioning; NPD and kubelet
do not depend on this optional service. Enabling the unit restores it after
reboot without another CSE run. systemd restarts a failed daemon with a delay and
start-rate limit; every start reconciles the three owned collectors. Exhausted
start limits require operator investigation and, after repair,
`systemctl reset-failed ig-diagnostics.service`.

`Running` alone is not readiness: IG can report it before gadget initialization
finishes. After creating the collectors, the bootstrap invokes the packaged
reader with `--check-startup` and a private candidate manifest. It waits up to
30 seconds for fresh, non-initial reports from all three current instances
(including an empty throttle report). Only then is `collectors.json` published.
The reader command has a 35-second timeout, within the unit's existing overall
120-second startup limit. Missing/incompatible readers or unavailable reports
fail `ExecStartPost`, so systemd stops the daemon and applies its restart policy.
This is a startup check, not ongoing collector supervision.

The root-only socket is `/run/ig-diagnostics/ig.socket`. The daemon uses its own
persistent definitions directory, bind-mounted over IG's fixed
`/var/lib/ig/gadgets` path inside the service mount namespace. Imported images
remain shared. Reconciliation replaces only `aks-npd-cpu-v1`,
`aks-npd-memory-v1` and `aks-npd-throttle-v1`, with no registry pulls. The initial
settings are five-second reports and fifteen server-limited entries per
collector. CPU and memory use separate process sorts; throttle uses its own
cgroup datasource. These settings and the combined resource budget still need
representative AKS validation with the pinned IG 0.57.0 package.

Reports are root-restricted under `/var/log/ig-diagnostics`, with a 20 MB active
file and two rotations, maximum age one day, uncompressed. This is a disk budget,
not a guaranteed retention duration. NPD reads at most 8 MiB across the active
file and two rotations. `/run/ig-diagnostics/collectors.json` atomically identifies
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
`--check-startup` mode; publish and bake the compatible package before shipping
the image that starts the daemon by default.

**Publication gate:** local companion changes do not update the PMC
`node-problem-detector-aks-config` package. The extension's checked-in
`dependencies/node-problem-detector/npd.mk` packages its `baseline/`, not the
`config/` plugin tree. Coordinate the actual PMC config-package source/revision
with its release owner and include both changed plugins before releasing images
with this feature. No package publication or revision change is made
here. The existing `npdConfig` updater can deliver the compatible NPD reader, but
cannot deliver the IG daemon, binary and images as an atomic feature update.

Absent capability, NPD intentionally uses the one-shot IG path (with its minimum
version check for CPU throttling). On supported, capable images, an unavailable
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

## Validation

ShellSpec covers package selection, ownership, CSE activation, version/no-action
logic, dispatch isolation on both updaters, failed downloads, activation failure,
transaction recovery and Azure Linux lock/config handling. VHD content checks
require the marker, executables, monitors, cached package, holds, disabled service
and generic handler. The E2E validator checks baked service startup.

`spec/parts/linux/cloud-init/artifacts/npd-package-integration.sh` runs only in a
disposable Ubuntu or Azure Linux 3 container. It uses the published package and
the real package manager, creates a local test revision, and tests held upgrade,
local-edit replacement, removed-config cleanup, marker preservation and downgrade
recovery. Only service restart is stubbed.

Like the generic updaters and securityPatch handlers, `npd-update.sh` is delivered
only by the VHD. It is not written by provisioning CustomData or hotfix delivery,
which avoids CustomData growth. Nodes gain the handler through new VHDs.

Config package content comes from the AKS VM extension's NPD configuration. The
`1.0.0` revision 2 packages (`1.0.0-ubuntu<release>u2`, `1.0.0-2.azl3`) carry current
extension content, including the node-exporter, intrusive-probe, CPU-throttle IG and
GPU ECC monitors. Because the pins are revisionless, VHD builds bake the newest
published revision, so new VHDs start on current config and running nodes only need
LPC for changes published after their image was built.
