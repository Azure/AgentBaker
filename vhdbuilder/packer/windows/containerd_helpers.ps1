function Test-ContainerdReady
{
    try
    {
        $output = & ctr.exe -n k8s.io version 2>&1
        $exitCode = $LASTEXITCODE
    }
    catch
    {
        if ($_.FullyQualifiedErrorId -notlike "NativeCommandError*")
        {
            throw
        }

        $output = $_
        $exitCode = $LASTEXITCODE
    }

    return [pscustomobject]@{
        Ready = ($exitCode -eq 0)
        Output = ($output | Out-String).Trim()
    }
}

function Receive-ContainerdJobOutput
{
    param (
        [Parameter(Mandatory = $true)]
        $Job
    )

    return (Receive-Job -Job $Job -Keep 2>&1 | Out-String).Trim()
}

function Remove-ContainerdJob
{
    param (
        [Parameter(Mandatory = $true)]
        $Job
    )

    if ($Job.State -eq "Running")
    {
        Stop-Job -Job $Job
    }
    Remove-Job -Job $Job -Force
}

function Invoke-WithContainerd
{
    param (
        [Parameter(Mandatory = $true)]
        [scriptblock]$ScriptBlock,

        [int]$MaxAttempts = 12,

        [int]$DelaySeconds = 5
    )

    $jobName = "containerd"
    $job = Start-Job -Name $jobName -ScriptBlock { containerd.exe }
    $lastProbeOutput = ""

    try
    {
        for ($attempt = 1; $attempt -le $MaxAttempts; $attempt++)
        {
            $job = Get-Job -Id $job.Id
            if ($job.State -in @("Completed", "Failed", "Stopped"))
            {
                $jobOutput = Receive-ContainerdJobOutput -Job $job
                throw "containerd exited before becoming ready. Job state: $($job.State). Output: $jobOutput"
            }

            $probe = Test-ContainerdReady
            $lastProbeOutput = $probe.Output
            if ($probe.Ready)
            {
                return & $ScriptBlock
            }

            if ($attempt -lt $MaxAttempts)
            {
                Start-Sleep -Seconds $DelaySeconds
            }
        }

        $jobOutput = Receive-ContainerdJobOutput -Job $job
        throw "containerd did not become ready after $MaxAttempts attempts. Last probe output: $lastProbeOutput. Job output: $jobOutput"
    }
    finally
    {
        $job = Get-Job -Id $job.Id -ErrorAction SilentlyContinue
        if ($null -ne $job)
        {
            Remove-ContainerdJob -Job $job
        }
    }
}
