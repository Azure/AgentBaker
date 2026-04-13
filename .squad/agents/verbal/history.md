# Verbal — Session History

## Initial Context (seeded)

### AgentBaker Reliability & Observability Architecture

#### Error Code System
- ~145 named `ERR_*` codes defined in `cse_helpers.sh` (lines 2-145+)
- Organized by category: system (1-9), docker (20-29), k8s (30-39), CNI (41), packages (42-43), ORAS (45), systemd (48-49), connectivity (50-53), kata (60+)
- Some deprecated codes kept as comments (e.g., `ERR_SYSTEMCTL_ENABLE_FAIL=3`)
- Exit codes propagate through `cse_main.sh` via `|| exit $ERR_*` pattern

#### Event Logging Infrastructure
- `logs_to_events` (cse_helpers.sh:732-758) writes per-function JSON events
- Events stored in `/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/`
- Files named by epoch-ms, containing `Timestamp` (start) and `OperationId` (end)
- `EVENTS_LOGGING_DIR` defined at cse_helpers.sh:203

#### Service Health Logging
- Pattern: `systemctl status $service --no-pager -l > /var/log/azure/$service-status.log || true`
- Applied to containerd, kubelet, and other critical services (cse_helpers.sh:618-672)
- Captures service status even when start/restart fails — critical for diagnostics
- `journalctl -u $service` available for deeper investigation

#### Retry Infrastructure
- `retrycmd_if_failure` uses `timeout <val> "$@"` — commands must be external executables
- Shell builtins/keywords need `bash -c` wrapping
- Configurable retry count, wait time, and timeout per invocation

#### Node Problem Detector
- NPD installed as VM extension, may arrive before/after/during CSE
- `check_fs_corruption.sh` checks `journalctl -u containerd --since "5 min ago"` for "structure needs cleaning"
- NPD condition message: "Found 'structure needs cleaning' in containerd journal."
- `systemctl restart node-problem-detector || true` — tolerant of NPD absence (cse_config.sh:973)

#### Windows Diagnostics
- `collect-windows-logs.ps1` — comprehensive log collector producing UTF-16LE files
- Captures: kubectl output (nodes, pods), Hyper-V compute logs, service states, network info
- Timestamp format: `yyyyMMdd-HHmmss` (24h), CSV exports via `Export-CSV` (UTF-16LE default)
- `networkhealth.ps1` — network troubleshooting with replay/save capability
- `analyze-windows-logs.py` — Python-based log analysis tool
- Debug helpers in `staging/cse/windows/debug/` (VFP, HNS modules, packet capture)

#### Key Log Locations
- Linux: `/var/log/azure/` — service status logs, CSE events, extension logs
- Linux: `/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/` — CSE timing events
- Windows: `c:\k\debug\` — base directory for diagnostic output

---
*No sessions yet.*
