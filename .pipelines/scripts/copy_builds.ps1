[CmdletBinding()]
param(
    [string]
    #[Parameter(Mandatory=$true)]
    $dropPat,
    [string]
    #[Parameter(Mandatory=$true)]
    $buildJsonPath,
    [string]
    #[Parameter(Mandatory=$true)]
    $storageAccount
)

function Log {
    param (
        [string] 
        $text
    )

    if ($LOGPATH) {
        "$(Get-Date)> $text" | Tee-Object -FilePath $LOGPATH -Append
    } else {
        Write-Host ("$(Get-Date)> $text")
    } 
}

# https://dev.azure.com/mseng/AzureDevops/_git/AzureDevops?path=%2FDrop%2FApp%2FApp%2FScripts%2FDropExeSamples.psm1
function Get-DropClient()
{
    [CmdletBinding()]
    param(
        [Parameter(Mandatory=$false)]
        [string] $DestinationDirectory = [System.IO.Path]::Combine($env:TEMP, "Drop.App"),
        [Parameter(Mandatory=$false)]
        [bool] $RequireServerVersion
    )

    $sourceUrl = "https://artifacts.dev.azure.com/microsoft/_apis/drop/client/exe"
    $destinationZip = [System.IO.Path]::Combine($env:TEMP, "Drop.App.zip")
    $destinationDir = [System.IO.Path]::Combine($DestinationDirectory, "lib", "net45")
    $destinationExe = [System.IO.Path]::Combine($destinationDir, "drop.exe")

    if ([System.IO.File]::Exists($destinationExe))
    {
        Write-Verbose "Found drop.exe at $destinationExe."

        if ($RequireServerVersion)
        {
            $commitTxt = [System.IO.Path]::Combine($destinationDir, "commit.txt")
            $localVersion = [System.IO.File]::ReadAllText($commitTxt).Trim()
            $serverVersion = (Get-DropClientVersion -Account $Account).Trim()
            if ($localVersion -ne $serverVersion)
            {
                Write-Error "Local version of drop.exe ($localVersion) does not match server version ($serverVersion). Please remove your local version or omit the -RequireServerVersion parameter."
            }
        }
    }
    else
    {
        # Note the equivalent PowerShell cmdlet, Invoke-WebRequest, requires a workaround for a perf issue
        # $ProgressPreference = "SilentlyContinue"
        # Invoke-WebRequest -Uri "https://[your-account].artifacts.visualstudio.com/DefaultCollection/_apis/drop/client/exe" -UseDefaultCredentials -OutFile "$($env:TEMP)\Drop.App.zip"
        Write-Verbose "Downloading client from ""$sourceUrl"" to ""$destinationZip""..."
        $webClient = New-Object System.Net.WebClient
        $webClient.DownloadFile($sourceUrl, $destinationZip)

        Write-Verbose "Extracting client from ""$destinationZip"" to ""$DestinationDirectory""..."
        Add-Type -AssemblyName System.IO.Compression.FileSystem
        [System.IO.Compression.ZipFile]::ExtractToDirectory($destinationZip, $DestinationDirectory)
    }

    Write-Output $destinationExe
}
#
# MAIN
#


$buildFolder = "$PSScriptRoot"
$dropPath = ".\VHD"
$dropExePath = Get-DropClient

Log -text "BEGIN DOWNLOAD SERVICE BUILD"
Log -text "LOCATION=$(Get-Location)"
Log -text ($PSVersionTable | Out-String)

Log -text "BUILDFOLDER=$buildFolder"

$builds = Get-Content -Raw -Path $buildJsonPath | ConvertFrom-Json

#Assume that we have already azure login
foreach ($build in $builds) {
    Log -text ("BUILD={0}" -f ($build | Out-String))

    if ($build.product -match "RS5") { 
        $productName = "ws2019"
    }else{
        $productName = $build.product.ToLower()
    }  

    Log -text ("PRODUCT NAME=$productName")

    $release = $build.release.Split(" ")
    $temp=$release[0].Split(".") -join "" 
    $patch= $temp.substring(2,4) + $release.Split(" ")[-1].ToLower()


    $buildName = $build.buildName.Split(".")
    $folderName=$buildName[0] +"."+$buildName[1]+"."+$buildName[-1]
    $ver = $buildName[-1].Split("-")[0]

    Log -text ("RELEASE=$release") 
    Log -text ("PATCh=$patch")
    Log -text ("BUILDNAME=$buildName")
    Log -text ("FOLDERNAME=$folderName") 
    Log -text ("VER=$ver")

    $basePath = ("{0}\{1}" -f $dropPath, $build.buildGuid)
    $sourcePath = ("{0}" -f $basePath)
    $edition = ""

    Log -text ("basePath=$basePath") 
    Log -text ("DropExePath=$dropExePath")

    $buildPropertyList = @{}
    $buildPropertyList.release = $release
    $buildPropertyList.patch=$patch
    $buildPropertyList.buildName=$buildName
    $buildPropertyList.folderName=$folderName
    $buildPropertyList.ver=$ver

    if (-not (Test-Path -Path $basePath)) {
        New-Item -Path $basePath -ItemType Directory -Force
    }

    Set-Content -Path "$basePath\build.json" -Value ($build | ConvertTo-Json -Depth 4)
    Set-Content -Path "$basePath\build.property.json" -Value ($buildPropertyList | ConvertTo-Json -Depth 4)

    Foreach ($item in $build.mediaInfo) { 
        $cloudDropName = $item.cloudDropName
        $destinationPath = $null

        $destinationContainer = $productName
        if (${item}.dropName -like "*gen2*") {
            $destinationContainer = "${productName}-gen2"
            $sourcePath = "$basePath\gen2"
        }

        if($cloudDropName -like "*vhd*"){
            $edition = $item.dropName
            $destinationPath = $sourcePath
        }elseif ($cloudDropName -like "*DATACENTERCORE*") {
            $destinationPath = "$sourcePath\servercore"
        }elseif ($cloudDropName -like "*sac*") {
            $destinationPath = "$sourcePath\servercoresac"
        }elseif ($cloudDropName -like "*nano*") {
            $destinationPath = "$sourcePath\nanocore"
        }

        if ($null -ne $destinationPath) {
            Log -text ("DOWNLOADING {0} {1}" -f $cloudDropName, $destinationPath)

            # Disable the cache in pipeline
            $env:TOKEN = $(az account get-access-token --query accessToken --resource 499b84ac-1321-427f-aa17-267ca6975798 -o tsv)
            Log -text ("TOKEN={0}" -f $env:TOKEN)
            #$dropResult = & $dropExePath get --writable:true -s "https://artifacts.dev.azure.com/microsoft" --patAuthEnvVar ${dropPat} -n $cloudDropName -d $destinationPath --cachesizeoverride 0
            $dropResult = & $dropExePath get --writable:true -s "https://artifacts.dev.azure.com/microsoft" --patAuthEnvVar TOKEN -n $cloudDropName -d $destinationPath --cachesizeoverride 0
            Log -text ("DROP RESULT={0}" -f ($dropResult | Out-String))

            # Prefix all files under $destinationPath with the build version info.
            Get-ChildItem -Path $destinationPath | ForEach-Object {
                if ($_.Name -like "*.vhd") {
                    $newName = "${folderName}." + $_.Name
                    Rename-Item -Path $_.FullName -NewName $newName
                    
                    $vhd = Get-VHD -Path $destinationPath\$newName
                    #check to see if its dynamic or fixed, need to change it to be fixed to be used by agentbaker
                    if ($vhd.VhdType -like "Dynamic") {
                        Log -text ("Conver from dynmaic VHD {0}  to fixed one" -f $newName)
                        Convert-VHD -Path $vhd.Path -DestinationPath "$destinationPath\$($newName -replace '\.vhd$', '.fixed.vhd')" -VHDType Fixed -DeleteSource
                        $vhd = Get-VHD -Path "$destinationPath\$($newName -replace '\.vhd$', '.fixed.vhd')"
                        #Remove-Item -Path $destinationPath\$newName
                    }
                }
            }
            
            # Assume all the vhds are with .vhd instead of .vhdx.
            Log -text "Copying vhd to the storage account"

            # $sasExpiryDate = (Get-Date).AddDays(1).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")

            # $sasToken = $(az storage container generate-sas `
            #  --account-name ${storageAccount} `
            #  --as-user --auth-mode login `
            #  --name ${destinationContainer} `
            #  --permissions "rwdl" `
            #  --expiry $sasExpiryDate `
            #  --https-only `
            #  --output tsv)

            #c:\AZCopy\azcopy.exe copy "$destinationPath\*.vhd" "https://${storageAccount}.blob.core.windows.net/${destinationContainer}?${sasToken}" --recursive
            azcopy copy "$destinationPath\*.vhd" "https://${storageAccount}.blob.core.windows.net/${destinationContainer}" --recursive

            # Delete all .vhd files in $destinationPath to save space
            Remove-Item -Path "$destinationPath\*.vhd" -Force

            Log -text "Copying the latest build info to the storage account"
            #c:\AZCopy\azcopy.exe copy "$basePath\*.json" "https://${storageAccount}.blob.core.windows.net/${destinationContainer}?${sasToken}" --recursive
            azcopy copy "$basePath\*.json" "https://${storageAccount}.blob.core.windows.net/${destinationContainer}" --recursive
        }
    }
}