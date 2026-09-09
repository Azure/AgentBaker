---
name: renovate
description: >
  Review, triage, or configure Renovate in AgentBaker. Use for
  Renovate-authored pull requests that update parts/common/components.json,
  any Renovate-managed changes to that manifest, .github/renovate.json changes,
  component onboarding, missing Renovate PRs, datasource no-result errors, or
  questions about whether an AgentBaker component-manifest update is safe to
  merge. Do not use for ordinary dependency files outside
  parts/common/components.json or for external downloads changed only in
  provisioning code; use code-review for those cases.
---

# AgentBaker Renovate

Analyze Renovate dependency updates and configuration changes. Research the
upstream version range, trace AgentBaker consumers, validate every intended OS
and architecture, and provide an evidence-based recommendation.

## Required references

- For dependency updates and `parts/common/components.json`, read
  [Renovate architecture and risk](references/renovate-architecture.md).
- For `.github/renovate.json`, component onboarding, datasource errors, or
  missing updates, read
  [Renovate lookup and configuration](references/renovate-lookup.md).

## Dependency review workflow

1. Parse the diff and identify:
   - component and package names;
   - every old-to-new version;
   - affected and omitted OS releases and architectures;
   - `latestVersion` and `previousLatestVersion` rotation;
   - changed download locations, tags, or artifact names.
2. Classify each change as major, minor, patch, distro revision, or another
   scheme supported by the component. Do not force non-semver versions into
   semver.
3. Research the exact upstream range using authoritative releases,
   changelogs, commits, advisories, and package metadata. Capture breaking
   changes, fixes, CVEs, deprecations, defaults, dependencies, and kernel or
   runtime requirements.
4. Trace affected flags, configuration, APIs, systemd units, and install paths
   through:
   - `parts/linux/cloud-init/artifacts/`;
   - `staging/cse/windows/`;
   - `vhdbuilder/packer/`;
   - other direct consumers found by code search.
5. For material artifact growth, assess VHD size, build duration,
   provisioning and cache effects, and storage impact.
6. Validate affected `downloadLocation` and `downloadURIs` structures against
   `schemas/components.cue`, including required OS keys and fallback paths.
7. Verify artifacts exist for every intended OS and architecture. Do not treat
   a plausible URL pattern as proof.
8. Check VHD cache coordination. Determine whether the update rotates out a
   version still requested by AKS-RP, unless the component always uses the
   version baked into the VHD.
9. Check effective assignees and reviewers from all matching package rules and
   verify required PR gates. Do not recommend merging over a configured
   component owner. If no owner is configured, treat that as a configuration
   gap, identify the responsible team, and require its approval before
   recommending merge.

If no reliable upstream changelog exists, say so explicitly and recommend the
smallest appropriate manual validation. Do not manufacture release details.

## Risk classification

- **High**: breaking changes, major updates, boot/runtime/networking/GPU
  compatibility risk, missing artifacts, harmful cache rotation, or changed
  consumer contracts.
- **Medium**: non-critical minor updates, intentional partial OS coverage,
  deprecations, new defaults, or material VHD size impact.
- **Low**: verified patch or distro revision with compatible consumers,
  complete intended coverage, available artifacts, and safe cache rotation.

Risk classification is analysis, not automatically a code-review finding.
Report a defect only when the changed code supports a concrete failure mode.

## Review output

```text
## Package Update Analysis: <component-name>
**Version change**: X.Y.Z -> A.B.C (<update type>)
**Component criticality**: Critical / Important / Standard
**OS variants affected**: <list>
**OS variants not updated**: <list or "None - full coverage">

### Changes between X.Y.Z and A.B.C

| Change | Description | Risk |
|--------|-------------|------|
| <type> | <description> | Low / Medium / High |

### Overall Risk: Low / Medium / High
**Justification**: <evidence-based explanation>
**Recommendation**: Approve / Request more information / Require manual testing
```

For multiple components, repeat the analysis per component and then summarize
cross-component or shared-cache risks.

## Configuration and troubleshooting

For Renovate configuration changes:

1. Apply every guardrail in
   [Renovate lookup and configuration](references/renovate-lookup.md).
2. Determine the effective result of all matching package rules; do not review
   one rule in isolation.
3. Confirm each custom-manager expression owns the intended entries without
   overlapping another manager.
4. Validate the datasource and versioning scheme against real package
   metadata.
5. Prefer local dry runs or an isolated fork over experiments on the
   production default branch.

When diagnosing a missing update, distinguish persistent configuration defects
from transient upstream feed failures before proposing a change.
