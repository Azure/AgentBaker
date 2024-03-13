function Test-FileExists {
    param (
        [string]$path
    )
    if (-not (Test-Path $path)) {
        throw "$path not found"
    }
}

function Get-WindowsBuildNumber {
    return (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion").CurrentBuild
}

function Get-WindowsVersion {
    $buildNumber = Get-WindowsBuildNumber
    switch ($buildNumber) {
        "17763" { return "1809" }
        "20348" { return "ltsc2022" }
        "22631" { return "ltsc2022" }
        Default {
            throw "Unsupported Windows build number: $buildNumber"
        }
    }
}

$kubectlPath = "c:\k\kubectl.exe"
Test-FileExists -path $kubectlPath

$configPath = "c:\k\config"
Test-FileExists -path $configPath

$templatePath = "c:\k\initnode.template.yaml"
Test-FileExists -path $templatePath

$nodeName = $env:COMPUTERNAME.ToLower()
$nodeJobYamlPath = "c:\k\init.$nodeName.yaml"

$osVersion = Get-WindowsVersion

$yamlContent = Get-Content -Path $templatePath -Raw
$yamlContent = $yamlContent -replace "<node-name>", $nodeName
$yamlContent = $yamlContent -replace "<os-version>", $osVersion
$yamlContent | Out-File -FilePath $nodeJobYamlPath -Encoding UTF8

Write-Output "$kubectlPath --kubeconfig $configPath apply -f $nodeJobYamlPath"
& $kubectlPath --kubeconfig $configPath apply -f $nodeJobYamlPath

<#
Error from server (Forbidden): error when retrieving current configuration of:    
Resource: "batch/v1, Resource=jobs", GroupVersionKind: "batch/v1, Kind=Job"       
Name: "init-akswin22000000", Namespace: "default"
from server for: "c:\\k\\init.akswin22000000.yaml": jobs.batch "init-akswin22000000" is forbidden: 
  User "system:node:akswin22000000" cannot get resource "jobs" in API group "batch" in the namespace "default"
#>

<#
Error from server (Forbidden): error when retrieving current configuration of:                                                                                                                                                                                                         ~
Resource: "batch/v1, Resource=jobs", GroupVersionKind: "batch/v1, Kind=Job"                                                                                                                                                                                                            ~
Name: "init-akswin22000000", Namespace: "kube-system"                                                                                                                                                                                                                                  ~
from server for: ".\\init.akswin22000000.yaml": jobs.batch "init-akswin22000000" is forbidden: 
  User "system:node:akswin22000000" cannot get resource "jobs" in API group "batch" in the namespace "kube-system"
#>