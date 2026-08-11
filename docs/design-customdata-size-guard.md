# Design: CustomData Size Guard for Hotfix PRs

**Author**: Abigail Liang  
**Status**: Draft
**Date**: 2026-08-05

---

## 1. Problem Statement

VMSS CustomData has a hard limit of **87,380 bytes** (base64 encoded). When a hotfix injects modified CSE scripts into `nodecustomdata.yml` via the scriptless path, CustomData can exceed this limit, causing node provisioning failures (`FailedPrepareContainerDatamodel`).

A **second, independent** limit also applies: the CSE extension **protectedSettings** (~65,535 bytes, a CRP hard limit) carries the NBC command — including customer CA certs — in the Mode B fallback. See §5–§6 for how H and R map to these two separate limits and the two guards that cover them.

### Current Mitigations

| PR | What it does | Limitation |
|----|-------------|-----------|
| #9045 | Dynamic fallback: if Mode A CustomData > 87,380, switch to Mode B | Mode B still contains hotfix scripts |
| #9079 | Moved NBC cmd + AKSNodeConfig to CSE protectedSettings (Mode B) | Hotfix script content remains in CustomData |

**Gap**: If the hotfix script injection (H) is large enough, **Mode B also exceeds the limit** — and there is no further fallback.

---

## 2. Formula

```
C = 87,380 bytes (VMSS CustomData limit, base64 encoded)
P = Platform-owned CustomData (boothook template, nodeCustomData rendering, enabled_features, cseDownloader)
R = Customer-controlled inputs (Custom CA certs, HTTP proxy CA, custom kubelet config, custom Linux OS config)
H = Hotfix-introduced additional size (injected scripts in nodecustomdata.yml)

Constraint:  P + H + R ≤ C

Or equivalently:  H ≤ C − max(P + R)
```

Where `max(P + R)` represents the worst-case production scenario (customer with maximum custom CAs + proxy config).

---

## 3. Encoding Chain

The final CustomData value sent to the VMSS API goes through multiple encoding stages:

### Scriptless Path (Mode B — the fallback with no further safety net):

```
Individual files:
  raw content → gzip → base64 = "encodedFile"

Final CustomData:
  boothookTemplate (plaintext shell)
    + encodedFiles (each is base64(gzip(content)))
    + cseDownloaderTemplate (plaintext shell)
  → base64
  = final string checked against 87,380
```

### Where H lives:

```
nodeCustomData = render(nodecustomdata.yml)
                        ↓
              {{if EnableScriptlessCSECmd}} section
              contains hotfix-injected write_files blocks
                        ↓
              gzip → base64 → embedded in boothook
                        ↓
              boothook → base64 = final CustomData
```

**H is present in BOTH Mode A and Mode B** because `nodeCustomData` (which contains the hotfix scripts) is included in both modes' CustomData.

---

## 4. Current Architecture

```
getLinuxNodeBootstrappingPayload()
  │
  ├─ supportsScriptlessPhase2() == true
  │   └─ getScriptlessBoothook()
  │       ├─ Render "slim" CustomData (nodeCustomData + hotfix.json + features + cseDownloader)
  │       ├─ Render "full" CustomData (slim + NBC cmd + AKSNodeConfig + serviceStart)
  │       │
  │       ├─ if len(full_base64) < 87,380 → return full (MODE A)
  │       └─ else → return slim (MODE B), put NBC cmd in CSE protectedSettings
  │
  └─ else (traditional path)
      └─ Full nodecustomdata.yml with all scripts (always large, but H is not additive)
```

### Key insight:

- **Traditional path**: H doesn't matter (scripts are always there, hotfix just replaces content)
- **Scriptless path**: H is additive (normally no scripts in CustomData; hotfix injects them)
- **Mode B has no fallback**: if slim CustomData > 87,380, provisioning breaks silently

---

## 5. Two Guards (Implemented)

Investigation of the Mode B fallback revealed that **H and R end up in different Azure fields**, so a single CustomData check is insufficient. Two independent guards are implemented in
`pkg/agent/customdata_size_guard_test.go` (plain `Test*` funcs so they run under `go test -run`).

### Guard 1 — `TestCustomDataSizeWithHotfix` (H vs CustomData)

- Builds a scriptless config and forces Mode B (`config.ScriptlessCSEProvisionMode = true`) so
  `getScriptlessBoothook` early-returns the **slim** CustomData (`P + H`).
- Asserts `len(slim) < MaxCustomDataLength` (87,380).
- In the hotfix CI this runs **after** `hotfix_generate.py` injects the real scripts into
  `nodecustomdata.yml`; because the template is `//go:embed`'d, `go test` recompiles and measures
  the **actual injected H** — no synthetic H is needed.

### Guard 2 — `TestProtectedSettingsSizeWithWorstCaseCerts` (R vs protectedSettings)

- Builds a worst-case customer-input (`R`) config and renders the Mode B CSE command via
  `getNodeBootstrappingCmd` (the string RP places into the extension `protectedSettings`).
- Asserts `len(cseCmd) < protectedSettingsMaxLength` (65,535, the CRP hard limit), and **warns**
  (without failing) once it crosses a 60,000 soft margin.

```yaml
# In hotfix-generate.yml, AFTER hotfix generation:
- name: Validate CustomData / protectedSettings size
  run: go test ./pkg/agent/ -run 'TestCustomDataSizeWithHotfix|TestProtectedSettingsSizeWithWorstCaseCerts' -v -count=1
```

> Guard 1 is the true hotfix gate (H is hotfix-controlled). Guard 2 is **orthogonal to the
> hotfix** (R never changes with a hotfix — see §6) but is co-located as a standing invariant.

---

## 6. Where H and R Actually Land (corrected model)

The original formula `P + H + R ≤ C` assumed everything shares one CustomData budget. **In Mode B
that is false:**

| Component | Lives in (Mode B) | Azure limit |
|-----------|-------------------|-------------|
| `P` platform slim + `H` hotfix scripts | **CustomData** (slim boothook) | 87,380 base64 bytes (`baker.go MaxCustomDataLength`) |
| `R` customer certs / kubelet config (inside the NBC/CSE command) | **CSE protectedSettings** | ~65,535 bytes (CRP hard limit) |

The certs (`CUSTOM_CA_CERT_*`, `HTTP_PROXY_TRUSTED_CA`) are emitted by `cse_cmd.sh`, i.e. the
**NBC command** — which Mode B moves out of CustomData into protectedSettings
(`getNodeBootstrappingCmd` → `getBase64EncodedGzippedCustomScriptFromStr(getScriptlessNBCCmd(...))`).
So **Mode B CustomData overflow is driven by H, not R**, and **protectedSettings overflow is driven
by R, not H**. Two limits ⇒ two guards.

### RP validates cert COUNT, not byte size

| Input | RP limit | Enforced in | Kind |
|-------|----------|-------------|------|
| Custom CA trust certs | **10** unique certs | `customcatrustvalidator.go` `maxAllowedCertsAmountInSecurityProfile` | count only |
| HTTP proxy `trustedCa` | **20** certs in the PEM bundle | `httpproxyconfigvalidator.go` `maxAllowedTrustedCACerts` | count only |
| protectedSettings **byte size** | *(none)* | — | **not validated by RP** |

`totalInputLen` is logged but never checked. So worst-case `R` = 10 CA certs + a 20-cert proxy
bundle, at a *chosen* per-cert size (there is no RP byte cap). The fixture uses **30 distinct
4096-bit self-signed certs** (~2 KB PEM each) as a conservative upper bound, stored as static
testdata (`pkg/agent/testdata/customdata_size_guard/`) so the guard is fast and deterministic.

**Update (2026-08-10)**: Lily Pan's PR #16755346 adds RP-side **byte size validation** for CA certs
(proposed limit: 10 KB for new clusters). Once merged, worst-case R will have a deterministic upper
bound, significantly reducing protectedSettings overflow risk. Guard 2 fixture should be updated to
reflect the new RP limit.

### Empirical worst-case protectedSettings size (RP count-max R)

Measured through the real double `gzip+base64` pipeline via `getNodeBootstrappingCmd`:

| Fixture (10 CA + 20 proxy, RP-max) | protectedSettings CSE cmd | vs 60,000 soft margin | vs 65,535 CRP hard limit |
|---|---|---|---|
| 2048-bit real certs (common case) | ~36,832 B | ✅ pass | ✅ |
| **4096-bit real certs (realistic worst)** | **~64,736 B** | ❌ exceeds margin | ✅ **only ~800 B headroom** |

---

## 7. Open Questions for Review

1. **Threshold margin**: Should we use a hard 87,380 check or leave a safety margin (e.g., 80,000)?
2. **Should Mode B also have a fallback?** (e.g., strip hotfix scripts and rely on VHD-baked versions — accepting that the hotfix won't apply on those nodes)
3. **Should we alert or just fail the PR?** If the hotfix is critical (security fix), blocking the PR may not be acceptable.
4. **Lily's cert size validation**: Once PR #16755346 lands, Guard 2 fixture should use 10 KB as worst-case R instead of unbounded cert sizes.

---

## 8. Related Code References

| File | Relevance |
|------|-----------|
| `pkg/agent/baker.go:145-227` | `getLinuxNodeBootstrappingPayload` + `getScriptlessBoothook` |
| `pkg/agent/baker.go:29` | `MaxCustomDataLength = 87380` |
| `pkg/agent/const.go:105,122` | hotfix JSON file paths |
| `hotfix/hotfix_generate.py` | Hotfix script injection logic |
| `.github/workflows/hotfix-generate.yml` | CI workflow to add the check |
| `aks-node-controller/hotfix.go` | ANC hotfix download + apply logic |
| `aks-node-controller/nodecustomdata.go` | `applyNodeCustomData` — writes hotfix scripts to disk |
| `parts/linux/cloud-init/nodecustomdata.yml:19` | `{{if EnableScriptlessCSECmd}}` block where hotfix injects |
| `parts/linux/cloud-init/artifacts/cse_cmd.sh:100-103` | Where R (Custom CA certs) is emitted |

---

## 9. Timeline

| Step | Owner | ETA |
|------|-------|-----|
| Design doc review | Nishchay | This week |
| Write Go test (`customdata_size_guard_test.go`) | Abigail | After review |
| Add CI step to hotfix-generate.yml | Abigail | After test |
| Update Guard 2 fixture after Lily's cert size PR lands | Abigail | TBD |
