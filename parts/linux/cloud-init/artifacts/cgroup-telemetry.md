# Cgroup CPU telemetry

`cgroup-cpu-telemetry.timer` emits an
`AKS.Runtime.cpu_usage_telemetry_cgroupv2` observation every five minutes. CPU usage
and throttling counters are emitted in its `CPUUsage` object; they are
intentionally separate from the CPU PSI values emitted by
`cgroup-pressure-telemetry.sh`. The collector requires cgroup v2.

The monitored persistent services include containerd, kubelet, WALinuxAgent,
node diagnostics and telemetry agents, LocalDNS, and supported GPU services.
Each persistent service has the following cumulative counters:

| Counter | Unit | cgroup v2 source |
| --- | --- | --- |
| `usage_usec` | microseconds | `cpu.stat:usage_usec` |
| `user_usec` | microseconds | `cpu.stat:user_usec` |
| `system_usec` | microseconds | `cpu.stat:system_usec` |
| `nr_periods` | count | `cpu.stat:nr_periods` |
| `nr_throttled` | count | `cpu.stat:nr_throttled` |
| `throttled_usec` | microseconds | `cpu.stat:throttled_usec` |

Values are raw cumulative counters. Consumers calculate utilization and
throttling rates from deltas between observations. Counters reset when a service
cgroup is recreated or when the node reboots; a negative delta therefore marks a
reset and must be discarded rather than interpreted as a rate.

An unavailable service is emitted as `"Not Found"`. If a service cgroup exists
but a counter is absent or malformed, only that counter is emitted as
`"Not Found"` so the remaining counters in the observation remain usable.
The WALinuxAgent cgroup is resolved from systemd because distributions can
place the service in `azure.slice` instead of `system.slice`. The CPU and memory
collectors try `walinuxagent.service` first, then `waagent.service` if the first
lookup fails or returns an empty or root cgroup. The metric names remain
`walinuxagent_service_cpu_usage` and `walinuxagent_service_memory` for either
unit name.

LocalDNS is also resolved through its actual `ControlGroup`, so CPU, memory,
and pressure observations work with both legacy and nested slice placement.

For cgroup v2, the CPU and memory collectors discover top-level `.slice`
directories and report the raw `cpu.max` and `memory.max` contents as
`<slice>_max_cpu` and `<slice>_max_memory`, respectively. Missing limit files
are reported as `"Not Found"`; unlimited values remain `"max"` or the raw
`cpu.max` quota and period string.

# Cgroup telemetry size limits

The pressure, CPU, and memory collectors check the final message against 3,072 UTF-8
bytes and the serialized event against 6,144 bytes before publishing it for
WALinuxAgent. Samples that fit keep their original message. Oversized samples
are split into groups of at most four entries, preserving metric names,
values, `CgroupVersion`, and the original event metadata. Every part is checked
again; a sample that still exceeds either bound is rejected with an error before
any parts are published, rather than silently truncated.
The bounds work with jq 1.5: 3,072 and 6,144 bytes correspond exactly to
4,096 and 8,192 Base64 characters. Base64 is used only to measure size;
the emitted messages remain JSON strings.

Each split message includes `TelemetryChunk` with a shared `id`, zero-based
`index`, total `count`, and `encoding: "json"`. Consumers must read entries across
parts instead of assuming one event contains every service. To reconstruct a
sample, group by VM identity, TaskName and chunk id, verify every index is present,
and merge the `Pressure`, `CPUUsage`, or `Memory` objects for the corresponding
collector. Missing parts are incomplete data, not zeros.
Unsplit messages have no chunk metadata.

Files are published atomically with numeric names and a distinct part index.
Temporary files are staged in the parent directory on the same filesystem,
outside `events/`, so WALinuxAgent's queue cleanup cannot delete them before
publication.
Splitting increases event count without changing WALinuxAgent's collection
quotas; it prevents size truncation, not other ingestion failures. This
shared `cgroup-telemetry-events.sh` helper is installed in the VHD alongside
the collectors and can also publish other cgroup observation types.

# AKS-owned slice placement

Placement is independent of Node Memory Hardening, on Ubuntu and Azure
Linux/Mariner. The service names and their lifecycle dependencies are unchanged:

| Services | Slice | cgroup path |
| --- | --- | --- |
| `localdns.service`, `localdns-fallback.service` | `system-localdns.slice` | `/system.slice/system-localdns.slice` |
| `aks-log-collector.service` | `system-aks-log-collector.slice` | `/system.slice/system-aks.slice/system-aks-log.slice/system-aks-log-collector.slice` |

Systemd derives slice parentage from hyphenated names. `Requires=system.slice`
and `After=system.slice` alone do not place a slice under `system.slice`.
The AKS collector's original `aks.slice/aks-log.slice/aks-log-collector.slice`
hierarchy is retained below `system.slice`, with the existing CPU accounting,
`CPUShares=100`, `CPUWeight=1`, and `MemoryMax=256M` on the collector slice.
LocalDNS retains the CPU quota and memory limit provided by provisioning.

New VHDs install the nested AKS collector slice and reference the nested slices
in the service units, including the OSGuard imagecustomizer path. Scripted
provisioning additionally installs explicit service drop-ins for older VHDs.
Both `basePrep` and `nodePrep` reconcile placement, copying the effective legacy
slice definitions (including overrides) when present. Thus a PIS-cached node
does not need `basePrep` to run again or retain variables from the image bake.
The legacy `localdns.slice` configuration file remains installed for older
LocalDNS scripts; an installed unit file does not imply an active root cgroup.
The new LocalDNS supervisor also copies that configuration to the nested slice
before starting CoreDNS, covering new VHDs used with older provisioners.

An already-running legacy LocalDNS instance is restarted only in `nodePrep`
when LocalDNS is enabled and its actual cgroup is still at the root. This uses
the existing DNS cleanup and fallback handoff; it is not a manual cgroup move.
It briefly interrupts DNS during node provisioning, before kubelet is started.
No other service is restarted. An in-flight legacy AKS log collection is not
interrupted and moves on its next invocation; its old slices are left alone
on that provisioning pass. Otherwise `StopWhenUnneeded=yes` lets systemd retire
legacy slices only when their consumers no longer require them. Nothing is
masked, and unrelated consumers can keep a legacy slice active.
Root-slice validation may still observe an in-flight legacy collector or a
legacy slice required by another workload. A new VHD/reboot avoids the
in-flight migration case.

Older VHD telemetry scripts/supervisors that hardcode the old LocalDNS cgroup
path require a VHD refresh to receive the telemetry lookup changes. Scriptless
provisioning likewise requires a VHD containing these updated provisioning
helpers; it cannot retrofit them into an older image without script overrides.

## Unresolved Azure hierarchy

**This is a partial migration: `azure.slice` is intentionally unchanged, and
root-slice validation still fails on nodes where it is active.**
The pinned WALinuxAgent version is `2.16.0.2` in `parts/common/components.json`.
Its `CGroupConfigurator._Impl.initialize` accepts the agent only in
`/azure.slice/<agent>.service` or `/system.slice/<agent>.service`; a nested
`system-azure.slice` path disables cgroup initialization and resets CPU quota.
See [the accepted ControlGroup paths](https://github.com/Azure/WALinuxAgent/blob/v2.16.0.2/azurelinuxagent/ga/cgroupconfigurator.py#L151-L155)
and [the quota-reset finally block](https://github.com/Azure/WALinuxAgent/blob/v2.16.0.2/azurelinuxagent/ga/cgroupconfigurator.py#L199-L206).

The agent also launches extension install/enable commands as transient scopes
in `azure-vmextensions-<extension>.slice`, below
`azure.slice/azure-vmextensions.slice`, and hardcodes those names and paths.
See [extension slice naming](https://github.com/Azure/WALinuxAgent/blob/v2.16.0.2/azurelinuxagent/ga/cgroupapi.py#L75-L83)
and [`_SystemdCgroupApi.start_extension_command`](https://github.com/Azure/WALinuxAgent/blob/v2.16.0.2/azurelinuxagent/ga/cgroupapi.py#L352-L386).
The persistent agent service is `walinuxagent.service` on Ubuntu or
`waagent.service` on RPM-based images; the exact installed distro unit remains
authoritative. Additional agent operations use the
`azure-walinuxagent-logcollector.slice` and
`azure-walinuxagent-pkgsignaturevalidation` names
([constants](https://github.com/Azure/WALinuxAgent/blob/v2.16.0.2/azurelinuxagent/ga/cgroupconfigurator.py#L59-L70)).
Agent log collection is disabled by the VHD configuration (`Logs.Collect=n`),
but this does not disable extension scopes.
Extension-provided persistent services can also be advertised dynamically by
their `HandlerManifest.json`; their complete installed inventory cannot be
established from AgentBaker alone.

No supported parent-slice configuration option was found in the pinned
[configuration definitions](https://github.com/Azure/WALinuxAgent/blob/v2.16.0.2/azurelinuxagent/common/conf.py#L121-L239).
The inspected upstream default-branch implementation retains the same
restriction; no compatible newer version was established.
Safe Azure relocation needs upstream agent support for nested names and
resource enforcement, not just a service drop-in, an agent restart during its
own provisioning extension, disabling its limits, or masking the root slice.
No agent upgrade or upstream patch is included here.

# Collector unit activation

All three cgroup collector units order themselves with
`After=kubelet.service containerd.service`. Ordering only applies when the
units share a start transaction, so this affects the `OnBootSec=0min`
invocation alone: it stops the first observation from reporting `"Not Found"`
for services whose cgroup does not exist yet. The five-minute invocations are
unaffected, and no `Wants=` or `Requires=` is declared, so a collector still
runs when kubelet or containerd never start.

The CPU and pressure collectors additionally declare
`ConditionPathExists=/sys/fs/cgroup/cgroup.controllers`. Both require cgroup
v2 and exit non-zero otherwise, which would surface as a failed unit on a node
that boots cgroup v1 even though the VHD was built on v2. The condition makes
systemd skip the unit instead. The memory collector carries no such condition
because it supports cgroup v1 and emits
`AKS.Runtime.memory_telemetry_cgroupv1` there.

# Recurring service execution telemetry

`service-execution-telemetry.sh` emits one
`AKS.Runtime.systemd_service_execution` event after each execution of the
following timer-triggered services:

| Service | Schedule |
| --- | --- |
| `cgroup-memory-telemetry.service` | Every five minutes |
| `cgroup-pressure-telemetry.service` | Every five minutes |
| `aks-log-collector.service` | Every hour |

The allowlist is explicit; the collector rejects all other systemd units. Each
event contains:

| Field | Unit or meaning |
| --- | --- |
| `ServiceName` | systemd unit name |
| `ExecMainStartTimestamp` | systemd execution start timestamp |
| `ExecMainExitTimestamp` | systemd execution exit timestamp |
| `DurationUsec` | microseconds, calculated from monotonic timestamps |
| `Result` | systemd result such as `success`, `exit-code`, `timeout`, or `signal` |
| `ExecMainCode` | systemd main-process termination code |
| `ExecMainStatus` | process exit status or signal |
| `CPUUsageNSec` | nanoseconds of CPU used by the execution cgroup |
| `MemoryPeakAvailable` | whether systemd exposed a reliable peak-memory value |
| `MemoryPeakBytes` | peak bytes; omitted when unavailable |

`CPUAccounting` and `MemoryAccounting` are enabled on the monitored units.
`OnSuccess` and `OnFailure` start a separate
`service-execution-telemetry@.service` instance after the monitored unit has
finished. Keeping the collector outside the monitored cgroup prevents its CPU
and memory usage from contaminating the observed values. Values describe one
execution and reset when systemd recreates the service cgroup; consumers must
not calculate deltas between timer invocations.

Peak memory is capability-based. The collector emits `MemoryPeakBytes` only
when a numeric peak is available and does not emit a misleading zero otherwise.

Systemd retains the CPU accounting of an exited unit, but retains memory
accounting only from version 256. On older versions the peak is discarded with
the cgroup, so a unit measured after it exits reports `CPUUsageNSec` while
`MemoryPeak` reads empty. This is why Ubuntu 22.04 (systemd 249) and 24.04
(systemd 255) reported no peak while 26.04 always did.

Below systemd 256 the monitored units therefore snapshot `memory.peak` straight
from their own cgroup through
`ExecStopPost=-/opt/scripts/service-execution-telemetry.sh --snapshot-memory-peak %n`.
`ExecStopPost` runs synchronously inside the cgroup before systemd releases it,
and writes `/run/aks-service-telemetry/<unit>.peak`. The collector consumes and
deletes that file when systemd itself exposes no peak, so a stale value can
never be attributed to a later execution. Unlike the collector, this snapshot
does run inside the monitored cgroup and adds its own small footprint to the
reported peak; the VHD build removes it on systemd 256 and newer, where it is
both unnecessary and a source of bias.

The snapshot depends on `memory.peak`, which the kernel exposes only from
version 5.19. Ubuntu 22.04 ships a 5.15 kernel, so the file is absent and the
snapshot returns without writing anything: 22.04 reports no `MemoryPeakBytes`
regardless of the systemd version. Every other supported image runs a 6.x
kernel and is unaffected. cgroup v2 exposes no alternative peak counter below
5.19, so this is a limitation of the image rather than a gap in the collector.

The completion handlers run after both successful and failed executions. A
telemetry failure belongs to the separate collector unit and therefore cannot
change the result of the monitored workload.

Ubuntu 20.04 uses systemd 245, before `OnSuccess` was introduced. During the VHD
build, `packer_source.sh` replaces the completion handlers on systemd versions
older than 249 with an asynchronous `ExecStopPost` that starts the same separate
collector unit. Peak memory relies on the `ExecStopPost` snapshot described
above on these older versions, and therefore on a 5.19 or newer kernel; the
small legacy trigger overhead can affect `CPUUsageNSec` and is an explicit
limitation of the compatibility path.

Starting with systemd 258, cgroup v2 accounting is always enabled and the
`CPUAccounting` and `MemoryAccounting` unit directives are obsolete. The VHD
build removes those directives on 258 and newer while retaining the modern
completion handlers.
