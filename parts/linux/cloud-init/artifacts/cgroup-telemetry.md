# Cgroup CPU telemetry

`cgroup-cpu-telemetry.timer` emits one
`AKS.Runtime.cpu_usage_telemetry_cgroupv2` event every five minutes. CPU usage
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
place the service in `azure.slice` instead of `system.slice`.

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
