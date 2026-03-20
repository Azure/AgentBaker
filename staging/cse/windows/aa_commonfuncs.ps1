# Internal wrapper around nssm.exe; isolated so tests can mock it without needing a real binary.
function Invoke-NssmExe
{
    param(
        [Parameter(Mandatory = $true)][string]$KubeDir,
        [Parameter(Mandatory = $true, ValueFromRemainingArguments = $true)][string[]]$NssmArguments
    )
    & "$KubeDir\nssm.exe" @NssmArguments | RemoveNulls
}

# Invokes nssm.exe with the given arguments and throws if the exit code is non-zero.
function Invoke-Nssm
{
    param(
        [Parameter(Mandatory = $true)][string]$KubeDir,
        [Parameter(Mandatory = $true, ValueFromRemainingArguments = $true)][string[]]$NssmArguments
    )
    Invoke-NssmExe -KubeDir $KubeDir @NssmArguments
    if ($LASTEXITCODE -ne 0)
    {
        throw "nssm.exe $( $NssmArguments -join ' ' ) failed (exit code $LASTEXITCODE)"
    }
}
