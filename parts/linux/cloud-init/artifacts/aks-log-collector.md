# Incident-time I/O evidence in AKS support bundles

The AKS VM extension owns the I/O recorder, its NPD trigger, capture
schema, redaction, and retention. AgentBaker does not install a second recorder
or sampling timer. Activation policy and production rollout require separate
approval; this integration does not enable the producer. The existing
extension candidate gates all sampling on `enableIORecorder`: false or absent
disables it; enabled runs periodically scan bounded task detail. A minimal
always-on baseline versus triggered detail remains undecided and is not
implemented by these candidates. The existing
`aks-log-collector.timer` starts collection
10 minutes after activation and every 60 minutes thereafter; `nodePrep` starts
the timer on real nodes, including PIS-cached nodes.

## Producer/export contract

The extension publishes flat, immutable
`/var/log/azure/aks/io-recorder/capture-<id>.json` files. IDs contain only ASCII
letters, digits, and hyphens. The directory and files must be root-only.
Publication uses atomic rename from the sibling
`/var/log/azure/aks/.io-recorder-staging` directory on the same filesystem.
Published files are never modified. Producer staging is never exported.

The agreed producer limits are one MiB (1,048,576 bytes) per capture, eight
captures/eight MiB total, and a maximum age of 24 hours. These are **upper
bounds**, not a promise to keep every capture for 24 hours: count/byte eviction,
VM deletion/reimage, node disk loss, and log-upload failure can shorten
availability. Retention is producer-owned and may race an export read.

Each capture includes its schema version, capture ID, collection UTC, boot and
VM identity or explicit unavailability, trigger, and incident-history sections.
The producer owns process-start/PID/TID, cgroup/pod, mount/device correlation,
section completeness/errors/truncation, and privacy policy. AgentBaker copies
valid single-object JSON without reinterpreting or redacting its schema. Do
not publish raw credentials, environment variables, command lines, customer
file contents, or other unapproved data in this export directory.

## Collector behavior

Before point-in-time system and runtime collection, `aks-log-collector.sh`
snapshots at most eight captures, reading at most one MiB plus one detection
byte per file. Each read has a five-second timeout with a one-second kill
grace. This limits ordinary userspace reads, not uninterruptible kernel I/O.
The collector neither starts a capture nor deletes producer files.

Accepted captures are stored whole under `collect/io-recorder/` in the ZIP.
`collect/io-recorder-export.json` records the export schema version,
collection UTC, source directory, limits, number of unexamined files, and each
examined filename, status, byte count, and SHA-256 for an included file.
`included` means locally added to this ZIP, **not uploaded**.

The manifest distinguishes `not_present`, `empty`, `invalid_directory`,
`complete`, and `partial` exports. Per-file failures distinguish invalid names,
nonregular files/symlinks, vanished files, read errors/timeouts, oversize,
invalid JSON, hash failures, and ZIP failures. Rejected files are never
tail-truncated. Both the recorder directory and its producer staging directory
are excluded from subsequent generic file collection, even if a path is added
directly to its file list, so rejected or unpublished captures cannot reappear
through that path. The collector no longer accepts custom file lists through
IMDS configuration; this change does not restore them. Export errors also
produce warnings in the
collector's existing logs; inability to archive the manifest is itself logged.

The recorder adds at most eight MiB of capture data plus a small manifest and
ZIP metadata. Other support logs retain their existing collection behavior and
100 MiB stopping threshold; this is not a new hard cap on the entire archive.
Captures are selected in shell filename order if the producer exceeds its
eight-file contract, with remaining files counted as unexamined.

## Transport acknowledgment and retrospective access

The existing `aks-log-collector-send.py` sends the bundle through the existing
guest-agent `vmAgentLog` endpoint. This is AgentBaker's uploader, not the
WALinuxAgent hourly collector. No transport or collection schedule is added.
The uploader receives this invocation's private ZIP path to avoid uploading a
different collector's replacement of `/var/lib/waagent/logcollector/logs.zip`.
A root-only conventional local copy is still attempted at that shared path
and removed on exit by the existing cleanup.
Calling the uploader without a path retains its legacy behavior.

For each HTTP response, the uploader logs UTC, request correlation ID, SHA-256
of the exact transmitted bytes, byte count, and HTTP status. Only HTTP 200 is
reported as success; final upload failure retains the existing nonzero exit.
These messages use existing collector logging, not a new telemetry schema.
The extension emits its compact redacted capture summary through its existing
telemetry path.

An HTTP 200 proves only host transport acceptance. Neither the capture/export
manifest nor repository code proves downstream ingestion, searchable retention,
access authorization, or a retrieval SLA. Before claiming incident evidence
is available after recovery, an approved-environment rollout must demonstrate:

- Extension activation approval, bounded overhead, trigger operation, and retention behavior
  on the supported Ubuntu/Azure Linux kernels.
- A successful upload log correlated to the actual bundle hash, followed by
  authorized retrieval containing matching capture hashes and incident IDs.
- Backend retention, access controls, and discoverability covering the support
  window, including upload-disabled/offline nodes and node deletion/reimage.

Local validation does not perform a production upload or establish any of
these backend policies. Existing collector disable configuration is respected.

## Targeted tests

```sh
shellspec --shell bash --format d spec/parts/linux/cloud-init/artifacts/aks_log_collector_spec.sh
python3 -m unittest discover -s spec/parts/linux/cloud-init/artifacts -p 'test_aks_log_collector_send.py' -v
```

The ShellSpec tests use temporary capture files and real ZIP archives. The
Python standard-library tests mock the transport and do not contact Azure.
