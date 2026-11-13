For tutorial for the descriptions of components.json, please refer to this [doc](README-COMPONENTS.md).

For CA refresh logging and Linux opt-in certificate diagnostics, see
[CA refresh diagnostics](../../../../doc/ca-refresh.md).

## Containerd temporary-disk startup

For `KubeletDiskType=Temporary`, the shared CSE/ANC `ensureContainerd` path
waits for an actual `/mnt` mount using the existing bounded retry helper.
It also writes a separate `10-mount-guard.conf` drop-in with an inline
`ExecStartPre` check. Every containerd start, including reboots and PIS cache
boots that skip basePrep, must confirm the mount before initializing runtime
state. The check uses standard image utilities, not additional artifacts or
`containerd config dump`. OS-disk configurations remove a stale drop-in and
do not acquire a mount wait or new service-start dependency.

The per-start check is bounded by `timeout 120` and fails with a journal
diagnostic; guarded services allow 150 seconds for startup. A dependency on
`mnt.mount` alone does not assert mount readiness when a unit is
condition-skipped. Waiting for all of cloud-init would risk a dependency cycle.

Protection requires the updated shared configuration script. Traditional CSE
can deliver it to older images; scriptless provisioning uses the baked script.
Updated PIS bakes retain the drop-in but no mount-readiness marker. Existing
unguarded caches need rebuilding. This prevents new hidden snapshotter state;
it does not repair an already-affected running node.

Selection follows the AKS disk-placement setting. AgentBaker can internally
render a `dataDir` override, but this is not a documented public AKS setting;
independent custom-root placement is outside this guard's scope.

## SSH-disable telemetry

When `DISABLE_SSH=true` and `nodePrep` runs, `disableSSH` emits structured
guest-agent events. A persistent systemd condition blocks future starts; existing
listeners are stopped without changing unit enablement. Stop retry parameters and
missing-unit handling are unchanged. Policy-installation and stop failures surface
as CSE exit code 172 after every SSH unit has been attempted. A successful stop
does not have to clear a unit's existing `failed` state.
Telemetry failures remain warnings, not new
provisioning failures. Other `systemctlDisableAndStop` callers and
`logs_to_events` keep their existing telemetry behavior.

| TaskName | Scope |
| --- | --- |
| `AKS.CSE.disableSSH` | One `Started` and one `Completed` event per SSH-disable attempt |
| `AKS.CSE.disableSSH.unit` | `Started` and `Completed` for each unit reached: `ssh.socket`, `sshd.socket`, `ssh`, `sshd` |

The event envelope's `Message` is a JSON string, available as `Context1` in
`GuestAgentGenericLogs`. Parse it with `parse_json(Context1)`.

| Message field | Meaning |
| --- | --- |
| `SchemaVersion` | `1` |
| `AttemptId` | Correlates summary and unit events; deduplicate with `VMId`, task, unit, and phase |
| `Phase`, `Unit` | `Started` or `Completed`; summary events have an empty unit |
| `ExitCode` | Null for starts; 0/1 for completed units; 0/172 for the completed attempt |
| `CatExitCode`, `Skipped` | Whether the unit was readable; nonzero `systemctl cat` preserves the existing skip behavior |
| `StopExitCode` | Stop retry-helper result; null when skipped, not the raw systemctl exit code |
| `DisableExitCode` | Always null for this stop-only SSH path; retained for schema compatibility |

The last three rows apply to completed unit events. The envelope's `Timestamp` and
`OperationId` contain start and emission times. The schema is kept compact to fit
the custom-data size limit. A skipped unit can be absent or unreadable;
it is not evidence of a successful stop. Readable masked units are stopped and
emit per-unit events without removing their masks. Command exit 0 alone does not prove port
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
cover nodes running the instrumented scripts. Both ordinary nodes and PIS-cached
nodes execute this policy in `nodePrep`. Image preparation skips `nodePrep`, so
it does not emit SSH-disable events.
