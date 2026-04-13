# Work Routing

How to decide who handles what.

## Routing Table

| Work Type | Route To | Examples |
|-----------|----------|----------|
| Windows VHD builds | Hockney | Packer configs, image caching, VHD configuration scripts |
| Windows CSE scripts | Hockney | PowerShell CSE, provisioning scripts, staging/cse/windows/ |
| Windows node provisioning | Hockney | Node bootstrap, container setup, Windows-specific components |
| Windows compatibility | Hockney | Bidirectional VHD/CSE compat, cross-version (2019/2022/2025) |
| Windows PR review | Hockney | Review Windows changes, PowerShell quality, regression checks |
| Linux VHD builds | Fenster | Packer configs, install-dependencies.sh, post-install scripts |
| Linux provisioning scripts | Fenster | cloud-init artifacts, CSE helpers, cse_main.sh, cse_start.sh |
| Cross-distro compatibility | Fenster | Ubuntu vs Azure Linux/Mariner, apt vs dnf/tdnf, distro-specific |
| Linux components | Fenster | parts/common/components.json (Linux entries), Renovate updates |
| Linux PR review | Fenster | Shell script quality, shellcheck, ShellSpec, backward compat |
| Shared components | Hockney + Fenster | parts/common/ changes affecting both OS families |
| Unit test coverage | Kujan | Go tests, ShellSpec tests, snapshot tests, test gaps |
| E2e test strategy | Kujan | When e2e is needed, scenario design, validator logic |
| Test infrastructure | Kujan | Makefile test targets, CI test config, test helpers |
| Code style & readability | Keaton | Naming, comments, function length, self-documenting code |
| Brittleness & coupling | Keaton | Implicit dependencies, cross-file variable leakage, fragility |
| Refactoring | Keaton | Structural improvements, duplication removal, decoupling |
| PR maintainability review | Keaton | Flag fragile patterns, misleading names, missing context |
| CSE provisioning performance | McManus | Critical path timing, logs_to_events, serial→parallel |
| VHD build performance | McManus | install-dependencies.sh timing, image caching, download speed |
| Package install overhead | McManus | apt/dnf cold start, dpkg lock contention, cache effectiveness |
| Parallelization opportunities | McManus | Background tasks, concurrent downloads, overlapping work |
| VHD-time vs provision-time | McManus | Moving work from CSE to VHD build for faster provisioning |
| Error handling & exit codes | Verbal | ERR_* codes, exit code governance, failure paths |
| Logging & diagnostics | Verbal | logs_to_events, service status logs, /var/log/azure/ |
| Retry reliability | Verbal | retrycmd_if_failure patterns, timeout bounds, retry logging |
| Production supportability | Verbal | Diagnostic breadcrumbs, log collection, failure reconstruction |
| Windows diagnostics | Verbal + Hockney | collect-windows-logs.ps1, networkhealth.ps1, debug tools |
| NPD health monitors | Verbal | Node Problem Detector checks, health conditions |
| Scope & priorities | Hockney or Fenster | Domain-specific decisions and trade-offs |
| Session logging | Scribe | Automatic — never needs routing |

## Issue Routing

| Label | Action | Who |
|-------|--------|-----|
| `squad` | Triage: analyze issue, assign `squad:{member}` label | Lead |
| `squad:{name}` | Pick up issue and complete the work | Named member |

### How Issue Assignment Works

1. When a GitHub issue gets the `squad` label, the **Lead** triages it — analyzing content, assigning the right `squad:{member}` label, and commenting with triage notes.
2. When a `squad:{member}` label is applied, that member picks up the issue in their next session.
3. Members can reassign by removing their label and adding another member's label.
4. The `squad` label is the "inbox" — untriaged issues waiting for Lead review.

## Rules

1. **Eager by default** — spawn all agents who could usefully start work, including anticipatory downstream work.
2. **Scribe always runs** after substantial work, always as `mode: "background"`. Never blocks.
3. **Quick facts → coordinator answers directly.** Don't spawn an agent for "what port does the server run on?"
4. **When two agents could handle it**, pick the one whose domain is the primary concern.
5. **"Team, ..." → fan-out.** Spawn all relevant agents in parallel as `mode: "background"`.
6. **Anticipate downstream work.** If a feature is being built, spawn the tester to write test cases from requirements simultaneously.
7. **Issue-labeled work** — when a `squad:{member}` label is applied to an issue, route to that member. The Lead handles all `squad` (base label) triage.
