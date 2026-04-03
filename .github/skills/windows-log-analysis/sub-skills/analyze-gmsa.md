# Analyze gMSA — Container Credential Guard (CCG) & gMSA Authentication

## Purpose

Detect gMSA (group Managed Service Account) authentication failures, CCG (Container Credential Guard) plugin errors, Kerberos ticket acquisition problems, and credential spec misconfigurations on Windows AKS nodes. gMSA enables Windows containers to authenticate to Active Directory without domain-joining the node — the CCG plugin (`ccg.exe`) retrieves gMSA credentials on behalf of containers using a portable identity (e.g., Azure Key Vault user-assigned managed identity).

## Input Files

| File Pattern | Encoding | Contents |
|-------------|----------|----------|
| `Microsoft-Windows-Containers-CCG%4Admin.evtx` | Binary (EVTX) | CCG event log — credential fetch attempts, plugin load, errors |
| `<ts>_hyper-v-compute-operational.csv` | UTF-16-LE with BOM, CSV with embedded newlines | HCS events including credential setup errors (Event ID 11507) |
| `kubelet.log` | UTF-8 | Kubelet logs — container creation failures with credential spec errors |

**EVTX handling**: The CCG log is in Windows Event Log binary format (`.evtx`). It cannot be parsed as text. Use one of:
1. Check if a CSV export exists alongside it (e.g., `ccg-admin.csv` or similar)
2. Use PowerShell: `Get-WinEvent -Path '.\Microsoft-Windows-Containers-CCG%4Admin.evtx' | Export-Csv ccg-events.csv`
3. Use `python-evtx` library if available: `from Evtx.Evtx import FileHeader`
4. If none of the above work, note the file exists and recommend the user export it on a Windows machine for analysis

When a CSV export is available, parse it using the same rules as other event CSVs (see common-reference.md): skip `#TYPE` lines, handle embedded newlines with proper CSV parsing.

**Process ALL snapshots** — CCG errors may be intermittent (DC connectivity flaps, ticket renewal failures).

## Analysis Steps

### 1. CCG Event Log Classification (`Microsoft-Windows-Containers-CCG%4Admin.evtx`)

If the EVTX can be parsed (CSV export or python-evtx), classify events by Event ID:

| Event ID | Level | Meaning |
|----------|-------|---------|
| 1 | Information | CCG plugin loaded successfully — credential fetch initiated |
| 2 | Error | CCG plugin failed to fetch gMSA credentials |
| 3 | Error | CCG plugin COM object creation failed — plugin not registered |
| 4 | Error | Credential spec parsing failed — malformed JSON |
| 5 | Information | CCG successfully retrieved gMSA credentials |

**What to report**:
- Count events by ID and level
- 🔴 CRITICAL: Event ID 2 (fetch failures) — gMSA authentication will fail for affected containers
- 🔴 CRITICAL: Event ID 3 (plugin load failures) — CCG plugin not installed or not registered
- 🔴 CRITICAL: Event ID 4 (credential spec errors) — invalid credential spec configuration
- 🔵 INFO: Event ID 1 + Event ID 5 pairs — successful credential fetch (healthy)
- 🟡 WARNING: Event ID 1 without matching Event ID 5 — credential fetch was initiated but never completed

**Extract error details from Message field**:
- Plugin CLSID (GUID) — identifies which CCG plugin is being used
  - `{CCC2A336-D7F3-4818-A213-272B7924213E}` = Azure AD CCG plugin (AKS managed gMSA)
  - Other GUIDs = third-party or custom plugins
- gMSA account name — which service account failed
- Error reason text — specific failure cause

### 2. CCG Plugin Error Classification

From Event ID 2 messages, classify the error reason:

| Error Reason Pattern | Meaning | Severity |
|---------------------|---------|----------|
| `The attempted logon is invalid` | Plugin identity (managed identity) cannot retrieve gMSA password from AD | 🔴 CRITICAL |
| `A specified logon session does not exist` | Token expired or managed identity credential unavailable | 🔴 CRITICAL |
| `The RPC server is unavailable` (0xC0020017) | CCG cannot communicate with its plugin — service crashed or not registered | 🔴 CRITICAL |
| `The network path was not found` | Domain controller unreachable — DNS or network connectivity issue | 🔴 CRITICAL |
| `The specified domain either does not exist or could not be contacted` | AD domain resolution failure — DNS misconfiguration | 🔴 CRITICAL |
| `Access is denied` | Managed identity or user does not have permission to retrieve gMSA password | 🔴 CRITICAL |
| `There are currently no logon servers available` | No domain controllers reachable | 🔴 CRITICAL |
| `The security database on the server does not have a computer account` | Node not recognized by AD — stale computer account or wrong domain | 🟡 WARNING |
| `Clock skew too great` | Time difference between node and DC exceeds Kerberos tolerance (5 min) | 🔴 CRITICAL |

### 3. Credential Spec Validation (`kubelet.log`)

Search kubelet logs for credential spec errors:

| Error Pattern | Meaning | Severity |
|--------------|---------|----------|
| `failed to create credentialspec` | Credential spec resource not found or invalid | 🔴 CRITICAL |
| `credentialspec not found` / `no such credential spec` | GMSACredentialSpec or GMSACredentialSpecName reference doesn't exist | 🔴 CRITICAL |
| `error validating credential spec` | JSON schema validation failed | 🔴 CRITICAL |
| `WindowsOptions.GMSACredentialSpec` | Credential spec referenced in pod spec — extract spec name for correlation | 🔵 INFO |
| `RunAsUserName` | gMSA RunAsUserName specified — must match gMSA account for non-Kerberos auth | 🔵 INFO |

### 4. HCS Credential Setup Errors (`*_hyper-v-compute-operational.csv`)

Parse Hyper-V Compute operational events for credential setup failures:

- Search for Event ID **11507** — "Failed to setup the external credentials for Container"
- Extract container ID (64-char hex) and error message

| Error Pattern in Event 11507 | Meaning | Severity |
|------------------------------|---------|----------|
| `The RPC server is unavailable` (0xC0020017) | CCG service not responding — most common gMSA failure on AKS | 🔴 CRITICAL |
| `The parameter is incorrect` (0x80070057) | Malformed credential spec passed to HCS | 🔴 CRITICAL |
| `Access is denied` (0x80070005) | Permission error setting up credentials in container | 🔴 CRITICAL |
| `Element not found` (0x80070490) | CCG plugin not found for the specified CLSID | 🔴 CRITICAL |

**Cross-reference container IDs**: Match container IDs from Event 11507 with HCS Create/Start events (Event 2000/2001) to identify which pods were affected.

### 5. Kerberos Ticket Acquisition Failures

Search across all parseable logs for Kerberos-related errors:

| Error Pattern | Meaning | Severity |
|--------------|---------|----------|
| `KDC_ERR_S_PRINCIPAL_UNKNOWN` | gMSA SPN not registered in AD | 🔴 CRITICAL |
| `KDC_ERR_C_PRINCIPAL_UNKNOWN` | Client principal (gMSA account) not found in AD | 🔴 CRITICAL |
| `KRB_AP_ERR_SKEW` / `Clock skew` | Time sync issue between node and domain controller | 🔴 CRITICAL |
| `KRB_AP_ERR_MODIFIED` | gMSA password out of sync — password was recently rotated | 🟡 WARNING |
| `KDC_ERR_ETYPE_NOSUPP` | Encryption type mismatch — AD doesn't support required encryption | 🟡 WARNING |

### 6. Domain Controller Connectivity

Infer DC connectivity from CCG and HCS error patterns:

- Multiple containers failing with "network path not found" or "no logon servers" → DC is unreachable
- Intermittent failures (some succeed, some fail) → DC connectivity flapping or load balancing issue
- All failures with "RPC server unavailable" → CCG plugin itself is broken (not a DC issue)

**Cross-reference with DNS**: If DC connectivity failures are present, check if DNS resolution is working:
- DNS issues → containers can't resolve DC hostname → gMSA fails
- Look for DC FQDN in error messages and cross-reference with DNS analysis

## Findings Format

```markdown
### gMSA / CCG Authentication Findings

🔴 **CRITICAL** (HIGH confidence): CCG credential fetch failures detected
  - 12 Event ID 2 errors in CCG Admin log
  - Plugin: {CCC2A336-D7F3-4818-A213-272B7924213E} (Azure AD CCG plugin)
  - gMSA account: webapp01$
  - Error: "The attempted logon is invalid" — managed identity cannot retrieve gMSA password
  - All affected containers will fail AD authentication

🔴 **CRITICAL** (HIGH confidence): HCS credential setup failed for 3 containers
  - Event ID 11507: "Failed to setup the external credentials" with 0xC0020017
  - Container IDs: abc123..., def456..., ghi789...
  - CCG RPC server unavailable — plugin may have crashed

🟡 **WARNING** (MEDIUM confidence): Intermittent CCG failures
  - 8 successful credential fetches (Event ID 5) and 4 failures (Event ID 2)
  - Failures cluster around 03:40-03:45 UTC — possible transient DC connectivity issue

🔵 **INFO** (HIGH confidence): CCG EVTX file present but cannot be parsed in this environment
  - File: Microsoft-Windows-Containers-CCG%4Admin.evtx
  - Recommend exporting to CSV on a Windows machine for detailed analysis
```

## Known Patterns

| Pattern | Indicators | Severity | Root Cause |
|---------|-----------|----------|------------|
| CCG RPC server unavailable | Event 11507 with 0xC0020017 + no CCG events | 🔴 CRITICAL | CCG service not running or plugin COM registration missing. On older Windows builds, CCG may not be installed. (Windows-Containers#221) |
| Managed identity cannot fetch gMSA | CCG Event ID 2 "attempted logon is invalid" | 🔴 CRITICAL | User-assigned managed identity not authorized in AD to retrieve gMSA password. Check PrincipalsAllowedToRetrieveManagedPassword. |
| Domain controller unreachable | CCG "network path not found" / "no logon servers" | 🔴 CRITICAL | DNS resolution failing for AD domain or NSG blocking DC ports (389/636/88/135). Cross-ref with networking sub-skill. |
| Credential spec not found | kubelet "credentialspec not found" | 🔴 CRITICAL | GMSACredentialSpec CRD not created or name mismatch. Verify `kubectl get gmsa` output. |
| Clock skew (Kerberos) | "Clock skew too great" / `KRB_AP_ERR_SKEW` | 🔴 CRITICAL | Node time drifted >5 minutes from DC. Check W32Time service health in services analysis. |
| CCG plugin not registered | CCG Event ID 3 (COM creation failed) | 🔴 CRITICAL | CCG plugin DLL not registered. On AKS, the Azure AD plugin should be pre-installed. Node may need reimaging. |
| gMSA password rotation race | Intermittent `KRB_AP_ERR_MODIFIED` errors | 🟡 WARNING | gMSA password recently rotated by AD. Old password cached on some DCs. Self-resolves within AD replication interval. |
| Multiple containers same gMSA + hostname | Intermittent auth failures, "trust failure" in nltest | 🟡 WARNING | Race condition when containers with same hostname talk to same DC. Use unique hostnames per container. |
| CCG succeeds but app still fails auth | CCG Event ID 5 (success) but app reports auth errors | 🟡 WARNING | gMSA credential retrieved but app not configured to use it (e.g., IIS app pool identity mismatch, SPN not registered for the service). |

## Cross-References

- **→ analyze-hcs.md**: HCS Event ID 11507 (credential setup failure) is the HCS-side view of CCG failures. This sub-skill provides the CCG-specific context and error classification.
- **→ analyze-hns.md**: DNS resolution failures prevent domain controller discovery. If gMSA fails with "domain not found", check DNS health first.
- **→ analyze-hns.md**: Network policy or endpoint issues can block AD traffic (ports 88, 389, 636, 135, 445) needed for gMSA.
- **→ analyze-containers.md**: Containers failing gMSA will show as crash-looping or stuck in ContainerCreating if the credential spec is invalid.
- **→ analyze-services.md**: W32Time service health affects Kerberos. Clock skew >5 minutes causes all gMSA auth to fail.
- **→ common-reference.md**: For CSV parsing rules (embedded newlines, #TYPE headers, encoding) when processing the Hyper-V Compute operational CSV.
