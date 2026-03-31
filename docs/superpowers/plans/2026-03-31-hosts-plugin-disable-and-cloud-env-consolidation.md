# Hosts Plugin Disable + cloud-env Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the hosts plugin disable logic from e2e test code into production CSE code, consolidate `/etc/localdns/cloud-env` into `/etc/localdns/environment`, and add CSE re-run capability to e2e tests.

**Architecture:** Add `disableAKSHostsSetup()` to `cse_config.sh` with an `else` branch in `enableLocalDNS()` following the `configureManagedGPUExperience()` pattern. Consolidate `LOCALDNS_CRITICAL_FQDNS` from a separate `cloud-env` file into the existing `/etc/localdns/environment` file. Add `RerunCSE()` to the e2e framework so rollback tests exercise the production code path via Azure extension update. Add backward-compatibility tests for the new-CSE/old-VHD scenario.

**Tech Stack:** Bash (CSE scripts), Go (e2e tests), Azure SDK (`armcompute/v7`), systemd

**Spec:** `docs/superpowers/specs/2026-03-31-hosts-plugin-disable-and-cloud-env-consolidation-design.md`

---

### Task 1: Consolidate cloud-env into /etc/localdns/environment (cse_config.sh)

**Files:**
- Modify: `parts/linux/cloud-init/artifacts/cse_config.sh:1252-1402`
- Modify: `parts/linux/cloud-init/artifacts/aks-hosts-setup.service:9`
- Modify: `parts/linux/cloud-init/artifacts/aks-hosts-setup.sh:6-7`

This task adds `LOCALDNS_CRITICAL_FQDNS` to the environment file, removes cloud-env file creation from `enableAKSHostsSetup()`, and updates the service unit to read from `/etc/localdns/environment`.

- [ ] **Step 1: Add `LOCALDNS_CRITICAL_FQDNS` to the environment file write block in `generateLocalDNSFiles()`**

In `cse_config.sh`, find the `cat > "${LOCALDNS_ENV_FILE}" <<EOF` block at lines 1284-1289. Add `LOCALDNS_CRITICAL_FQDNS` as the last variable before `EOF`:

```bash
# Before (lines 1284-1289):
    cat > "${LOCALDNS_ENV_FILE}" <<EOF
LOCALDNS_BASE64_ENCODED_COREFILE=${corefile_base}
LOCALDNS_COREFILE_BASE=${corefile_base}
LOCALDNS_COREFILE_EXPERIMENTAL=${LOCALDNS_COREFILE_EXPERIMENTAL:-}
SHOULD_ENABLE_HOSTS_PLUGIN=${SHOULD_ENABLE_HOSTS_PLUGIN:-false}
EOF

# After:
    cat > "${LOCALDNS_ENV_FILE}" <<EOF
LOCALDNS_BASE64_ENCODED_COREFILE=${corefile_base}
LOCALDNS_COREFILE_BASE=${corefile_base}
LOCALDNS_COREFILE_EXPERIMENTAL=${LOCALDNS_COREFILE_EXPERIMENTAL:-}
SHOULD_ENABLE_HOSTS_PLUGIN=${SHOULD_ENABLE_HOSTS_PLUGIN:-false}
LOCALDNS_CRITICAL_FQDNS=${LOCALDNS_CRITICAL_FQDNS:-}
EOF
```

- [ ] **Step 2: Remove cloud-env file creation from `enableAKSHostsSetup()`**

In `cse_config.sh`, in the `enableAKSHostsSetup()` function:

1. Remove the `cloud_env_file` local variable declaration (line 1353):
```bash
# Remove this line:
    local cloud_env_file="${AKS_CLOUD_ENV_FILE:-/etc/localdns/cloud-env}"
```

2. Remove the cloud-env file creation block (lines 1383-1386):
```bash
# Remove these lines:
    echo "Setting LOCALDNS_CRITICAL_FQDNS for aks-hosts-setup"
    mkdir -p "$(dirname "${cloud_env_file}")"
    echo "LOCALDNS_CRITICAL_FQDNS=${LOCALDNS_CRITICAL_FQDNS}" > "${cloud_env_file}"
    chmod 0644 "${cloud_env_file}"
```

Keep the `LOCALDNS_CRITICAL_FQDNS` empty-check guard (lines 1377-1381) — it prevents starting the timer when RP didn't pass FQDNs.

- [ ] **Step 3: Update aks-hosts-setup.service to read from `/etc/localdns/environment`**

In `aks-hosts-setup.service`, change line 9:

```
# Before:
EnvironmentFile=-/etc/localdns/cloud-env

# After:
EnvironmentFile=-/etc/localdns/environment
```

- [ ] **Step 4: Update aks-hosts-setup.sh comment**

In `aks-hosts-setup.sh`, update lines 6-7:

```bash
# Before:
# LOCALDNS_CRITICAL_FQDNS is set by CSE (cse_cmd.sh) and persisted via /etc/localdns/cloud-env
# as a systemd EnvironmentFile so it's available on both initial and timer-triggered runs.

# After:
# LOCALDNS_CRITICAL_FQDNS is set by CSE (cse_cmd.sh) and persisted via /etc/localdns/environment
# as a systemd EnvironmentFile so it's available on both initial and timer-triggered runs.
```

- [ ] **Step 5: Commit**

```bash
git add parts/linux/cloud-init/artifacts/cse_config.sh parts/linux/cloud-init/artifacts/aks-hosts-setup.service parts/linux/cloud-init/artifacts/aks-hosts-setup.sh
git commit -m "consolidate cloud-env into /etc/localdns/environment

Move LOCALDNS_CRITICAL_FQDNS from a separate /etc/localdns/cloud-env file
into the existing /etc/localdns/environment file. This eliminates a
confusing extra file — all localdns state now lives in one place.

- generateLocalDNSFiles() now writes LOCALDNS_CRITICAL_FQDNS to environment
- enableAKSHostsSetup() no longer creates cloud-env
- aks-hosts-setup.service reads from /etc/localdns/environment"
```

---

### Task 2: Add disableAKSHostsSetup() and reorder enableLocalDNS() (cse_config.sh)

**Files:**
- Modify: `parts/linux/cloud-init/artifacts/cse_config.sh:1308-1402`

This task adds the production disable function and restructures `enableLocalDNS()` to call it on the `else` branch, following the `configureManagedGPUExperience()` pattern.

- [ ] **Step 1: Add `disableAKSHostsSetup()` function after `enableAKSHostsSetup()`**

Insert the new function immediately after `enableAKSHostsSetup()` (after line 1402 — adjust for Task 1 changes):

```bash
# disableAKSHostsSetup disables the hosts plugin on a node where it was previously enabled.
# Called from enableLocalDNS() when SHOULD_ENABLE_HOSTS_PLUGIN is false.
# This handles the production rollback case where a customer disables the hosts plugin
# on an existing agentpool and AKS-RP re-runs CSE with SHOULD_ENABLE_HOSTS_PLUGIN=false.
# All operations are idempotent — safe to call when hosts plugin was never enabled.
disableAKSHostsSetup() {
    local hosts_file="${AKS_HOSTS_FILE:-/etc/localdns/hosts}"
    local hosts_setup_timer="${AKS_HOSTS_SETUP_TIMER:-/etc/systemd/system/aks-hosts-setup.timer}"

    echo "disableAKSHostsSetup called, cleaning up hosts plugin state..."

    # Stop and disable the hosts-setup timer if it exists and is active.
    # This prevents further updates to the hosts file.
    if [ -f "${hosts_setup_timer}" ]; then
        systemctl disable --now aks-hosts-setup.timer 2>/dev/null || true
        echo "Disabled and stopped aks-hosts-setup.timer"
    else
        echo "aks-hosts-setup.timer not found on this VHD, skipping"
    fi

    # Remove the hosts file. Without it, select_localdns_corefile() in localdns.sh
    # will fall back to the base corefile even if SHOULD_ENABLE_HOSTS_PLUGIN were somehow still true.
    if [ -f "${hosts_file}" ]; then
        rm -f "${hosts_file}"
        echo "Removed ${hosts_file}"
    else
        echo "${hosts_file} does not exist, skipping"
    fi

    echo "disableAKSHostsSetup complete"
}
```

- [ ] **Step 2: Restructure `enableLocalDNS()` — move if/else after generateLocalDNSFiles and add else branch**

The current `enableLocalDNS()` (lines 1311-1340) has the wrong call order — `enableAKSHostsSetup()` is called before `generateLocalDNSFiles()`. Fix the order and add the `else` branch:

```bash
# Before (current enableLocalDNS):
enableLocalDNS() {
    if [ ! -f /etc/systemd/system/localdns.service ]; then
        echo "Warning: localdns.service not found on this VHD, skipping localdns setup"
        return 0
    fi
    if [ ! -f /opt/azure/containers/localdns/localdns.sh ]; then
        echo "Warning: localdns.sh not found on this VHD, skipping localdns setup"
        return 0
    fi

    if [ "${SHOULD_ENABLE_HOSTS_PLUGIN}" = "true" ]; then
        logs_to_events "AKS.CSE.enableLocalDNS.enableAKSHostsSetup" enableAKSHostsSetup
    fi

    echo "enableLocalDNS called, generating corefile..."
    generateLocalDNSFiles
    echo "Generated corefile: $(grep -q 'hosts /etc/localdns/hosts' "${LOCALDNS_CORE_FILE}" 2>/dev/null && echo 'WITH hosts plugin' || echo 'WITHOUT hosts plugin')"

    echo "localdns should be enabled."
    systemctlEnableAndStart localdns 30 || exit $ERR_LOCALDNS_FAIL
    echo "Enable localdns succeeded."
}

# After (new enableLocalDNS):
enableLocalDNS() {
    if [ ! -f /etc/systemd/system/localdns.service ]; then
        echo "Warning: localdns.service not found on this VHD, skipping localdns setup"
        return 0
    fi
    if [ ! -f /opt/azure/containers/localdns/localdns.sh ]; then
        echo "Warning: localdns.sh not found on this VHD, skipping localdns setup"
        return 0
    fi

    echo "enableLocalDNS called, generating corefile..."
    generateLocalDNSFiles
    echo "Generated corefile: $(grep -q 'hosts /etc/localdns/hosts' "${LOCALDNS_CORE_FILE}" 2>/dev/null && echo 'WITH hosts plugin' || echo 'WITHOUT hosts plugin')"

    # Enable or disable the hosts plugin based on SHOULD_ENABLE_HOSTS_PLUGIN.
    # This follows the configureManagedGPUExperience() pattern — the setting is mutable,
    # so on CSE re-run we must handle both enable and disable (cleanup) paths.
    if [ "${SHOULD_ENABLE_HOSTS_PLUGIN}" = "true" ]; then
        logs_to_events "AKS.CSE.enableLocalDNS.enableAKSHostsSetup" enableAKSHostsSetup
    else
        logs_to_events "AKS.CSE.enableLocalDNS.disableAKSHostsSetup" disableAKSHostsSetup
    fi

    echo "localdns should be enabled."
    systemctlEnableAndStart localdns 30 || exit $ERR_LOCALDNS_FAIL
    echo "Enable localdns succeeded."
}
```

Key changes:
1. `generateLocalDNSFiles` moves **before** the if/else (fixes existing bug — environment file with FQDNs must be written before the timer starts)
2. `else` branch added calling `disableAKSHostsSetup` (new production disable path)

- [ ] **Step 3: Commit**

```bash
git add parts/linux/cloud-init/artifacts/cse_config.sh
git commit -m "add disableAKSHostsSetup() for production hosts plugin disable

When AKS-RP re-runs CSE with EnableHostsPlugin=false on a node that
previously had it enabled, the else branch now actively cleans up:
stops aks-hosts-setup timer and removes /etc/localdns/hosts.

Also fixes call order bug: generateLocalDNSFiles() now runs before
enableAKSHostsSetup() so the environment file (with FQDNs) is written
before the timer starts."
```

---

### Task 3: Regenerate snapshot test data

**Files:**
- Modify: Various generated files in `pkg/agent/testdata/`

After modifying files in `parts/`, we must regenerate snapshot test data. The changes to `cse_config.sh` and `aks-hosts-setup.service` will affect generated test snapshots.

- [ ] **Step 1: Run `make generate`**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && make generate
```

Expected: Regenerated files in `pkg/agent/testdata/` will be updated to reflect the new `LOCALDNS_CRITICAL_FQDNS` in the environment file, the new `disableAKSHostsSetup()` function, the reordered `enableLocalDNS()`, and the updated `aks-hosts-setup.service`.

- [ ] **Step 2: Verify tests pass**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && go test ./pkg/agent/... -count=1 -timeout 5m
```

Expected: All snapshot tests pass with the regenerated data.

- [ ] **Step 3: Commit**

```bash
git add -A pkg/agent/testdata/
git commit -m "regenerate snapshot test data after cse_config.sh changes"
```

---

### Task 4: Add VirtualMachineScaleSetExtensionsClient to e2e framework

**Files:**
- Modify: `e2e/config/azure.go:40-79` (struct) and `e2e/config/azure.go:337-345` (constructor)

Add the Azure SDK client needed to update VMSS extensions (for CSE re-run).

- [ ] **Step 1: Add `VMSSExtensions` field to `AzureClient` struct**

In `e2e/config/azure.go`, find the `AzureClient` struct. Add the new field after `VMExtensionImages` (line 77) or `ResourceSKUs` (line 78):

```go
// Before (around line 78):
    ResourceSKUs              *armcompute.ResourceSKUsClient
}

// After:
    ResourceSKUs              *armcompute.ResourceSKUsClient
    VMSSExtensions            *armcompute.VirtualMachineScaleSetExtensionsClient
}
```

- [ ] **Step 2: Instantiate the client in `NewAzureClient()`**

In `e2e/config/azure.go`, find the `NewAzureClient()` function. Add instantiation after the last client creation (line ~340, after `cloud.Galleries`), before `cloud.Credential = credential` (line 342):

```go
// After the Galleries client block, before cloud.Credential = credential:
    cloud.VMSSExtensions, err = armcompute.NewVirtualMachineScaleSetExtensionsClient(Config.SubscriptionID, credential, opts)
    if err != nil {
        return nil, fmt.Errorf("create vmss extensions client: %w", err)
    }
```

- [ ] **Step 3: Verify compilation**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && go build ./e2e/...
```

Expected: Compiles without errors.

- [ ] **Step 4: Commit**

```bash
git add e2e/config/azure.go
git commit -m "add VirtualMachineScaleSetExtensionsClient to e2e AzureClient

Required for CSE re-run capability — allows updating the vmssCSE
extension on an existing VMSS to trigger a new CSE execution."
```

---

### Task 5: Add RerunCSE() helper to e2e framework

**Files:**
- Modify: `e2e/vmss.go` (add `RerunCSE` function)

This helper regenerates a CSE command from a modified NBC and pushes it to an existing VMSS, triggering re-execution. This is how production works when AKS-RP re-runs CSE after an agentpool setting change.

**Important context for the implementer:**
- The extension name is `"vmssCSE"` (defined in `getBaseVMSSModel`, line 1035 of `vmss.go`)
- For **legacy** (bash CSE) path: call `ab.GetNodeBootstrapping(ctx, nbc)` to regenerate CSE from modified NBC. The CSE string embeds all env vars inline.
- For **scriptless** path: the CSE command is constant (`nodeconfigutils.CSE`) but the config is in a JSON file on disk written by cloud-init. For CSE re-run, we must write the updated config JSON to disk via SSH, then re-run the CSE. The simplest approach: generate the new CSE using the **legacy** path (call `ab.GetNodeBootstrapping` with a modified NBC) regardless of whether the original boot was scriptless. This works because the CSE scripts are the same — only the delivery mechanism differs.
- The extension update uses `config.Azure.VMSSExtensions.BeginCreateOrUpdate` with the `"vmssCSE"` extension name
- After the extension update completes, the VM has executed the new CSE. No separate reimage needed — Azure CustomScript extension re-runs when `commandToExecute` changes in `ProtectedSettings`.

- [ ] **Step 1: Add `RerunCSE()` function to `vmss.go`**

Add the following function to `e2e/vmss.go`:

```go
// RerunCSE regenerates a CSE command from the given NBC and pushes it to
// the existing VMSS, triggering re-execution of the Custom Script Extension.
// This simulates production behavior when AKS-RP re-runs CSE after an
// agentpool setting change (e.g., toggling EnableHostsPlugin).
//
// The function always uses the legacy (bash CSE) path for regeneration,
// regardless of the original bootstrap path. This works because:
//   - Legacy CSE embeds all env vars (SHOULD_ENABLE_HOSTS_PLUGIN, etc.) inline
//   - The CSE scripts are the same shell code in both legacy and scriptless paths
//   - On re-run, the CSE scripts re-execute enableLocalDNS() with the new env vars
func RerunCSE(ctx context.Context, s *Scenario, nbc *datamodel.NodeBootstrappingConfiguration) {
	s.T.Helper()

	ab, err := agent.NewAgentBaker()
	require.NoError(s.T, err, "failed to create AgentBaker")

	nodeBootstrapping, err := ab.GetNodeBootstrapping(ctx, nbc)
	require.NoError(s.T, err, "failed to regenerate node bootstrapping for CSE re-run")

	newCSE := nodeBootstrapping.CSE
	require.NotEmpty(s.T, newCSE, "regenerated CSE command is empty")

	s.T.Logf("Re-running CSE on VMSS %s (CSE length: %d)", s.Runtime.VMSSName, len(newCSE))

	cluster := s.Runtime.Cluster
	resourceGroupName := *cluster.Model.Properties.NodeResourceGroup

	ext := armcompute.VirtualMachineScaleSetExtension{
		Name: to.Ptr("vmssCSE"),
		Properties: &armcompute.VirtualMachineScaleSetExtensionProperties{
			Publisher:               to.Ptr("Microsoft.Azure.Extensions"),
			Type:                    to.Ptr("CustomScript"),
			TypeHandlerVersion:      to.Ptr("2.1"),
			AutoUpgradeMinorVersion: to.Ptr(true),
			Settings:                map[string]interface{}{},
			ProtectedSettings: map[string]interface{}{
				"commandToExecute": newCSE,
			},
		},
	}

	poller, err := config.Azure.VMSSExtensions.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		s.Runtime.VMSSName,
		"vmssCSE",
		ext,
		nil,
	)
	require.NoError(s.T, err, "failed to begin CSE extension update")

	_, err = poller.PollUntilDone(ctx, config.DefaultPollUntilDoneOptions)
	require.NoError(s.T, err, "CSE re-run failed")

	s.T.Log("CSE re-run completed successfully")
}
```

Also add the necessary imports at the top of `vmss.go` if not already present:
- `"github.com/Azure/agentbaker/pkg/agent"` (for `agent.NewAgentBaker`)
- `"github.com/Azure/agentbaker/pkg/agent/datamodel"` (for `datamodel.NodeBootstrappingConfiguration`)
- `"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"` (for `to.Ptr`)

Check existing imports first — most of these are likely already imported.

- [ ] **Step 2: Verify compilation**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && go build ./e2e/...
```

Expected: Compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add e2e/vmss.go
git commit -m "add RerunCSE() helper for CSE re-run on existing VMSS

Regenerates CSE from a modified NBC and pushes it via Azure extension
update, triggering re-execution. Used by rollback tests to exercise
the production disable path in cse_config.sh."
```

---

### Task 6: Update rollback tests to use RerunCSE()

**Files:**
- Modify: `e2e/scenario_localdns_hosts_test.go:186-271`

Replace `disableHostsPluginOnRunningVM(ctx, s)` calls with `RerunCSE()` using a modified NBC with `EnableHostsPlugin=false`.

- [ ] **Step 1: Update `Test_LocalDNSHostsPlugin_Rollback` (legacy path)**

In `e2e/scenario_localdns_hosts_test.go`, replace the Validator function in `Test_LocalDNSHostsPlugin_Rollback` (lines 222-228).

**⚠️ Three-level deep copy is required.** The NBC is shared with `ValidateCommonLinux` which calls `IsHostsPluginEnabled()` — if we mutate the original NBC, Phase 1's validation (which already ran) would retroactively have wrong state. Copy in this exact order: NBC struct → AgentPoolProfile struct → LocalDNSProfile struct → then set `EnableHostsPlugin = false`. This pattern is already used in `enableHostsPluginOnRunningVM` (lines 1611-1619).

```go
// Before:
					Validator: func(ctx context.Context, s *Scenario) {
						// Phase 1 already ran via ValidateCommonLinux (IsHostsPluginEnabled=true)
						// Phase 2: Disable hosts plugin on running VM
						disableHostsPluginOnRunningVM(ctx, s)
						// Phase 3: Validate hosts plugin is fully disabled
						validateHostsPluginDisabled(ctx, s)
					},

// After:
					Validator: func(ctx context.Context, s *Scenario) {
						// Phase 1 already ran via ValidateCommonLinux (IsHostsPluginEnabled=true)
						// Phase 2: Re-run CSE with EnableHostsPlugin=false (production disable path)
						nbcCopy := *s.Runtime.NBC
						appCopy := *nbcCopy.AgentPoolProfile
						nbcCopy.AgentPoolProfile = &appCopy
						localDNSCopy := *appCopy.LocalDNSProfile
						appCopy.LocalDNSProfile = &localDNSCopy
						localDNSCopy.EnableHostsPlugin = false
						RerunCSE(ctx, s, &nbcCopy)
						// Phase 3: Validate hosts plugin is fully disabled
						validateHostsPluginDisabled(ctx, s)
					},
```

- [ ] **Step 2: Update `Test_LocalDNSHostsPlugin_Rollback_Scriptless`**

The scriptless rollback test (lines 260-266) originally used `AKSNodeConfigMutator`. Since `RerunCSE()` uses the legacy path for CSE regeneration, we need to get an NBC. The scriptless path doesn't store NBC on `s.Runtime`, so regenerate one via `getBaseNBC()`:

```go
// Before:
					Validator: func(ctx context.Context, s *Scenario) {
						// Phase 1 already ran via ValidateCommonLinux (IsHostsPluginEnabled=true)
						// Phase 2: Disable hosts plugin on running VM
						disableHostsPluginOnRunningVM(ctx, s)
						// Phase 3: Validate hosts plugin is fully disabled
						validateHostsPluginDisabled(ctx, s)
					},

// After:
					Validator: func(ctx context.Context, s *Scenario) {
						// Phase 1 already ran via ValidateCommonLinux (IsHostsPluginEnabled=true)
						// Phase 2: Re-run CSE with EnableHostsPlugin=false (production disable path)
						// Scriptless path doesn't store NBC, so regenerate one for CSE generation
						nbc, err := getBaseNBC(s.T, s.Runtime.Cluster, s.VHD)
						require.NoError(s.T, err)
						nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = false
						RerunCSE(ctx, s, nbc)
						// Phase 3: Validate hosts plugin is fully disabled
						validateHostsPluginDisabled(ctx, s)
					},
```

- [ ] **Step 3: Update test comments to reflect new approach**

Update the doc comments for both rollback test functions to describe CSE re-run instead of SSH mutation:

For `Test_LocalDNSHostsPlugin_Rollback` (starting at line 186):
```go
// Test_LocalDNSHostsPlugin_Rollback tests disabling the hosts plugin on an already-running VM
// using the legacy (bash CSE) bootstrap path. This simulates a production rollback where a
// customer disables the hosts plugin on an existing agentpool and AKS-RP re-runs CSE.
//
// Phase 1 (automatic via ValidateCommonLinux): VM boots with EnableHostsPlugin=true, so the
// full hosts-plugin validation suite runs automatically — hosts file populated, service healthy,
// localdns restarted, AA flag proves authoritative response. This confirms the hosts plugin
// is fully working before we disable it.
//
// Phase 2: RerunCSE regenerates the CSE with EnableHostsPlugin=false and pushes it to the
// existing VMSS. The CSE re-runs enableLocalDNS() which hits the new else branch —
// disableAKSHostsSetup() stops the timer and removes the hosts file.
//
// Phase 3: validateHostsPluginDisabled runs comprehensive checks — environment file state,
// removed files, inactive timer, corefile without hosts directive, AA flag absent from dig,
// and DNS still resolves through localdns.
```

For `Test_LocalDNSHostsPlugin_Rollback_Scriptless` (starting at line 235):
```go
// Test_LocalDNSHostsPlugin_Rollback_Scriptless tests disabling the hosts plugin on an
// already-running VM using the scriptless (aks-node-controller) bootstrap path.
// Same three-phase flow as Test_LocalDNSHostsPlugin_Rollback. RerunCSE uses the legacy
// CSE generation path (ab.GetNodeBootstrapping) since it embeds env vars directly in the
// CSE command string, avoiding the need to update the on-disk AKSNodeConfig JSON.
```

- [ ] **Step 4: Verify compilation**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && go build ./e2e/...
```

Expected: Compiles without errors. (The `disableHostsPluginOnRunningVM` function is still present in `validators.go` at this point — it becomes unused. Go doesn't warn about unused functions, only unused imports/variables, so this compiles fine. We clean it up in Task 8.)

- [ ] **Step 5: Commit**

```bash
git add e2e/scenario_localdns_hosts_test.go
git commit -m "update rollback tests to use RerunCSE instead of SSH mutation

Rollback tests now exercise the actual production code path:
RerunCSE pushes a new CSE with EnableHostsPlugin=false to the VMSS,
which triggers enableLocalDNS() -> disableAKSHostsSetup()."
```

---

### Task 7: Add backward-compatibility e2e tests

**Files:**
- Modify: `e2e/scenario_localdns_hosts_test.go` (add new test functions)

Add tests that verify safe behavior when new CSE runs on an old VHD (where `aks-hosts-setup.service` still reads from `cloud-env`) and when disable runs on a node where hosts plugin was never enabled.

**Note on "old CSE on new VHD" coverage gap:** The spec describes a second backward-compat scenario (old CSE writing `cloud-env` on a new VHD where the service reads from `environment`). We cannot easily test this in e2e — it would require running an old CSE version. The spec confirms this degrades safely: `aks-hosts-setup.sh` exits gracefully on empty `LOCALDNS_CRITICAL_FQDNS`. This is acceptable and does not need an e2e test.

- [ ] **Step 1: Add `Test_LocalDNSHostsPlugin_DisableNeverEnabled` test**

This test verifies that `disableAKSHostsSetup()` is idempotent — safe to call on a node that never had the hosts plugin enabled (the `else` branch runs on every CSE where `SHOULD_ENABLE_HOSTS_PLUGIN=false`, including fresh nodes that never had it enabled).

Add to `e2e/scenario_localdns_hosts_test.go`:

```go
// Test_LocalDNSHostsPlugin_DisableNeverEnabled tests that the disable path is idempotent —
// calling disableAKSHostsSetup() on a node where the hosts plugin was never enabled
// does not cause errors. This is the common case: most nodes boot with
// EnableHostsPlugin=false and the else branch in enableLocalDNS() runs harmlessly.
//
// The test boots with EnableHostsPlugin=false, then validates:
// 1. LocalDNS is running and DNS resolves through 169.254.10.10
// 2. Hosts plugin is NOT active (no hosts file, no timer, base corefile)
// 3. All validators pass without errors
func Test_LocalDNSHostsPlugin_DisableNeverEnabled(t *testing.T) {
	tests := []struct {
		name string
		vhd  *config.Image
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests disable path is idempotent (never enabled) on " + tt.name,
				Config: Config{
					Cluster: ClusterKubenet,
					VHD:     tt.vhd,
					BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = false
					},
					Validator: func(ctx context.Context, s *Scenario) {
						// ValidateCommonLinux already ran — LocalDNS enabled, hosts plugin skipped
						// Explicitly verify hosts plugin is not active
						validateNoHostsPlugin(ctx, s)
					},
				},
			})
		})
	}
}
```

- [ ] **Step 2: Add `Test_LocalDNSHostsPlugin_BackwardCompat_NewCSEOldServiceUnit` test**

This test simulates the new-CSE/old-VHD backward compatibility scenario. On a real old VHD, `aks-hosts-setup.service` still reads `EnvironmentFile=-/etc/localdns/cloud-env`. Since we no longer write `cloud-env`, the timer would fire but `aks-hosts-setup.sh` sees empty `LOCALDNS_CRITICAL_FQDNS` and exits gracefully.

We simulate this by verifying that when `LOCALDNS_CRITICAL_FQDNS` is empty, the hosts plugin gracefully degrades — the timer is a no-op and the base corefile is used.

```go
// Test_LocalDNSHostsPlugin_BackwardCompat_NewCSEOldServiceUnit tests backward compatibility
// for the cloud-env consolidation. After this change, new CSE no longer writes
// /etc/localdns/cloud-env. On old VHDs where aks-hosts-setup.service still reads from
// cloud-env, the timer fires but aks-hosts-setup.sh sees empty LOCALDNS_CRITICAL_FQDNS
// and exits gracefully. We verify this by enabling hosts plugin but clearing the FQDNs.
func Test_LocalDNSHostsPlugin_BackwardCompat_NewCSEOldServiceUnit(t *testing.T) {
	tests := []struct {
		name string
		vhd  *config.Image
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunScenario(t, &Scenario{
				Description: "Tests backward compat (new CSE, old service unit) on " + tt.name,
				Config: Config{
					Cluster: ClusterKubenet,
					VHD:     tt.vhd,
					BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
						// Enable hosts plugin but clear CriticalFQDNs to simulate
						// the old-VHD scenario where FQDNs aren't available
						nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
						nbc.AgentPoolProfile.LocalDNSProfile.CriticalFQDNs = nil
					},
					Validator: func(ctx context.Context, s *Scenario) {
						// With empty CriticalFQDNs, enableAKSHostsSetup() should skip
						// starting the timer (LOCALDNS_CRITICAL_FQDNS guard).
						// LocalDNS should still be running with the base corefile.
						ValidateLocalDNSService(ctx, s, "enabled")
						ValidateLocalDNSResolution(ctx, s, "169.254.10.10")

						// Verify hosts plugin did NOT activate (no FQDNs = graceful skip)
						script := `set -euo pipefail
errors=0

# hosts file should not have IP mappings (no FQDNs were provided)
hosts_file="/etc/localdns/hosts"
if [ -f "$hosts_file" ] && grep -qE '^[0-9a-fA-F.:]+[[:space:]]+[a-zA-Z]' "$hosts_file"; then
    echo "ERROR: hosts file has IP mappings but no CriticalFQDNs were provided"
    errors=$((errors + 1))
else
    echo "OK: hosts file has no IP mappings (expected — no CriticalFQDNs)"
fi

# Active corefile should NOT include hosts plugin
corefile="/opt/azure/containers/localdns/localdns.corefile"
if grep -q "hosts /etc/localdns/hosts" "$corefile"; then
    echo "ERROR: active corefile contains hosts plugin directive"
    errors=$((errors + 1))
else
    echo "OK: active corefile does not contain hosts plugin"
fi

exit $errors`
						execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0,
							"backward compat validation failed")
					},
				},
			})
		})
	}
}
```

- [ ] **Step 3: Verify compilation**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && go build ./e2e/...
```

Expected: Compiles without errors.

- [ ] **Step 4: Commit**

```bash
git add e2e/scenario_localdns_hosts_test.go
git commit -m "add backward-compatibility e2e tests for hosts plugin disable

- DisableNeverEnabled: verifies else branch is idempotent on fresh nodes
- BackwardCompat_NewCSEOldServiceUnit: verifies graceful degradation
  when CriticalFQDNs are empty (simulates new-CSE/old-VHD scenario)"
```

---

### Task 8: Clean up validators.go — remove disableHostsPluginOnRunningVM, update cloud-env references

**Files:**
- Modify: `e2e/validators.go:1532-1895`

Remove the now-unused `disableHostsPluginOnRunningVM` function and update all validators that reference `cloud-env` to use `/etc/localdns/environment` instead.

- [ ] **Step 1: Remove `disableHostsPluginOnRunningVM()` function**

Delete the entire function from `e2e/validators.go` (lines 1689-1758, including the doc comment starting at line 1689).

- [ ] **Step 2: Update `validateNoHostsPlugin()` — remove cloud-env check**

In `validateNoHostsPlugin()` (lines 1532-1595), find the SSH script's Check 2 (lines 1561-1568) that verifies `/etc/localdns/cloud-env` does NOT exist. Remove this check entirely since `cloud-env` no longer exists in either the old or new code path:

```bash
# Remove this check (lines ~1561-1568 inside the SSH script):
# Check 2: /etc/localdns/cloud-env must NOT exist
cloud_env="/etc/localdns/cloud-env"
if [ -f "$cloud_env" ]; then
    echo "ERROR: $cloud_env exists but hosts plugin should not be enabled"
    cat "$cloud_env"
    errors=$((errors + 1))
else
    echo "OK: $cloud_env does not exist"
fi
```

Renumber remaining checks accordingly (Check 3 becomes Check 2, etc.).

- [ ] **Step 3: Update `enableHostsPluginOnRunningVM()` — write FQDNs to environment instead of cloud-env**

In `enableHostsPluginOnRunningVM()` (lines 1597-1687), find Step 3 in the SSH script (lines ~1665-1668) that creates `/etc/localdns/cloud-env`. Replace it with appending FQDNs to `/etc/localdns/environment`.

**Format string context:** The SSH script is wrapped in `fmt.Sprintf(script, experimentalB64, criticalFQDNs)` (line 1683). The script uses `%%s` for literal `%s` in bash (printf format) and bare `%s` for Go substitution points. The two `%s` substitution slots are: (1) `experimentalB64` at line 1658, (2) `criticalFQDNs` at line 1667.

Replace Step 3 (the second `%s` substitution — `criticalFQDNs`):

```bash
# Before (lines 1665-1668 inside the script string):
# Step 3: Write /etc/localdns/cloud-env with critical FQDNs
cloud_env="/etc/localdns/cloud-env"
printf '%%s\n' "LOCALDNS_CRITICAL_FQDNS=%s" | sudo tee "$cloud_env" > /dev/null
echo "OK: Wrote $cloud_env"

# After:
# Step 3: Add critical FQDNs to /etc/localdns/environment
# The environment file was written by the original CSE; brownfield enable appends FQDNs
env_file="/etc/localdns/environment"
if grep -q '^LOCALDNS_CRITICAL_FQDNS=' "$env_file" 2>/dev/null; then
    sudo sed -i "s|^LOCALDNS_CRITICAL_FQDNS=.*|LOCALDNS_CRITICAL_FQDNS=%s|" "$env_file"
else
    printf '\nLOCALDNS_CRITICAL_FQDNS=%%s\n' "%s" | sudo tee -a "$env_file" > /dev/null
fi
echo "OK: Updated LOCALDNS_CRITICAL_FQDNS in $env_file"
```

**⚠️ Format string escaping is critical here.** The `%s` on the `sed` and `printf` lines is the Go `fmt.Sprintf` substitution for `criticalFQDNs`. The `%%s` in `printf '..%%s..'` becomes a literal `%s` in bash. Review the existing `enableHostsPluginOnRunningVM` function carefully — the pattern for `%%s` vs `%s` is already established in the Step 2 `printf` block (lines 1655-1659). Match that pattern exactly.

- [ ] **Step 4: Update `validateHostsPluginDisabled()` — remove Check 2 (LOCALDNS_COREFILE_EXPERIMENTAL) and Check 3 (cloud-env)**

In `validateHostsPluginDisabled()` (lines 1760-1895), two checks must be removed:

**Remove Check 2** (lines ~1791-1800) — `LOCALDNS_COREFILE_EXPERIMENTAL must be empty`:
```bash
# Remove this check:
# Check 2: LOCALDNS_COREFILE_EXPERIMENTAL must be empty in /etc/localdns/environment
if [ -f "$env_file" ]; then
    exp_val=$(grep '^LOCALDNS_COREFILE_EXPERIMENTAL=' "$env_file" | cut -d= -f2- || true)
    if [ -z "$exp_val" ]; then
        echo "OK: LOCALDNS_COREFILE_EXPERIMENTAL is empty"
    else
        echo "ERROR: LOCALDNS_COREFILE_EXPERIMENTAL is not empty (${#exp_val} chars)"
        errors=$((errors + 1))
    fi
fi
```

**Why:** The old `disableHostsPluginOnRunningVM` SSH script explicitly cleared `LOCALDNS_COREFILE_EXPERIMENTAL`. But the production `RerunCSE` path calls `GetNodeBootstrapping` which always generates `LOCALDNS_COREFILE_EXPERIMENTAL` via `GetGeneratedLocalDNSCoreFileExperimental` (hardcoded in `cse_cmd.sh` line 189 — always called regardless of `EnableHostsPlugin`). After `RerunCSE`, the environment file will have a populated `LOCALDNS_COREFILE_EXPERIMENTAL`, which is correct — `select_localdns_corefile()` gates its use on `SHOULD_ENABLE_HOSTS_PLUGIN=false`. The experimental corefile being present but unused is the designed behavior.

**Remove Check 3** (lines ~1802-1810) — `/etc/localdns/cloud-env must NOT exist`:
```bash
# Remove this check:
# Check 3: /etc/localdns/cloud-env must NOT exist
cloud_env="/etc/localdns/cloud-env"
if [ -f "$cloud_env" ]; then
    echo "ERROR: $cloud_env still exists after disable"
    cat "$cloud_env"
    errors=$((errors + 1))
else
    echo "OK: $cloud_env does not exist"
fi
```

Renumber remaining checks accordingly (old Check 4 → Check 2, etc.).

- [ ] **Step 5: Verify compilation**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && go build ./e2e/...
```

Expected: Compiles without errors.

- [ ] **Step 6: Commit**

```bash
git add e2e/validators.go
git commit -m "clean up validators.go: remove disableHostsPluginOnRunningVM, drop cloud-env refs

- Remove disableHostsPluginOnRunningVM (replaced by production code path)
- Update validateNoHostsPlugin: remove cloud-env existence check
- Update enableHostsPluginOnRunningVM: write FQDNs to environment file
- Update validateHostsPluginDisabled: remove cloud-env existence check"
```

---

### Task 9: Update brownfield test comments for cloud-env removal

**Files:**
- Modify: `e2e/scenario_localdns_hosts_test.go:88-184`

Update doc comments on brownfield tests to reflect that `cloud-env` no longer exists.

- [ ] **Step 1: Update brownfield test comments**

In `e2e/scenario_localdns_hosts_test.go`, update the doc comments for `Test_LocalDNSHostsPlugin_Brownfield` (line 94) and `Test_LocalDNSHostsPlugin_Brownfield_Scriptless` (around line 98) to remove references to `cloud-env`:

```go
// Before (line 94):
// Phase 1b: validateNoHostsPlugin confirms SHOULD_ENABLE_HOSTS_PLUGIN=false, no cloud-env,
// no hosts directive in active corefile.

// After:
// Phase 1b: validateNoHostsPlugin confirms SHOULD_ENABLE_HOSTS_PLUGIN=false,
// no hosts directive in active corefile.

// Before (line 98):
// creates cloud-env, starts aks-hosts-setup timer/service.

// After:
// writes FQDNs to environment file, starts aks-hosts-setup timer/service.
```

- [ ] **Step 2: Commit**

```bash
git add e2e/scenario_localdns_hosts_test.go
git commit -m "update brownfield test comments to remove cloud-env references"
```

---

### Task 10: Final verification

- [ ] **Step 1: Run full Go build**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && go build ./...
```

Expected: No compilation errors.

- [ ] **Step 2: Run unit tests**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && go test ./pkg/agent/... -count=1 -timeout 5m
```

Expected: All tests pass.

- [ ] **Step 3: Run e2e build check**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && go build ./e2e/...
```

Expected: Compiles without errors.

- [ ] **Step 4: Check for any remaining cloud-env references**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && grep -r "cloud-env" --include="*.go" --include="*.sh" --include="*.service" e2e/ parts/ | grep -v "_test.go" | grep -v "testdata/"
```

Expected: No remaining references to `cloud-env` in production code. Test files may still reference it in backward-compat tests (which is fine — they test the old behavior).

- [ ] **Step 5: Verify no unused functions**

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && grep -rn "disableHostsPluginOnRunningVM" e2e/
```

Expected: No results (function fully removed and no remaining callers).

- [ ] **Step 6: Commit any remaining changes**

If `make generate` needs to be re-run after validator changes:

```bash
cd /home/sakwa/go/src/go.goms.io/AgentBaker && make generate
git add -A pkg/agent/testdata/
git commit -m "regenerate snapshot test data after final cleanup"
```
