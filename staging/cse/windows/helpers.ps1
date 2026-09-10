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
        Stop-Service -Name $ServiceName -Force -ErrorAction Stop
    }

    sc.exe delete "$ServiceName"
    if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne 1072) {
        throw "sc.exe failed to delete existing $ServiceName service (exit code $LASTEXITCODE)"
    }

    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
        if ($null -eq $svc) {
            return
        }

        Start-Sleep -Seconds 1
    }

    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($null -eq $svc) {
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
