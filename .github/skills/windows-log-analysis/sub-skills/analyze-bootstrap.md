> **Prerequisite:** Read `common-reference.md` before using this sub-skill.

# Analyze Bootstrap — Node Provisioning & Bootstrap Health

## Purpose

Detect node provisioning failures, CSE execution errors, bootstrap configuration mismatches, API server connectivity issues during join, and service startup ordering problems on Windows AKS nodes.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `windowsnodereset.log` | UTF-8 or UTF-16-LE with BOM | Node reset/reimage flow log — full provisioning timeline |
| `bootstrap-config` | UTF-8 or UTF-16-LE with BOM | Bootstrap parameters passed to CSE (JSON or key-value) |
| `<ts>-aks-info.log` | UTF-16-LE with BOM | `kubectl describe node` + node YAML, component versions |
| `CustomDataSetupScript.log` | UTF-8 | CSE main execution log |
| `CSEResult.log` | UTF-8 | Final CSE result with exit code |
| Extension logs (`*.status`, `*.settings`) | JSON | Azure VM extension status and settings |

## Analysis Steps

### 1. CSE Exit Code Check (`CSEResult.log`, Extension Logs)

If `CSEResult.log` or extension status files exist, extract the exit code.

**WINDOWS_CSE_ERROR codes** (from AgentBaker `windowscsehelper.ps1`):

| Code | Name | Meaning |
|------|------|---------|
| 0 | SUCCESS | CSE completed successfully |
| 1 | UNKNOWN | Unexpected error in catch block |
| 2 | DOWNLOAD_FILE_WITH_RETRY | File download failed after retries |
| 3 | INVOKE_EXECUTABLE | Executable invocation failed |
| 4 | FILE_NOT_EXIST | Required file missing |
| 5 | CHECK_API_SERVER_CONNECTIVITY | Cannot reach API server |
| 6 | PAUSE_IMAGE_NOT_EXIST | Pause container image missing |
| 7 | GET_SUBNET_PREFIX | Failed to get subnet prefix |
| 8 | GENERATE_TOKEN_FOR_ARM | ARM token generation failed |
| 9 | NETWORK_INTERFACES_NOT_EXIST | No network interfaces found |
| 10 | NETWORK_ADAPTER_NOT_EXIST | Network adapter missing |
| 11 | MANAGEMENT_IP_NOT_EXIST | Management IP not found |
| 12 | CALICO_SERVICE_ACCOUNT_NOT_EXIST | Calico SA missing |
| 13 | CONTAINERD_NOT_INSTALLED | containerd binary not found |
| 14 | CONTAINERD_NOT_RUNNING | containerd service not running |
| 15 | OPENSSH_NOT_INSTALLED | OpenSSH not installed |
| 16 | OPENSSH_FIREWALL_NOT_CONFIGURED | OpenSSH firewall rule missing |
| 17 | INVALID_PARAMETER_IN_AZURE_CONFIG | Bad azure.json parameter |
| 19 | GET_CA_CERTIFICATES | CA cert retrieval failed |
| 20 | DOWNLOAD_CA_CERTIFICATES | CA cert download failed |
| 21 | EMPTY_CA_CERTIFICATES | CA certs empty |
| 22 | ENABLE_SECURE_TLS | Secure TLS enablement failed |
| 23–28 | GMSA_* | gMSA setup failures |
| 29 | NOT_FOUND_MANAGEMENT_IP | Management IP lookup failed |
| 30 | NOT_FOUND_BUILD_NUMBER | Windows build number not found |
| 31 | NOT_FOUND_PROVISIONING_SCRIPTS | Provisioning scripts missing |
| 32 | START_NODE_RESET_SCRIPT_TASK | Node reset task failed to start |
| 33–40 | DOWNLOAD_*_PACKAGE | Package download failures (CSE, K8s, CNI, HNS, Calico, gMSA, CSI proxy, containerd) |
| 41 | SET_TCP_DYNAMIC_PORT_RANGE | TCP port range configuration failed |
| 43 | PULL_PAUSE_IMAGE | Pause image pull failed |
| 45 | CONTAINERD_BINARY_EXIST | containerd binary check failed |
| 46–48 | SET_*_PORT_RANGE | Port range exclusion failures |
| 49 | NO_CUSTOM_DATA_BIN | CustomData.bin missing (very early failure) |
| 50 | NO_CSE_RESULT_LOG | CSE did not produce result log |
| 52 | RESIZE_OS_DRIVE | OS drive resize failed |
| 53–61 | GPU_* | GPU driver installation failures |
| 62 | UPDATING_KUBE_CLUSTER_CONFIG | Kube cluster config update failed |
| 64 | GET_CONTAINERD_VERSION | containerd version detection failed |
| 65–67 | CREDENTIAL_PROVIDER_* | Credential provider install/config failures |
| 68 | ADJUST_PAGEFILE_SIZE | Pagefile resize failed |
| 70–71 | SECURE_TLS_BOOTSTRAP_* | Secure TLS bootstrap client failures |
| 72 | CILIUM_NETWORKING_INSTALL_FAILED | Cilium install failed |
| 73 | EXTRACT_ZIP | Zip extraction failed |
| 74–75 | LOAD/PARSE_METADATA | Metadata failures |
| 76–83 | ORAS_* | Network-isolated cluster artifact pull failures |

**Severity**:
- 🔴 CRITICAL: Any non-zero exit code — node failed to provision
- 🔵 INFO: Exit code 0 — CSE succeeded

### 2. CSE Execution Timeline (`CustomDataSetupScript.log`, `windowsnodereset.log`)

Parse log files for timestamped entries to reconstruct the provisioning flow:

**Expected bootstrap sequence**:
1. CustomData.bin decoded → provisioning scripts extracted
2. OS drive resized (if needed)
3. Packages downloaded (containerd, kubelet, kubectl, CNI, CSI proxy)
4. containerd installed and configured
5. kubelet configured (kubelet flags, kubeconfig written)
6. Network configured (HNS network created, CNI config written)
7. Services started (containerd → kubelet → kube-proxy)
8. API server connectivity verified
9. Node joins cluster

Search for:
- `"Write-Log"` or timestamped `[YYYY-MM-DD HH:MM:SS]` entries — build ordered timeline
- `"Error"`, `"Exception"`, `"Failed"` — failure points
- `"Set-ExitCode"` — where the CSE decided to fail
- Duration gaps >5 minutes between steps — potential hangs

- 🔴 CRITICAL: CSE execution stopped partway (missing later steps)
- 🟡 WARNING: Steps took abnormally long (>5 min for downloads, >2 min for config)
- 🔵 INFO: Full timeline with step durations

### 3. windowsnodereset.log Analysis

This log captures the node reset/reimage flow (used during node image upgrades and repairs).

Search for:
- `"Starting node reset"` / `"Node reset complete"` — flow boundaries
- `"Error"`, `"Failed"`, `"Exception"` — failure points in the reset flow
- `"Stop-Service"` / `"Start-Service"` entries — service lifecycle during reset
- `"Remove-"` operations — cleanup steps
- `"kubeconfig"` — kubeconfig regeneration
- `"HNS"` — network cleanup/recreation

- 🔴 CRITICAL: Reset flow failed (error without subsequent completion)
- 🟡 WARNING: Reset completed but with errors/retries
- 🔵 INFO: Clean reset flow with timestamps

### 4. Bootstrap Config Validation (`bootstrap-config`)

Parse bootstrap configuration and validate expected parameters:

**Key parameters to check**:
- `KubeletConfig` — kubelet flags and settings
- `ContainerRuntime` — should be `containerd`
- `KubernetesVersion` — should match node binary versions
- `APIServerName` / `APIServerEndpoint` — must be reachable
- `ClusterCIDR`, `ServiceCIDR`, `DNSServiceIP` — network config
- `WindowsProfile` settings — CSI proxy, containerd version

**Validation**:
- 🔴 CRITICAL: Missing required parameters (APIServerName, KubernetesVersion, ClusterCIDR)
- 🟡 WARNING: Version mismatches between bootstrap-config and actual binary versions
- 🔵 INFO: Report all key parameters for context

### 5. API Server Connectivity During Bootstrap

Search CSE logs and kubelet logs for API server connection issues:

- `"Unable to connect to the server"` — API server unreachable
- `"dial tcp"` + `"timeout"` or `"refused"` — network-level failure
- `"certificate"` + `"expired"` or `"unknown authority"` — TLS issues
- `"Unauthorized"` or `"401"` — auth token issues
- `"WINDOWS_CSE_ERROR_CHECK_API_SERVER_CONNECTIVITY"` (exit code 5)

- 🔴 CRITICAL: API server unreachable during bootstrap (node cannot join)
- 🟡 WARNING: Intermittent connectivity issues (retries succeeded)

### 6. Component Version Extraction (`*-aks-info.log`)

From the node description and bootstrap config, extract:
- Kubernetes version (kubelet binary vs node object)
- containerd version
- OS build number
- CSI proxy version (if present)
- kube-proxy version

**Cross-check**: Compare versions from bootstrap-config against actual running versions.

- 🟡 WARNING: Version mismatch between expected (bootstrap-config) and actual (node describe)
- 🔵 INFO: All component versions for reference

### 7. Service Startup Ordering

In CSE logs, verify services started in correct order:
1. containerd must start before kubelet
2. kubelet must start before kube-proxy
3. HNS must be running before network configuration

Search for `"Start-Service"` or `"service started"` entries and verify ordering.

- 🟡 WARNING: Services started out of order
- 🔵 INFO: Correct startup sequence confirmed

## Findings Format

```markdown
### Bootstrap & Provisioning Findings

🔴 **CRITICAL** (HIGH confidence): CSE failed with exit code 5 (CHECK_API_SERVER_CONNECTIVITY)
  - Node could not reach API server at api.example.com:443
  - "dial tcp 10.0.0.1:443: i/o timeout" in CustomDataSetupScript.log
  - Bootstrap stopped at step: API server connectivity check

🟡 **WARNING** (MEDIUM confidence): Package downloads took 8 minutes (expected <2 min)
  - Possible network throttling or registry latency

🔵 **INFO** (HIGH confidence): Bootstrap component versions
  - Kubernetes: v1.29.4, containerd: 1.7.15, OS build: 20348.2340
  - CSI proxy: v1.1.2, kube-proxy: v1.29.4
```

## Known Patterns

| Pattern | Severity | Confidence | Meaning |
|---------|----------|------------|---------|
| Exit code 5 (API server connectivity) | 🔴 CRITICAL | HIGH | Node cannot reach API server — check NSG, DNS, private endpoint |
| Exit code 13/14 (containerd not installed/running) | 🔴 CRITICAL | HIGH | containerd setup failed — check download logs |
| Exit code 49 (no CustomData.bin) | 🔴 CRITICAL | HIGH | Very early failure — VM extension did not deliver payload |
| Exit code 50 (no CSE result log) | 🔴 CRITICAL | HIGH | CSE crashed before producing output |
| Exit code 33–40 (package download failures) | 🔴 CRITICAL | HIGH | Network issue or registry unavailable |
| Exit code 76–83 (ORAS failures) | 🔴 CRITICAL | HIGH | Network-isolated cluster artifact pull failed |
| Version mismatch between config and actual | 🟡 WARNING | MEDIUM | Possible incomplete upgrade or config drift |
| Reset flow with errors but completed | 🟡 WARNING | MEDIUM | Node reset had issues — may have residual state |
| Download steps >5 minutes | 🟡 WARNING | LOW | Network latency — may indicate throttling |

## Cross-References

- **analyze-extensions.md**: CSE exit codes are also reported in extension status; cross-reference for additional context
- **analyze-services.md**: Service crash events during bootstrap correlate with startup ordering failures
- **analyze-kubelet.md**: First-start kubelet failures (lease renewal, NotReady) often follow bootstrap issues
- **analyze-services.md**: Post-bootstrap service states should be validated — a stopped containerd after bootstrap indicates a startup failure
