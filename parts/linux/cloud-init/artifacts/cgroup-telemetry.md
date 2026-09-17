# Cgroup CPU telemetry

`cgroup-cpu-telemetry.timer` emits one
`AKS.Runtime.cpu_usage_telemetry_cgroupv2` event every five minutes. CPU usage
and throttling counters are emitted in its `CPUUsage` object; they are
intentionally separate from the CPU PSI values emitted by
`cgroup-pressure-telemetry.sh`. The collector requires cgroup v2.

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
when the runtime `MemoryPeak` property is present and numeric. It does not infer
support from the distribution or systemd version and does not emit a
misleading zero when the property is unavailable.

The completion handlers run after both successful and failed executions. A
telemetry failure belongs to the separate collector unit and therefore cannot
change the result of the monitored workload.

Ubuntu 20.04 uses systemd 245, before `OnSuccess` was introduced. During the VHD
build, `packer_source.sh` replaces the completion handlers on systemd versions
older than 249 with an asynchronous `ExecStopPost` that starts the same separate
collector unit. Peak memory is unavailable on these older versions; the small
legacy trigger overhead can affect `CPUUsageNSec` and is an explicit limitation
of the compatibility path.

Starting with systemd 258, cgroup v2 accounting is always enabled and the
`CPUAccounting` and `MemoryAccounting` unit directives are obsolete. The VHD
build removes those directives on 258 and newer while retaining the modern
completion handlers.
