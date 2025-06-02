param (
    [Parameter(Mandatory = $true)]
    [string]$StorageAccount
)

function Log {
    param (
        [string] 
        $text
    )

    if ($LOGFILEPATH) {
        "$(Get-Date)> $text" | Tee-Object -FilePath $LOGFILEPATH -Append
    } else {
        Write-Host ("$(Get-Date)> $text")
    } 
}

Push-Location .
$path = $env:root
Set-Location $path

$products = @("ws2022","ws2019")

# TODO: add the clean up logic but remember to keep the following
# They are from the folder of 
# \\ntdev\release\fe_release_svc_refresh
# \\ntdev\release\rs5_release_svc_refresh
# this will only for the reference purpose, we will rename them to be Base_serverDatacentercore.tar.gz after manully downloaded
$drops= @{
    "ws2022" = "CBaseOs_fe_release_svc_refresh_20348.2700.240905-2338_amd64fre_ServerDatacenterCore_ltsc_en-us_vl.tar.gz"
    "ws2019" = "CBaseOs_rs5_release_svc_refresh_17763.6293.240906-0050_amd64fre_ServerDatacenterCore_en-us.tar.gz"
  }
  
foreach ($product in $products) {
    Log("QUERYING PRODUCT - {0}" -f $product)
    Remove-Item -Path $product -Recurse -Force  -ErrorAction SilentlyContinue
    New-Item -Path $product -ItemType Directory -Force | Out-Null
    Set-Location $product
    
    # We only download the sererdatacentercore, the nano server is download and exported in export_nano_images.ps1
    # foreach($edition in $env:editions) {
    foreach($edition in @("ServerDatacenterCore")) {
        New-Item -Path $edition -ItemType Directory -Force | Out-Null
        Set-Location $edition

        $imageFile = (az storage blob list --auth-mode login --container-name ${product}-container --account-name ${StorageAccount}  `
        --query "[?contains(name, '$edition') && starts_with(name, 'CBaseOs') && ends_with(name, 'tar.gz')].[name, properties.lastModified]" --output tsv | Sort-Object { [DateTime]::Parse($_.Split("`t")[1]) } -Descending  `
        | Select-Object -First 1)

        $imageFileName = $imageFile.Split("`t")[0]
        Log("The image file to download is - {0}" -f $imageFileName)
        $cfgFileName = "${imageFileName}.config.json"
        $manifestFileName = "${imageFileName}.manifest.json"

        $baseFile = "Base_ServerDatacenterCore.tar.gz"

        azcopy copy "https://${StorageAccount}.blob.core.windows.net/${product}-container/${imageFileName}" $imageFileName
        Log("Downloaded: $imageFileName")

        azcopy copy "https://${StorageAccount}.blob.core.windows.net/${product}-container/${cfgFileName}" $cfgFileName
        Log("Downloaded: $cfgFileName")

        azcopy copy "https://${StorageAccount}.blob.core.windows.net/${product}-container/${manifestFileName}" $manifestFileName
        Log("Downloaded: $manifestFileName")

        azcopy copy "https://$StorageAccount.blob.core.windows.net/${product}-container/${baseFile}" "Base_ServerDatacenterCore.tar.gz"
        Log("Downloaded: $baseFile")

        Set-Location ..
    }

    Set-Location ..
}

# go back to where you started
Pop-Location