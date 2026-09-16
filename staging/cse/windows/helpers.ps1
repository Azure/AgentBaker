# common helper functions

function Wait-ForServiceRemoval
{
    # Get-Service (and any other name-based lookup) fails once a service is merely marked for
    # deletion (1072, ERROR_SERVICE_MARKED_FOR_DELETE) -- not only once it is fully removed --
    # so it can't prove deletion is complete. Query sc.exe directly instead: keep waiting on 0
    # (still present) or 1072 (marked for deletion, still blocking a reinstall), and only report
    # success on 1060 (ERROR_SERVICE_DOES_NOT_EXIST), i.e. truly gone. Any other exit code is an
    # unexpected sc.exe failure (e.g. access denied) and shouldn't be silently retried away.
    param(
        [Parameter(Mandatory = $true)][string]$ServiceName
    )

    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        sc.exe query "$ServiceName" | Out-Null
        if ($LASTEXITCODE -eq 1060) {
            return $true
        }
        if ($LASTEXITCODE -notin @(0, 1072)) {
            throw "sc.exe query failed unexpectedly for existing $ServiceName service (exit code $LASTEXITCODE)"
        }

        Start-Sleep -Seconds 1
    }

    sc.exe query "$ServiceName" | Out-Null
    if ($LASTEXITCODE -eq 1060) {
        return $true
    }
    if ($LASTEXITCODE -notin @(0, 1072)) {
        throw "sc.exe query failed unexpectedly for existing $ServiceName service (exit code $LASTEXITCODE)"
    }

    return $false
}

function Remove-ServiceIfExists
{
    param(
        [Parameter(Mandatory = $true)][string]$ServiceName
    )

    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($null -eq $svc) {
        # Get-Service also returns nothing when the service is merely marked for deletion
        # (1072) from an earlier provisioning attempt, not only when it's truly absent. Probe
        # with sc.exe query and wait it out rather than assuming there's nothing to remove.
        if (Wait-ForServiceRemoval -ServiceName $ServiceName) {
            return
        }

        throw "Timed out waiting for existing $ServiceName service to be deleted"
    }

    $pendingStatuses = @('StartPending', 'ContinuePending', 'PausePending')
    for ($attempt = 0; $svc.Status -in $pendingStatuses -and $attempt -lt 30; $attempt++) {
        Start-Sleep -Seconds 1
        $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
        if ($null -eq $svc) {
            # As above: null here could mean the service was deleted (by another provisioning
            # attempt racing with this one) and is now merely marked for deletion, not truly gone.
            if (Wait-ForServiceRemoval -ServiceName $ServiceName) {
                return
            }

            throw "Timed out waiting for existing $ServiceName service to be deleted"
        }
    }

    if ($svc.Status -in $pendingStatuses) {
        throw "Timed out waiting for existing $ServiceName service to leave the $($svc.Status) state"
    }

    if ($svc.Status -ne 'Stopped' -and $svc.Status -ne 'StopPending') {
        # Expected sc.exe stop outcomes: 0 (queued), 1061 (mid-transition, retried below), 1062
        # (already inactive), 1060/1072 (disappeared or marked for deletion by a concurrent
        # removal -- benign races the rest of this function already handles). Anything else is
        # a genuine failure (e.g. access denied) and should fail fast with its exit code rather
        # than silently burning the 60-second poll and reporting a generic timeout.
        $expectedStopExitCodes = @(0, 1060, 1061, 1062, 1072)

        sc.exe stop "$ServiceName" | Out-Null
        $stopExitCode = $LASTEXITCODE
        if ($stopExitCode -notin $expectedStopExitCodes) {
            throw "sc.exe stop failed unexpectedly for existing $ServiceName service (exit code $stopExitCode)"
        }

        for ($attempt = 0; $attempt -lt 60; $attempt++) {
            $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
            if ($null -eq $svc -or $svc.Status -eq 'Stopped') {
                break
            }

            # ERROR_SERVICE_CANNOT_ACCEPT_CTRL (1061): the service was mid-transition when we
            # requested the stop. Once it clears that transient state, retry the stop instead of
            # just waiting, otherwise a service that never received a stop request can time out.
            if ($stopExitCode -eq 1061 -and $svc.Status -notin $pendingStatuses) {
                sc.exe stop "$ServiceName" | Out-Null
                $stopExitCode = $LASTEXITCODE
                if ($stopExitCode -notin $expectedStopExitCodes) {
                    throw "sc.exe stop failed unexpectedly for existing $ServiceName service (exit code $stopExitCode)"
                }
            }

            Start-Sleep -Seconds 1
        }

        if ($null -ne $svc -and $svc.Status -ne 'Stopped') {
            $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
        }

        if ($null -ne $svc -and $svc.Status -ne 'Stopped') {
            throw "Timed out waiting for existing $ServiceName service to stop"
        }
    }

    sc.exe delete "$ServiceName"
    if ($LASTEXITCODE -notin @(0, 1060, 1072)) {
        throw "sc.exe failed to delete existing $ServiceName service (exit code $LASTEXITCODE)"
    }

    if (Wait-ForServiceRemoval -ServiceName $ServiceName) {
        return
    }

    throw "Timed out waiting for existing $ServiceName service to be deleted"
}

function Invoke-NssmExe
{
    # Thin wrapper around the path-qualified nssm.exe invocation so tests can Mock it;
    # Pester cannot intercept a call like `& "$KubeDir\nssm.exe"` directly since path-qualified
    # commands bypass function/command name resolution.
    param(
        [Parameter(Mandatory = $true)][string]$KubeDir,
        [Parameter(Mandatory = $true, ValueFromRemainingArguments = $true)][string[]]$NssmArguments
    )
    & "$KubeDir\nssm.exe" @NssmArguments
}

function Invoke-Nssm
{
    param(
        [Parameter(Mandatory = $true)][string]$KubeDir,
        [Parameter(Mandatory = $true, ValueFromRemainingArguments = $true)][string[]]$NssmArguments
    )
    Invoke-NssmExe -KubeDir $KubeDir -NssmArguments $NssmArguments | RemoveNulls
    if ($LASTEXITCODE -ne 0)
    {
        throw "nssm.exe $( $NssmArguments -join ' ' ) failed (exit code $LASTEXITCODE)"
    }
}
