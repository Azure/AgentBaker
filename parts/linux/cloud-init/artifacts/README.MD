For tutorial for the descriptions of components.json, please refer to this [doc](README-COMPONENTS.md).

For CA refresh logging and Linux opt-in certificate diagnostics, see
[CA refresh diagnostics](../../../../doc/ca-refresh.md).

## Containerd temporary-disk startup

The script and drop-in template are installed under `/opt/azure/containers`,
not activated globally. After writing or preserving `/etc/containerd/config.toml`,
the shared CSE/ANC `ensureContainerd` path installs `10-mount-guard.conf` only
when the provisioned top-level root is at or under `/mnt`. It removes that
drop-in for OS roots, which never execute the guard or `containerd config dump`.
Root overrides and preserved NVIDIA configs are selected from the installed
config rather than `KubeletDiskType` alone.

The activated drop-in runs `containerd-mount-guard.sh` before every containerd
start, including asynchronous node-controller startup and reboots. The guard
reads the effective root using `containerd config dump` and requires an actual
mount at `/mnt` before initializing temporary-root state.
Cloud-init's mount unit alone is insufficient because it can be condition-skipped
before resource-disk setup finishes. Waiting for all of cloud-init would introduce
a dependency cycle on provisioning paths that cloud-init itself invokes.

The guard waits up to 120 seconds, then fails startup with a journal diagnostic
rather than allowing snapshotter state to be created underneath the mount.
The unit allows 150 seconds for configuration parsing and the mount wait.
The script and dedicated drop-in are baked into Linux images and delivered by
script-based custom data for older images. Scriptless provisioning uses the
baked files. PIS bakes retain the conditionally activated drop-in alongside the
containerd config, but do not retain a mount-readiness marker. Existing caches
without the activated guard need rebuilding. Older CSE scripts remain compatible,
but cannot activate the new guard merely because its template is on the VHD.
Activation requires the updated shared configuration script. This prevents the
startup race; it does not repair already-hidden snapshotter state on an affected
running node. Activation follows the provisioner's top-level root; independently
changing that root through config imports requires corresponding guard activation.

## SSH-disable telemetry

When `DISABLE_SSH=true` and `basePrep` runs, `disableSSH` emits structured
guest-agent events. Stop/disable retry parameters and missing-unit handling are
unchanged, a unit that is not installed is still skipped without error. Stop and
disable failures are now propagated after every SSH unit has been attempted, and
surface as CSE exit code 172. Telemetry failures remain warnings, not new
provisioning failures. Other `systemctlDisableAndStop` callers and
`logs_to_events` keep their existing telemetry behavior.

| TaskName | Scope |
| --- | --- |
| `AKS.CSE.disableSSH` | One `Started` and one `Completed` event per SSH-disable attempt |
| `AKS.CSE.disableSSH.unit` | `Started` and `Completed` for each unit reached: `ssh`, `sshd`, `sshd.socket` |

The event envelope's `Message` is a JSON string, available as `Context1` in
`GuestAgentGenericLogs`. Parse it with `parse_json(Context1)`.

| Message field | Meaning |
| --- | --- |
| `SchemaVersion` | `1` |
| `AttemptId` | Correlates summary and unit events; deduplicate with `VMId`, task, unit, and phase |
| `Phase`, `Unit` | `Started` or `Completed`; summary events have an empty unit |
| `ExitCode` | Null for starts; 0/1 for completed units; 0/172 for the completed attempt |
| `CatExitCode`, `Skipped` | Whether the unit was readable; nonzero `systemctl cat` preserves the existing skip behavior |
| `StopExitCode`, `DisableExitCode` | Retry-helper results; null when skipped, not raw systemctl exit codes |

The last two rows apply to completed unit events. The envelope's `Timestamp` and
`OperationId` contain start and emission times. The schema is kept compact to fit
the custom-data size limit. A skipped unit can be absent, masked, or unreadable;
it is not evidence of a successful stop. Command exit 0 alone does not prove port
22 is closed: inspect the unit states and use the reboot E2E validation for
listener checks.

Example completed-attempt failure rate, with incomplete attempts kept separate:

```kusto
GuestAgentGenericLogs
| where PreciseTimeStamp > ago(1d)
| where Environment == "PROD" and TaskName == "AKS.CSE.disableSSH"
| extend Event = parse_json(Context1)
| where Event.SchemaVersion == 1 and isnotempty(VMId) and isnotempty(Event.AttemptId)
| extend AttemptId = tostring(Event.AttemptId), Phase = tostring(Event.Phase),
         ExitCode = toint(Event.ExitCode)
| summarize arg_max(PreciseTimeStamp, *) by VMId, AttemptId, Phase
| summarize Started = countif(Phase == "Started"),
            Completed = countif(Phase == "Completed"),
            Failed = countif(Phase == "Completed" and ExitCode != 0)
    by VMId, AttemptId
| summarize StartedAttempts = countif(Started > 0),
            CompletedAttempts = countif(Completed > 0),
            FailedAttempts = countif(Failed > 0),
            StartedWithoutCompletion = countif(Started > 0 and Completed == 0)
| extend CompletedFailurePercent = iff(CompletedAttempts > 0,
    100.0 * FailedAttempts / CompletedAttempts, real(null))
```

Use summary events for the denominator, not all CSE completions or the number
of configured SSH-disabled pools. Missing completions can indicate interruption,
timeout, ingestion delay, logging failure, or a query-window boundary; do not
classify them automatically as successes or SSH failures. These events only
cover nodes running the instrumented scripts. PIS-cached nodes that skip
`basePrep` do not execute `disableSSH` and will not emit these events.
