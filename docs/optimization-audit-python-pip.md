# Optimization Audit: Python/pip Usage and `uv` Migration Feasibility

**Date:** 2026-03-05
**Scope:** VHD image building, CI/CD pipelines, node runtime boot (TTN)
**Goal:** Reduce wall-clock time by evaluating replacement of `pip` with `uv` and identifying related optimizations.

---

## Table of Contents

- [Executive Summary](#executive-summary)
- [A. VHD/Image Layer](#a-vhdimage-layer)
  - [Complete pip/pip3 Inventory](#complete-pippip3-inventory)
  - [Python Runtime Inventory](#python-runtime-inventory)
  - [Recommendations](#vhd-recommendations)
- [B. Pipeline Layer](#b-pipeline-layer)
  - [Python/pip in CI/CD](#pythonpip-in-cicd)
  - [Non-Python CI/CD Optimizations](#non-python-cicd-optimizations)
- [C. AgentBaker Logic — Go/Shell CSE Boot Path](#c-agentbaker-logic--goshell-cse-boot-path)
  - [Python on the Critical Boot Path](#python-on-the-critical-boot-path)
  - [CSE Injection Feasibility](#cse-injection-feasibility)
- [Impact Map](#impact-map)
- [Cold Start Strategy](#cold-start-strategy)
- [Breaking Change Audit](#breaking-change-audit)
- [Final Recommendation](#final-recommendation)

---

## Executive Summary

After an exhaustive search across the entire AgentBaker codebase — Packer templates, Azure DevOps pipelines, GitHub Actions workflows, Go templates, cloud-init artifacts, and CSE scripts — **Python/pip is a negligible contributor to wall-clock time**. The codebase is overwhelmingly Go + Bash + PowerShell.

- **Only 2 scripts** in the entire repo run `pip install`.
- **Zero** GitHub Actions workflows use `setup-python` or `pip install`.
- **Zero** `pip install` commands execute at node provisioning time (CSE).
- **No** `uv`, `pyenv`, `deadsnakes`, `pyproject.toml`, or `setup.py` files exist in the repo.

The single largest Python-related bottleneck is `pip install azure-cli` in `trivy-scan.sh`, which takes **5-15 minutes** on ARM64 builds. This is better fixed by switching to the native apt package than by adopting `uv`.

---

## A. VHD/Image Layer

### Complete pip/pip3 Inventory

Every instance of `pip install` or `pip3 install` in the repository:

#### 1. `vhdbuilder/packer/trivy-scan.sh` (lines 73-76) — Ubuntu 22.04 ARM64

```bash
apt_get_install 5 1 60 python3-pip
pip install azure-cli                         # unpinned!
export PATH="/home/$TEST_VM_ADMIN_USERNAME/.local/bin:$PATH"
CHECKAZ=$(pip freeze | grep "azure-cli==")
```

- **Stage:** VHD build time (Trivy security scan phase, runs on builder VM, NOT baked into VHD)
- **Package:** `azure-cli` (~200+ transitive dependencies)
- **Pinned:** No
- **Estimated time:** 5-15 minutes (ARM64 must compile C extensions like `cryptography`, `cffi`)

#### 2. `vhdbuilder/packer/trivy-scan.sh` (lines 98-101) — Flatcar/ACL/AzureLinuxOSGuard

```bash
python3 -m venv "/home/$TEST_VM_ADMIN_USERNAME/venv"
export PATH="/home/$TEST_VM_ADMIN_USERNAME/venv/bin:$PATH"
pip install azure-cli                         # unpinned, inside venv
CHECKAZ=$(pip freeze | grep "azure-cli==")
```

- **Stage:** VHD build time (Trivy scan phase)
- **Package:** `azure-cli` (same as above)
- **Pinned:** No
- **Estimated time:** 5-15 minutes

#### 3. `vhdbuilder/packer/test/linux-vhd-content-test.sh` (lines 1437-1447) — PAM tests

```bash
python3 -m venv .
source ./bin/activate
pip3 install --disable-pip-version-check -r requirements.txt
pytest -v -s --reruns 5 test_pam.py
```

Requirements (`vhdbuilder/packer/test/pam/requirements.txt`):
```
pexpect>=4.8.0,<4.9.0
pytest>=7.3.1,<7.4.0
pytest-rerunfailures>=14.0,<15.0
```

- **Stage:** VHD build time (PAM validation, CBLMariner/AzureLinux only)
- **Packages:** pexpect, pytest, pytest-rerunfailures (3 small packages)
- **Pinned:** Range-pinned
- **Estimated time:** 30-90 seconds

#### Context: How `trivy-scan.sh` installs Azure CLI across all OS/arch combos

| OS / Arch | Method | Time |
|-----------|--------|------|
| Ubuntu 22.04 ARM64 | ~~`apt python3-pip` + `pip install azure-cli`~~ → `apt-get install azure-cli` (Microsoft repo, `[arch=arm64]`) ✅ | ~30 sec |
| Ubuntu 24.04 ARM64 | `apt-get install azure-cli` (Microsoft repo, `[arch=arm64]`) | ~30 sec |
| Ubuntu 20.04/22.04/24.04 AMD64 | `apt-get install azure-cli` (Microsoft repo, `[arch=amd64]`) | ~30 sec |
| CBLMariner / AzureLinux | `rpm --import` + `dnf install azure-cli` | ~30 sec |
| Flatcar / ACL / AzureLinuxOSGuard | `python3 -m venv` + `pip install azure-cli` | 5-15 min |

The Microsoft apt repo already supports `[arch=arm64]` for Ubuntu 24.04. The Ubuntu 22.04 ARM64 path could use the same approach.

### Python Runtime Inventory

Python packages installed via OS package managers (apt/dnf) during VHD build:

| Package | File | Purpose |
|---------|------|---------|
| `python3` | `vhdbuilder/scripts/linux/ubuntu/tool_installs_ubuntu.sh:28` | BCC tools build dependency |
| `python3-distutils` | `vhdbuilder/scripts/linux/ubuntu/tool_installs_ubuntu.sh:35` | BCC tools build dependency (Ubuntu 22.04) |
| `python` | `vhdbuilder/scripts/linux/ubuntu/tool_installs_ubuntu.sh:30` | BCC tools (older Ubuntu, includes Python 2 reference) |
| `python3-pip` | `vhdbuilder/packer/trivy-scan.sh:73` | To pip install azure-cli (Ubuntu 22.04 ARM64 only) |

Python scripts deployed to VHDs (executed at provisioning time, NOT pip-installed):

| File | Purpose | Third-Party Imports |
|------|---------|---------------------|
| `parts/linux/cloud-init/artifacts/cse_redact_cloud_config.py` | Redact secrets from cloud-config.txt | `yaml` (PyYAML) |
| `parts/linux/cloud-init/artifacts/cse_send_logs.py` | Upload logs to Azure Wireserver | `urllib3` |
| `parts/linux/cloud-init/artifacts/aks-log-collector-send.py` | Upload collected log bundles | `urllib3` |
| `parts/linux/cloud-init/artifacts/aks-diagnostic.py` | Upload diagnostic logs to Blob Storage | `requests` |

All third-party libraries (PyYAML, urllib3, requests) are pre-installed on VHDs as OS packages — no pip install occurs at boot.

### VHD Recommendations

> **Actionable tasks extracted to:** [vhd-azure-cli-install-improvements.md](./vhd-azure-cli-install-improvements.md)

#### High Priority: Eliminate `pip install azure-cli` in `trivy-scan.sh`

**Ubuntu 22.04 ARM64 (line 71-80): ✅ DONE** — Backported the Microsoft apt repo approach already used for Ubuntu 24.04 ARM64. See [vhd-azure-cli-install-improvements.md](./vhd-azure-cli-install-improvements.md#1-ubuntu-2204-arm64--switch-from-pip-to-apt--done).

**Flatcar/ACL/AzureLinuxOSGuard (line 97-105):** If `dnf` is available on these images, use the RPM-based install path (lines 93-96). If not, `uv pip install --system azure-cli` would reduce the 5-15 min install to ~30-60 seconds. See [vhd-azure-cli-install-improvements.md](./vhd-azure-cli-install-improvements.md#2-flatcar--azurecontainerlinux--azurelinuxosguard--tracked-for-reference).

**Estimated savings:** 10-30 minutes per affected VHD build.

#### Low Priority: Use `uv` for PAM test dependencies

Replace `pip3 install -r requirements.txt` with `uv pip install -r requirements.txt` in `linux-vhd-content-test.sh`. Marginal improvement (~15-45 sec) on an already fast step.

#### Not Recommended: `uv` for Python version management inside VHDs

Python3 on VHDs comes from the OS package manager and is a dependency of `cloud-init` and `WALinuxAgent`. Replacing it with a `uv`-managed installation would break these dependency chains and violate the SRE guideline of cross-OS consistency.

---

## B. Pipeline Layer

### Python/pip in CI/CD

| Metric | Count |
|--------|-------|
| `actions/setup-python` steps across all workflows | **0** |
| `pip install` in any pipeline YAML file | **0** |
| `requirements.txt` referenced by CI agents | **0** |
| `actions/cache` for pip | **0** |
| `pyproject.toml` / `setup.py` / `setup.cfg` files | **0** |
| Python test suites (`pytest`, `flake8`, `black`, `mypy`) on CI agents | **0** |

The CI/CD infrastructure is entirely Go-centric:

- **Go tests:** `go test ./...` via `actions/setup-go@v6` (built-in module caching)
- **Go lint:** `golangci-lint-action@v9` (built-in caching)
- **Shell tests:** ShellSpec (Docker-based), ShellCheck (Go binary)
- **Windows tests:** Pester (PowerShell)
- **Protobuf:** `buf` CLI
- **Security:** CodeQL, ClusterFuzzLite

**There is no `uv` migration opportunity in pipelines.** No Python dependencies are installed on CI agents.

### Non-Python CI/CD Optimizations

These are the actual opportunities for pipeline speedup:

| Optimization | Current State | Proposed Fix | Est. Savings |
|-------------|---------------|-------------|-------------|
| Remove `setup-go` from `shellcheck.yml` | Installs Go 1.24 but only runs bash linting scripts | Remove `actions/setup-go` step entirely | 15-30 sec/PR |
| Docker layer caching for ShellSpec | `shellspec.yaml` rebuilds Docker image from scratch every PR | Use `docker/build-push-action` with GHA cache backend | 1-3 min/PR |
| Parallelize Windows Pester tests | `validate-windows-ut.yml` runs 3 test suites sequentially in one job | Split into 3-job matrix strategy | 30-60 sec/PR |
| Cache `hack/tools/bin/` | golangci-lint, cue, ginkgo, oras downloaded fresh each run | Add `actions/cache` for `hack/tools/bin/` directory | 30-60 sec/PR |
| Remove dead Makefile target | `generate-azure-constants` references non-existent `pkg/helpers/generate_azure_constants.py` | Delete the stale target | Cleanup only |

**Total estimated CI/CD savings: 2-5 minutes per PR** (none involving `uv`).

---

## C. AgentBaker Logic — Go/Shell CSE Boot Path

### Python on the Critical Boot Path

The CSE boot sequence invokes Python at exactly **one point** on the critical path:

```
VM Boot
  -> cloud-init (write_files drops scripts to /opt/azure/containers/)
  -> CSE (CustomScriptExtension) starts
       -> cse_main.sh
            -> python3 provision_redact_cloud_config.py   <-- ON CRITICAL PATH
            -> [provisioning logic continues...]
       -> cse_start.sh (error path only)
            -> python3 aks-log-collector-send.py          <-- NOT critical path
            -> python3 provision_send_logs.py             <-- NOT critical path
```

`provision_redact_cloud_config.py` runs early in every boot to sanitize cloud-config before logging. It adds approximately **0.5-2 seconds** to every node boot. It's fast because Python3 is already loaded by cloud-init moments before, and PyYAML is cached.

### Go Embedding Mechanism

Two Python files are registered in `pkg/agent/const.go` (lines 62-63) and loaded via `getBase64EncodedGzippedCustomScript()` in `pkg/agent/variables.go` (lines 34-35):

- `kubernetesCSESendLogs` -> `parts/linux/cloud-init/artifacts/cse_send_logs.py`
- `kubernetesCSERedactCloudConfig` -> `parts/linux/cloud-init/artifacts/cse_redact_cloud_config.py`

These are gzipped, base64-encoded, and delivered to the VM via cloud-init `write_files` in `parts/linux/cloud-init/nodecustomdata.yml`.

### CSE Injection Feasibility

**Injecting `uv` into CSE for on-the-fly provisioning: Not recommended.**

Reasons:
1. **No pip install happens at provisioning time.** All Python packages are pre-installed on the VHD.
2. **No log collectors or security scanners are pip-installed at boot.** The Python scripts are pre-baked into the VHD and re-delivered via cloud-init `write_files`.
3. **The `uv` binary would add ~20MB to VHD size** with zero corresponding benefit at provisioning time.
4. Adding it to `components.json` creates a new component to track and update via Renovate with no return.

### Potential TTN Optimization (non-`uv`)

| Optimization | Impact | Difficulty |
|-------------|--------|-----------|
| Replace `provision_redact_cloud_config.py` with pure bash (`sed`/`awk`) | Save ~0.5-1.5s per node boot by eliminating Python3 interpreter startup | Medium (YAML parsing in bash is fragile) |
| Rewrite `cse_send_logs.py` / `aks-log-collector-send.py` using `urllib.request` instead of `urllib3` | Eliminate third-party dependency; no TTN impact (error path only) | Easy |

---

## Impact Map

| Domain | Optimization | Est. Time Saved | Confidence | Uses `uv`? |
|--------|-------------|-----------------|------------|-----------|
| **VHD Build** | ~~Replace~~ Replaced `pip install azure-cli` with apt (Ubuntu 22.04 ARM64) ✅ | **5-15 min** per build | High | No (apt) |
| **VHD Build** | Replace `pip install azure-cli` with dnf or `uv` (Flatcar/ACL/OSGuard) | **4-14 min** per build | Medium | Only if dnf unavailable |
| **VHD Build** | Use `uv pip install` for PAM test deps | **15-45 sec** per build | High | Yes |
| **CI/CD** | Remove unnecessary `setup-go` from `shellcheck.yml` | **15-30 sec** per PR | High | No |
| **CI/CD** | Docker layer caching for ShellSpec | **1-3 min** per PR | High | No |
| **CI/CD** | Cache `hack/tools/bin/` across GHA runs | **30-60 sec** per PR | High | No |
| **CI/CD** | Parallelize Windows Pester tests | **30-60 sec** per PR | Medium | No |
| **Node Boot** | Replace `provision_redact_cloud_config.py` with bash | **0.5-1.5 sec** per boot | Medium | No |
| **Node Boot** | Inject `uv` into CSE | **0 sec** (no pip at boot) | N/A | N/A |

### Totals

| Domain | Savings with `uv` | Savings with better alternatives |
|--------|-------------------|----------------------------------|
| VHD Build | ~5-15 min (Flatcar path only) | **10-30 min** (apt/dnf for azure-cli) |
| CI/CD | 0 | **2-5 min** per PR (caching + parallelization) |
| Node Boot (TTN) | 0 | **0.5-1.5 sec** (bash rewrite) |

---

## Cold Start Strategy

If `uv` is adopted for the Flatcar/ACL pip install path or PAM tests:

### Option 1: Bake into ADO agent pool image (preferred)

- Add `uv` installation to the agent pool base image.
- One-time download (~20MB static binary).
- Available instantly for all pipeline runs.
- Zero cold-start latency.

### Option 2: Download at pipeline start (fallback)

```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

- Takes ~2-3 seconds (single static binary).
- Net positive even accounting for download time when replacing a 5-15 min `pip install azure-cli`.

### Option 3: Cache in Azure Storage (for test VMs)

- Upload `uv` binary to the same storage account used for VHD artifacts.
- Download in `trivy-scan.sh` before pip operations.
- Uses internal Azure network (fast, no egress cost).

### For production VHDs and CSE: Do NOT add `uv`

There is no `pip install` at provisioning time, so `uv` would be dead weight (~20MB across all SKUs, a new component to track, zero benefit at runtime).

---

## Breaking Change Audit

### `python2.7` — Legacy Dependency (High Risk)

**File:** `parts/linux/cloud-init/artifacts/setup-custom-search-domains.sh:10`

```bash
apt-get -y install realmd sssd sssd-tools samba-common samba samba-common python2.7 samba-libs packagekit
```

| Concern | Detail |
|---------|--------|
| EOL status | Python 2.7 has been EOL since January 1, 2020 |
| Ubuntu 24.04 | `python2.7` is **not in default repos** — this script **fails** on 24.04 nodes |
| Ubuntu 22.04 | Available via `universe` repo but may be removed |
| Azure Linux | Script uses `apt-get` — **cannot work** on Azure Linux/Mariner (cross-OS bug) |
| Purpose | Historical dependency of `samba`/`realmd`; modern versions (>=4.14) use Python 3 |
| **Action** | Remove `python2.7` from package list; test realm join on 22.04/24.04 without it |

### `crypt` Module — Deprecated in Python 3.11, Removed in 3.13 (Medium Risk)

**File:** `parts/linux/cloud-init/artifacts/cis.sh:13-15`

```bash
CMD="import crypt, getpass, pwd; print(crypt.crypt('$SECRET', '\$6\$$SALT\$'))"
if [ "${VERSION}" = "22.04" ] || [ "${VERSION}" = "24.04" ]; then
    HASH=$(python3 -c "$CMD")
else
    HASH=$(python -c "$CMD")
fi
```

| Concern | Detail |
|---------|--------|
| Deprecation | `crypt` deprecated in Python 3.11, **removed in Python 3.13** |
| Impact | Breaks VHD builds on any base OS shipping Python >= 3.13 |
| Timeline | Ubuntu 26.04 (April 2026) will likely ship Python 3.13+ |
| **Fix** | Replace with: `HASH=$(openssl passwd -6 -salt "$SALT" "$SECRET")` |
| Urgency | Should be fixed before Ubuntu 26.04 adoption |

### `python` (unversioned) Binary Name (Low Risk)

**File:** `parts/linux/cloud-init/artifacts/cis.sh:15`

```bash
HASH=$(python -c "$CMD")   # for Ubuntu < 22.04
```

| Concern | Detail |
|---------|--------|
| Status | `python` maps to Python 2 on Ubuntu <=20.04; doesn't exist on 22.04+ without `python-is-python3` |
| Impact | Only triggered for Ubuntu < 22.04 (approaching EOL) |
| **Action** | Remove this code path when Ubuntu 20.04 support is dropped |

### Third-Party Python Libraries (Low Risk)

| Library | Used By | Pre-installed Via | Risk |
|---------|---------|-------------------|------|
| PyYAML | `cse_redact_cloud_config.py` | Bundled with `cloud-init` on all Linux VHDs | Low |
| urllib3 | `cse_send_logs.py`, `aks-log-collector-send.py` | `python3-urllib3` system package | Low (could replace with stdlib `urllib.request`) |
| requests | `aks-diagnostic.py` | Pre-installed on VHDs | Low (on-demand diagnostics only, not boot-critical) |

### No `python-is-python2` Usage

No references to `python-is-python2` found anywhere in the codebase. The only Python 2 reference is the explicit `python2.7` package in `setup-custom-search-domains.sh`.

---

## Final Recommendation

**`uv` is not the right optimization lever for this repository.** The codebase has minimal Python/pip usage, and the few instances that exist are better addressed by:

1. **✅ Switch to OS package managers** (apt/dnf) for azure-cli in `trivy-scan.sh` — saves 10-30 min per VHD build. *(Ubuntu 22.04 ARM64 done; Flatcar/ACL/OSGuard pending)*
2. **Fix the deprecated `crypt` module** with `openssl passwd -6` in `cis.sh` — prevents breakage on Python 3.13+ / Ubuntu 26.04.
3. **Remove `python2.7`** from `setup-custom-search-domains.sh` — fixes a latent Ubuntu 24.04 failure.
4. **CI/CD caching and parallelization** (Docker layers, tool binaries, Pester matrix) — saves 2-5 min per PR.
5. **Rewrite `cse_send_logs.py` / `aks-log-collector-send.py`** to use stdlib `urllib.request` — eliminates third-party dependency.

The **only place** where `uv` adds genuine value is the Flatcar/ACL/AzureLinuxOSGuard path in `trivy-scan.sh` if no native Azure CLI package is available, reducing a 5-15 minute `pip install` to ~30-60 seconds.
