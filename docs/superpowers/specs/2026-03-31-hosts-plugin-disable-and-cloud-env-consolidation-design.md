# Hosts Plugin Disable Logic + cloud-env Consolidation

## Problem

Two issues with the current hosts plugin implementation:

1. **No production disable path.** When AKS-RP sends `EnableHostsPlugin=false` on a CSE re-run (customer disables hosts plugin on an existing agentpool), `enableLocalDNS()` skips `enableAKSHostsSetup()` but does not actively clean up a previously-enabled hosts plugin. The `aks-hosts-setup.timer` keeps running, `/etc/localdns/hosts` persists, and the hosts plugin remains active until the node is reimaged. The only disable logic exists in `e2e/validators.go` (`disableHostsPluginOnRunningVM`) as a test-only SSH helper.

2. **Unnecessary `cloud-env` file.** `LOCALDNS_CRITICAL_FQDNS` is stored in a separate `/etc/localdns/cloud-env` file, solely so `aks-hosts-setup.service` can read it via `EnvironmentFile=`. This is confusing — it's the only variable in that file, and all other localdns state already lives in `/etc/localdns/environment`.

## Design

### Part 1: Consolidate `cloud-env` into `/etc/localdns/environment`

Add `LOCALDNS_CRITICAL_FQDNS` to the environment file that `generateLocalDNSFiles()` already writes. The environment file becomes the single source of truth for all localdns state:

```
LOCALDNS_BASE64_ENCODED_COREFILE=<base64>
LOCALDNS_COREFILE_BASE=<base64>
LOCALDNS_COREFILE_EXPERIMENTAL=<base64>
SHOULD_ENABLE_HOSTS_PLUGIN=true|false
LOCALDNS_CRITICAL_FQDNS=mcr.microsoft.com,login.microsoftonline.com,...
```

**Bug fix — call order in `enableLocalDNS()`:** The current code calls `enableAKSHostsSetup()` *before* `generateLocalDNSFiles()`. This means the timer could fire before the environment file is written. Part 1 fixes this by moving the if/else block after `generateLocalDNSFiles()`, so the environment file (now containing FQDNs) is written before the timer starts.

**Files changed:**

- **`parts/linux/cloud-init/artifacts/cse_config.sh`**
  - `generateLocalDNSFiles()`: Add `LOCALDNS_CRITICAL_FQDNS` to the environment file write block.
  - `enableAKSHostsSetup()`: Remove cloud-env file creation (lines 1383-1386). Keep the `LOCALDNS_CRITICAL_FQDNS` empty-check guard — it prevents starting the timer when RP didn't pass FQDNs (without it, the timer would fire but `aks-hosts-setup.sh` exits 0 on empty FQDNs, leaving an empty hosts file that confuses corefile selection).
  - `enableLocalDNS()`: Move the if/else block **after** `generateLocalDNSFiles()` so the environment file (now containing FQDNs) is written before the timer starts.

- **`parts/linux/cloud-init/artifacts/aks-hosts-setup.service`**
  - Change `EnvironmentFile=-/etc/localdns/cloud-env` to `EnvironmentFile=-/etc/localdns/environment`.

- **`parts/linux/cloud-init/artifacts/aks-hosts-setup.sh`**
  - Update comment on line 6: change "persisted via `/etc/localdns/cloud-env`" to "persisted via `/etc/localdns/environment`".

### Part 2: Add `disableAKSHostsSetup()` to production code

Follow the `configureManagedGPUExperience()` pattern — add an `else` branch in `enableLocalDNS()` that actively cleans up hosts plugin state on CSE re-run.

**New function `disableAKSHostsSetup()`** (placed after `enableAKSHostsSetup()` in `cse_config.sh`):

1. Disable and stop `aks-hosts-setup.timer` (idempotent — no-ops if already stopped/missing).
2. Remove `/etc/localdns/hosts`.

The corefile update and environment file rewrite are already handled by `generateLocalDNSFiles()` + `systemctlEnableAndStart localdns` which run after the if/else. `select_localdns_corefile()` in `localdns.sh` reads `SHOULD_ENABLE_HOSTS_PLUGIN=false` from the environment file and picks `LOCALDNS_COREFILE_BASE` (no hosts plugin).

`LOCALDNS_CRITICAL_FQDNS` remains in the environment file after disable — this is intentional. It's harmless (the timer is stopped, nothing reads it) and avoids unnecessary complexity in the disable path.

**Modified `enableLocalDNS()` flow:**

```
enableLocalDNS()
  ├─ guards (VHD has localdns assets?)
  ├─ generateLocalDNSFiles()                  ← writes environment file with SHOULD_ENABLE_HOSTS_PLUGIN + FQDNs
  ├─ if SHOULD_ENABLE_HOSTS_PLUGIN=true
  │     → enableAKSHostsSetup()               ← start timer, touch hosts file
  │   else
  │     → disableAKSHostsSetup()              ← NEW: stop timer, rm hosts file
  ├─ systemctlEnableAndStart localdns         ← restart picks correct corefile variant
```

### Part 3: E2E — CSE re-run for rollback tests

Add CSE re-run capability to the e2e framework so rollback tests exercise the actual production code path instead of SSH-based simulation.

**Files changed:**

- **`e2e/config/azure.go`**: Add `armcompute.VirtualMachineScaleSetExtensionsClient` to `AzureClient`.
- **`e2e/vmss.go`**: New `RerunCSE()` helper that:
  1. Regenerates CSE from a modified NBC (legacy) or AKSNodeConfig (scriptless) with `EnableHostsPlugin=false`.
  2. Pushes the new CSE to the existing VMSS via `VirtualMachineScaleSetExtensionsClient.BeginCreateOrUpdate`.
- **`e2e/scenario_localdns_hosts_test.go`**: Rollback tests (`Test_LocalDNSHostsPlugin_Rollback`, `Test_LocalDNSHostsPlugin_Rollback_Scriptless`) call `RerunCSE()` instead of `disableHostsPluginOnRunningVM()`.
- **`e2e/validators.go`**:
  - Remove `disableHostsPluginOnRunningVM()`.
  - Keep `validateHostsPluginDisabled()`.
  - Update `validateHostsPluginDisabled()` to remove the cloud-env check (cloud-env no longer exists).
  - Update `validateNoHostsPlugin()` to remove the cloud-env check (Check 2, lines 1561-1569).
  - Update `enableHostsPluginOnRunningVM()` to write FQDNs to `/etc/localdns/environment` instead of creating `/etc/localdns/cloud-env`.

### Part 4: Regenerate snapshots

Run `make generate` to regenerate snapshot test data since files in `parts/` are changed.

## Backward Compatibility

- **New CSE on old VHD:** Not an issue for `disableAKSHostsSetup()` — it's in `cse_config.sh` which is part of CSE, not VHD. CSE carries its own functions. For the service file change: old VHD's `aks-hosts-setup.service` still reads `EnvironmentFile=-/etc/localdns/cloud-env`, but new CSE no longer writes `cloud-env`. When the timer fires, `aks-hosts-setup.sh` sees empty `LOCALDNS_CRITICAL_FQDNS` and exits gracefully (line 23-26). This is the most common production scenario during rollout (new CSE deploys before VHD refreshes) and degrades safely — the hosts plugin doesn't activate, and the base corefile is used.
- **Old CSE on new VHD:** Old CSE still writes `cloud-env`. The new `aks-hosts-setup.service` reads from `/etc/localdns/environment`, which old CSE also writes (without FQDNs). `aks-hosts-setup.sh` will see empty `LOCALDNS_CRITICAL_FQDNS` and exit gracefully. Same safe degradation as above.
- **`disableAKSHostsSetup()` idempotency:** All operations are safe to call when hosts plugin was never enabled. `systemctl disable --now` on an inactive timer is a no-op. `rm -f` on a nonexistent file is a no-op.
