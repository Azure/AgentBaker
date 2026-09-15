# common helper functions

function Remove-ServiceIfExists
{
    param(
        [Parameter(Mandatory = $true)][string]$ServiceName
    )

    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($null -eq $svc) {
        return
    }

    $pendingStatuses = @('StartPending', 'ContinuePending', 'PausePending')
    for ($attempt = 0; $svc.Status -in $pendingStatuses -and $attempt -lt 30; $attempt++) {
        Start-Sleep -Seconds 1
        $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
        if ($null -eq $svc) {
            return
        }
    }

    if ($svc.Status -in $pendingStatuses) {
        throw "Timed out waiting for existing $ServiceName service to leave the $($svc.Status) state"
    }

    if ($svc.Status -ne 'Stopped' -and $svc.Status -ne 'StopPending') {
        sc.exe stop "$ServiceName" | Out-Null
        $stopExitCode = $LASTEXITCODE

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

    # Get-Service (and any other name-based lookup) fails once a service is merely marked for
    # deletion (1072) -- not only once it is fully removed -- so it can't prove deletion is
    # complete. Query sc.exe directly instead: keep waiting on 0 (still present) or 1072
    # (marked for deletion, still blocking a reinstall), and only report success on 1060
    # (ERROR_SERVICE_DOES_NOT_EXIST), i.e. truly gone.
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        sc.exe query "$ServiceName" | Out-Null
        if ($LASTEXITCODE -eq 1060) {
            return
        }

        Start-Sleep -Seconds 1
    }

    sc.exe query "$ServiceName" | Out-Null
    if ($LASTEXITCODE -eq 1060) {
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
