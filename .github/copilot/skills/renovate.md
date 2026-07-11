---
name: renovate
description: >
  Use when reviewing, triaging, or configuring Renovate PRs in AgentBaker.
  Triggers: "review this Renovate PR", "is this package update safe",
  "triage dependency bump", "check component version change", "renovate PR
  analysis", "package update risk", "components.json version bump",
  "should I merge this Renovate PR", "what changed in this version",
  "dependency update review", "version bump risk assessment",
  "onboard component to Renovate", "configure renovate.json",
  "add auto-update for package", "debug renovate", "renovate not creating PR".
---

# Renovate PR Review & Configuration Skill

Analyse Renovate-generated package update PRs in AgentBaker. Determine risk, research upstream changes, assess OS coverage, and produce a structured review with actionable recommendation. Also handles Renovate configuration, onboarding new components, and debugging.

## Operating Principles

- **Every version bump matters.** Even patch versions can introduce regressions that affect production nodes. Never rubber-stamp.
- **Research before judging.** Look up upstream changelogs, release notes, and GitHub releases. Evidence > intuition.
- **OS coverage is critical.** Flag partial updates where some OS variants are bumped but others are not — inconsistency across node pools causes hard-to-diagnose issues.
- **Classify risk precisely.** Use the structured format below so reviewers can quickly assess and act.
- **VHD lifespan = 6 months.** Components baked into VHDs stay in production for months. Breaking changes in dependencies have long blast radius.
- **Work autonomously.** Fetch changelogs, check OS entries, verify download URLs. Don't stop at "I can't find info" — try multiple sources (GitHub releases, commit history, package registry).
- **Understand the two-file system.** `components.json` defines what to cache; `renovate.json` defines how Renovate monitors and updates those entries. Changes to one often require awareness of the other.

## Architecture: How Renovate Works in AgentBaker

AgentBaker uses a **custom manifest** (`parts/common/components.json`) instead of standard package files. Renovate is configured via `.github/renovate.json` with three major blocks:

1. **Package Rules** — control which update types are enabled/disabled, auto-merge policies, and assignees.
2. **Custom Managers** — regex-based parsers that extract version info from `components.json` using `renovateTag` markers.
3. **Custom Datasources** — define where to look up latest versions (MCR for containers, PMC for Ubuntu packages, RPM repos for Azure Linux).

### renovateTag Format

The `renovateTag` field in `components.json` tells Renovate how to find and update a version:

- **Container images**: `"renovateTag": "registry=https://mcr.microsoft.com, name=<image-path>"`
  - Example: `"registry=https://mcr.microsoft.com, name=oss/kubernetes/autoscaler/addon-resizer"`
  - `name` has no leading slash
- **Ubuntu packages**: `"renovateTag": "name=<pkg-name>, repository=<repo>, os=ubuntu, release=<version>"`
  - Example: `"name=moby-containerd, repository=production, os=ubuntu, release=22.04"`
  - `repository` is typically `production` for most packages
- **OCI artifacts (MAR)**: `"renovateTag": "OCI_registry=https://mcr.microsoft.com, name=<artifact-path>"`
  - Example: `"OCI_registry=https://mcr.microsoft.com, name=oss/binaries/kubernetes/kubernetes-node"`
- **Azure Linux RPM packages**: `"renovateTag": "RPM_registry=<repodata-url>, name=<pkg-name>, os=azurelinux, release=3.0"`
  - Example: `"RPM_registry=https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native/x86_64/repodata, name=containernetworking-plugins, os=azurelinux, release=3.0"`
  - The `RPM_registry` URL varies by package category (`base`, `cloud-native`, `ms-oss`, `extended`)
  - For Mariner 2.0: replace `azurelinux/3.0` with `cbl-mariner/2.0` in the URL
  - ⚠️ **Do NOT use the Ubuntu-style `repository=production` key for Azure Linux.** A tag like `name=azure-acr-credential-provider, repository=production, os=azurelinux, release=3.0` does not resolve against the RPM feed — Renovate silently finds no versions and never opens a PR. Azure Linux entries **must** use `RPM_registry=<repodata-url>` instead.
  - ⚠️ **The category (`base`/`cloud-native`/`ms-oss`/`extended`) in the `RPM_registry` URL must match the repo that actually publishes the package**, or the lookup returns `no-result` even though the version exists. Verify by fetching `repodata/repomd.xml` -> `primary.xml.gz` for the category and grepping `<name>...</name>`. (Real bug: `azure-acr-credential-provider` was pointed at `ms-oss`, which does not contain it — it actually lives in `cloud-native`. `dcgm-exporter`, `containernetworking-plugins`, and `kubernetes-cri-tools` for azurelinux 3.0 are also in `cloud-native`.)
- **Disabled**: `"renovateTag": "<DO_NOT_UPDATE>"` — Renovate ignores this entry. Use sparingly: this pins the entry forever, so Renovate will never propose *any* future version (not just the one you wanted to skip). Prefer closing individual PRs over `<DO_NOT_UPDATE>` when you only want to reject one version.

**Critical rule**: `renovateTag` must immediately precede `latestVersion` with no intervening keys. The regex parser depends on this adjacency.

### Version Schema

Each version entry supports:
- `latestVersion` (required) — the current latest version to cache
- `previousLatestVersion` (optional) — keeps the prior patch for rollback; Renovate auto-rotates: current `latestVersion` → `previousLatestVersion`, new version → `latestVersion`
- `k8sVersion` (optional) — ties entry to a specific Kubernetes minor version

### OS Variant Structure (Packages)

```
downloadURIs:
  default.current        — fallback for all OS if specific not defined
  ubuntu.r2004           — Ubuntu 20.04
  ubuntu.r2204           — Ubuntu 22.04
  ubuntu.r2404           — Ubuntu 24.04
  mariner.current        — Azure Linux 2.0 (reports as "mariner")
  azurelinux."v3.0"      — Azure Linux 3.0
  azurelinux."DEFAULT/v3.0"  — Azure Linux 3.0 non-OS Guard
  azurelinux."OSGUARD/v3.0"  — Azure Linux 3.0 OS Guard
```

### Current Update Policies (as of Jan 2025)

| Components       | Major  | Minor  | Patch  |
|-----------------|--------|--------|--------|
| runc, containerd | Manual | Manual | **Auto-merge** |
| Others           | Manual | Manual | Manual |

- All container images onboarded for auto-update
- PMC packages (moby-runc, moby-containerd) are auto-merge patch
- OCI artifacts (kubernetes-binaries, azure-acr-credential-provider) onboarded
- Minor updates disabled by default (noisy with multi-version components); enabled via scoped rules

### Dalec-Built Container Images

Images from `oss/v2/*` use static tags `vMAJOR.MINOR.PATCH-REVISION`. A dedicated package rule uses:
```
"versioning": "regex:^v(?<major>\\d+)\\.(?<minor>\\d+)\\.(?<patch>\\d+)-(?<prerelease>\\d+)$"
```
New `oss/v2/**` images are automatically covered by the wildcard rule.

### RPM Datasource Stability

Azure Linux RPM versions (e.g., `0.18.0-1.azl3`) are treated as "unstable" by default. For minor updates to work, the package rule must include `"ignoreUnstable": false`.

## When to Merge (and When Not to)

### Safe to Merge ✅
- **All PR gates pass** — `Agentbaker E2E`, `AKS Linux VHD Build - PR check-in gate`, `Agentbaker Windows E2E`, `AKS Windows VHD Build - PR check-in gate` all green
- **Patch update** of a non-critical component with no breaking changes in upstream changelog
- **Auto-merge components** (runc, containerd) — if gates pass, merge immediately; these are owned by Node SIG with high confidence in test coverage
- **Security patches** (CVE fixes) — prioritize merging quickly after gates pass

### Merge with Caution ⚠️
- **Minor version bumps** — verify upstream changelog for behavioural changes; consider whether downstream CSE scripts or systemd units depend on specific defaults
- **Partial OS coverage** — if only some OS variants are updated, confirm this is intentional (e.g., the component doesn't exist on the missing OS) before merging
- **Components with known fragility** — GPU drivers, networking (azure-cni, cilium), credential providers

### Version Caching and AKS-RP Coordination ⚠️

A version bump in `components.json` means that version will be cached in the **next weekly VHD release**. AKS-RP (the client that deploys nodes using these VHDs) must also be in sync with what's cached.

**Background — how the system works end-to-end:**
1. **VHD Build** (weekly): AgentBaker reads `components.json` and caches the listed component versions (binaries, container images) into the VHD during Packer build.
2. **VHD Release** (weekly): The built VHD is published as a Shared Image Gallery (SIG) image and becomes available for new node deployments.
3. **AKS-RP** (its own cadence, ~6-week cycles): When a customer creates/scales a node pool, AKS-RP tells the node which component versions to use. If that version is already cached in the VHD, provisioning is fast (no download needed). If not cached, the node must download it at provisioning time → **increased latency** (seconds to minutes depending on component size).
4. **The `n` and `n-1` strategy**: `components.json` caches both `latestVersion` and `previousLatestVersion` for most components. This means at any point, the VHD has two versions available. When Renovate bumps a version, the old `latestVersion` rotates to `previousLatestVersion`, preserving it. This gives AKS-RP a buffer period to transition.

**The coordination problem:**
- VHDs release weekly; AKS-RP releases on its own cadence (~6-week cycles)
- If a Renovate PR bumps a version and the rotation pushes out a version that AKS-RP still actively requests → nodes must download at provisioning time → **provisioning latency regression**
- Example timeline:
  - Week 1: VHD has `v1.5.3` (latest) and `v1.5.2` (previous). AKS-RP requests `v1.5.2`.
  - Week 2: Renovate bumps to `v1.5.4`. VHD now has `v1.5.4` (latest) and `v1.5.3` (previous). `v1.5.2` is gone.
  - If AKS-RP still requests `v1.5.2` (hasn't released yet) → every new node downloads `v1.5.2` at runtime.
- Conversely, if AKS-RP updates to request a newer version we haven't cached yet → also a mismatch causing downloads (though less common since VHD releases weekly, so it usually leads)

**Before merging, consider:**
1. **Is AKS-RP currently requesting this version (or about to)?** If yes, safe to cache.
2. **Will the bump remove a version that AKS-RP still needs?** The `previousLatestVersion` rotation helps (keeps `n-1`), but if AKS-RP is still on `n-2` or older, there's a gap.
3. **How to check what AKS-RP is requesting:**
- Check the AKS-RP repo: `https://dev.azure.com/msazure/CloudNativeCompute/_git/aks-rp` — search for the component name to see which version AKS-RP currently requests during node provisioning.
- If you have a local clone of `aks-rp`, update to the latest `master` branch before checking.
- Compare the version AKS-RP requests against what will remain in `components.json` after the Renovate bump (both `latestVersion` and `previousLatestVersion`).
- **Ignore testdata/fixture files** — these are test inputs, not production configuration. Only look at actual production code paths (e.g., provisioning logic, component version constants, configuration files used at deploy time).
- **Use ADO code search**: `https://almsearch.dev.azure.com/msazure/CloudNativeCompute/_apis/search/codesearchresults` or the ADO web UI to search for the component name in aks-rp.
- If the component version (including the `uX` suffix) is **not explicitly referenced** in aks-rp production code, the bump is safe from a coordination standpoint.

**Why CI passing is NOT sufficient to merge:**
CI gates validate that the VHD builds correctly and nodes can provision with the *new* cached versions. But they don't test the scenario where AKS-RP requests an *older* version that was just rotated out. That cache-miss path still "works" (the node downloads the component at runtime), so CI passes — but provisioning latency increases in production. This is why human/Copilot judgment about AKS-RP coordination is needed beyond green CI.

3. **How many Renovate bumps have stacked up?** If multiple patch bumps merged quickly (e.g., `v1.5.2` → `v1.5.3` → `v1.5.4` in consecutive weeks), the version AKS-RP needs may have been rotated out before they could release.
4. **Is this a critical-path component?** For kubelet, containerd, and networking components, a cache miss means slow provisioning. For optional/small components, the latency impact is minimal.

**Typical safe scenario:** Renovate bumps a patch version weekly. AKS-RP picks up the new version in their next release cycle. Because `n-1` is preserved, AKS-RP has at least one week's buffer. Since AKS-RP's integration tests run against the latest AgentBaker, they typically validate against the currently cached versions.

**Revision bumps (uX / -N suffix) are generally safe:** These are *revisions* — rebuilds of the same upstream version with distro-level fixes (e.g., security patches, dependency updates):
- **Ubuntu Debian packages**: revision is the `u3` in `1.34.0-ubuntu24.04u3` (format: `<version>-ubuntu<release>u<revision>`)
- **Azure Linux RPM packages**: revision is the `3` in `1.34.0-3.azl3` (format: `<version>-<revision>.azl3`)

AKS-RP typically does not pin to a specific revision number. If the revision is not explicitly referenced in AKS-RP production code, the bump is safe to merge. These are lower risk than actual patch/minor/major version changes.

**"Use what's baked" components don't need AKS-RP coordination:** Some components (e.g., `kubernetes-cri-tools`/crictl) are purely VHD-baked — the CSE script at provisioning time simply uses whatever binary is already installed on the VHD, with no version negotiation from AKS-RP. For these components, the version in `components.json` determines what gets cached during VHD build, and provisioning just uses it. AKS-RP never requests a specific version. You can identify these by checking the install function in CSE scripts — if it checks "is binary already present? → skip" without downloading, it's a "use what's baked" component. For these, Renovate bumps are always safe from an AKS-RP coordination standpoint.

**When to be careful:** When merging multiple bumps for the same component in quick succession, or when AKS-RP is known to be pinned to a specific older version for stability reasons (e.g., mid-release freeze).

### Do NOT Merge ❌
- **PR gates failing** — never override failing gates for Renovate PRs
- **Major version bumps** — require thorough review, upstream changelog analysis, and likely manual testing
- **Upstream changelog mentions breaking changes** — even for patch versions, block and investigate
- **Download URL validation failed** — the new version may not be available at the expected URL
- **Component owner has not approved** — if the PR has an assignee/reviewer configured in `renovate.json`, their approval is required before merging. The owner may know that a specific version is a bad release, not fully tested upstream, or incompatible with other components. Never merge over their head even if CI is green.
- **No assignee/reviewer configured at all** — some Renovate PRs have no owner assigned (missing `matchPackageNames` rule in `renovate.json` for that component). These PRs sit unreviewed indefinitely. Do not merge without first identifying the responsible team and getting their approval. Flag this as a configuration gap — the component should be added to a `packageRules` entry with appropriate `assignees`/`reviewers`.
- **Conflicts with in-flight releases** — if a VHD release is in progress, coordinate timing to avoid shipping untested versions

### General Rule
As long as PR gates pass, Renovate patch updates are generally safe to merge. The PR gates (E2E tests, VHD build checks) are designed to catch regressions. For `runc` and `containerd`, auto-merge is enabled because Node SIG has high confidence in the test coverage. For other components, the assigned owner must approve.

**Always wait for CI to pass — no exceptions.** Even when a version bump looks straightforward (e.g., a simple revision bump), never merge before all CI tests pass. We have caught real issues this way:
- New version available for `amd64` but not `arm64`
- Package published to Ubuntu PMC but not to the Azure Linux RPM repo
- Binary uploaded for one OS release but missing for another

These issues are invisible from the diff alone but are caught by VHD build gates that attempt to download and install on all architectures and OS variants.

## Workflow: Reviewing a Renovate PR

### 1. Identify the Change

Parse the diff (usually in `parts/common/components.json`) to extract:
- Component name (from `renovateTag` or package `name` field)
- Old version → New version (for each OS/release entry)
- Which OS variants are affected (Ubuntu 22.04, 24.04, Azure Linux 3.0, etc.)
- Which OS variants are NOT updated (flag inconsistency)
- Whether `previousLatestVersion` rotation looks correct

### 2. Classify Update Type

- **Major** (X.0.0): High scrutiny. Breaking changes likely.
- **Minor** (0.X.0): Medium scrutiny. New features may change behaviour.
- **Patch** (0.0.X): Lower scrutiny but still verify — regressions happen.

### 3. Research Upstream Changes

For each version bump, search for changelog information:
1. GitHub releases page of the upstream project
2. CHANGELOG.md in the upstream repo
3. Git commit log between old and new tags
4. Package registry metadata (MCR tags list, PMC package index)

Summarize each change with its own risk assessment.

### 4. Assess Component Criticality

Rate based on what the component does on AKS nodes:

- 🔴 **Critical** (node boot): kubelet, containerd, runc, azure-cni, aks-node-controller
- 🔴 **Critical** (GPU): nvidia-driver, nvidia-container-toolkit, dcgm-exporter, gpu-device-plugin
- 🟡 **Important** (networking/security): cilium, azure-vnet, ip-masq-agent, credential-provider
- 🟢 **Standard** (monitoring/utilities): node-exporter, retina, blobfuse, csi-drivers

### 5. Check for Configuration/API Changes

Verify the update doesn't change:
- CLI flags consumed by CSE scripts or systemd units
- Config file formats read during provisioning
- Default values that provisioning logic depends on
- Systemd unit behaviour or socket activation patterns

Cross-reference with scripts in:
- `parts/linux/cloud-init/artifacts/` (Linux CSE)
- `staging/cse/windows/` (Windows CSE)
- `vhdbuilder/packer/` (VHD build scripts)

### 6. Verify Download URL Validity

Confirm that `downloadLocation` and `downloadURIs` in components.json remain valid:
- Check if new version changes artifact naming convention
- Verify URL patterns still resolve (especially for `packages.aks.azure.com`)
- For OCI artifacts: confirm tag format matches `extractVersion` regex
- Flag if repository layout changed between versions

### 7. Produce Structured Review

Output MUST follow this format:

```
## Package Update Analysis: <component-name>
**Version change**: X.Y.Z → A.B.C (<major|minor|patch> update)
**Component criticality**: 🔴 Critical / 🟡 Important / 🟢 Standard
**OS variants affected**: <list all>
**OS variants NOT updated**: <list any missing, or "None — full coverage">

### Changes between X.Y.Z and A.B.C

| Change | Description | Risk |
|--------|-------------|------|
| <type> | <description> | 🟢 Low / 🟡 Medium / 🔴 High |

### Overall Risk: 🟢 Low / 🟡 Medium / 🔴 High
**Justification**: <1-2 sentences>
**Recommendation**: Approve / Request more info / Flag for manual testing
```

## Workflow: Onboarding a New Component to Renovate

### Container Images (MCR)
1. Add to `components.json` with correct `renovateTag`: `"registry=https://mcr.microsoft.com, name=<path>"`
2. The existing custom manager already monitors all MCR container images — no `renovate.json` change needed
3. Add a `packageRules` entry for assignees/reviewers
4. Test: set `latestVersion` to a known older version, run `npx renovate --platform=local --dry-run=true`

### Ubuntu Packages (PMC)
1. Add to `components.json` under appropriate OS releases with `renovateTag`: `"name=<pkg>, repository=production, os=ubuntu, release=<ver>"`
2. Separate entries needed per release (r2004, r2204, r2404) — each has its own PMC datasource URL
3. Existing custom managers cover Ubuntu; no `renovate.json` change needed unless new datasource required
4. Add assignee rule in `packageRules`

### Azure Linux RPM Packages
1. Add to `components.json` with `renovateTag`: `"RPM_registry=https://packages.microsoft.com/azurelinux/3.0/prod/<category>/x86_64/repodata, name=<pkg>, os=azurelinux, release=3.0"`
   - Replace `<category>` with the appropriate repo section: `base`, `cloud-native`, or `ms-oss`
2. May need `"ignoreUnstable": false` package rule if minor updates desired (AzureLinux suffixes like `-1.azl3` are classified unstable)
3. Ensure the RPM datasource URL covers the package

### OCI Artifacts (MAR)
1. Use `"renovateTag": "OCI_registry=https://mcr.microsoft.com, name=<artifact-path>"`
2. Add `extractVersion` regex in `packageRules` if tags contain architecture/distribution suffixes
3. datasource = `docker` (same as container images)

## Workflow: Debugging Renovate Issues

### Testing Workflow

Changes to `renovate.json` or `components.json` must be merged to AgentBaker's `main` branch for the official Renovate app to pick them up. Only after merging will you know if the configuration is valid and effective. If it doesn't work, you need to push another fix to `main`.

**Safer alternative — use a personal fork:**
1. Fork `Azure/AgentBaker` to your own GitHub account (e.g., `yourname/AgentBaker`)
2. Onboard your fork to https://developer.mend.io/ (install the Renovate GitHub App on your fork) — this won't affect the official repo and is only visible to your GitHub ID
3. Merge your `renovate.json`/`components.json` changes to the fork's `main` branch
4. Check the Mend.io dashboard or wait for Renovate to create PRs on your fork
5. **Pro tip**: Remove all other components from `components.json` (keep only the one you're testing) to isolate your changes and avoid noise from other components. Similarly, trim `renovate.json` to only the relevant custom managers and package rules.

This lets you iterate quickly without risking the production Renovate configuration.

### "Renovate failed to look up ... no-result"

A `no-result` warning means the datasource returned zero releases for that package. Two very different causes — diagnose before acting:

1. **Real, persistent config bug** — the package genuinely isn't reachable at the configured location. Confirm by fetching the feed yourself and grepping for the package:
   - Ubuntu/PMC deb: `https://packages.microsoft.com/ubuntu/<ver>/prod/dists/<focal|jammy|noble>/main/binary-amd64/Packages` (served as `application/octet-stream`; decode bytes as UTF-8, or gunzip if gzip magic `1f 8b`), grep `^Package: <name>`.
   - Azure Linux RPM: fetch `<RPM_registry>/repomd.xml` -> `primary.xml.gz`, grep `<name><pkg></name>`. **Most common real bug: wrong category** in the `RPM_registry` URL (e.g. `ms-oss` vs `cloud-native`) — the version exists but not in that repo.
   - If the package IS present at the configured URL, it's not a config bug -> cause #2.
2. **Transient PMC/upstream hiccup** — the large plain-text `Packages` datasource (and NVIDIA/PMC feeds) intermittently return errors/timeouts/partial bodies during a run, surfacing as `no-result` for a scattered subset of packages. These self-resolve on the next Renovate run. Signal it's transient: the package exists at its feed right now, the datasource has produced successful PRs recently, and only some (not all) packages sharing that datasource failed.

Rule of thumb: if you can fetch the feed and see the version, and the failures are a scattered subset across multiple datasources, treat it as transient and wait for the next run. If one specific package fails every run, it's a real config bug (usually wrong category/URL/tag format).

### "Why isn't Renovate creating a PR?"

1. **Check `renovateTag`** — must match expected format exactly (order matters: `registry=`, `name=` for containers; `name=`, `repository=`, `os=`, `release=` for Ubuntu packages; `RPM_registry=`, `name=`, `os=`, `release=` for Azure Linux RPMs)
2. **Run locally**: `npx renovate --platform=local --dry-run=true` with `$Env:LOG_LEVEL='trace'`
3. **Check stability**: RPM/deb versions with suffixes may be classified unstable — add `"ignoreUnstable": false`
4. **Check package rules**: a broader rule may be disabling the update type (specific-to-generic ordering matters)
5. **Verify datasource**: confirm the package exists in the datasource URL (check PMC endpoint, MCR tags list) — for Azure Linux RPMs, confirm the **category** (`base`/`cloud-native`/`ms-oss`/`extended`) is the one that publishes the package
6. **Check matchStrings regex**: `renovateTag` must immediately precede `latestVersion` with no intervening keys

### Common Issues
- **Minor updates not appearing**: Default `minor` disabled. Need explicit `packageRules` with `"matchUpdateTypes": ["minor"], "enabled": true` scoped to the component.
- **RPM minor updates blocked**: Add `"ignoreUnstable": false` for the RPM datasource match.
- **Two managers fighting**: Each component must be matched by exactly one custom manager. Verify `renovateTag` format uniqueness.
- **`previousLatestVersion` not rotating**: Check `autoReplaceStringTemplate` includes the `{{#if depType}}` conditional block.
- **"This branch is out-of-date with the base branch"**: In the PR description, check the checkbox labelled `If you want to rebase/retry this PR, check this box`. Renovate will rebase the branch with latest main within a few minutes and update the PR. Avoid clicking GitHub's `Update branch` button manually — Renovate treats that as you "taking over" the branch and will stop managing it.
- **PR stuck / not picking up latest version**: If a Renovate PR is stuck (e.g., not fetching the latest version despite it being available), it may be because the PR was previously modified by a user or other factors caused Renovate to stop managing it correctly. **Workaround**: rename the PR title, then close the PR. Renovate will detect the component needs updating and recreate a fresh PR with the latest version.
- **Closed PR never comes back, even for newer versions**: Caused by the global `recreateWhen: "never"` setting. `never` suppresses *every* closed/ignored PR permanently — so once you close a PR, that branch is blocked forever, including for newer versions that appear later (observed: CoreDNS stalled ~2 months after a close). Fix: use `recreateWhen: "auto"` (Renovate's default), where closing a PR only ignores that specific version and a fresh PR is created once a newer version appears. Only use `never` if you truly want closes to be permanent.
- **Renaming a *closed* Renovate PR's title spawns a duplicate**: Renovate matches its own PRs by branch name + expected title. If you rename a closed Renovate PR, the link breaks and Renovate opens a brand-new PR on the same branch (observed: renaming closed #8501 caused #8892 to be created minutes later). Don't rename closed Renovate PRs you want to stay ignored. (This is the same mechanism the "PR stuck" workaround above exploits on purpose.)
- **Grouped entries sharing one `renovateTag` come as a single bundled PR**: When multiple `components.json` entries share the same `renovateTag` and a `groupName`, Renovate collapses them onto one branch/PR (e.g. the 4 CoreDNS k8s-version lines all share one tag -> one `renovate/patch-coredns` PR). You cannot reject one line without rejecting the whole bundle, and Renovate may propose cross-line jumps (e.g. `1.13.1 -> 1.13.2`). Options: (a) manually take over the Renovate branch and set the versions you want — Renovate detects the edit and stops overwriting it (don't tick the rebase checkbox); or (b) split into per-line `packageRules` using `matchCurrentVersion` + `allowedVersions` regexes with distinct `groupName`s. Option (b) removes recurring toil but hardcodes version lines that must be updated manually when a k8s line rolls forward, and remember you cannot combine `matchUpdateTypes` with `allowedVersions` in the same rule (see guardrail #3).

## Risk Assessment Framework

### 🔴 High Risk — Block or flag for manual testing
- Major version bumps of any component
- Major/minor bumps of critical node-boot components (kubelet, containerd, runc). Patch bumps for `runc`/`containerd` follow the documented auto-merge policy when gates pass.
- Upstream changelog mentions breaking changes or behavioural changes
- Partial OS coverage for critical components
- Download URL pattern changes
- Removed CLI flags or config options used by CSE scripts

### 🟡 Medium Risk — Approve with caveats
- Minor version bumps of non-critical components
- Updates that only affect specific OS variants
- New features that could subtly change default behaviour
- Upstream changelog shows deprecations (even if not yet removed)
- VHD size impact > 10MB

### 🟢 Low Risk — Approve
- Patch version bumps with only bug fixes or security patches
- No breaking changes in upstream changelog
- Full OS coverage across all variants
- No configuration or API changes
- Component is not in critical boot path

## renovate.json Guardrails

When reviewing changes to `renovate.json` itself (not component bumps), verify:

1. Valid JSON (double quotes, no comments, no trailing commas)
2. **`packageRules` ordering is critical** — later rules override earlier ones for the same package. A broad rule placed after a narrow one will override it. Example: if minor updates are enabled for GPU images in an early rule but then disabled for all container images in a later rule, the GPU images won't get minor updates. Always place narrow/specific rules **after** broad rules, or verify override behavior. Source: [renovatebot/config-help#684](https://github.com/renovatebot/config-help/issues/684)
3. No combining `matchUpdateTypes` and `allowedVersions` in same rule (Renovate rejects this — see PR #8420)
4. `minor` updates disabled by default; only enabled via explicit narrow `packageRules`
5. Regex fields (`versioning`, `extractVersion`, `matchStrings`) have properly escaped backslashes for JSON strings
6. Renovate template tokens (`{{{newValue}}}`, `{{#if ...}}`, `{{/if}}`) intact — don't convert to JSON interpolation
7. `assignees`/`reviewers` lists consistent across grouped rules; update all related rules together
8. `team:<slug>` handles exist in AKS org with at least read permission to AgentBaker repo
9. `separateMinorPatch` must be `true` at root level to allow disabling minor while enabling patch
10. Custom manager `matchStrings` must not overlap — each component matched by exactly one manager
11. `autoReplaceStringTemplate` must use Handlebars syntax and preserve the `depType` hack for `previousLatestVersion` rotation
12. **Pre-release/timestamped versions**: Set `"ignoreUnstable": false` for packages whose valid versions have suffixes that Renovate classifies as unstable (e.g., GPU container images with timestamped suffixes, Azure Linux RPM packages with `-X.azl3` suffixes). Without this, Renovate silently skips those versions.
13. **Renovate only monitors the default branch** (main/master). Config changes on feature branches have no effect until merged.
14. **`recreateWhen` should be `"auto"`** (the default), not `"never"`. `"never"` permanently suppresses any closed PR — including newer versions of the same package — which silently stalls updates after any single close. Only set `"never"` if permanent suppression of all closes is genuinely intended.

## Dependencies

- **Mend.io Renovate Dashboard** — `https://developer.mend.io/github/Azure/AgentBaker`
  The hosted Renovate App UI for AgentBaker. Use it to:
  - View all detected dependencies and their update status (Dependency Dashboard)
  - Inspect Renovate run logs (up to DEBUG level) to understand why PRs were or weren't created
  - See rate-limiting, scheduling, and PR creation queue
  - Validate `renovate.json` configuration (syntax errors surface here)
  - Check which datasource lookups succeeded/failed
  - Review "what-if" scenarios before merging config changes
  Note: No true dry-run available via the hosted app — it will create/update branches and PRs. For dry-run use the local CLI instead.
- **Web search** for upstream changelogs, release notes, MCR tag lists
- **GitHub code search** for cross-referencing component usage in CSE scripts
- **`parts/common/components.json`** — source of truth for component versions and download URIs
- **`.github/renovate.json`** — Renovate configuration, package rules, custom managers, custom datasources
- **PMC endpoints** — `https://packages.microsoft.com/ubuntu/<ver>/prod/dists/<codename>/main/binary-amd64/Packages`
- **MCR tag lists** — `https://mcr.microsoft.com/v2/<image-path>/tags/list`
- **RPM repos** — `https://packages.microsoft.com/azurelinux/3.0/prod/cloud-native/x86_64/repodata`

## Key Files

- `parts/common/components.json` — component versions, download URLs, OS variant mappings, renovateTags
- `.github/renovate.json` — Renovate configuration, package rules, custom managers, custom datasources
- `parts/linux/cloud-init/artifacts/README-COMPONENTS.md` — schema documentation for components.json
- `.github/README-RENOVATE.md` — detailed Renovate configuration documentation and debugging guide
- `schemas/components.cue` — CUE schema definition for components.json
- `parts/linux/cloud-init/artifacts/` — Linux provisioning scripts that consume components
- `staging/cse/windows/` — Windows CSE scripts
- `vhdbuilder/packer/` — VHD build scripts that install components

## Inspecting Open Renovate PRs via Git

When GitHub API/CLI access is unavailable (e.g., SAML/SSO not authorized), you can inspect active Renovate PRs directly using git:

```bash
# List all active Renovate branches
git ls-remote origin "refs/heads/renovate/*"

# Fetch specific branches to inspect
git fetch origin renovate/patch-runc-containerd --quiet

# See what a branch changes vs main
git diff origin/main...origin/renovate/patch-runc-containerd -- parts/common/components.json

# Quick version-change summary (grep for latestVersion lines)
git diff origin/main...origin/renovate/patch-runc-containerd -- parts/common/components.json | grep -E '^\+.*latest|^\-.*latest'
```

**Interpreting results:**
- Each `renovate/*` branch corresponds to an open (or recently closed) PR
- Branches with **no diff against main** are stale (already merged or superseded) — can be cleaned up
- Branches showing a **version downgrade** (newer → older) indicate a Renovate bug (version comparison failure)
- Use the diff to identify: component name, old/new versions, which OS variants are affected, whether `previousLatestVersion` rotation looks correct
