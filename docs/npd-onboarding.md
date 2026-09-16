# NPD in Ubuntu VHDs and generic live patching

This work builds on generic snapshot PR #8952. The RP producer is
[aks-rp PR 17168496](https://dev.azure.com/msazure/CloudNativeCompute/_git/aks-rp/pullrequest/17168496).

## Lifecycle

- `components.json` independently pins `node-problem-detector-kubernetes` and
  `node-problem-detector-aks-config` from PMC.
- The VHD installs both packages, holds them against unattended upgrades, and
  caches the config deb for recovery. The config package supplies the monitors,
  scripts, `/opt/bin/node-problem-detector-startup.sh`, and systemd unit.
- NPD is disabled for capture. `/etc/node-problem-detector.d/skip_vhd_npd` tells
  the VM extension to leave it alone, matching the exporter/IG ownership pattern.
- CSE starts the baked service in `nodePrep`, after kubelet configuration and GPU
  setup. This also runs on PIS-cached images. No public-settings file is generated;
  monitor defaults and hardware selection come from the config package.
- Other OSes remain extension-managed. Ubuntu 26.04 has the generic handler but
  empty package pins until its config package is published. Current binary/config
  pins exist for Ubuntu 20.04, 22.04 and 24.04, amd64 and arm64.

## Live configuration

The existing snapshot timer dispatches `npdConfig` with the RP payload:

```json
{"ubuntuPackageVersions":{"22.04":"1.0.0-ubuntu22.04u1","24.04":"1.0.0-ubuntu24.04u1"}}
```

The node selects its actual Ubuntu release and compares the target against dpkg's
installed config version before accessing repositories. No target, an equal/newer
installed version, or an extension-managed node is successful no-action.

For an update, the handler downloads and checks the desired deb before installing
it. A durable pending marker records the prior cached version before mutation.
It installs the held package, cleans obsolete package-owned config files, reloads
systemd, restarts NPD, and checks the local health endpoint. A failure reinstalls
the cached prior deb to restore both dpkg and file state. Interrupted transactions
are recovered on the next invocation. `/healthz` establishes process startup,
not successful reporting of every monitor or node condition.

The generic updater owns successful-goal checkpoints and node status. NPD uses
the shared event transport with task prefix `AKS.LivePatching.npdConfig`.
Direct PMC servicing requires egress; RP currently excludes network-isolated
clusters from NPD publication. Their baked baseline still starts without a pull.

## Validation

ShellSpec covers package selection, ownership, CSE activation, version/no-action
logic, dispatch isolation, failed downloads, activation failure and transaction
recovery. VHD content checks require the marker, executables, monitors, cached deb,
disabled service and generic handler. The E2E validator checks baked service startup.

`spec/parts/linux/cloud-init/artifacts/npd-package-integration.sh` runs only in a
disposable Ubuntu container. It uses the published deb and real apt/dpkg, creates
a local test revision, and tests held upgrade, removed-conffile cleanup, marker
preservation and downgrade recovery. Only service restart is stubbed.

Before release, validate a PR-built VHD on real CPU/GPU nodes, PIS provisioning,
NPD condition/event reporting, and the actual LPC hash/status flow. A matching
generic hash without an explicit `npdConfig` result on an older node is not
evidence that NPD was applied. New-handler hotfixes onto already-converged nodes
need goal invalidation/reissue; this initial migration targets fresh VHD nodes.

The current PMC config pin is a usable integration baseline. Refreshing its source
content and supplying Ubuntu 26.04 packages are separate package-production work.
