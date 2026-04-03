# Analyze CSI — CSI Proxy & Volume Mount Failures

## Purpose

Detect CSI proxy service failures, SMB mount issues (Azure Files), Azure Disk attach/detach problems, and volume mount timeouts on Windows AKS nodes. CSI proxy (`csi-proxy.exe`) is the privileged host process that exposes gRPC APIs over named pipes (`\\.\pipe\csi-proxy-*`) to enable CSI node plugins (running as unprivileged pods) to perform storage operations on Windows.

## Input Files

| File Pattern | Encoding | Contents                                                               |
|-------------|----------|------------------------------------------------------------------------|
| `csi-proxy.log` | UTF-8 | CSI proxy operational log — gRPC calls, mount/unmount, disk operations |
| `csi-proxy.err.log` | UTF-8 | CSI proxy error/stderr output — panics, fatal errors                   |
| `kubelet.log` | UTF-8 | Kubelet logs — volume mount/unmount errors, CSI driver timeouts        |
| `processes.txt` | UTF-16-LE with BOM | `Get-Process` snapshot — csi-proxy process health                      |
| `scqueryex.txt` | UTF-16-LE with BOM | Service states — csi-proxy service status                              |

**Process ALL snapshots** — cross-snapshot comparison detects CSI proxy restarts, persistent mount failures, and resource leaks.

## Analysis Steps

### 1. CSI Proxy Service Health (`scqueryex.txt`, `processes.txt`)

**Check service status (`scqueryex.txt`)**:
- Search for `csiproxy` and `csi-proxy` (service name)
- 🔴 CRITICAL: Service is STOPPED or not found — all CSI volume operations will fail, pods requiring volumes will be stuck in `ContainerCreating`
- 🔵 INFO: Service is RUNNING — normal

**Check process health (`processes.txt`)**:
- Find `csi-proxy` process entry. Note working set (memory) and handle count.
- 🟡 WARNING: Working set > 200 MB — possible memory leak (normal is ~20-50 MB)
- Compare across snapshots: increasing memory = confirmed leak
- No `csi-proxy` process but service shows RUNNING = service just crashed and hasn't restarted yet

### 2. CSI Proxy Error Log (`csi-proxy.err.log`)

Parse the error log for panics and fatal errors:

| Error Pattern | Meaning | Severity |
|--------------|---------|----------|
| `panic:` | CSI proxy crashed with unhandled panic | 🔴 CRITICAL |
| `fatal` | Fatal error — process will exit | 🔴 CRITICAL |
| `Failed to listen on` / `pipe` | Named pipe creation failed — another instance running or permission issue | 🔴 CRITICAL |
| `connection error` / `transport is closing` | gRPC connection dropped between CSI driver pod and proxy | 🟡 WARNING |

- 🔴 CRITICAL: Any panic or fatal error — CSI proxy crashed, all volume operations fail until service restarts
- 🟡 WARNING: Connection errors — transient issues, but repeated occurrences indicate instability

### 3. SMB Mount Failures — Azure Files (`csi-proxy.log`, `kubelet.log`)

Search for SMB-related errors in CSI proxy log and kubelet log:

| Error Pattern | Meaning | Severity |
|--------------|---------|----------|
| `NewSmbGlobalMapping failed` | SMB global mapping creation failed — credential or connectivity issue | 🔴 CRITICAL |
| `Multiple connections to a server or shared resource by the same user` | Stale SMB global mapping exists with different credentials | 🔴 CRITICAL |
| `The network path was not found` (0x80070035) | SMB server unreachable — DNS or firewall issue | 🔴 CRITICAL |
| `Access is denied` (0x80070005) | Storage account key/SAS token invalid or expired | 🔴 CRITICAL |
| `The specified network password is not correct` | Credential rotation happened but stale mapping persists | 🔴 CRITICAL |
| `smb mapping failed` | Generic SMB mapping failure in kubelet | 🟡 WARNING |
| `Remove-SmbGlobalMapping` | Mapping removal attempted (may be remediation in progress) | 🔵 INFO |

**Stale global mapping detection**:
- The most common Azure Files issue on Windows. After credential rotation or PV recreation, old SMB global mappings persist and block new mounts with different credentials.
- Look for pattern: `Multiple connections` followed by repeated `NewSmbGlobalMapping failed`
- 🔴 CRITICAL: Stale mapping pattern detected — requires `Remove-SmbGlobalMapping` on the node or node reboot

**Mount timeout detection (`kubelet.log`)**:
- Search for `MountVolume.MountDevice failed` or `MountVolume.SetUp failed` with `smb` or `file.csi.azure.com`
- Search for `timed out waiting for the condition` near volume mount context
- 🔴 CRITICAL: Repeated mount failures for the same PVC — pod will stay in ContainerCreating indefinitely

### 4. Azure Disk CSI Failures (`csi-proxy.log`, `kubelet.log`)

Search for disk-related errors:

| Error Pattern | Meaning | Severity |
|--------------|---------|----------|
| `AttachVolume.Attach failed` / `timed out waiting for external-attacher` | Azure Disk attach timeout — ARM API or disk lock issue | 🔴 CRITICAL |
| `failed to get disk number` | CSI proxy cannot find the attached disk on the node — SCSI rescan needed | 🔴 CRITICAL |
| `disk is already attached to a different node` | Stale VolumeAttachment — disk didn't detach from previous node | 🔴 CRITICAL |
| `open \\.\pipe\csi-proxy-filesystem-v1beta1: The system cannot find the file specified` | CSI proxy not running or wrong version — named pipe doesn't exist | 🔴 CRITICAL |
| `open \\.\pipe\csi-proxy-disk-v1` | Disk API pipe missing — CSI proxy version incompatible | 🔴 CRITICAL |
| `FormatVolume failed` / `format and mount failed` | Disk format failure — disk may be corrupted or locked | 🟡 WARNING |
| `RescanDisk` | Disk rescan in progress (normal during attach) | 🔵 INFO |

**Named pipe missing**:
- CSI driver pods connect to CSI proxy via named pipes (`\\.\pipe\csi-proxy-*-v1`)
- If pipes don't exist, CSI proxy is either not running or an incompatible version
- Cross-reference with Step 1 (service health)

### 5. CSI Proxy Crash/Restart Detection

**Detect restarts across snapshots**:
- Compare CSI proxy PID in `processes.txt` across snapshots
- Different PID = process restarted (service recovery kicked in)
- 🟡 WARNING: CSI proxy PID changed between snapshots — at least one restart occurred

**Detect restart from logs**:
- Look for CSI proxy startup messages (e.g., `Starting CSI Proxy`, `Listening on`) appearing mid-log (not just at the beginning)
- Multiple startup messages = multiple restarts
- 🔴 CRITICAL: Multiple restarts detected — CSI proxy is unstable

### 6. Volume Mount Timeout Correlation (`kubelet.log`)

Search kubelet logs for volume-related errors and correlate with CSI proxy health:

- `WaitForAttach` / `WaitForVolumeToAttach` — kubelet waiting for disk attachment
- `FailedMount` event — mount operation failed
- `UnmountVolume.TearDown failed` — unmount failure, can block pod termination and new mounts

**Correlation**:
- If mount failures coincide with CSI proxy restart timestamps → CSI proxy crash caused mount failures
- If mount failures are all SMB → Azure Files credential or network issue
- If mount failures are all disk → Azure Disk attach/detach issue
- If mount failures span both types → CSI proxy service-level problem

## Findings Format

```markdown
### CSI Proxy & Volume Mount Findings

🔴 **CRITICAL** (HIGH confidence): CSI proxy service (csi-proxy) is STOPPED
  - All CSI volume operations will fail
  - Pods requiring PVCs will be stuck in ContainerCreating

🔴 **CRITICAL** (HIGH confidence): Stale SMB global mapping detected
  - Error: "Multiple connections to a server or shared resource by the same user"
  - 15 repeated NewSmbGlobalMapping failures for //storageaccount.file.core.windows.net/share
  - Remediation: Remove-SmbGlobalMapping on the node or reboot

🟡 **WARNING** (MEDIUM confidence): CSI proxy restarted between snapshots
  - Snapshot 1 PID: 4520, Snapshot 2 PID: 7832
  - csi-proxy.err.log shows panic at 03:42:15 UTC

🔵 **INFO** (HIGH confidence): Azure Disk CSI operations normal
  - 3 successful disk attach/format/mount sequences in log
  - No disk-related errors detected
```

## Known Patterns

| Pattern | Indicators | Severity | Root Cause |
|---------|-----------|----------|------------|
| Stale SMB global mapping | "Multiple connections to a server" + repeated NewSmbGlobalMapping failures | 🔴 CRITICAL | Credential rotation or PV recreation left old mapping. Remove-SmbGlobalMapping or reboot required. (csi-driver-smb#219) |
| CSI proxy not running | Service STOPPED + "pipe not found" errors in CSI driver pods | 🔴 CRITICAL | csi-proxy.exe crashed or was never started. Check csi-proxy.err.log for panic. |
| Named pipe version mismatch | "csi-proxy-filesystem-v1beta1: file not found" | 🔴 CRITICAL | CSI driver expects v1beta1 API but csi-proxy only serves v1. Version skew between node image and CSI driver. (Azure/AKS#2693) |
| Azure Disk ghost attach | "disk is already attached to a different node" + attach timeout | 🔴 CRITICAL | Stale VolumeAttachment from previous node. Disk didn't detach during node drain/upgrade. |
| SMB mount blue screen | Repeated mount failures → node becomes unresponsive | 🔴 CRITICAL | Rare: SMB mount operations can trigger BSOD on certain Windows builds. (csi-driver-smb#211) |
| Storage account key expired | "Access is denied" / "network password is not correct" on all SMB mounts | 🔴 CRITICAL | Storage account key rotated but Kubernetes secret not updated. |
| CSI proxy memory leak | Working set growing across snapshots, >200 MB | 🟡 WARNING | Possible leak from unfinished gRPC calls. Monitor trend. |
| Globalmount path missing | "globalmount does not exist" in kubelet | 🟡 WARNING | Race condition: NodePublishVolume before NodeStageVolume completes. Transient, usually retries succeed. (csi-driver-smb#737) |
| Disk format failure | "FormatVolume failed" after successful attach | 🟡 WARNING | Disk may have existing filesystem or be locked by another process. |

## Cross-References

- **→ analyze-kubelet.md**: Kubelet logs contain the pod-level view of mount failures. CSI proxy errors are the underlying cause; kubelet errors are the symptom.
- **→ analyze-services.md**: CSI proxy service crashes will appear in `*_services.csv`. Correlate crash timestamps with mount failures found here.
- **→ analyze-containers.md**: Pods stuck in ContainerCreating due to volume mount failures will show as not-Ready in crictl pods output.
- **→ analyze-disk.md**: Disk space pressure can cause Azure Disk format failures. Low free space on C: drive affects CSI operations.
- **→ analyze-hns.md**: SMB mount failures with "network path not found" may be caused by DNS resolution failures or NSG rules blocking port 445.
- **→ common-reference.md**: For encoding details (UTF-16-LE with BOM for processes.txt, scqueryex.txt) and severity/confidence level definitions.
