# Analyze Extensions — Azure VM Extension Errors

## Purpose

Detect failures in Azure VM extension execution logs that may indicate provisioning failures or node join issues.

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `Extension-Logs*.zip` | Binary (zip archive) | Zipped extension logs (as produced by the collector) |
| `Extension-Logs-*/` | Directory | Extracted extension log directories (if zip was manually extracted) |

**Zip structure**: `Extension-Logs-<ts>/<plugin-name>/<version>/CommandExecution*.log`

Both zips and extracted directories should be scanned. Track seen `(plugin_name, filename)` pairs to avoid duplicate findings.

Extension logs are NOT timestamped per snapshot — the collector produces a single archive. Snapshot filters don't apply.

## Analysis Steps

### 1. Non-Zero Exit Code Detection

For each `CommandExecution*.log` file, search for: `exited with Exit code:\s*(-?\d+)`

- If exit code is not "0" → report the plugin name, filename, exit code, and last ~500 chars of the file as context

### 2. Execution Error Extraction

For each `CommandExecution*.log` file, look for the text `Execution Error:` in the file content.

- Extract text after "Execution Error:" up to the next `######` separator (or end of text)

**CRITICAL: Filter out curl progress false positives!** The AKSNode extension downloads binaries via curl, and curl's stderr progress output appears in the Execution Error block. These are NOT real errors. Filter out lines matching:
- `^\s*%\s+Total` (curl progress header)
- `^\s*Dload\s+Upload` (curl progress header)
- `^\s*\d+\s+\d+.*--:--` (curl progress data)
- `^\s*0\s+0\s+0\s+0` (curl zero-progress line)

If any non-curl-progress error lines remain after filtering → report first 5 lines.

### 3. Additional Error Pattern Detection

Beyond exit codes and execution errors, also check for:
- Extension timeouts (look for timeout-related messages)
- Network connectivity failures during extension execution
- Permission/access denied errors
- Extension status files (`status/*.status`) with non-zero codes
- Patterns of the same extension failing repeatedly across different CommandExecution files
- Unexpected extension plugins (could indicate misconfiguration)

### 4. Clean Result Reporting

If no errors found in any of the above steps, report as INFO: "No extension execution errors found"

## Findings Format

```markdown
### VM Extension Findings

<severity> **<LEVEL>** (<confidence> confidence): <description>
  - <detail line 1>
  - <detail line 2>
```

**Example**:
```markdown
🟡 **WARNING** (HIGH confidence): Extension error in Microsoft.AKS.Compute.AKS-Windows / CommandExecution_20260323.log (exit=1)
  - Error output: Failed to download kubernetes binaries...

🔵 **INFO** (HIGH confidence): No extension execution errors found
```

## Known Patterns

| Pattern | Severity | Confidence | Indicators | Remediation |
|---------|----------|------------|------------|-------------|
| Non-zero exit code | 🟡 WARNING | HIGH | `exited with Exit code: <N>` where N ≠ 0 | Check error output for root cause; may need reprovisioning |
| Execution Error (non-curl) | 🟡 WARNING | HIGH | Text after "Execution Error:" that isn't curl progress | Investigate specific error message |
| AKSNode extension failure | 🔴 CRITICAL | HIGH | AKS-Windows or AKS.Compute extension with non-zero exit | Node likely never joined cluster; check bootstrap |
| Extension timeout | 🟡 WARNING | MEDIUM | Timeout-related messages in execution logs | Network issues or slow provisioning; retry or check connectivity |
| Repeated same-extension failures | 🟡 WARNING | HIGH | Same plugin failing across multiple CommandExecution files | Persistent issue; not transient |
| Network connectivity failure | 🟡 WARNING | MEDIUM | Connection refused, DNS resolution failed, download errors | Check NSG rules, DNS, proxy configuration |

## Cross-References

- **analyze-bootstrap.md**: If extensions failed, the node likely never joined the cluster — check bootstrap for node join status
- **analyze-kubelet.md**: Extension errors are usually independent of runtime health but AKSNode extension failures prevent kubelet from starting
- **analyze-containers.md**: Extension errors during provisioning are independent of container runtime issues; they matter most during initial node setup
