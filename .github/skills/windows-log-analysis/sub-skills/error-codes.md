# Error Code Reference

HCS, HNS, and Windows Event ID reference tables. See [common-reference.md](common-reference.md) for encoding, parsing, and verification protocols.

## HCS Error Codes

| Code | Name | Meaning |
|------|------|---------|
| `0x80370100` / `0xC0370100` | `HCS_E_TERMINATED_DURING_START` | Container crashed during startup |
| `0x80370101` / `0xC0370101` | `HCS_E_IMAGE_MISMATCH` | OS version mismatch between container image and host |
| `0x80370103` / `0xC0370103` | `HCS_E_PROCESS_ALREADY_STOPPED` | Container process already exited but HCS didn't clean up — zombie state |
| `0x8037011F` / `0xC037011F` | `HCS_E_PROCESS_ALREADY_STOPPED` | Similar zombie state — distinct error code; both present identically in practice. (`0x80370103` is the expected HRESULT pair for `0xC0370103`; the relationship between `0x8037011F` and `0xC0370103` is unconfirmed — treat them separately.) |
| `0x80370106` / `0xC0370106` | `HCS_E_UNEXPECTED_EXIT` | Container exited unexpectedly |
| `0x80370109` / `0xC0370109` | `HCS_E_CONNECTION_TIMEOUT` | HCS operation timed out — vmcompute overloaded |
| `0x8037010E` / `0xC037010E` | `HCS_E_SYSTEM_NOT_FOUND` | Containerd referencing unknown container — race condition |
| `0x8037010F` | `HCS_E_SYSTEM_ALREADY_EXISTS` | Stale HCS state from previous containerd instance |
| `0x80370110` / `0xC0370110` | `HCS_E_SYSTEM_ALREADY_STOPPED` | Stopping already-stopped container — usually benign |
| `0x80370118` / `0xC0370118` | `HCS_E_OPERATION_TIMEOUT` | Operation exceeded internal timeout — HCS performance issue |
| `0x8037011E` / `0xC037011E` | `HCS_E_SERVICE_DISCONNECT` | vmcompute crashed/restarted — all containers affected |
| `0x800705AA` | Insufficient system resources | Resource exhaustion creating network compartments |

**Note**: Error codes appear in both HRESULT (`0x8037xxxx`) and NTSTATUS (`0xC037xxxx`) forms in logs. Match both patterns.

## HNS Error Codes

**⚠️ There is no official HNS error code reference.** The codes below are assembled from community knowledge, AKS field experience, and CNI log patterns. Treat them as best-effort rather than authoritative.

HNS surfaces Win32 error codes in CNI log messages of the form `hnsCall failed in Win32: <message>`:

| Win32 Code | Message | Meaning |
|------------|---------|---------|
| `0x1392` (5010) | `The object already exists` | Stale endpoint blocking new creation — cleanup incomplete after previous deletion |
| `0x490` (1168) | `Element not found` | HNS endpoint/network was deleted or state was reset — reference to non-existent object |
| `0x57` (87) | `The parameter is incorrect` | Malformed HNS request — misconfigured CNI or credential spec |
| `0x5` (5) | `Access is denied` | Permission error during HNS operation |

## Windows Event IDs

| Event ID | Source | Meaning |
|----------|--------|---------|
| 2000 | Hyper-V Compute | Create compute system |
| 2001 | Hyper-V Compute | Start compute system |
| 2002 | Hyper-V Compute | Shut down compute system |
| 2003 | Hyper-V Compute | Terminate compute system |
| 2004 | Microsoft-Windows-Resource-Exhaustion-Detector | Resource exhaustion detected / low memory condition |
| 6008 | System | Unexpected shutdown (preceding crash/BSOD) |

**Note:** Windows Event IDs are **not globally unique** — the same numeric ID can appear under different providers with different meanings. When writing analysis logic, always key on **Event ID + Source** together.

**CCG (Container Credential Guard) Events** — used by gMSA:

| Event ID | Source | Meaning |
|----------|--------|---------|
| 1 | CCG | Credential retrieval success |
| 2 | CCG | Credential retrieval failure |
