param(
    [string]$arg1,
    [string]$arg2
)

Invoke-WebRequest -UseBasicParsing https://aka.ms/downloadazcopy-v10-windows -OutFile azcopy.zip;expand-archive azcopy.zip;cd .\azcopy\*;.\azcopy.exe copy "C:\azuredata\CustomDataSetupScript.log" "https://abe2ecselog.blob.core.windows.net/cselogs/$arg1-cse.log?$arg2"