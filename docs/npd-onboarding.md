# NPD in Ubuntu and Azure Linux 3 VHDs and generic live patching

This work builds on the generic Ubuntu snapshot updater (#8952) and the generic
Azure Linux updater (#9567). The AKS resource provider publishes the `npdConfig`
component through the Live Patching Controller.

Ubuntu and Azure Linux 3 share one handler (`parts/linux/cloud-init/artifacts/npd-update.sh`)
and one lifecycle. Only the package primitives differ: dpkg/apt on Ubuntu and
rpm/tdnf on Azure Linux.

## Lifecycle

- `components.json` independently pins `node-problem-detector-kubernetes` and
  `node-problem-detector-aks-config` from PMC. Both distributions ship the same
  config payload (monitors, scripts, `/opt/bin/node-problem-detector-startup.sh`,
  and the systemd unit).
- The VHD installs both packages, holds them, and caches the config package for
  recovery. Ubuntu uses `apt-mark hold`. Azure Linux uses
  `/etc/tdnf/locks.d/node-problem-detector.conf`; this matters because Azure Linux
  security patching runs `tdnf update` over the ms-oss repository that carries the
  config package. A locked package is skipped without failing that update.
- NPD is disabled for capture. `/etc/node-problem-detector.d/skip_vhd_npd` tells
  the VM extension to leave it alone, matching the exporter/IG ownership pattern.
- CSE starts the baked service in `nodePrep`, after kubelet configuration and GPU
  setup. This also runs on PIS-cached images. No public-settings file is generated;
  monitor defaults and hardware selection come from the config package.
- Pins exist for Ubuntu 20.04, 22.04 and 24.04 and Azure Linux 3.0, amd64 and
  arm64. Ubuntu 26.04 has empty pins until its config package is published.
  Azure Linux OSGuard, Kata, Azure Container Linux, Mariner 2 and other OSes remain
  extension-managed.

## Live configuration

Each generic updater dispatches `npdConfig` with the RP payload. Each node reads
only its own distribution's map:

```json
{
  "ubuntuPackageVersions": {"22.04": "1.0.0-ubuntu22.04u1", "24.04": "1.0.0-ubuntu24.04u1"},
  "azureLinuxPackageVersions": {"3.0": "1.0.0-1.azl3"}
}
```

Versions are exact package versions. The release embedded in the version must
match the map key (`ubuntu24.04` for `24.04`, `azl3` for `3.0`).

The node compares the target against its installed config version with the native
package ordering (`dpkg --compare-versions` or `rpm.vercmp`) before accessing
repositories. No target, an equal/newer installed version, or an extension-managed
node is successful no-action. Updates are forward-only; recover from a bad config
by publishing known-good content under a higher version.

For an update, the handler downloads and checks the desired package before
installing it. A durable pending marker records the prior cached version before
mutation. It installs the held package, cleans obsolete package-owned config
files, reloads systemd, restarts NPD, and checks the local health endpoint. A
failure reinstalls the cached prior package to restore both package-manager and
file state. Interrupted transactions are recovered on the next invocation.
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

The current PMC config pins are a usable integration baseline. Refreshing their
source content and supplying Ubuntu 26.04 packages are separate package-production
work.
