# Renovate Architecture and Risk

Use this reference from the `/renovate` skill when reviewing Renovate-managed
changes to `parts/common/components.json`.

## Data flow

AgentBaker uses `parts/common/components.json` as its component manifest.
`.github/renovate.json` contains:

1. Package rules controlling update types, ownership, and automerge.
2. Custom managers that extract versions from `components.json`.
3. Built-in `docker`, `rpm`, and `github-releases` datasources, plus custom
   datasources for Microsoft and NVIDIA package feeds.

The VHD build caches the selected versions. AKS-RP later requests component
versions while provisioning nodes. If the requested version is absent from the
VHD, provisioning downloads it at runtime and becomes slower.

## Manifest invariants

Validate changed `downloadLocation` and `downloadURIs` structures against
`schemas/components.cue`, including required OS-specific keys and fallback
paths.

`renovateTag` must immediately precede `latestVersion`; the custom-manager
regular expressions depend on that adjacency.

Common tag forms:

| Component type | `renovateTag` form |
|---|---|
| MCR image | `registry=https://mcr.microsoft.com, name=<image-path>` |
| Ubuntu package | `name=<package>, repository=production, os=ubuntu, release=<version>` |
| OCI artifact | `OCI_registry=https://mcr.microsoft.com, name=<artifact-path>` |
| Azure Linux RPM | `RPM_registry=<repodata-url>, name=<package>, os=azurelinux, release=3.0` |
| Disabled | `<DO_NOT_UPDATE>` |

For Azure Linux packages sourced from PMC RPM repositories, do not use the
Ubuntu `repository=production` form. The `RPM_registry` category (`base`,
`cloud-native`, `ms-oss`, or `extended`) must contain the package. A valid feed
in the wrong category produces `no-result`. Packages sourced from a dedicated
datasource must instead use the tag form matched by that manager; for example,
NVIDIA packages use `name=<package>, repository=nvidia, os=azurelinux, release=3.0`.

`<DO_NOT_UPDATE>` disables all future updates for the entry. It is not a way to
skip only one bad version.

Each version entry can contain:

- `latestVersion`: version selected for the next VHD.
- `previousLatestVersion`: prior version retained for rollback and cache
  compatibility.
- `k8sVersion`: Kubernetes minor version associated with the entry.

Check OS coverage across every applicable manifest path. For package entries, inspect:

- `downloadURIs.default.current`
- `downloadURIs.ubuntu.r2004`, `.r2204`, `.r2404`, and `.r2604`
- `downloadURIs.mariner.current` and `downloadURIs.marinerkata.current`
- `downloadURIs.azurelinux.current`, `."v3.0"`, `."DEFAULT/v3.0"`, and `."OSGUARD/v3.0"`
- `downloadURIs.azurelinuxkata.current`, `."v3.0"`, `."DEFAULT/v3.0"`, and `."OSGUARD/v3.0"`
- `downloadURIs.windows.default`, `.ws2022`, `.ws23h2`, and `.ws2025`
- `downloadURIs.flatcar.current`

For container images, also compare `amd64OnlyVersions`,
`multiArchVersionsV2`, and `windowsVersions`. For OCI artifacts, compare
`windowsVersions`.

Most Dalec-built `oss/v2/*` images use tags shaped like
`vMAJOR.MINOR.PATCH-REVISION`, but system-extension artifacts append a
distribution suffix and use dedicated rules. Match the component's actual tag
shape before choosing regex versioning. Azure Linux RPM suffixes such as
`-1.azl3` and timestamped image tags may be classified as unstable; relevant
rules need `"ignoreUnstable": false`.

## Cache and AKS-RP coordination

Most entries retain `n` and `n-1`. A new update changes:

```text
latestVersion:         n   -> n+1
previousLatestVersion: n-1 -> n
```

This removes `n-1`. Determine whether production AKS-RP code still requests
that removed version. Search production provisioning paths, not fixtures or
testdata. If AKS-RP requests a removed critical-path component, nodes still
work by downloading it, but provisioning latency regresses.

Pay particular attention when:

- multiple updates for the same component land in quick succession;
- AKS-RP is pinned during a release or stability freeze;
- kubelet, containerd, networking, or another large artifact is rotated out;
- the update changes both the upstream version and distro revision.

Distro revisions such as Ubuntu `u3` or Azure Linux `-3.azl3` are generally
lower risk than upstream major/minor changes, but only after confirming AKS-RP
does not explicitly pin the full revision.

Some components use whatever is already baked into the VHD and have no
AKS-RP-selected version. Confirm this by tracing the install path: if
provisioning uses an existing binary and does not negotiate or download a
requested version, AKS-RP cache coordination does not apply.

CI proves that the new artifacts can build and provision. It does not prove
that a version rotated out of the cache is no longer requested in production.

## Risk model

Treat these as high risk:

- major updates;
- major or minor updates to kubelet, containerd, runc, networking, GPU, or
  other boot-critical components;
- breaking behavior, removed flags, changed defaults, or new kernel/system
  requirements;
- missing architectures or OS releases;
- artifact naming or repository layout changes;
- rotations that remove a version still requested by AKS-RP.

Treat these as medium risk:

- minor updates to non-critical components;
- partial but intentional OS coverage;
- new features or deprecations that could change runtime behavior;
- material binary or image growth, after assessing VHD size, build duration,
  provisioning and cache effects, and storage impact.

Treat a patch or distro revision as low risk only when the evidence shows:

- bug or security fixes without breaking behavior;
- complete intended OS and architecture coverage;
- valid artifacts and unchanged consumer contracts;
- no harmful cache rotation.

Required gates and configured component-owner approval are merge conditions,
not substitutes for the review above.

## Key files

- `parts/common/components.json`
- `.github/renovate.json`
- `parts/linux/cloud-init/artifacts/README-COMPONENTS.md`
- `.github/README-RENOVATE.md`
- `schemas/components.cue`
- `parts/linux/cloud-init/artifacts/`
- `staging/cse/windows/`
- `vhdbuilder/packer/`
