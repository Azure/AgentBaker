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

$products = ("ws2022","ws2019")
$drops= @{
    "ws2022" = "\\winbuilds\Release\fe_release_svc_refresh"
    "ws2019" = "\\winbuilds\release\rs5_release_svc_refresh"
  }
$deltaLayerDirectories = @{
    "ws2019" = @("cbaseospkg_nanoserver_en-us", "cbaseospkg_serverdatacentercore_en-us")
    "ws2022" = @("cbaseospkg_nanoserver_en-us", "cbaseospkg_serverdatacentercore_ltsc_en-us_vl")
}

$runningDir = $PSScriptRoot
#use the same log file and version file
# $timeStamp = Get-Date -Format "ddMMyyHHmmss"
# $logFile = "log_$($timeStamp).log"
# $LOGFILEPATH = Join-Path $runningDir -ChildPath "winbuildlogs\$logFile"
# $versionPath = Join-Path -Path $runningDir -ChildPath "winbuildsversion.txt"
$LOGFILEPATH = $logFileFullName
$versionPath = Join-Path -Path $runningDir -ChildPath $versionFile


Log("Start to get container layers from builds")
log("=========================================")
foreach ($product in $products) {
    Log("QUERYING PRODUCT - {0}" -f $product)
    $dropPath = $drops[$product]

    $blobContainer = "${product}-container"
    # $sasExpiryDate = (Get-Date).AddDays(1).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
    # $sasToken=$(az storage container generate-sas --account-name wcctagentbakerstorage --as-user --auth-mode login --permissions "rwld" --expiry $sasExpiryDate --name $blobContainer --https-only --output tsv)
    #Log("the sasToken is ${sasToken}.")

    # Pick the first x build to try to avoid the scenerio where the latest build image are still being built
    $latestBuilds = Get-ChildItem -Path $dropPath | Sort-Object -Property LastWriteTime -Descending | Select-Object -First 8 | Sort-Object -Property LastWriteTime

    # clean up the former directories if any
    $targetDirectory = Join-Path $runningDir -ChildPath $product
    new-Item -Path $targetDirectory -ItemType Directory -Force | Out-Null

    foreach ($sourceDir in $deltaLayerDirectories[$product]) {
        $found = $false
        $fileToCopy = $null
        foreach ($latestBuild in $latestBuilds)
        {
            $sourcePath = Join-Path -Path $latestBuild -ChildPath "amd64fre\containerbaseospkgs\$sourceDir"
        
            if (Test-Path -Path $sourcePath -PathType Container) {
                Log("The source path $sourcePath exists.")
            } else {
                Log("The source path $sourcePath does not exist. skip to try the next builds")
                continue
            }
            
            $filesToCopy = Get-ChildItem -Path $sourcePath -File
            foreach ($file in $filesToCopy) {
                if ($file.Name.EndsWith(".tar.gz", [System.StringComparison]::InvariantCultureIgnoreCase)) {
                    $found = $true
                    $fileToCopy = $file
                }
            }
        }
        if ($found) {
            $version = ($fileToCopy.Name -split '_|\.' | Select-Object -Index 5, 6, 7) -join '.'
            $tag = ($sourceDir -split '_|\.')[1]
            $destinationFile = Join-Path -Path $targetDirectory -ChildPath ($sourceDir + '\' +  $fileToCopy.Name)

            $imageFilePath = Split-Path -Path $fileToCopy.FullName
            
            # Remove the older version one if found
            $destinationDir = Split-Path -Path $destinationFile
            Remove-Item -Path $destinationDir -Recurse -Force  -ErrorAction SilentlyContinue
            New-Item -Path $destinationDir -ItemType Directory -Force | Out-Null
            
            Copy-Item -Path $fileToCopy.FullName -Destination $destinationFile -Force
            Log("Copied: $($fileToCopy.FullName) -> $destinationFile");
            
            $zippedFileName = $fileToCopy.Name
            $cfgFileName = "$zippedFileName.config.json"
            $cfgFile = Join-Path -Path $imageFilePath -ChildPath $cfgFileName
            $destinationCfgFile = Join-Path -Path $destinationDir -ChildPath $cfgFileName
            Copy-Item -Path $cfgFile -Destination $destinationCfgFile -Force
            Log("Copied: $cfgFile -> $destinationCfgFile");

            $manifestFileName =  "$zippedFileName.manifest.json"
            $manifestFile = Join-Path -Path $imageFilePath -ChildPath $manifestFileName
            $destinationManifestFile = Join-Path -Path $destinationDir -ChildPath $manifestFileName
            Copy-Item -Path $manifestFile -Destination $destinationManifestFile -Force
            Log("Copied: $manifestFile -> $destinationManifestFile");

            $containerImageUrl = "https://wcctagentbakerstorage.blob.core.windows.net/$blobContainer/$zippedFileName"
            azcopy copy "${destinationFile}" "${containerImageUrl}"

            $cfgFileUrl = "https://wcctagentbakerstorage.blob.core.windows.net/$blobContainer/${cfgFileName}"
            azcopy copy "${destinationCfgFile}" "${cfgFileUrl}"

            $manifestFileUrl = "https://wcctagentbakerstorage.blob.core.windows.net/$blobContainer/${manifestFileName}"
            azcopy copy "${destinationManifestFile}" "${manifestFileUrl}"

            if ($LastExitCode -eq 0) {
                Log("Image: $fileToCopy.FullName has been pushed to blob storage successfully.")
                # update the version file
                $index = "${product}_${tag}"
                
                Log("Update $index version to $version")
                $fileContent =  Get-Content -Path $versionPath

                $lineMatched = $false
                # Iterate through each line in the file content
                for ($i = 0; $i -lt $fileContent.Count; $i++) {
                    # Check for container version match
                    if ($fileContent[$i].Contains($index)) {
                        $fileContent[$i] = "${index}:${version}"
                        $lineMatched = $true
                        break
                    }
                }

                if (-not $lineMatched) {
                    $newLine = "`n` ${index}:${version}"
                    $fileContent += $newLine
                }

                $fileContent | Set-Content -Path $versionPath
            } else {
                Log("Error happaned when trying to push  $file.FullName to storage")
                Log("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
                continue
            }
        }
        else {
            Log("No images have been found in recent builds for $sourceDir")
        } 
    }

}

