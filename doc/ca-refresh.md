# CA refresh diagnostics

## Linux

`init-aks-cloud.sh` is embedded in CustomData, which has a strict size limit.
Keep diagnostics small and account for the encoded payload size, not just the
script's source size.

Normal runs retain the existing OS, endpoint, opt-in, retry, and failure messages.
Before updating the trust store, they print `Refreshing CA trust store`.
The existing unconditional refresh behavior is unchanged: there is no certificate
comparison, change tracking, or new skip logic.
OS trust-store updater output is passed through unchanged; warnings and update
hooks may produce additional output.
Shell tracing, raw WireServer response dumps, per-file progress, and before/after
trust-store directory listings are removed. The script disables tracing even
when invoked with `bash -x`.

Public-certificate PEM diagnostics use a single opt-in:
`AKS_CLOUD_LOG_CERTIFICATES=true`. Unset or `false` disables PEM output without
reading or parsing certificate files for logging. There is no separate log-level
switch.

When enabled, the script re-encodes public certificates from downloaded `.crt`
files, including bundles, excluding private keys and unrelated response content.
It sends the result through `logger -p user.debug -t azure-ca-refresh`. The syslog
priority is a fixed classification for the optional certificate output, not
another enablement switch. It applies to all lines, including PEM bodies, under
cron, systemd, and manual execution. Certificate decoding or logging failures
produce a warning without preventing certificate installation.

For a one-off diagnostic refresh, set `LOCATION` to the actual node's location:

```sh
sudo env AKS_CLOUD_LOG_CERTIFICATES=true \
  /opt/azure/containers/init-aks-cloud.sh ca-refresh "$LOCATION"
journalctl -t azure-ca-refresh -p debug..debug
```

This performs a real certificate refresh, not a read-only inspection. The
environment override applies only to that invocation. For scheduled diagnostics,
add the assignment to the root cron command on Ubuntu, Mariner, and Azure Linux.
On Flatcar and ACL, use
`Environment=AKS_CLOUD_LOG_CERTIFICATES=true` in a `[Service]` drop-in for
`azure-ca-refresh.service` and reload systemd. Remove the override after the
investigation.

Enabled certificate output may be stored and forwarded by journald and downstream
collectors; `journalctl -p` only filters viewing. Existing console output and Guest
Agent JSON event levels are otherwise unchanged. This is deliberately not a
general severity/filtering framework, and it does not change refresh schedules.

## Windows

`Get-CACertificates` in `staging/cse/windows/kubernetesfunc.ps1` downloads and
imports CA certificates during provisioning and scheduled refreshes. Normal
output includes the endpoint mode, concise opt-in status for RCV1P, and a count
of certificates successfully imported. Legacy mode does not perform an opt-in
check.

Successful first-attempt requests and individual certificate writes/imports are
not logged. The CA-specific request helper still logs genuine retry attempts and
retains the existing request timeout, retry count, delay, and error behavior.
The shared `Retry-Command` helper used by other provisioning operations is
unchanged.

Warnings for missing data, rejected filenames, empty certificate content, and
failed requests/imports remain visible. A failed refresh does not log a success
summary. A partial refresh counts only successful imports and preserves the
existing return behavior; it does not imply that every requested certificate was
installed. Raw error responses are unchanged by this Windows logging reduction.

Windows does not log PEM bodies on the successful CA refresh path. The Linux
`AKS_CLOUD_LOG_CERTIFICATES` flag does not apply to Windows. Windows messages use
the existing timestamped `Write-Log`/`Write-Host` output, which is not suppressed
by the scheduled task's `Get-CACertificates | Out-Null` pipeline.

There is no change detection, skip logic, or scheduling change. Every eligible
run still downloads and imports the certificates, including root/intermediate
store selection and existing `-FailOnError` semantics.
