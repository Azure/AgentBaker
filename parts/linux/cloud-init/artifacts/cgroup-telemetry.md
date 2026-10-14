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
