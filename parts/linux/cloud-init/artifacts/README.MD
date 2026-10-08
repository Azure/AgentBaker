For tutorial for the descriptions of components.json, please refer to this [doc](README-COMPONENTS.md).

For CA refresh logging and Linux opt-in certificate diagnostics, see
[CA refresh diagnostics](../../../../doc/ca-refresh.md).

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
| `DurationSeconds` | Elapsed time since the corresponding start |
| `OS`, `OSVersion`, `OSVariant` | Distro context from the provisioning environment |
| `PreProvisionOnly` | Distinguishes image preparation from ordinary node provisioning |
| `CatExitCode`, `Skipped` | Whether the unit was readable; nonzero `systemctl cat` preserves the existing skip behavior |
| `StopExitCode`, `DisableExitCode` | Retry-helper results; null when skipped, not raw systemctl exit codes |
| `StateQueryExitCode` | Result of the post-operation state query, bounded to two seconds per unit |
| `LoadState`, `ActiveState`, `UnitFileState` | Observed state after the operations; diagnostic only, not an additional failure gate |

The last five rows apply to completed unit events. A skipped unit can be absent,
masked, or unreadable; it is not evidence of a successful stop. A failed state
query is unknown state, not proof that SSH is stopped. Command exit 0 alone also
does not prove port 22 is closed: inspect the unit states and use the reboot E2E
validation for listener checks.

Example completed-attempt failure rate, with incomplete attempts kept separate:

```kusto
GuestAgentGenericLogs
| where PreciseTimeStamp > ago(1d)
| where Environment == "PROD" and TaskName == "AKS.CSE.disableSSH"
| extend Event = parse_json(Context1)
| where Event.SchemaVersion == 1 and isnotempty(VMId) and isnotempty(Event.AttemptId)
| extend AttemptId = tostring(Event.AttemptId), Phase = tostring(Event.Phase),
         ExitCode = toint(Event.ExitCode), OS = tostring(Event.OS),
         OSVariant = tostring(Event.OSVariant), PreProvisionOnly = tobool(Event.PreProvisionOnly)
| summarize arg_max(PreciseTimeStamp, *) by VMId, AttemptId, Phase
| summarize Started = countif(Phase == "Started"),
            Completed = countif(Phase == "Completed"),
            Failed = countif(Phase == "Completed" and ExitCode != 0)
    by VMId, AttemptId, OS, OSVariant, PreProvisionOnly
| summarize StartedAttempts = countif(Started > 0),
            CompletedAttempts = countif(Completed > 0),
            FailedAttempts = countif(Failed > 0),
            StartedWithoutCompletion = countif(Started > 0 and Completed == 0)
    by OS, OSVariant, PreProvisionOnly
| extend CompletedFailurePercent = iff(CompletedAttempts > 0,
    100.0 * FailedAttempts / CompletedAttempts, real(null))
```

Use summary events for the denominator, not all CSE completions or the number
of configured SSH-disabled pools. Missing completions can indicate interruption,
timeout, ingestion delay, logging failure, or a query-window boundary; do not
classify them automatically as successes or SSH failures. These events only
cover nodes running the instrumented scripts. PIS-cached nodes that skip
`basePrep` do not execute `disableSSH` and will not emit these events.
