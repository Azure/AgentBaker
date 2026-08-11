# common helper functions

function Remove-ServiceIfExists
{
    param(
        [Parameter(Mandatory = $true)][string]$ServiceName
    )
    # A prior provisioning attempt may have already registered this service (e.g. CSE re-invoked
    # after a partial failure). Best-effort remove it so the subsequent nssm.exe install doesn't
    # fail against an already-existing service.
    $svc = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($null -ne $svc) {
        sc.exe delete $ServiceName
        # sc.exe delete can legitimately return non-zero here (e.g. 1072 - service already marked for deletion)
        # since this is best-effort cleanup of a pre-existing service, don't treat that as fatal.
        if ($LASTEXITCODE -ne 0) { Write-Log "sc.exe failed to delete existing $ServiceName service (exit code $LASTEXITCODE), continuing anyway" }
    }
}

function Invoke-Nssm
{
    param(
        [Parameter(Mandatory = $true)][string]$KubeDir,
        [Parameter(Mandatory = $true, ValueFromRemainingArguments = $true)][string[]]$NssmArguments
    )
    & "$KubeDir\nssm.exe" @NssmArguments | RemoveNulls
    if ($LASTEXITCODE -ne 0)
    {
        throw "nssm.exe $( $NssmArguments -join ' ' ) failed (exit code $LASTEXITCODE)"
    }
}
