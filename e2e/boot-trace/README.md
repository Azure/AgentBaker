# AKS node boot observatory

Experimental first-boot tracing for a **dedicated AKS Linux VMSS test pool**:
find work worth optimizing between Linux boot and NodeReady, including work
running in parallel. Capture with **ftrace on separate new nodes**, then
produce the same standalone, interactive HTML report.

**Experimental tooling, not a production image configuration.** Tracing changes
boot ordering and adds overhead. Validate the selected image, kernel and backend
together before interpreting timings. Captures and generated reports contain
operational metadata and must remain private unless reviewed and anonymized.

## AgentBaker integration

The supported image targets are **Ubuntu 26.04 Minimal Gen2 x64** and
**Azure Linux 3 Gen2 x64** (ftrace only). This experimental branch keeps its
tracing-oriented pipeline defaults; publishing source changes does not run
the manual VHD pipeline. Tracing E2E execution requires explicit selection.

### Prepared VHD and E2E path

The existing manual VHD pipeline `.pipelines/.vsts-vhd-builder-release.yaml`
provides an instrumented Ubuntu 26.04 Minimal Gen2 x64 build:
`2604minimalgen2containerd-boot-trace-ftrace`. It never replaces the normal
`2604minimalgen2containerd` image. Publishing-info generation, production
publication and prefetch optimization are disabled for this experimental path;
the pipeline still creates billable build VMs and separate versions in its
normal staging gallery when explicitly run. Standard VHD build/content checks
are retained. The tracing and standard-reference jobs inherit the pipeline's
normal cleanup settings. When enabled, the existing cleanup also purges eligible
image versions older than seven days across the shared staging gallery; it is
not limited to this experiment. The standard build helper may
recreate a failed staging gallery; the content test also deletes its temporary
resource group unless debugging is enabled.

The Ubuntu ftrace image and its uninstrumented Ubuntu reference are selected
by default on this branch; other image builds, including Azure Linux ftrace,
default to disabled. Review the selections in the pipeline run form.
Keep `build2604minimalgen2containerdBootTraceFtrace` enabled and enable
`runBootTraceE2E` for tracing E2E and the comparison report, and ensure
`SKIP_E2E_TESTS` is not `true`.
`useOverrides=false` does not require an aks-rp branch.
This automatically builds both tracing VHDs and runs the complete **21-capture
Ubuntu/Azure Linux CA matrix**, in three successive waves of seven fresh nodes.
The individual tracing-image checkboxes can stay disabled when this option is
selected; both required images are selected automatically. Parallel captures
help spot recurring patterns, but shared-cluster load can affect timings;
they are not a statistical performance verdict.
This selects the tracing E2E stage instead of the regular Linux E2E stage.
Without that option, the regular E2E stage still follows its normal setting;
set `SKIP_E2E_TESTS=true` if only building images.

The shared `bootTraceCanonicalBaseVersion` defaults to `26.04.202610280`, with URN
`Canonical:ubuntu-26_04-lts:minimal:26.04.202610280`. Check availability in the
actual `PACKER_BUILD_LOCATION` before launching; its value comes from the
pipeline environment. Marketplace availability does not verify the guest kernel
or tracing runtime.
Any override must also be an exact version, not `latest`.
The Ubuntu tracing job and the optional
`build2604minimalgen2containerd` reference use that version,
with publishing-info generation and prefetch optimization disabled like the
tracing images. These reference-job overrides remain in place on this
experimental branch even when tracing image builds are deselected. The reference retains
the normal Packer template for an uninstrumented timing comparison; the tracing
E2E and automatic report cover the three ftrace captures.
Template generation for the tracing images fails if Packer settings silently
override it, or if production naming/publishing or another OS/architecture is
selected. Normal builds retain their original Packer template.
`prepare_vhd.py` derives the instrumented template instead of duplicating it.

The image helper supports Ubuntu 26.04 x86_64 and Azure Linux 3.0 x86_64
(ftrace only), not other AgentBaker operating systems. It prepares tools after
the last builder reboot, before the package inventory. It only **enables**, never starts, the recorder
after final cleanup. ftrace kernel parameters are added through a GRUB drop-in
then, preserving Ubuntu's initrdless boot path. The main initramfs is not modified:
Ubuntu can skip it and load only the early microcode archive.
The helper refuses a changed kernel or an existing builder capture.
No bootconfig utility or kernel source download is required.
Prepared images stop ftrace collection at a sampled 512 MiB threshold.
Final flushing and metadata can exceed this threshold; binary ftrace pages are
never byte-truncated.
E2E archive transfer and extraction are bounded separately at 1 GiB.
ftrace finalization combines temporary per-CPU files into `trace.dat`, so
reserve at least twice the capture size in free guest disk space plus headroom.

| Component | Prepared version/source |
| --- | --- |
| Azure kernel | `7.0.0-1020-azure`, package `7.0.0-1020.21` |
| ftrace recorder | Ubuntu `trace-cmd=3.4-2`, with `libtraceevent1` and `libtraceevent1-plugin=1:1.9.0-2` |
| Report-job ftrace decoder | Ubuntu 26.04 `trace-cmd=3.3.3-1ubuntu2`; captures use trace.dat version 6 |

These are intentional experiment pins, not proof of runtime compatibility.
Recheck them together before changing the kernel. Missing pinned packages fail
the build rather than falling back to another version. Ubuntu 26.04's recorder
lacks `record -K`: it would reset early boot buffers. The ftrace-only preparation
therefore installs the three pinned Ubuntu 26.10 packages above from the official
Ubuntu archive after SHA-512 verification, with remaining dependencies resolved
from Ubuntu 26.04. It does not add another APT repository or change the kernel.

`runBootTraceE2E` defaults to **false**. Enabling **Run boot trace E2E and
comparison report** runs five Ubuntu variants (original, skipped CA update,
Rust, parallel retrieval, parallel retrieval plus Rust) and two Azure Linux
variants (original and parallel retrieval), repeated three times.
Concurrency is seven, waves are balanced, and scenario retries are disabled.
Separate scenarios are necessary because each capture needs its own fresh VM.
They are skipped in ordinary suites unless
`E2E_BOOT_TRACE=true` and exact VHD metadata are provided; they are excluded
from automatic NBC sampling and do not support PIS/VHD caching.

Boot-trace E2E runs build an isolated, test-only ANC binary that records
`CSE command start (cse_cmd.sh)` before certificate initialization. This also
applies when `--disable-scriptless-compilation` disables ordinary ANC builds.
Both AKSNodeConfig and NBC-command entry points are instrumented; normal
production templates and binaries are unchanged. These captures require
scriptless provisioning without ANC hotfix fixtures.
Building happens before VM creation; downloading the test binary adds boot
overhead before command entry and is recorded in the capture configuration.
The marker is stored in the boot-specific bundle as `cse-command-start.json`
using `/proc/uptime` (10 ms resolution, assuming no suspend).
Older captures cannot provide this marker retroactively. A new E2E capture,
not a VHD rebuild, is required; the later `cse_start.sh` entry remains distinct.
New E2E captures fail collection if the marker is missing or invalid; offline
analysis remains compatible with older bundles and reports its absence.

The tracing scenarios pin the test node's kubelet/kubectl to **1.36.5**,
matching the Ubuntu 26.04 VHD cache rather than downloading a newer version
during the measured boot. This applies to both NBC and AKSNodeConfig provisioning.
The shared latest-GA kubenet cluster and its control-plane version are unchanged.

### Azure Linux 3 ftrace image

The manual VHD pipeline also accepts
`buildAzureLinuxV3gen2BootTraceFtrace=true` (disabled by default). It builds
the separate staging-gallery definition `AzureLinuxV3gen2-boot-trace-ftrace`
using the standard Azure Linux Packer template, not the Ubuntu template.
The normal `AzureLinuxV3gen2` image and production publication are unchanged.

`bootTraceAzureLinuxBaseVersion` defaults to `3.20261105.01`, giving the URN
`MicrosoftCBLMariner:azure-linux-3:azure-linux-3-gen2:3.20261105.01`.
Check this non-FIPS x64 Gen2 image's availability in the actual build region.
`latest`, FIPS, Trusted Launch, ARM64 and non-ftrace backends are rejected for this experiment.
The running kernel must be on the Azure Linux 3 `6.6.*.azl3` track, expose the
required tracing configuration/events, and remain identical between preparation
and arming. Its exact release is recorded in the baked provenance.
No kernel package is installed or replaced by the tracing helper.

Azure Linux's packaged trace-cmd 3.2 cannot preserve the early boot buffers with
`record -K`. At image-build time the helper therefore compiles trace-cmd 3.4
and libtraceevent 1.9.0 from the upstream source archives carried by the official
Ubuntu archive. Their SHA-256 values are pinned from the corresponding signed
source-package descriptors; no Ubuntu binaries or APT repository are installed.
This explicit build-time source download is needed because the native package
lacks the required recorder option. Native `libtracefs-1.8.0-2.azl3` is used.
The new recorder and libtraceevent are isolated under `/opt/aks-node-boot-trace`;
an RPATH selects the private library without changing the system loader config.
Only the tracing service prepends that recorder's directory to PATH.
Newly installed top-level compiler/development packages are removed after the
build; packages already present on the standard builder are preserved.

The same bounded recorder, diagnostics, boot event set and trace.dat v6 format
are used. GRUB arming uses `/etc/default/grub` and `grub2-mkconfig`, preserving
the normal Azure Linux initramfs/boot path. The recorder is enabled only after
the final VHD cleanup and is not started on the builder.

For an **image-only** run, disable all standard image builds, enable only the
Azure Linux tracing build, leave `runBootTraceE2E=false`, and set
`SKIP_E2E_TESTS=true`. The existing VHD content checks still run.
For the full matrix instead, enable `runBootTraceE2E`: it selects both required
images and all Ubuntu/Azure Linux captures automatically.
Preparing an image alone does not establish its first-boot timings.

### Experimental Rust certificate updater (test-only)

`update-ca-certificates-rs/` contains a standalone Linux Rust rewrite of the
Debian/Ubuntu shell updater. It is **not installed or selected by the VHD build
or any production path**. The local comparison matrix below explicitly selects
it on the Rust variants. Azure Linux's `update-ca-trust` is a different tool and
is not replaced.

The executable has no Cargo dependencies. Certificate discovery, filename
mapping, symlink checks, orphan cleanup and bundle assembly run in-process.
It still invokes the system's `openssl rehash` when links change, optionally
`/sbin/restorecon`, and `run-parts --test` to select the original hooks. Hooks
receive additions before removals on stdin and `--fresh` when appropriate;
hook failures are reported without preventing subsequent hooks, as upstream.
The bundle is replaced atomically with mode `0644`; existing unrelated files
are not purged. This is not a transaction over the entire trust store: a failure
can leave changed symlinks, just as with the shell command.

```bash
cd e2e/boot-trace/update-ca-certificates-rs
cargo build --release --locked
cargo fmt --check
cargo clippy --all-targets --locked -- -D warnings
python3 -B tests/compatibility.py \
  --binary target/release/update-ca-certificates-rs \
  --reference /usr/sbin/update-ca-certificates -v
```

The differential suite uses temporary stores, generated test certificates and
the installed distro command; it never updates the host trust store. The
supported flags are `-v`/`--verbose`, `-f`/`--fresh`, `-d`/`--default`,
`--certsconf`, `--certsdir`, `--localcertsdir`, `--etccertsdir`, `--hooksdir`,
`--certbundle`, and `--sysroot`. Run `--help` for usage. Do not run without path
overrides on a real node until image-level compatibility has been validated.

Ordering matches the shell command under `LC_ALL=C` (byte ordering), not
locale-specific collation. Invalid arguments and filesystem, discovery or
rehash errors fail explicitly instead of reproducing shell-pipeline error
masking. Relative input paths are resolved before processing. Source-directory
symlink cycles fail explicitly. Paths containing newlines cannot be represented
by the hooks protocol and are rejected when adding a certificate. Selection
otherwise follows upstream, including its unnormalized basename on explicit
removal, duplicate bundle entries with `--default`, and retaining an existing
bundle when no certificates are selected. These compatibility quirks are not
security fixes and should be reconsidered separately, not silently changed
during a performance comparison.

This standalone derivative follows the
[Ubuntu/Debian implementation](https://git.launchpad.net/ubuntu/+source/ca-certificates/plain/sbin/update-ca-certificates?h=ubuntu/devel)
and is licensed **GPL-2.0-or-later**, separately from AgentBaker's MIT-licensed
code; its license is included in its own directory. No binary or benchmark
result establishes a node boot improvement until a new paired VM run is made.

### CA comparison matrix (ADO and local)

The ADO checkbox prepares all matrix inputs from this checkout. It compiles
the dependency-free Rust tool offline in the official Rust 1.85.1 Bookworm
container (pinned by digest), with static CRT linkage for both supported OSes.
`prepare_matrix.py` derives the instrumented scripts from the current provisioning
source and the checked-in historical serial control functions.
No local session files, external source checkout, expiring SAS URL or old
campaign resource is needed.

The CI-built binary is hashed, uploaded once to the trial's isolated E2E blob
storage and downloaded with the scenario VM's managed identity. Every variant
uses the same binary and verifies its SHA-256. Missing image metadata, caches
or fixtures fail explicitly. Resource-group and storage-account names include
the build ID and job attempt; no shared test infrastructure is repurposed.

The `boot-trace-matrix-inputs` artifact retains fixture inputs and JUnit results.
The `boot-trace-report` artifact contains the complete standalone `index.html`,
all individual reports and JSON summaries, and `ca-comparison-report.zip`.
All 21 timelines, threads, services, AKS.CSE tasks and measured CSE command-start
markers are embedded in `index.html`. Statistics and recurring CPU patterns
stay grouped by identical OS and variant, with same-OS original baselines.
Report generation also runs after E2E failures: missing/skipped/failed captures
remain visible, the report is marked incomplete, and the report step fails.
The previous `bootTraceCAExperiment` checkbox is superseded by this full matrix;
the unsafe skip control is always included only in its dedicated Ubuntu cases.

`E2E_BOOT_TRACE_MATRIX_DIR` opts into 21 fresh-node captures: three repetitions
of Ubuntu original, skipped CA update, Rust, parallel retrieval, and parallel
retrieval plus Rust; and Azure Linux 3 original and parallel retrieval.
Ordinary tracing scenarios remain unchanged when this variable is unset.
Parallel retrieval in `parts/linux/cloud-init/artifacts/init-aks-cloud.sh` also
applies to ordinary RCV1P provisioning, independently of tracing. The matrix uses
pinned original and parallel script copies for comparison; Rust and skipped CA
updates remain experiment-only.
The directory supplies `fixture.json` with the pinned `rust_sha256`,
`prepare-fixture.sh`, and instrumented `init-original.sh` / `init-parallel.sh`.
Exact image metadata, `E2E_BOOT_TRACE=true`, and an isolated
`--resource-group-prefix abe2e-ca-<unique>` are required. The infrastructure prefix
defaults to `abe2e` for existing suites.

For three balanced waves, add `--boot-trace-waves --parallel 7 --retries 0`
and select all 21 matrix scenarios. Wave 1 runs one fresh VM for each of the
seven OS/variant combinations; waves 2 and 3 repeat those same combinations.
The runner waits for every scenario in a wave, including collection and scenario
cleanup, before starting the next wave. A failed scenario is retained in the
final result and does not skip subsequent waves; suite cancellation stops further
waves. All waves share one runner invocation, infrastructure cache, log directory
and final JUnit report. Incomplete matrices, duplicate variants and NBC sampling
are rejected before Azure initialization. Without the flag, scheduling is unchanged.

Additional boothook fixtures are gzip-compressed and Base64-encoded, then restored
before the controller starts. Injection rejects a final CustomData payload above
Azure's 87,380-character encoded limit before attempting VM creation.

The same versioned internal Rust fixture is prepared on every variant before
the CA steps, using the test VM's managed identity and verifying its SHA-256.
This preparation is outside the measured CA spans but **is included in total
boot/NodeReady time**; these runs are not pristine-image timing measurements.
Collection verifies the actual guest script/binary hashes, requested mode,
nonempty downloaded CA set, and successful CA-step timing evidence. Bundle and
downloaded-certificate-set fingerprints are retained for subsequent comparisons.
The fixture does not replace the system command globally.

### Targeted diagnostics

The recorder captures network device activity alongside block I/O: TX
`net_dev_xmit` and RX `netif_receive_skb_entry`,
`netif_receive_skb_list_entry`, and `netif_rx_entry`. The report adds separate green TX and
blue RX lanes with event counts in 100 ms bins. These are not packet counts:
receive batching, multiple receive paths and offloads prevent that interpretation.
TX does not prove delivery, and neither lane measures DNS/TCP/HTTP latency or
network wait. Bastion/SSH traffic is included. Only device tracepoint metadata
is captured, not packet contents or IP/transport headers. Additional events
increase trace volume and overhead; check the existing loss and size warnings.
Older captures without these events show network coverage as unavailable, not
zero traffic. Rebuild the tracing VHDs and capture fresh boots to collect them.

The recorder samples containerd threads in uninterruptible sleep (D state) every
200 ms, discovering the daemon at most once per second until it appears.
`containerd-waits.json` retains bounded kernel-stack/wait-channel samples,
read/sync syscall numbers and descriptor IDs when available, and contemporaneous
descriptor paths/inodes/devices. No workload file contents, syscall buffers or
process command lines are read. Up to 128 wait intervals and 128 descriptors per
snapshot are retained in memory and written only after the recorder stops.
The `/proc` reads and sampling CPU time still perturb the experiment.
Short waits can be missed; snapshots narrow an investigation but do not by
themselves establish the file behind a block request or its backend latency.

Only the experimental images' baked `init-aks-cloud.sh` copy is instrumented,
after final VHD cleanup. The probe refuses an unexpected wrapper or a second
application. `cloud-timings.jsonl` retains the allowlisted certificate timings
used to validate that experiment. During capture, the recorder also samples the
Custom Script events directory and writes `cse-task-timings.jsonl` with bounded
`AKS.CSE.*` task names and monotonic start/end times mapped from `Timestamp` and
`OperationId`; timezone-free values are interpreted in the node's local timezone
(up to 2,048 event files and 512 task intervals). Event messages,
commands, arguments and other fields are never exported. The report displays
these tasks in gold with an independent checkbox filter; when the sanitized
events are unavailable, it falls back to the targeted certificate intervals and
reports that full task coverage is unavailable.

After tracing stops, the E2E collector makes a bounded, read-only CSR API request.
`bootstrap-csr.json` includes only CSRs whose subject matches the new node:
creation, approval/denial condition times, signer and certificate-issued status.
Requests, certificates, identities and condition messages are not exported.
Missing permissions/API failures are explicitly recorded as unavailable and
shown in the report; they do not invalidate the guest trace. This is a
post-Ready snapshot, not an apiserver audit log or proof of PodCIDR controller
timing. The HTML includes all targeted diagnostics in an expandable section.

The framework opens its Kubernetes list/watch **before** creating the VMSS.
It retains the first Ready event even if ProviderID arrives later, rejects watch
gaps/expired versions, old node UIDs, observed reboots and identity mismatches,
and disables both VMSS recreation retry loops for these trials.
It opens the existing Bastion/SSH transport as soon as the new VM/IP is
available, without waiting for ARM/CSE completion. Once Ready and SSH are both
available, it stops the owned recorder and streams a bounded tar archive
**before workload validation**. Archive extraction rejects traversal, links,
duplicates, unexpected files and excessive sizes/counts. Captures that ended
before Ready fail; incomplete archives are retained as diagnostics, not accepted
measurements. Never-Ready failures retain normal E2E diagnostics, not a validated
boot-to-Ready bundle; the guest watchdog remains the safety stop.

Successful bundles are under each scenario's `boot-trace/` artifact directory.
They include exact image provenance, VM/cluster/network settings, Ready evidence
and only the four recognized CSE boot datapoints, never full CSE output.
The report stage uses an Ubuntu 26.04 container, requires exactly three
successful ftrace bundles. It
rejects mismatched base/kernel/VM/cluster settings and reused node identities,
and writes the autonomous `boot-trace-report/comparison.html` plus each trial's
HTML/JSON. The comparison page keeps each capture selectable and reports
per-backend minimum, median and maximum boot-to-Ready times. It is not a
statistical performance verdict.
Second-resolution CSE timestamps can map slightly before the precise boot
origin: such milestones remain in the raw sanitized export but are excluded
from the rendered timeline with a warning, never silently clamped to zero.

The opt-in pipeline parameter `bootTraceCAExperiment=true` keeps exactly three
parallel ftrace nodes: trials 1 and 2 run `update-ca-certificates`, while trial 3
skips only that command in the instrumented image's `init-aks-cloud.sh`.
All three use the same tracing VHD; per-node cloud-init
files select the mode. The skip mode exists only in the instrumented image's copy;
standard images still update their trust stores. Retrieval and copying of CA
files still run, and TLS verification is
not disabled, so the skip node can fail provisioning because its new CAs are not
activated. This is diagnostic only, never a production configuration.
Readiness configuration records the mode, and guest timing evidence must confirm
the skip before a Ready capture is accepted. The report labels the experiment
and excludes it from repeat statistics. Two controls and one variant cannot
establish a statistically reliable speedup. Failure artifacts remain diagnostics,
not validated boot-to-Ready measurements.

For downloaded E2E artifacts, the same offline report recipe is:

```bash
python3 -B render_e2e.py /private/path/to/downloaded-artifacts --output reports/e2e-comparison
```

The report job launches its own Ubuntu 26.04 Docker container explicitly as root
for dependency installation, without requiring `sudo` in the minimal image.
Only the report output is writable; its ownership is returned to the agent user
even when rendering fails.
Legacy ftrace text is merged offline across CPUs with bounded memory and temporary
disk spooling. Ordering corrections are reported; timestamps and events are retained.
Backwards per-CPU timestamps, or reordering combined with lost/undecodable records,
fail explicitly rather than hiding ambiguous scheduler intervals.

VHD builds and Azure E2E execution require explicit authorization and an exact
base image version.

## What is measured

| Evidence | Output | Important limit |
| --- | --- | --- |
| Node Ready condition + guest clock samples | Linux boot to first observed Ready=True transition | Kubelet's `lastTransitionTime`, not exact API server acceptance |
| `sched_switch` | Observed CPU seconds per thread | Missing prefixes, lost events and discontinuities limit accounting |
| Wakeups and preemptions | Observed runnable wait per thread | Not all off-CPU time, disk wait or network wait |
| Forks and scheduler identities | TID generations, last observed thread name, observed span | Not guaranteed lifetime, process ancestry, or service membership |
| Block issue/complete events | Counts and a time-binned activity lane | **No request latency or per-thread I/O attribution yet** |
| `systemctl list-units/show` | Boot timestamps and lifecycle state for loaded service units | Most recent recorded invocation; unloaded/garbage-collected units and earlier restarts may be unavailable |
| Exported CSE boot datapoints | Kernel, Guest Agent, CSE and kubelet milestones | Optional local import, not automatic extraction of CSE internals |

The report inventories loaded systemd service units with boot timestamps rather
than maintaining a hand-picked service allowlist. It records process start/exit,
active/inactive timestamps and the collected unit state. The systemd CLI
properties describe the most recent invocation; earlier restarts are not a
complete history, and a oneshot that systemd unloaded before collection may be
missing. Per-unit query failures are shown as warnings. Journal collection
remains limited to the known cloud-init, Guest Agent, containerd and kubelet
units.

The timeline is ordered by process start and includes a per-service visibility
filter. Service names are selectable in the timeline labels. Turquoise service
bars represent only an observed active interval: `ActiveEnterTimestamp` to
`InactiveEnterTimestamp`, or to Ready when the unit was still active then.
Start, process exit, active and inactive transitions are separate markers. A
oneshot with no active interval has markers only, never a duration bar. These
intervals show systemd state, not application readiness.

Elapsed time, CPU time, runnable wait and service active duration are different
measurements. Never sum parallel work into a boot duration. An expensive thread
is an optimization candidate, not proof it delays Ready. Unclassified off-CPU
time is not relabeled as I/O.

Existing RP code already reports broad CSE boot datapoints and readiness
latency. This experiment adds scheduler-level evidence, coverage information,
an inspectable timeline and before/after comparison. It does not modify aks-rp.
The existing boot-data parser unit test is not a first-boot performance benchmark.

## Files and prerequisites

- `trace_boot.py`: local CLI; `orchestrate.py`: explicit scale-out/watch/SSH.
- `guest/`: image-side recorder, systemd unit, safety stop and ftrace GRUB drop-in.
- `events.py`: trace.dat and legacy tracefs-text adapters; `timeline.py`: shared accounting.
- `milestones.py`: CSE datapoint import; `report.py`: embedded HTML/CSS/JavaScript.
- `prepare_vhd.py`, `guest/install.sh`: opt-in Packer image integration.
- `render_e2e.py`: offline E2E artifact selection and comparison.
- `tests/`: synthetic offline `unittest` cases.
- `../scenario/boot_trace*.go`: E2E watch, Bastion collection and focused Go tests.
- `../../spec/e2e/boot_trace_install_spec.sh`: image-guard ShellSpec tests.

Local requirements: Python **3.10+**; for collection, existing authenticated
`az`, `kubectl` and OpenSSH clients, an explicitly selected kubeconfig/context,
Azure pool-scale and VMSS-read permissions, and Kubernetes list/watch nodes.
No packages are installed by the report/collection CLI. Rendering binary ftrace
requires `trace-cmd` on the report host; legacy `ftrace.txt` bundles still need
only the Python standard library.

The guest needs Python 3.10+, systemd, writable local storage, root permissions
and tracefs with ftrace boot parameters and trace-cmd 3.4 (including
coreutils `stdbuf`).
All configured events must exist on that image. No full-syscall or function-graph
capture is enabled.

The **standalone pool collector** uses SSH to the new node's InternalIP;
AgentBaker E2E uses the framework's existing Bastion transport instead. Establish an
authorized route, trusted host keys or host certificates, authentication and
noninteractive sudo for the service stop and capture tar commands beforehand.
Strict host-key verification is required; the tool never auto-trusts a new key.
Use `--ssh-config` for existing bastion/proxy settings. No SSH installation,
credential distribution or alternate AKS transport is implemented.

## Preparing first-boot capture

The following is the standalone collector's image-integration contract.
The AgentBaker-specific build recipe is described above.
First establish a supported, authorized test-image deployment path. An ordinary
AKS pool is not assumed to accept arbitrary custom images. Do not prepare a
production pool, modify its VMSS directly, or replace first boot with a reboot.

In that image-build process, arrange the following before the measured VM exists:

| Source | Destination in the test image |
| --- | --- |
| `guest/capture.py`, `guest/stop_capture.py` and `guest/diagnostics.py` | `/opt/aks-node-boot-trace/` (root-owned) |
| `guest/aks-node-boot-trace.service` | `/etc/systemd/system/aks-node-boot-trace.service`, enabled for `sysinit.target` |
| Adapted `guest/config.env.example` | `/etc/aks-node-boot-trace.env`, root-owned mode 0600 |

Set `BACKEND=ftrace`, the actual `IMAGE_LABEL`, `MAX_SECONDS` (1..600) and `MAX_MIB`
(at least 16). Do not bake credentials or earlier build/boot captures into the
image. Do not start the recorder on the image builder. Remove only experiment
artifacts from the image before sealing; retain required OS state.

For **ftrace**, install `guest/ftrace-grub.cfg` as
`/etc/default/grub.d/99-aks-boot-trace.cfg` and run `update-grub` before sealing
the image. It sets `trace_clock=mono`, `trace_buf_size=4M` and the selected
`trace_event` list directly on the kernel command line, without requiring the
main initramfs. Ensure tracefs is mounted. Simply copying the file through
cloud-init does not enable early-boot tracing. The recorder requires an already
active `mono` event trace with `current_tracer=nop`.
It starts `trace-cmd record -K -a --no-filter --poll -s 10000` to drain the
per-CPU binary buffers without resetting the early boot trace or changing its
clock/events. `-k` alone is not sufficient. No rotating `-m` files are used.
`--no-filter` retains the collector's own scheduler activity rather than hiding
its cost; polling with a 10 ms idle interval avoids waking a reader for every
new event. Python only supervises the native recorder every 200 ms. Readiness
notification waits for the native recorder and its per-CPU files.
The supervisor and native recorder PIDs/command are saved in `metadata.json`.
On stop, tracing is disabled, native readers drain, and trace-cmd finalizes
`trace.dat`. A failed recorder or finalization fails the capture explicitly.
Reserve 4 MiB per CPU for the early buffer and verify that no other consumer
owns the root trace instance. The reduction in overhead still requires a new
guest measurement; binary recording is not overhead-free.

The unit orders recorder activation before cloud-init, the Azure agent,
containerd and kubelet. `Type=notify` waits for recorder setup, so this
**perturbs boot ordering and adds setup overhead**. It is not a production
configuration or an overhead-free measurement. Check the target image for
systemd ordering cycles and compare with an uninstrumented baseline.

The local watchdog samples duration and disk usage. systemd adds a 12-minute
runtime limit and a safety stop for the owned trace. Size limits are **not a hard
filesystem quota**: kernel buffers, metadata, journal output and final flushing
can overshoot. Use a dedicated bounded filesystem when a hard cap is required.
Early ftrace buffers also consume RAM before the recorder starts. Validate the
failure/stop paths on the chosen image before any measurement campaign.

## Standalone pool collection CLI (not the AgentBaker E2E path)

Commands without `--execute` perform read-only preflight checks.

Run from this directory. A read-only preflight queries Azure and Kubernetes,
prints the proposed +1 operation, and does not create an output directory:

```bash
python3 -B trace_boot.py collect \
  --subscription "$SUBSCRIPTION" --resource-group "$RESOURCE_GROUP" \
  --cluster "$CLUSTER" --pool "$DEDICATED_TEST_POOL" \
  --kubeconfig "$KUBECONFIG" --context "$CONTEXT" \
  --ssh-user azureuser --backend ftrace --output captures/ftrace-01
```

Only after image/transport feasibility has been verified, the **same command
with `--execute --instrumentation-confirmed` creates one billable node**.
That confirmation is the operator's assertion, not remote proof of preparation.
The pool must already have a registered node, use Linux VMSS, have no autoscaler
and not use deallocated-instance reuse. Do not run concurrent pool operations.
The tool checks recorded counts and provisioning state but does not acquire
a cluster-wide lock.

The collector starts a node watch from the baseline resource version before
requesting the AKS scale-out. It distinguishes new Node UIDs and VMSS instance
IDs, records the first observed Ready condition and local observation time,
then stops and fetches that boot's capture. Fetching after Ready is expected:
analysis clips accounting to the mapped transition. The service may already
have stopped itself through its watchdog.

Watch disconnects/expiration, multiple new nodes, deletion, timeouts, missing
host trust and incomplete guest finalization fail explicitly. **Automatic
watch reconnection and collection from never-Ready nodes are not implemented.**
When capture fails after scale-out, inspect `result.json`, `watch.stderr` and
`ssh.stderr`; the node may still exist and the guest watchdog remains responsible
for stopping recording. An incomplete run is not an end-to-end success.

The 900-second default collector timeout can exceed the guest's 600-second
capture window. Choose image-side limits deliberately; longer collector waits
cannot recover events after the recorder stops. `--max-mib` on the collector
bounds archive transfer and extraction independently (default 512 MiB).
Use `--max-mib 1024` when collecting an image with a 512 MiB ftrace capture cap
to leave room for archive headers and capture metadata.

## Offline reports and comparison

```bash
python3 -B trace_boot.py report captures/ftrace-01 --output reports/ftrace-01
python3 -B trace_boot.py compare \
  reports/ftrace-01/report.json reports/ftrace-02/report.json \
  --output reports/before-after
```

Open `report.html` or `comparison.html` locally. There are no external scripts,
CDNs, telemetry uploads or browser network calls. Reports contain filters,
time-window zoom, click details, sortable thread accounting, semantic evidence,
warnings, and configuration-aware duration/CPU/runnable deltas. HTML CPU and
block activity lanes use **100 ms bins**, not full event resolution. Binary
ftrace is decoded only on this report host with
`trace-cmd report -N -t --raw-ts`: CPUs are merged chronologically, kernel
event field names/flags are retained, and timestamps remain boot-monotonic.
Accounting uses event timestamps; the raw traces retain deeper evidence. The view limits
displayed threads, not the normalized JSON accounting.

Legacy text ftrace must already be chronologically merged across CPUs.
Prefer the original binary capture and its embedded event formats over a
manual text conversion with unverified clock/fields.

Compare repeated trials with the same image, kernel, VM SKU, network, workload
and instrumentation. The HTML highlights recorded configuration differences,
but cannot prove comparability or statistical significance. Thread-name
comparison groups the last observed `comm` values, which can be truncated and
shared across processes; it is not stable process or service identity.

### Optional existing CSE milestones

Export just the existing `BootDatapoints` object locally, without full CSE output
or credentials. Upper/lower initial JSON field names are accepted:

```json
{
  "BootDatapoints": {
    "KernelStartTime": "2026-11-06T17:00:00Z",
    "GuestAgentStartTime": "2026-11-06T17:00:03Z",
    "CSEStartTime": "2026-11-06T17:00:08Z",
    "KubeletStartTime": "2026-11-06T17:00:23Z"
  }
}
```

```bash
python3 -B trace_boot.py import-cse /private/path/boot-datapoints.json \
  --output captures/ftrace-01/milestones.json
```

ISO-8601 with an explicit offset and existing UTC/GMT systemd/UnixDate layouts
are accepted. Ambiguous zone abbreviations fail unless the source's actual
offset is explicitly supplied, for example `--timezone-offset=-08:00`.
No missing datapoint is inferred. `ExecDuration` and `SystemdSummary` are not
expanded into invented internal CSE phases.

Alternatively add `milestones.json`, a list of objects with `name`, `source`
and either `boot_seconds` or a timezone-qualified `timestamp`. Such milestones
must come from this same boot; provenance is mandatory, but external milestone
boot identity is the operator's responsibility.

## Bundle and interpretation limits

```text
capture/
  metadata.json        # boot/kernel/backend, paired clocks, status, warnings
  readiness.json       # Node UID/providerID/bootID, transition and observation
  services.json        # loaded systemd units with boot lifecycle properties
  containerd-waits.json # bounded D-state stacks and descriptor snapshots
  cloud-timings.jsonl  # allowlisted certificate task timings, when run
  cse-task-timings.jsonl # sanitized AKS.CSE task names and intervals
  bootstrap-csr.json   # sanitized post-Ready CSR snapshot from E2E
  trace.dat            # native binary ftrace capture
  trace-cmd.log        # native recorder diagnostics
  ftrace.txt           # legacy text ftrace bundles only
  ftrace-stats.json    # ftrace per-CPU buffer diagnostics
  milestones.json      # optional imported semantic evidence
  operation.json       # standalone pool collector's requested operation only
  cse-boot-datapoints.json # sanitized CSE timestamps from E2E, when available
  result.json          # collected or explicitly incomplete
```

Missing Ready evidence stays unknown. Boot-ID mismatches and invalid clock
origins are rejected. Clock drift, trace loss, scheduler discontinuities and
missing coverage are surfaced. Clock samples bound only local sampling skew;
they do not quantify cross-host clock error or a change that occurred before
sampling. No CPU work before the first known switch is invented, and accounting
is not extrapolated beyond the last event to fill a missing tail.

Systemd data can reflect a restart rather than the first activation. Journal
export is opt-in through the guest recorder's `--include-journal` option and is
not enabled by the provided unit; journal lines are not automatically parsed
into semantic phases. Raw process names, paths, optional journals and reports
may contain sensitive operational information.

## Safety, cost and cleanup

The standalone pool collector does not install dependencies, build images, change pool image
settings, scale down, delete resources or roll back a requested scale-out.
Capture files and local outputs use restrictive permissions. Tar extraction
rejects traversal, links and special files, and enforces a byte limit.
Keep captures, HTML, JSON, SSH settings, kubeconfigs and credentials private;
generated `captures/` and `reports/` directories are ignored.
HTML reports embed the normalized capture data, including Azure resource IDs,
node identities and diagnostic paths; hiding a section does not anonymize it.
Review and redact those values before external sharing. Serve only the intended
report directory, never a campaign directory containing credentials or raw logs.

**Every executed standalone pool collection leaves a billable node allocated.** After reviewing
the run and checking workloads/concurrent operations, explicitly retire the
dedicated test pool/node using your authorized AKS procedure. Scaling a shared
pool down does not guarantee deletion of the measured node; this tool does not
choose that action for you. Stop an owned recorder before removing its files.
Remove only identified experiment outputs and discard the instrumented image
when finished; do not recursively clean shared directories or tracing sessions.
AgentBaker E2E instead retains the framework's existing cleanup of its own VMSS
and tunnel (subject to the framework's keep-resources setting). It does not
delete the shared AKS cluster, Bastion, unrelated nodes or system pools.

## Validation

Synthetic tests cover scheduler accounting, parallel CPUs, PID reuse, coverage
gaps, clocks/boot IDs, CSE parsing, HTML escaping, unsafe archives and dry-run/
watch-before-scale behavior. From the repository root, the local checks are:

```bash
(cd e2e/boot-trace && python3 -B -m unittest discover -s tests)
(cd e2e && go test ./scenario -run '^TestBootTrace' -count=1)
shellspec --shell bash --format d \
  spec/e2e/boot_trace_install_spec.sh spec/e2e/e2e_runner_scripts_spec.sh
make validate-shell
systemd-analyze verify e2e/boot-trace/guest/aks-node-boot-trace.service
```

ShellSpec 0.28.1 is the version used by the repository's test container.
Offline checks do not establish guest runtime compatibility. Validate ftrace
on each target image, including kernel options, image
integration, stop/watchdog behavior, trace-cmd field compatibility, HTML
interactions and instrumentation overhead against an uninstrumented control.
The integration adds Python cases in `tests/test_integration.py`, Go cases named
`TestBootTrace*` in `e2e/scenario/boot_trace_test.go`, and ShellSpec cases in
`spec/e2e/boot_trace_install_spec.sh`.

References: [kernel boot tracing](https://docs.kernel.org/trace/boottime-trace.html),
[systemd-analyze](https://www.freedesktop.org/software/systemd/man/latest/systemd-analyze.html).
