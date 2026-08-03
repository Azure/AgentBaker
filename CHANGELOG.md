# Changelog

## E2E gallery image replication: convergent target regions

**Goal**: stop concurrent E2E runs from clobbering each other's gallery image
`publishingProfile.targetRegions`, which surfaces as `GalleryImageNotFound` on VMSS create.

**Findings**
- `targetRegions` is full desired state, not an atomic append. The previous
  GET -> append-my-region -> CreateOrUpdate loop made every writer compute a *different*
  desired state, so a stale write silently dropped another writer's region.
  `vhdbuilder/packer/replicate-captured-sig-image-version.sh` documents the same
  constraint: "SIG API requires specifying a complete set of replication targets".
- A lost update is only harmful when writers disagree. If every writer submits the same
  superset, concurrent writes are identical and interleaving stops mattering.
- The region set is not dynamic: every scenario `Location` is a compile-time literal and
  `E2E_LOCATION` is never overridden in any pipeline. The whole set is six regions.
- **#8893 already added a write-free path.** The VHD build pipeline publishes
  `{resourceId, version, regions}` metadata, and when `E2E_VHD_METADATA_FILE` is set
  `GetVHDResourceID` returns early and E2E never writes to the gallery at all. Both VHD
  builder pipelines enable it via `useVhdMetadataArtifacts: true`.
- The race therefore only survives in the six standalone E2E pipelines (`e2e.yaml`,
  `e2e-gpu.yaml`, `e2e-gpu-azurelinux.yaml`, `e2e-windows.yaml`, `e2e-tme.yaml`,
  `e2e-rcv1p-not-opted-in.yaml`), which resolve images by tag and take the legacy path.
  This change is a safety net for that path, not the strategic fix.
- **Windows images must not use the Linux region set.** No Windows scenario pins a location
  (0 `Location:` entries across all Windows scenario files), so they all run in the default
  region. Replicating ~30GB Windows images to the other five regions was a 6x storage cost
  for regions no Windows test can reach. The region set is therefore per-image-OS, which
  preserves convergence because every writer of a given image still computes the same set.
- Being listed in `targetRegions` does not mean the replica serves traffic;
  `RegionalReplicationStatus.State` must be awaited (kept from #8374).

**Failed attempts (discarded)**
- A two-pass design: a discovery-only `go test` pass computed the exact regions a run
  needed, wrote a metadata file, and the real pass consumed it read-only. It worked, but
  computed at runtime what is a compile-time constant and forced unrelated changes (VHD
  cloning that broke pointer-identity FIPS detection, a lazy RCV1P CSE build to avoid Azure
  side effects during discovery, seeding randomized VHD selection across processes).
  668 lines for the same Azure-side result as ~160.
- A narrowing retry (`canNarrowToRequiredRegion`) plus a self-healing repair inside
  `WaitForImageVersionReplicatedToRegion`. Both existed only to protect newly added code:
  review showed the narrowing reintroduced the very snapshot-derived write this change
  removes, and the repair could spend the VMSS context budget before the wait it guarded.
  Replaced by a single "did my own region make it?" check.

**Files changed**
- `e2e/config/regions.go` (new): region constants, `linuxE2ERegions` / `windowsE2ERegions`,
  and `(*Image).E2ERegions` backing both replication and scenario validation so the two
  cannot disagree.
- `e2e/config/azure.go`: `ensureReplication` now batches the whole desired set through
  `ensureTargetRegions` with a 409/412 re-read-and-merge retry; failure is only fatal when
  the caller's own region is missing. Removed `replicateImageVersionToCurrentRegion` and
  `replicatedToCurrentRegion`.
- `e2e/config/vhd.go`, `e2e/test_helpers.go`: `Image.Ephemeral` keeps runtime-captured
  per-test image versions single-region — they have one writer and are deleted on cleanup.
- `e2e/test_helpers.go`: fail fast when a scenario region is outside the E2E set.
- `e2e/scenario_test.go`, `e2e/scenario_gpu_managed_experience_test.go`: region literals now
  reference the constants so the list cannot drift.
- `e2e/config/azure_replication_test.go`: merge, convergence, ephemeral, and region tests.

**Next step**: confirm whether the `REPLICATIONS` pipeline variable already covers the six
E2E regions. If it does, images arrive pre-replicated, `missing` is always empty, and this
code never writes — which is the outcome to aim for. Longer term, move the six standalone
pipelines onto the #8893 metadata path so they stop writing entirely.
