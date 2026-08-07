# Standard Thresholds

Threshold tables for all monitored metrics. See [common-reference.md](common-reference.md) for encoding, parsing, and verification protocols.

### Disk

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| C: drive free space | ≥30 GB | <30 GB | <15 GB |

### Container Images & Snapshots

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| Dangling images | <5 | ≥5 | ≥20 |
| containerd snapshots | <500 | ≥500 | ≥1000 |

### Container Health

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| Container restart attempts | 1–9 | — | ≥10 (crash-looping) |

### HCS & Termination

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| HCS terminate failures (unmatched) | 1–5 | 6–50 | >50 |
| Orphaned shims | — | any | stable PID across ≥2 snapshots |
| HCS Create-to-Start duration | <30s | 30–120s | >120s |
| HCS Shutdown-to-Terminate duration | — | <30s (graceful failed fast) | >60s (HCS struggled) |
| HCS error rate per operation type | <5% | 5–20% | >20% |
| HCS creates per 5-minute window | <20 | 20–50 | >50 (creation storm) |
| vmcompute working set (memory) | <150 MB | >500 MB | >1 GB |
| vmcompute handle count | — | >5000 | — |

### Memory

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| Available physical RAM | ≥2 GB | <2 GB | <500 MB |
| Pagefile (manual) size | ≥1024 MB | <1024 MB | — |
| Pagefile current usage | — | peak >80% of allocated | >90% of allocated |
| Commit charge vs limit | — | within 25% of limit | within 10% of limit |
| Single process working set | — | containerd/kubelet >1 GB | any process >4 GB |

### kube-proxy & Port Exhaustion

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| kube-proxy sync delay after restart | <5 min | 5–30 min | >30 min |
| Excluded port ranges count | — | >20 | overlaps NodePort 30000–32767 |
| Available ephemeral ports | >5000 | <5000 | <1000 or "Couldn't reserve" |
| Stale LB deletions per sync cycle | — | ongoing beyond startup | >50 per cycle |

### GPU

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| GPU temperature | <80°C | 80–90°C | >90°C |
| GPU memory usage | — | >95% | — |
| Power usage | — | at/exceeding cap | — |
| Uncorrectable ECC errors | 0 | — | >0 |
| Single-bit ECC errors (aggregate) | — | >100 | — |
| Retired GPU memory pages | 0 | >0 but below limit | approaching limit (~48) |

### CSI Proxy

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| CSI proxy working set | <50 MB | >200 MB | — |
| CSI proxy service state | RUNNING | — | STOPPED |

### Bootstrap (CSE)

| Metric | 🔵 INFO | 🟡 WARNING | 🔴 CRITICAL |
|--------|---------|------------|-------------|
| CSE exit code | 0 | — | any non-zero (see analyze-bootstrap.md for full CSE exit code table) |
| Download step duration | <2 min | >5 min | — |
