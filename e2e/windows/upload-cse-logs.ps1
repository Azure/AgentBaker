param(
    [string]$arg1,
    [string]$arg2,
    [string]$arg3,
    [string]$arg4
)

Invoke-WebRequest -UseBasicParsing https://aka.ms/downloadazcopy-v10-windows -OutFile azcopy.zip
Expand-Archive azcopy.zip
cd .\azcopy\*
$env:AZCOPY_AUTO_LOGIN_TYPE="MSI"
$env:AZCOPY_MSI_RESOURCE_STRING=$arg4
.\azcopy.exe copy "C:\azuredata\CustomDataSetupScript.log" "https://$arg1.blob.core.windows.net/$arg2/$arg3-cse.log"
.\azcopy.exe copy "C:\AzureData\provision.complete" "https://$arg1.blob.core.windows.net/$arg2/$arg3-provision.complete"
C:\k\debug\collect-windows-logs.ps1
$CollectedLogs=(Get-ChildItem . -Filter "*$arg3*" -File)[0].Name
.\azcopy.exe copy $CollectedLogs "https://$arg1.blob.core.windows.net/$arg2/$arg3-collected-node-logs.zip"