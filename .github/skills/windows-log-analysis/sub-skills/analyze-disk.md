# Analyze Disk — Disk Usage

## Purpose

Diagnose disk pressure on Windows AKS nodes by analyzing C: drive free space trends.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `<ts>-disk-usage-all-drives.txt` | UTF-16-LE with BOM (see common-reference.md) | `Get-PSDrive` filesystem output |

## Analysis Steps

### 1. C: Drive Free Space Trend

Read ALL snapshot files (`*-disk-usage-all-drives.txt`) to build a trend over time.

**Format**: Lines contain drive letter followed by used/free values:
```
C   45.23   54.77   FileSystem   C:\   ...
```
The line starting with `C` has: `C  <used_gb>  <free_gb>  FileSystem  C:\`

See common-reference.md for severity thresholds.

**Trend analysis**: If multiple snapshots exist, compute the delta in used space between first and last snapshot. Report the growth rate (GB/hour if calculable from snapshot timestamps) — this helps predict when disk will fill.

**Additional checks**:
- Extremely rapid disk usage growth between snapshots (suggests active leak)
- Non-C: drives with unexpectedly high usage

## Findings Format

```markdown
### Disk Findings

<severity> **<LEVEL>** (<confidence> confidence): <description>
  - <detail line 1>
  - <detail line 2>
```

**Example**:
```markdown
🔴 **CRITICAL** (HIGH confidence): C: drive has 12.3 GB free (87.7 GB used)
  - Snapshot 20260323-034156: Used=80.1 GB  Free=19.9 GB
  - Snapshot 20260323-073319: Used=87.7 GB  Free=12.3 GB
  - Trend: +7.6 GB used over observed period
```

## Known Patterns

| Pattern | Severity | Confidence | Indicators | Remediation |
|---------|----------|------------|------------|-------------|
| C: free < 15 GB | 🔴 CRITICAL | HIGH | Disk nearly full | Check dangling images (analyze-images), prune with `crictl rmi --prune` |
| C: free < 30 GB | 🟡 WARNING | HIGH | Disk pressure building | Monitor trend; check image accumulation |
| C: free ≥ 30 GB | 🔵 INFO | HIGH | Healthy disk state | No action needed |
| Rapid disk growth between snapshots | 🔴 CRITICAL | MEDIUM | Large delta in used space over short time | Active disk leak; identify source (logs, images, temp files) |

## Cross-References

- **analyze-images.md**: Most common cause of disk pressure on Windows nodes is dangling image accumulation
- **analyze-containers.md**: Disk pressure causes pod evictions — correlate with pod readiness
- **analyze-memory.md**: Pagefile analysis (including exhaustion checks) is handled by the memory sub-skill
- **analyze-termination.md**: Zombie HCS containers consume disk resources
