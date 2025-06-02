param (
    [Parameter(Mandatory=$true, Position=0)]
    [string] $managedidentityresourceid,

    [Parameter(Mandatory=$true, Position=1)]
    [string] $subscription,

    [Parameter(Mandatory=$true, Position=2)]
    [string] $resourcegroup,

    [Parameter(Mandatory=$true, Position=3)]
    [string] $clustername,
	
	[Parameter(Mandatory=$true, Position=4)]
    [string] $configversion
)

Function Execute-Command ($commandTitle, $commandPath, $commandArguments) {
    $pinfo = New-Object System.Diagnostics.ProcessStartInfo
    $pinfo.FileName = $commandPath
    $pinfo.RedirectStandardError = $true
    $pinfo.RedirectStandardOutput = $true
    $pinfo.UseShellExecute = $false
    $pinfo.Arguments = $commandArguments
    $p = New-Object System.Diagnostics.Process
    $p.StartInfo = $pinfo
    $p.Start() | Out-Null
    $p.WaitForExit()
    [PSCustomObject]@{
        commandTitle = $commandTitle
        stdout       = $p.StandardOutput.ReadToEnd()
        stderr       = $p.StandardError.ReadToEnd()
        ExitCode     = $p.ExitCode
    }
}
    
Write-Host "Starting to check if the MI has been installed ..."

$timeoutSeconds = 30

try {
	$uri="http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https%3A%2F%2Fmanagement.azure.com%2F&msi_res_id=$managedidentityresourceid"
	$miTokenRequest = Invoke-WebRequest -Uri $uri -Headers @{Metadata="true"}  -UseBasicParsing -TimeoutSec $timeoutSeconds
}catch {
	Write-Host "Web request timed out or encountered an error: $($_.Exception.Message)"
}

if ($miTokenRequest.StatusCode -ne 200) {
	Write-Host "Required MI not installed on the node, please install the MI first by addmitocluster.sh"
	exit 1
}
Write-Host "MI checked succeeded"

Write-Host "Starting to set environment variables for monagent ..."
$roleInstanceValue = (Get-ChildItem -Path Env:\COMPUTERNAME).Value
Write-Verbose "roleInstance : $roleInstanceValue" -Verbose
$env:MONITORING_TENANT = $subscription + "_" + $resourcegroup
$env:MONITORING_ROLE = $clustername
$env:MONITORING_ROLE_INSTANCE = $roleInstanceValue
$env:MONITORING_DATA_DIRECTORY = "C:\localdir"

$env:MONITORING_GCS_ACCOUNT="DPLATWINDOWSSERVERCONTAINERSTEST"
$env:MONITORING_GCS_NAMESPACE="wccttest"
$env:MONITORING_GCS_ENVIRONMENT="Test"
$env:MONITORING_GCS_REGION="East US"
# The version will need to be updated along with the MDS config update
$env:MONITORING_CONFIG_VERSION=$configversion
$env:MONITORING_GCS_AUTH_ID_TYPE="AuthMSIToken"
#$env:MONITORING_MANAGED_ID_IDENTIFIER="client_id"
#$env:MONITORING_MANAGED_ID_VALUE="5af6054a-0d2d-451f-9eb5-6e428cd49964"
$env:MONITORING_MANAGED_ID_IDENTIFIER="mi_res_id"
$env:MONITORING_MANAGED_ID_VALUE=$managedidentityresourceid
Write-Verbose "Finished setting environment variable." -Verbose

$currentPath = Get-Location

Write-Host "Starting to check if monagent has been installed as VMSS extension ..."
$monAgentClientLocation = (Get-ChildItem -Path Env:\MonAgentClientLocation -ErrorAction SilentlyContinue).Value
 if ($null -ne $monAgentClientLocation ) {
    Write-Host "monAgentClientLocation: $MONITORING_TENANT"
    Write-Host "MonAgent extension has been installed, need to run monagent client to drop the config"

    $monAgentClientFullPath = Join-Path -Path $monAgentClientLocation -ChildPAth "MonAgentClient.exe"
    Execute-Command "MonAgentClient.exe" $monAgentClientFullPath "-useenv" | Format-List
	
    # keep alive
	while ($true) {
		Start-Sleep -Seconds 86400  # Sleep for 24 hours in each iteration
	}    
 }
 else {
    # Check if the process is running and Stop the process
    #Get-Process -Name "SiloEtwMonitor" -ErrorAction SilentlyContinue | Stop-Process -Force

    Write-Host "MonAgent vmss extension not isntalled, launch the monagent directly"

    $monAgentLauncherPath = Join-Path -Path $currentPath -ChildPath "Geneva\Agent\MonAgentLauncher.exe"
    Write-Host "MonAgent launcher path is $monAgentLauncherPath"

    #Do not wait for exit
    #Start-Process -FilePath $monAgentLauncherPath -ArgumentList "-useenv"
	#Execute-Command "MonAgentLauncher.exe" $monAgentLauncherPath "-useenv" | Format-List
	Invoke-Expression "$monAgentLauncherPath -useenv"
}

#$siloEtwMonitorPath = Join-Path -Path $currentPath -ChildPath "SiloEtwMonitor\SiloEtwMonitor.exe"
#$siloEtwMonitorParameter = Join-Path -Path $currentPath -ChildPath "SiloEtwMonitor\SiloEtwMonitor.json"

#Write-Host "SiloEtwMonitor path is $siloEtwMonitorPath"
#Write-Host "SiloEtwMonitor parameter is $siloEtwMonitorParameter"

#Execute-Command "SiloEtwMonitor.exe" $siloEtwMonitorPath $siloEtwMonitorParameter

 
 
