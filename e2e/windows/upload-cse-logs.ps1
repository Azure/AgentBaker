param(
    [string]$arg1,
    [string]$arg2,
    [string]$arg3,
    [string]$arg4
)

Invoke-WebRequest -UseBasicParsing https://aka.ms/downloadazcopy-v10-windows -OutFile azcopy.zip;expand-archive azcopy.zip;cd .\azcopy\*;.\azcopy.exe copy "C:\azuredata\CustomDataSetupScript.log" "https://$arg1.blob.core.windows.net/$arg2/$arg3-cse.log?$arg4"