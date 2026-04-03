# Analyze Memory — Physical Memory, Pagefile & OOM Sub-Skill

## Purpose

Detect physical memory pressure, pagefile health issues, process memory consumption anomalies, and OOM conditions on Windows AKS nodes.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `available-memory.txt` | UTF-16-LE with BOM | Available physical RAM at collection time |
| `processes.txt` | UTF-16-LE with BOM | `Get-Process` snapshot — per-process memory usage |
| `<ts>_pagefile.txt` | UTF-16-LE with BOM | Pagefile configuration and usage (size, auto-managed, peak) |
| `<ts>_services.csv` | UTF-16-LE with BOM, CSV with embedded newlines | Event ID 2004 = low memory condition |
| `<ts>-aks-info.log` | UTF-16-LE with BOM | Node YAML with allocatable memory |

## Analysis Steps

### 1. Available Physical Memory (`available-memory.txt`)

Parse available memory value (typically in MB or bytes).

- Calculate percentage of total if node capacity is known from `*-aks-info.log`
- 🔴 CRITICAL: Available memory < 500 MB (severe memory pressure)
- 🟡 WARNING: Available memory < 2 GB
- 🔵 INFO: Available memory ≥ 2 GB

### 2. Pagefile Health (`*_pagefile.txt`)

Parse pagefile configuration:
- **AutomaticManagedPagefile**: `True` or `False`
- **CurrentUsage** vs **AllocatedBaseSize**: how full is the pagefile
- **PeakUsage**: high water mark

Apply threshold from common-reference.md:
- 🟡 WARNING: Manual pagefile (AutomaticManagedPagefile=False) with AllocatedBaseSize < 1024 MB
- 🔵 INFO: Auto-managed pagefile (system handles sizing)

Check utilization:
- 🔴 CRITICAL: CurrentUsage > 90% of AllocatedBaseSize (pagefile nearly full)
- 🟡 WARNING: PeakUsage > 80% of AllocatedBaseSize (has been under pressure)

### 3. OOM Events (`*_services.csv`)

Parse services.csv (handle `#TYPE` header, embedded newlines — use CSV parser).

Search for Event ID `2004` (low memory condition):
- Extract timestamps and message details
- Count occurrences

- 🔴 CRITICAL: Multiple Event ID 2004 entries (recurring OOM conditions)
- 🟡 WARNING: Single Event ID 2004 entry
- Also check for Event ID `2003` from Resource Exhaustion Detector

### 4. Process Memory Consumption (`processes.txt`)

Parse Get-Process output to identify top memory consumers:
- Sort by WorkingSet (WS) or PM (PagedMemory) descending
- Identify the top 10 memory consumers
- Flag any single process using > 2 GB working set

Known high-memory processes on AKS nodes:
- `containerd` — if > 1 GB, possible memory leak
- `kubelet` — if > 1 GB, unusual
- `svchost` — normal to have multiple instances
- User workload processes — report names and memory

- 🔴 CRITICAL: Single process using > 4 GB (likely leaking)
- 🟡 WARNING: containerd or kubelet using > 1 GB
- 🔵 INFO: Top memory consumers listed for context

### 5. Commit Charge vs Physical+Pagefile

If both available memory and pagefile data are present:
- Calculate total commit limit = physical RAM + pagefile size
- If commit charge approaches this limit, system will refuse new allocations

- 🔴 CRITICAL: Commit charge within 10% of limit
- 🟡 WARNING: Commit charge within 25% of limit

### 6. Node Allocatable Memory (`*-aks-info.log`)

Parse node YAML for `allocatable.memory` and `capacity.memory`:
- Compare with actual available memory
- Large gap between allocatable and available suggests over-commitment

- 🔵 INFO: Report allocatable vs available for context

## Findings Format

```markdown
### Memory Findings

🔴 **CRITICAL** (HIGH confidence): Available physical memory critically low: 312 MB
  - Node capacity: 16 GB, allocatable: 14.5 GB
  - Only 2% of physical memory available

🟡 **WARNING** (HIGH confidence): Pagefile manually configured at 512 MB (< 1024 MB threshold)
  - AutomaticManagedPagefile=False
  - Peak usage: 480 MB (93% of allocated)

🟡 **WARNING** (MEDIUM confidence): containerd process using 1.3 GB working set
  - Possible memory leak — typical usage is < 500 MB
```

## Known Patterns

| Pattern | Severity | Confidence | Meaning |
|---------|----------|------------|---------|
| Available memory < 500 MB | 🔴 CRITICAL | HIGH | Severe memory pressure, OOM kills imminent |
| Multiple Event ID 2004 | 🔴 CRITICAL | HIGH | Recurring low memory conditions |
| Commit charge within 10% of limit | 🔴 CRITICAL | MEDIUM | System cannot allocate new memory |
| Pagefile usage > 90% | 🔴 CRITICAL | HIGH | Pagefile nearly exhausted |
| Single process > 4 GB WS | 🔴 CRITICAL | MEDIUM | Likely memory leak |
| Available memory < 2 GB | 🟡 WARNING | MEDIUM | Memory pressure building |
| Manual pagefile < 1024 MB | 🟡 WARNING | HIGH | Pagefile too small per threshold |
| containerd/kubelet > 1 GB WS | 🟡 WARNING | MEDIUM | Possible memory leak in system component |
| Pagefile peak > 80% | 🟡 WARNING | LOW | Has been under memory pressure |

## Cross-References

- **analyze-kubelet.md**: MemoryPressure node condition confirms memory pressure findings
- **analyze-containers.md**: OOM kills cause container restarts and crash-loops
- **analyze-crashes.md**: OOM conditions may trigger application crashes (containerd, kubelet)
- **analyze-services.md**: Event ID 2004 in services.csv correlates with service crashes; containerd reinstall events may indicate OOM-triggered restarts
- **analyze-disk.md**: Pagefile resides on disk; disk pressure can prevent pagefile growth when auto-managed
