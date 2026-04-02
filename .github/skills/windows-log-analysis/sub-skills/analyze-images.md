# Analyze Images — Container Image Health

## Purpose

Detect dangling image accumulation, mutable tag usage, imagefs growth, containerd snapshot bloat, GC failures, and snapshot removal issues on Windows AKS nodes.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `<ts>-cri-containerd-images.txt` | UTF-16-LE with BOM | `crictl images` output |
| `<ts>-cri-containerd-imageFsInfo.txt` | UTF-16-LE with BOM | `crictl imagefsinfo` JSON |
| `<ts>-ctr-logs/` | Directory | containerd snapshot files (count files with "snapshot-info" in name) |
| `<ts>-disk-usage-all-drives.txt` | UTF-16-LE with BOM | `Get-PSDrive` output (for GC cross-reference) |
| `<ts>-containerd-toml.txt` | UTF-16-LE with BOM | containerd configuration (GC settings) |
| `containerd.err-*.log` | UTF-8 or UTF-16-LE | containerd stderr (snapshot removal errors) |

## Analysis Steps

### 1. Dangling (Untagged) Image Detection

Parse the latest snapshot's `*-cri-containerd-images.txt`.

**Format** (crictl images output):
```
IMAGE                        TAG      IMAGE ID       SIZE
mcr.microsoft.com/aks/...   v1.2.3   abc123...      4.35GB
mcr.microsoft.com/aks/...   <none>   def456...      4.35GB   ← dangling!
```

- Skip the header line (starts with "IMAGE")
- Each line has: `NAME  TAG  IMAGE_ID  SIZE` (4+ whitespace-separated columns)
- Images with tag `<none>` are **dangling**
- Count dangling images per repository name

See common-reference.md for severity thresholds.

**Critical Windows-specific knowledge**: Dangling images on Windows nodes are **NOT reclaimed by kubelet GC** because they have no tag reference. They accumulate indefinitely until manually pruned.

### 2. Mutable Tag Detection

Scan ALL image lines for tags in the mutable set: `latest`, `development`, `main`, `master`, `edge`, `nightly`, `snapshot`.

- Report which images use mutable tags — this is the root cause of dangling image accumulation. Each deployment pulls a new image, the old one loses its tag and becomes dangling.

### 3. imagefs Usage Trend

Parse ALL snapshots' `*-cri-containerd-imageFsInfo.txt` files.

**JSON format varies by containerd version** (see common-reference.md for details):
- containerd 1.x: `status.usedBytes.value`
- containerd 2.x: `status.imageFilesystems[].usedBytes.value`
- Regex fallback: `"usedBytes"\s*:\s*\{\s*"value"\s*:\s*"(\d+)"`

Report usage in GB. If multiple snapshots exist, compute trend.

**Note**: Windows reports sparse-file-inflated sizes (often 5–10× actual disk usage). This is expected.

### 4. containerd Snapshot Count

Count files containing "snapshot-info" in their name within each `<ts>-ctr-logs/` directory.

See common-reference.md for severity thresholds.

High snapshot counts indicate orphaned layers from dangling images.

### 5. ImageFsStats GC Failure Detection

Check if kubelet's image garbage collection is failing due to the known Windows ImageFsStats bug ([kubernetes#116020](https://github.com/kubernetes/kubernetes/issues/116020)).

**How it manifests**: Windows reports sparse-file-inflated sizes for container images (5–10× actual disk). Kubelet's image GC compares `imagefs.available` against threshold (default 85%), but the inflated sizes make kubelet think the filesystem is always over threshold — so GC either runs constantly (deleting nothing useful) or never triggers correctly.

**Detection**:
- Compare `crictl imagefsinfo` reported usage against actual C: drive used space from `*-disk-usage-all-drives.txt`
- If imagefsinfo reports >2× the actual disk usage → sparse file inflation confirmed
- If dangling images exist AND disk is low AND imagefsinfo shows inflated numbers → GC is failing to reclaim

### 6. containerd Snapshotter GC Configuration

Check containerd config (`*-containerd-toml.txt`) for GC-related settings:

- Search for `discard_unpacked_layers` — if missing or `false`, containerd retains unpacked layer data even after image deletion, wasting disk
- Search for `[plugins."io.containerd.gc.v1.scheduler"]` section — check `pause_threshold`, `deletion_threshold`, `schedule_delay`
- If no GC scheduler section exists, containerd uses defaults which may be too conservative for Windows nodes with limited disk

### 7. Snapshot Removal Failure Patterns

Check containerd error logs (if available) for snapshot cleanup failures:

- `"Access is denied"` during snapshot removal — Windows Defender or another process has a file lock on the snapshot directory
- `"The process cannot access the file because it is being used by another process"` — sharing violation during cleanup
- `"remove"` + `"snapshot"` + error — any snapshot removal failure

These failures prevent containerd from cleaning up old snapshots even when GC runs correctly.

### 8. Additional Checks

- Unusually large individual images (>10 GB)
- Same image pulled with many different digest-based tags (possible CI/CD misconfiguration)
- imagefs usage growing significantly between snapshots while dangling count is stable (suggests other storage consumers)
- Images from unexpected registries (not mcr.microsoft.com)

## Findings Format

```markdown
### Image Health Findings

<severity> **<LEVEL>** (<confidence> confidence): <description>
  - <detail line 1>
  - <detail line 2>
```

**Example**:
```markdown
🔴 **CRITICAL** (HIGH confidence): 25 dangling (untagged) images accumulating on node
  - 10x dangling: mcr.microsoft.com/aks/some-image
  - 8x dangling: mcr.microsoft.com/oss/some-other
  - These are NOT cleaned by kubelet GC on Windows

🟡 **WARNING** (HIGH confidence): Mutable image tag(s) in use — cause of image accumulation
  - mcr.microsoft.com/aks/some-image:development
  - Switch to immutable (digest or version) tags

🔵 **INFO** (HIGH confidence): containerd imagefs: 45.2 GB used (Windows sparse-file accounting)
  - Trend: +5.3 GB over observed period

🟡 **WARNING** (HIGH confidence): containerd snapshot count: 750
  - High snapshot counts indicate orphaned layers from dangling images
```

## Known Patterns

| Pattern | Severity | Confidence | Indicators | Remediation |
|---------|----------|------------|------------|-------------|
| ≥20 dangling images | 🔴 CRITICAL | HIGH | Unreclaimed images filling disk | `crictl rmi --prune`; switch to immutable tags |
| ≥5 dangling images | 🟡 WARNING | HIGH | Dangling accumulation starting | Monitor; plan for immutable tag migration |
| Mutable tags in use | 🟡 WARNING | HIGH | Tags: latest, development, main, master, edge, nightly, snapshot | Switch to immutable (digest or version) tags with `imagePullPolicy: IfNotPresent` |
| ≥1000 containerd snapshots | 🔴 CRITICAL | HIGH | Orphaned layers consuming disk | Prune images; investigate snapshot removal failures |
| ≥500 containerd snapshots | 🟡 WARNING | HIGH | Snapshot accumulation trending | Monitor; check for dangling images |
| imagefs >2× actual disk usage | 🟡 WARNING | MEDIUM | Sparse file inflation — GC threshold miscalculation | Known Windows issue (kubernetes#116020); GC thresholds unreliable |
| Dangling + disk pressure + inflated imagefs | 🔴 CRITICAL | HIGH | GC cannot reclaim (kubernetes#116020) | Manual prune required; GC is ineffective |
| `discard_unpacked_layers` missing/false | 🟡 WARNING | MEDIUM | Unpacked layers not cleaned on image delete | Set `discard_unpacked_layers = true` in containerd config |
| "Access is denied" on snapshot removal | 🟡 WARNING | MEDIUM | Defender file lock preventing cleanup | Add Defender path exclusions for `C:\ProgramData\containerd\` |
| Repeated snapshot removal failures + high count | 🔴 CRITICAL | HIGH | Snapshots cannot be cleaned — disk will fill | Resolve file locks; add Defender exclusions; manual cleanup |

## Cross-References

- **analyze-disk.md**: Dangling images are the most common cause of disk pressure on Windows nodes; correlate with C: drive free space
- **analyze-termination.md**: Snapshot removal "Access is denied" errors suggest Defender interaction — correlate with Defender findings. High snapshot count amplifies Defender scanning latency (root cause chain: mutable tags → dangling images → snapshot bloat → Defender scans more files → container stop slower → grace period exceeded → pods stuck Terminating)
- **analyze-containers.md**: Mutable tags cause image churn; if a failing container uses a mutable tag, the image may have changed
- **analyze-hcs.md**: HCS container lifecycle affects snapshot retention
