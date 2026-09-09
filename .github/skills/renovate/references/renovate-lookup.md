# Renovate Lookup and Configuration

Use this reference from the `/renovate` skill when reviewing
`.github/renovate.json`, onboarding a component, or investigating why Renovate
did not create the expected update.

## Configuration guardrails

Verify all of the following:

1. The file is valid JSON: double quotes, no comments, and no trailing commas.
2. Later `packageRules` can override earlier rules. Put narrow rules after
   broad rules and reason about every matching rule.
3. Do not combine `matchUpdateTypes` and `allowedVersions` in one rule.
4. Minor updates remain disabled unless a narrow rule explicitly enables them.
5. JSON-embedded regular expressions preserve required escaping.
6. Handlebars tokens such as `{{{newValue}}}` and `{{#if ...}}` remain intact.
7. Grouped rules have consistent, valid assignees and reviewers.
8. `separateMinorPatch` is `true` when patch and minor policies differ.
9. Custom-manager `matchStrings` do not overlap; exactly one manager should
   own each component entry.
10. `autoReplaceStringTemplate` preserves the `depType` conditional used to
    rotate `previousLatestVersion`.
11. Valid prerelease, timestamp, Debian revision, or RPM revision values are
    not accidentally filtered as unstable.
12. `recreateWhen` normally remains `"auto"`. Setting it to `"never"`
    suppresses newer versions after a PR is closed.
13. Hosted Renovate monitors the default branch. Feature-branch configuration
    changes do not affect the official run until merged.

## Onboarding by component type

### MCR container image

1. Add `registry=https://mcr.microsoft.com, name=<path>` immediately before
   `latestVersion`.
2. Confirm an existing custom manager matches it.
3. Add an ownership rule.
4. Add special versioning only when the tag format requires it.

### Ubuntu package

1. Add one entry per supported Ubuntu release using
   `name=<package>, repository=production, os=ubuntu, release=<version>`.
2. Confirm the release-specific Microsoft package feed publishes the package.
3. Add an ownership rule.

### Azure Linux RPM

1. Locate the package in the correct `base`, `cloud-native`, `ms-oss`, or
   `extended` repository.
2. Use
   `RPM_registry=https://packages.microsoft.com/azurelinux/3.0/prod/<category>/x86_64/repodata, name=<package>, os=azurelinux, release=3.0`.
3. Set `"ignoreUnstable": false` where valid RPM revisions would otherwise be
   filtered.

### OCI artifact

1. Use
   `OCI_registry=https://mcr.microsoft.com, name=<artifact-path>`.
2. Add `extractVersion` or regex versioning when tags include architecture or
   distribution suffixes.

## Diagnosing `no-result`

First distinguish a persistent configuration error from a transient feed
failure.

For Ubuntu packages, fetch the appropriate Microsoft package index and search
for an exact `Package: <name>` entry. For Azure Linux RPMs, fetch
`repomd.xml`, locate `primary.xml.gz`, and search its metadata for the exact
package name. For MCR or OCI artifacts, query the registry tag list.

Treat the failure as a configuration defect when:

- the package is consistently absent from the configured feed;
- the RPM repository category is wrong;
- the tag shape does not match the custom-manager expression;
- every run fails for the same component.

Treat it as likely transient when:

- the package and target version are currently present at the configured URL;
- the datasource has produced successful updates recently;
- one run reports scattered failures across otherwise working feeds.

Do not rewrite configuration to compensate for a transient upstream timeout.

## Diagnosing a missing PR

Check in this order:

1. `renovateTag` syntax, key order, and adjacency to `latestVersion`.
2. Whether the configured datasource actually contains a newer version.
3. Whether stability filtering hides the version.
4. Every matching `packageRules` entry and its final effective update policy.
5. Whether two custom managers match the same entry.
6. Whether a closed PR suppresses the version under `recreateWhen`.
7. Whether a user modified the branch and caused Renovate to stop managing it.
8. Mend.io Renovate logs for extraction, lookup, scheduling, or rate-limit
   details.

For an out-of-date managed PR, prefer Renovate's rebase checkbox. Manually
updating the branch can mark it as user-managed.

Renaming and closing a stuck PR can intentionally break Renovate's match and
cause recreation, but do not use this casually: renaming a closed PR can also
produce an unwanted duplicate.

Entries sharing a `renovateTag` and `groupName` are normally bundled into one
branch. Splitting them requires distinct matching rules and creates ongoing
maintenance; do not recommend it without weighing that cost.

## Safe testing

The hosted app acts only on the repository default branch. Before merging a
configuration experiment, use local dry-run support where practical or test
the reduced configuration in a personal fork connected to a separate
Renovate app installation. Do not test by allowing the production app to
create branches speculatively.

## Useful sources

- Mend.io dashboard: `https://developer.mend.io/github/Azure/AgentBaker`
- Ubuntu feeds:
  `https://packages.microsoft.com/ubuntu/<version>/prod/dists/<codename>/main/binary-amd64/Packages`
- MCR tags: `https://mcr.microsoft.com/v2/<image-path>/tags/list`
- Azure Linux metadata:
  `https://packages.microsoft.com/azurelinux/3.0/prod/<category>/x86_64/repodata`

If GitHub API access is unavailable, inspect an active Renovate branch with
git:

```bash
git ls-remote origin "refs/heads/renovate/*"
git fetch origin renovate/<branch> --quiet
git diff origin/main...origin/renovate/<branch> -- parts/common/components.json
```
