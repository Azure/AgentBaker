# Please get the latest commit id in https://github.com/microsoft/SDN/tree/master/Kubernetes/windows
$commit="d9eaf8f330b9c8119c792ba3768bcf4c2da86123"
$urls=@(
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/debug/networkmonitor/networkhealth.ps1",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/debug/VFP.psm1",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/debug/captureNetworkFlows.ps1",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/debug/collectlogs.ps1",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/debug/dumpVfpPolicies.ps1",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/debug/portReservationTest.ps1",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/debug/starthnstrace.cmd",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/debug/starthnstrace.ps1",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/debug/startpacketcapture.cmd",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/debug/startpacketcapture.ps1",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/debug/stoppacketcapture.cmd",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/helper.psm1",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/hns.psm1",
    "https://github.com/microsoft/SDN/raw/$commit/Kubernetes/windows/hns.v2.psm1"
)

function DownloadFileWithRetry {
    param (
        $URL,
        $Dest,
        $retryCount = 5,
        $retryDelay = 0,
        [Switch]$redactUrl = $false
    )
    curl.exe -f --retry $retryCount --retry-delay $retryDelay -L $URL -o $Dest
    if ($LASTEXITCODE) {
        $logURL = $URL
        if ($redactUrl) {
            $logURL = $logURL.Split("?")[0]
        }
        throw "Curl exited with '$LASTEXITCODE' while attemping to download '$logURL'"
    }
}

function Retry-Command {
    [CmdletBinding()]
    Param(
        [Parameter(Position=0, Mandatory=$true)]
        [scriptblock]$ScriptBlock,

        [Parameter(Position=1, Mandatory=$true)]
        [string]$ErrorMessage,

        [Parameter(Position=2, Mandatory=$false)]
        [int]$Maximum = 5,

        [Parameter(Position=3, Mandatory=$false)]
        [int]$Delay = 10
    )

    Begin {
        $cnt = 0
    }

    Process {
        do {
            $cnt++
            try {
                $ScriptBlock.Invoke()
                if ($LASTEXITCODE) {
                    throw "Retry $cnt : $ErrorMessage"
                }
                return
            } catch {
                Write-Error $_.Exception.InnerException.Message -ErrorAction Continue
                if ($_.Exception.InnerException.Message.Contains("There is not enough space on the disk. (0x70)")) {
                    Write-Error "Exit retry since there is not enough space on the disk"
                    break
                }
                Start-Sleep $Delay
            }
        } while ($cnt -lt $Maximum)

        # Throw an error after $Maximum unsuccessful invocations. Doesn't need
        # a condition, since the function returns upon successful invocation.
        throw 'All retries failed. $ErrorMessage'
    }
}

# Download all debug scripts from microsoft/SDN
foreach ($url in $urls) {
    $fileName = [IO.Path]::GetFileName($url)
    Write-Host "Downloading $url to $dest"
    DownloadFileWithRetry -URL $url -Dest $fileName
}

# Update collectlogs.ps1
$collectLogsScript=(Get-Content collectlogs.ps1)
# Remove DownloadFile in collectlogs.ps1
$collectLogsScript=($collectLogsScript | Where { $_ -notlike '*DownloadFile*' })
# Remove Invoke-WebRequest in collectlogs.ps1
$collectLogsScript=($collectLogsScript | Where { $_ -notlike '*Invoke-WebRequest*' })
$collectLogsScript | Set-Content collectlogs.ps1
