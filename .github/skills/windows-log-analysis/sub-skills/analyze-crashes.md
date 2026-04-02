# Analyze Crashes — Application & Kernel Crashes Sub-Skill

## Purpose

Detect application crashes (kubelet, containerd, shim, HCS), kernel crashes (BSOD), and Windows Error Reporting (WER) data on Windows AKS nodes.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `WER-<ts>.zip` | Binary (zip) | Windows Error Reporting crash reports |
| `Minidump-<ts>.zip` | Binary (zip) | Kernel minidump files from BSODs |
| `MemoryDump-<ts>.zip` | Binary (zip) | Full kernel memory dumps |
| `<ts>_services.csv` | UTF-16-LE with BOM, CSV with embedded newlines | Event ID 6008 = unexpected shutdown, service crashes |
| `windowsnodereset.log` | UTF-8 | Node reset/reboot log |
| `<ts>_hyper-v-compute-operational.csv` | UTF-16-LE with BOM, CSV with embedded newlines | HCS process crash entries |

## Analysis Steps

### 1. Unexpected Shutdowns (`*_services.csv`)

Parse services.csv (handle `#TYPE` header, embedded newlines — use CSV parser).

Search for Event ID `6008`:
- Indicates previous unexpected shutdown (crash/BSOD)
- Extract timestamps — these are the crash times
- Multiple 6008 events indicate recurring crashes

- 🔴 CRITICAL: Any Event ID 6008 present (node crashed unexpectedly)
- Report count and timestamps of all 6008 events

### 2. Service Crash Events (`*_services.csv`)

Search for service crash/termination patterns:
- Messages containing `"terminated unexpectedly"` — application crash
- Messages containing `"hung on starting"` — service startup hang
- Focus on critical services: `containerd`, `kubelet`, `kubeproxy`

Extract the faulting service name and timestamp.

- 🔴 CRITICAL: containerd or kubelet terminated unexpectedly
- 🟡 WARNING: Other service crashes

### 3. WER Report Analysis (`WER-*.zip`)

If WER zip files are present:
- List contents to identify crash report directories
- Each subdirectory typically contains `Report.wer` with crash details
- Look for `AppName`, `AppPath`, `ExceptionCode`, `FaultModule`

Key binaries to watch for:
- `containerd.exe` — container runtime crash
- `kubelet.exe` — kubelet crash
- `containerd-shim-runhcs-v1.exe` — shim crash (panic or nil pointer)
- `vmcompute.exe` — HCS crash
- `vmwp.exe` — VM worker process crash

- 🔴 CRITICAL: WER reports for containerd, kubelet, or vmcompute
- 🟡 WARNING: WER reports for shim processes
- 🔵 INFO: WER reports for other processes (report for context)

Extract exception codes:
- `0xC0000005` — Access violation (nil pointer / bad memory access)
- `0xC00000FD` — Stack overflow
- `0x80000003` — Breakpoint (panic / assertion)
- `0xE0434352` — .NET CLR exception

### 4. Kernel Crash Dumps (`Minidump-*.zip`, `MemoryDump-*.zip`)

If minidump or memory dump zips are present:
- 🔴 CRITICAL: Kernel crash dump exists — node experienced a BSOD
- List the zip contents to report dump file names and sizes
- Dump filenames often encode the date (e.g., `MEMORY.DMP`, `Mini032326-01.dmp`)
- Full analysis requires WinDbg — flag for human review

### 5. Node Reset Log (`windowsnodereset.log`)

If present, parse for:
- Reboot reasons and timestamps
- Whether reboot was planned (Windows Update) or unplanned (crash recovery)
- `"Windows Update"` or `"wuauserv"` — update-triggered reboot
- `"unexpected"` or `"watchdog"` — crash/hang recovery

- 🟡 WARNING: Windows Update caused unexpected reboot
- 🔵 INFO: Planned reboot recorded

### 6. HCS Process Crashes (`*_hyper-v-compute-operational.csv`)

Parse Hyper-V Compute event log for:
- Error-level events indicating HCS process crashes
- `vmcompute` service failures
- Container creation failures due to HCS bugs

- 🔴 CRITICAL: vmcompute service crash events
- 🟡 WARNING: Repeated container creation failures suggesting HCS instability

## Findings Format

```markdown
### Crash Findings

🔴 **CRITICAL** (HIGH confidence): 2 unexpected shutdown(s) detected (Event ID 6008)
  - 2026-03-22T14:15:00Z — unexpected shutdown
  - 2026-03-23T03:30:00Z — unexpected shutdown
  - Node crashed twice in 13 hours

🔴 **CRITICAL** (HIGH confidence): WER report for containerd.exe
  - ExceptionCode: 0xC0000005 (access violation)
  - FaultModule: containerd.exe
  - Timestamp: 2026-03-23T03:29:45Z

🔴 **CRITICAL** (HIGH confidence): Kernel minidump present — BSOD occurred
  - Minidump-20260323.zip contains Mini032326-01.dmp (256 KB)
  - Full analysis requires WinDbg — escalate to Windows platform team
```

## Known Patterns

| Pattern | Severity | Confidence | Meaning |
|---------|----------|------------|---------|
| Event ID 6008 (unexpected shutdown) | 🔴 CRITICAL | HIGH | Node crashed (BSOD or power loss) |
| containerd terminated unexpectedly | 🔴 CRITICAL | HIGH | Container runtime crash — all containers affected |
| kubelet terminated unexpectedly | 🔴 CRITICAL | HIGH | Kubelet crash — node goes NotReady |
| Kernel minidump/memory dump present | 🔴 CRITICAL | HIGH | BSOD occurred — needs WinDbg analysis |
| WER for containerd — 0xC0000005 | 🔴 CRITICAL | HIGH | containerd nil pointer / memory corruption |
| WER for shim — 0x80000003 | 🟡 WARNING | HIGH | Shim panic — single container affected |
| vmcompute service crash | 🔴 CRITICAL | MEDIUM | HCS crash — affects all Hyper-V containers |
| Windows Update reboot | 🟡 WARNING | HIGH | Unplanned reboot from Windows Update |
| WER for shim — 0xC00000FD | 🟡 WARNING | MEDIUM | Shim stack overflow — likely deep recursion bug |
| Multiple 6008 in short period | 🔴 CRITICAL | HIGH | Recurring crash — likely hardware or driver issue |

## Cross-References

- **analyze-memory.md**: OOM conditions (Event ID 2004) often precede application crashes
- **analyze-termination.md**: containerd crash causes orphaned HCS containers and zombie pods
- **analyze-kubelet.md**: Kubelet crash causes NotReady transition and lease renewal failures
- **analyze-services.md**: Service crash events in services.csv provide timeline context
- **analyze-containers.md**: Container restarts spike after runtime crashes
