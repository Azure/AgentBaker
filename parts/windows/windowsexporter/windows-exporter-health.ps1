
function Invoke-WindowsExporterRequest {
    param([Parameter(Mandatory=$true)][string]$Path)

    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:19182/$Path" -TimeoutSec 10 -ErrorAction Stop
        return [string]$response.Content
    }
    catch {
        return ""
    }
}

function Get-Health {
    $result = Invoke-WindowsExporterRequest -Path "health"
    if ($null -ne $result -and $result.Contains("ok")) {
        return $result
    } else {
        return ""
    }
}

function Get-Version {
    $result = Invoke-WindowsExporterRequest -Path "version"
    if ($null -ne $result -and $result.Contains("version")) {
        # {"version":"v0.25.1","revision":"f70fa009de541dc99ed210aa7e67c9550133ef02","branch":"HEAD","buildUser":"cloudtest@781d70d7c000002","buildDate":"20240223-08:06:57","goVersion":"go1.21.3"}
        $version = $result -replace ".*""version"":""([^""]+)"".*", '$1'
        return $version
    } else {
        return ""
    }
}

function Get-MetricsExample {
    # The result may be too large in production node. I suggest to call it only when testing.
    $result = Invoke-WindowsExporterRequest -Path "metrics"
    $example = "windows_process_cpu_time_total"
    if ($result -match $example) {
        $example = $result -split "`n" | Where-Object {$_ -match $example} | Select-Object -Last 1
        return $example
    } else {
        return ""
    }
}
