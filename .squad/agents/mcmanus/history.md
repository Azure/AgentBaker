# McManus — Session History

## Initial Context (seeded)

### AgentBaker Performance Architecture
- **CSE timing system**: `logs_to_events` (cse_helpers.sh:732) writes per-task JSON with Timestamp=start, OperationId=end to `/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/` as epoch-ms filename. This is the primary measurement infrastructure.
- **CSE entry flow**: `cse_start.sh` → `cse_main.sh` → individual provisioning functions, each wrapped in `logs_to_events`.
- **Background task**: `aptmarkWALinuxAgent` runs with `&` early in CSE (cse_main.sh:62), holds dpkg lock.
- **TLS bootstrap**: Dedicated measurement via `measure-tls-bootstrapping-latency.sh` and matching `.service` unit.

### Known Performance Bottlenecks
- **apt cold start**: First apt-get/apt-mark call after Ubuntu VHD boot triggers expensive initialization (~30-50s). Mitigated by using `dpkg -i` instead of `apt-get -f install` for cached debs, and `dpkg --set-selections` instead of `apt-mark hold/unhold`.
- **dpkg lock contention**: Background `aptmarkWALinuxAgent` holds the dpkg lock. Removing `installContainerRuntime` barrier exposes lock contention for subsequent apt calls within ~1s of CSE start.
- **VHD apt state**: VHD build runs `apt-get clean/autoclean/autoremove` but does NOT remove `/var/lib/apt/lists/`. Lists survive into VHD. First `apt-get -f install` at boot still causes dpkg lock contention.
- **retrycmd_if_failure**: Uses `timeout <val> "$@"` — commands must be external executables; shell builtins/keywords need `bash -c` wrapping.

### VHD Build Performance
- `install-dependencies.sh` is the main build script — all package installations, component downloads, container image caching
- `post-install-dependencies.sh` handles cleanup (apt-get clean, temporary file removal)
- Container image pre-caching is the biggest time/size component of VHD build
- Component downloads come from packages.aks.azure.com

### Performance-Critical Files
- `cse_main.sh` — Orchestrator with ~40+ `logs_to_events` wrapped operations
- `cse_helpers.sh` — `logs_to_events` implementation, `retrycmd_if_failure`, error codes with timeouts
- `ubuntu/cse_helpers_ubuntu.sh` — `aptmarkWALinuxAgent` (dpkg lock holder), `installDebPackageFromFile`
- `ubuntu/cse_install_ubuntu.sh` — Ubuntu package installation with PMC source detection
- `mariner/cse_install_mariner.sh` — Mariner package installation with nvidia pkg updates
- `install-dependencies.sh` — VHD build main installer
- `measure-tls-bootstrapping-latency.sh` — TLS bootstrap timing

---
*No sessions yet.*
