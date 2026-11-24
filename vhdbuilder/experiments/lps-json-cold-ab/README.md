# Test-only JSON cold-cache A/B image

This isolated recipe is based on `39c49f59632fab867c8d12339db54fa075480586`.
Only Ubuntu 24.04 Gen2 amd64/containerd is qualified. Do not merge this recipe
into a release branch.

`prepare.py` froze seven current candidate image references to registry
index/amd64-manifest/config/layer descriptors and unpacked chain IDs, without
downloading layers. It removed selected repositories and known native aliases
from the shared cache input (`parts/common/components.json`). This also removes
their generated build prefetch entries. All unrelated entries are preserved.
`cache-input-record.json` records exact removed entries and before/after hashes.
Do not rerun preparation against the already-filtered input.

The normal trusted `enabled_features` map must set
`ENABLE_NETWORKING_IMAGE_CACHE=true` and `ENABLE_PROVISIONING_HOTFIX=false`.
JSON provisioning is required. Only the async arm additionally sets
`ENABLE_NETWORKING_IMAGE_CACHE_ASYNC=true`; absent/false remains synchronous.
Neither arm uses a hotfix binary or an NBC/scriptless-command replacement.
The selected ANC binary and exported JSON path/context are retained.

Both arms run the same read-only observer before prefetch and immediately before
kubelet service start, after sandbox-image import. Evidence is under
`/opt/azure/containers/lps-json-cold-ab/`:

* `before-prefetch.json`, `before-kubelet.json`: schema version 1, boot ID,
  wall/native monotonic/boottime timestamps, observation end timestamps,
  expected-inventory SHA256, exact stat-only content digest/sizes, each
  containerd namespace's image listing and snapshot info (including committed
  chain names/parents), and physical overlayfs snapshot IDs. `valid=false`,
  missing records, namespace failures, timeout, inventory mismatch, nonstandard
  root/snapshotter or boot mismatch must reject cold qualification.
* `worker-start.json`, `worker-end.json`: native timestamps, mode, worker PID,
  systemd invocation ID; end includes ANC/timeout exit code, effective worker
  exit code (`worker_exit_code`), and the real ANC
  prefetch outcome. ANC's fail-open CLI exit alone is not success.
* Async-only `launch-result.json` records the bounded launch result; timeout is
  ambiguous until worker/unit records are inspected. `unit-exit.json` records
  systemd `SERVICE_RESULT`, `EXIT_CODE`, `EXIT_STATUS` via `ExecStopPost`.
  While that unit still exists, the observer queries a fixed allowlist of
  effective native unit properties with a three-second timeout and retains
  them under `effective_unit.properties`. This survives later transient-unit
  garbage collection without retaining the unit or delaying CSE. No environment,
  command strings or unrecognized output is retained. Require both top-level
  `valid` and `effective_unit.valid`, complete/nonempty required properties,
  matching unit/invocation/boot identities and empty error lists; missing,
  partial, mismatched, nonzero or timed-out queries invalidate qualification.
  Native interval values are retained as systemctl strings, not invented
  numeric microseconds; monotonic timestamp fields are native microseconds.
* `source-commit`, `baked-sha256sums`, `expected-inventory.json`,
  `cache-input-record.json` provide baked source, binary and recipe identity.
  Qualify those independently against immutable build artifacts, not by trusting
  the first measured node. Actual ANC detail remains
  `/opt/azure/containers/image-cache-status.json`.

The async system service has independent lifetime, no kubelet ordering or wait
dependency, a 330-second runtime maximum and 10-second shutdown allowance.
The worker bounds ANC to 300 seconds plus a 10-second kill allowance. Exported
context stays root-only at `/run/aks-lps-json-prefetch/environment.sh`; never
collect that file, raw node configuration, tokens or environment in evidence.
Launch/worker/observation failures remain fail-open for provisioning, not for
experiment acceptance. A worker finishing before kubelet is an actual outcome,
not an implementation failure.

No running store is purged. Startup preload only reads local metadata/binaries.
The only startup archive import retained by this target is the unrelated
sandbox image; GPU imports and dependencies are unchanged. Shared layers may
remain. Verify selected complete graphs and final unpack absent using the
frozen graph plus before-prefetch metadata; quantify shared bytes. The
before-kubelet/final snapshots measure progress and may legitimately be warm
in either arm.
Neither BOM absence nor postboot store absence alone is cold-cache proof.

Focused source tests:

```sh
mkdir -p .test-tmp
TMPDIR="$PWD/.test-tmp" python3 -B -m unittest discover \
  -s spec/vhdbuilder -p lps_json_cold_ab_test.py -v
```

The focused suite also executes actual namespace, snapshot-list and committed
snapshot-info reads against a disposable user-owned local containerd daemon.
It does not touch the running host store, pull images or create any VM. This
qualifies local metadata-command behavior, not the future target node/image.
