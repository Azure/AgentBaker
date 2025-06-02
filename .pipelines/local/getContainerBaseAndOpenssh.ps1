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

$message = "Start to get container layers from builds"
Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
$message = "========================================="
Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append

# Check if the currentbuild has been processed
# Define regular expressions to match container patterns
$containerPattern = 'container:(\S*)'

# Initialize variables to store matches
$processedBuild = $null

$versionFilePath = Join-Path -Path $runningDir -ChildPath $versionFile
if (Test-Path -Path $versionFilePath -PathType Leaf) {
    $fileContent =  Get-Content -Path $versionFilePath

    # Iterate through each line in the file content
    foreach ($line in $fileContent) {
        # Check for container version match
        if ($line -match $containerPattern) {
            $processedBuild = $Matches[1].Trim()
        }
    }

    # Check if the line matches the predefined value
    if ($processedBuild -eq $currentBuildName) {
        $message = "The build has already been processed for container images."
        Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
        return 0
    }
}

# clean up the former directories if any
$imagesPath = Join-Path $runningDir -ChildPath "containerImages"
Remove-Item -Path $imagesPath -Recurse -Force  -ErrorAction SilentlyContinue

$targetDirectory = Join-Path -Path $imagesPath -ChildPath $currentBuildName
New-Item -Path $targetDirectory -ItemType Directory -Force | Out-Null

# Loop through source directories and copy all files from subdirectories to the target directory
# $sasExpiryDate = (Get-Date).AddDays(1).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
# $sasToken=$(az storage container generate-sas --account-name wcctagentbakerstorage --as-user --auth-mode login --permissions "rwld" --expiry $sasExpiryDate --name from-sparc --https-only --output tsv )
#$message = "the sasToken is ${sasToken}."

Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append

foreach ($sourceDir in $editionDirectories) {
    $sourcePath = Join-Path -Path $containerBaseImagePath -ChildPath $sourceDir

    $filesToCopy = Get-ChildItem -Path $sourcePath -File

    $parts = $sourceDir -split "_"
    # Get the desired substring (assuming it's always the third element)
    $imageRepository = $parts[1].ToLowerInvariant()

    foreach ($file in $filesToCopy) {
        if ($file.Name.EndsWith(".tar.gz", [System.StringComparison]::InvariantCultureIgnoreCase)) {
            $tag = $imageRepository
            # Tag and save to tar file, which will be consumed by AKS internal cluster
            if ($imageRepository -eq "serverdatacentercore") {
                $tag = "servercore"
            }
            $tarfileName=$tag+"-${osVersion}.tar.gz"

            $destinationFile = Join-Path -Path $targetDirectory -ChildPath ($sourceDir + '\' + $tarfileName)
            
            # Create the destination directory if it doesn't exist
            $destinationDir = Split-Path -Path $destinationFile
            New-Item -Path $destinationDir -ItemType Directory -Force | Out-Null
            
            Copy-Item -Path $file.FullName -Destination $destinationFile -Force
            $message = "Copied: $($file.FullName) -> $destinationFile"
            Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append

            #$containerImageUrl = "https://wcctagentbakerstorage.blob.core.windows.net/from-sparc/${tarfileName}?${sasToken}"
            $containerImageUrl = "https://wcctagentbakerstorage.blob.core.windows.net/from-sparc/${tarfileName}"
            azcopy copy "${destinationFile}" "${containerImageUrl}"
            #upload the tag file to the blob storage

            if ($LastExitCode -eq 0) {
                    $message = "Image: ${imageRepository}:$currentBuildName has been pushed to blob storage successfully."
                    Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
            } else {
                $message = "Error happaned when trying to push to storage"
                Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
                $message = "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
                Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
                return -1
            }
            
        }
    }
}


Remove-Item -Path $localHostImagePath -Recurse -Force  -ErrorAction SilentlyContinue
New-Item -Path $pathToVhdDirectory -ItemType Directory -Force

$cabDestinationName = ${osVersion} + "_" + $cabFile.Name
$cabDestination = Join-Path $pathToVhdDirectory -ChildPath $cabDestinationName


if (-Not (Test-Path $cabDestination)) {
    $message = "copying cab file to $cabDestination"
    Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
    
    Copy-Item -Path $cabFile.FullName -Destination $cabDestination -Force
} else {
    $message = "cab file already exists in $pathToVhdDirectory"
    Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
}

# Get the cabe file SAS url
$opensshSasUrl = "https://wcctagentbakerstorage.blob.core.windows.net/from-sparc/${cabDestinationName}?${sasToken}"
azcopy copy $cabDestination "${opensshSasUrl}"

$fileContent =  Get-Content -Path $versionFilePath

$lineMatched = $false
# Iterate through each line in the file content
for ($i = 0; $i -lt $fileContent.Count; $i++) {
    # Check for container version match
    if ($fileContent[$i] -match $containerPattern) {
        $fileContent[$i] = "container:$currentBuildName"
        $lineMatched = $true
        break
    }
}

if (-not $lineMatched) {
    $newLine = "`n`container:$currentBuildName"
    $fileContent += $newLine
}
$fileContent | Set-Content -Path $versionFilePath

# update the version file
$message = "Update container version to $currentBuildName"
Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append