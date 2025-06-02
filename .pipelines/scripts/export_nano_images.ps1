param(
    [string] $StorageAccount,
    [string] $ContainerRegistry,
    [string] $CsvFile
)

function Log {
    param (
        [string] 
        $text
    )    
    $msg = "$(Get-Date)> $text"
    if ($LOGPATH) {
        $msg | Tee-Object -FilePath $LOGPATH -Append
    } else {
        Write-Host $msg
    }
}

# conver to kusto timestamp format
function Convert-CustomTimestamp {
    param (
        [string]$timestamp  # Input string in "YYMMDD-HHmm" format
    )

    # Extract parts
    $year = 2000 + [int]$timestamp.Substring(0,2)   # Assumes 21st century (2000+YY)
    $month = [int]$timestamp.Substring(2,2)
    $day = [int]$timestamp.Substring(4,2)
    $hour = [int]$timestamp.Substring(7,2)
    $minute = [int]$timestamp.Substring(9,2)

    # Create DateTime object
    $parsedDate = Get-Date -Year $year -Month $month -Day $day -Hour $hour -Minute $minute -Second 0

    # Convert to desired format
    return $parsedDate.ToString("yyyy-MM-dd HH:mm:ss.0000000")
}

Push-Location .
$path = $env:root
Set-Location $path

# Remove all unused images
Log -text "Cleanup existing container images."
docker image prune -a -f

# clean up the former directories if any
$imagesPath = Join-Path $path -ChildPath "containerImages"
New-Item -Path $imagesPath -ItemType Directory -Force | Out-Null

Set-Location -Path $imagesPath

$products = @("ws2022","ws2019")

foreach ($product in $products) {
    if ($product -eq "ws2019") {
        $tag = "ltsc2019"
        $officialTag = "1809"
    } else {
        $tag = "ltsc2022"
        $officialTag = "ltsc2022"
    }

    $storageContainer = "${product}-container"

    # Get the latest 3 images, in case we somehow missed before
    # Update the logic to always check the latest file
    # $editionFiles = az storage blob list --account-name $StorageAccount --auth-mode login --container-name $storageContainer --query "[?contains(name, 'NanoServer') && ends_with(name, 'tar.gz')].[name, properties.lastModified]" `
    #     --output tsv | Sort-Object { [DateTime]::Parse($_.Split("`t")[1]) } -Descending | Select-Object -First 3 | Sort-Object -Property LastWriteTime
    $editionFiles = az storage blob list --account-name $StorageAccount --auth-mode login --container-name $storageContainer --query "[?contains(name, 'NanoServer') && ends_with(name, 'tar.gz')].[name, properties.lastModified]" `
    --output tsv | Sort-Object { [DateTime]::Parse($_.Split("`t")[1]) } -Descending | Select-Object -First 1 | Sort-Object -Property LastWriteTime

    foreach ($editionFile in $editionFiles) {
        $editionFileName = $editionFile.Split("`t")[0]
        $fileNameWithoutSuffix = $editionFileName -replace '\.tar\.gz$', ''
        $tarFileName = $fileNameWithoutSuffix + ".tar"
        $version = ($fileNameWithoutSuffix -split '_|\.' | Select-Object -Index 5, 6) -join '.'
        $buildTime = $fileNameWithoutSuffix -split '_|\.' | Select-Object -Index 7
        
        # $fileExists = az storage blob show --account-name $StorageAccount --auth-mode login --container-name $storageContainer --name $tarFileName

        # if ($null -ne $fileExists) {
        #     Log -text "Container base image exists for $fileNameWithoutSuffix, skip"
        #     continue
        # }

        $edtionZipImageUrl = "https://${StorageAccount}.blob.core.windows.net/${storageContainer}/${editionFileName}"
    
        azcopy copy "${edtionZipImageUrl}" "${editionFileName}"

        # Check the file size of the downloaded file
        $fileSize = (Get-Item "${editionFileName}").Length
        Log -text "The file size of ${editionFileName} is $fileSize bytes"
    
        Log -text "Start to import docker image"
    
        $imageId = docker import "${editionFileName}"
    
        # Work around for the issue https://github.com/docker/for-win/issues/13891
        if ($imageId -is [System.Collections.IEnumerable] -and $imageId -isnot [System.String]) {
            $imageId = $imageId[1]
        }
    
        Log -text "imported image id is $imageId"

        $shortId = $imageId.Substring(7, 12)
        
        # Need to quote, otherwise the output will not be the right string
        docker tag $shortId "${ContainerRegistry}.azurecr.io/windows/nanoserver:$version"
        docker push "${ContainerRegistry}.azurecr.io/windows/nanoserver:$version"
        Log -text "Container image pushed nanoserver:$version"
    
        # Also tag it to the lastest
        docker tag $shortId "${ContainerRegistry}.azurecr.io/windows/nanoserver:$tag"
        docker push "${ContainerRegistry}.azurecr.io/windows/nanoserver:$tag"
        Log -text "Container imag epushed nanoserver:$tag"
    
        # Please be noted that for 2019 nanoserver, the official tag is 1809
        Remove-Item -Path "${fileNameWithoutSuffix}.tar" -ErrorAction SilentlyContinue
        docker tag $shortId "mcr.microsoft.com/windows/nanoserver:$officialTag"
        docker save -o "${fileNameWithoutSuffix}.tar" "mcr.microsoft.com/windows/nanoserver:$officialTag"
        
        $expandedSize = (Get-Item "${fileNameWithoutSuffix}.tar").Length
        $formattedDate = Convert-CustomTimestamp $buildTime
        $timeStamp = Get-Date -Format "yyyy-MM-ddTHH:mm:ss.0000000Z"
        # Define data as an array of objects
        $data = @(
            [PSCustomObject]@{ TimeStamp=$timeStamp; WindowsSku=$product; Flavor="nanoserver";  Version=$version; Size=$fileSize; ExpandedSize = $expandedSize; BaseSize= 0; DeltaSize = 0; BuildTime=$formattedDate }
        )
        # Export to CSV file
        $data | Export-Csv -Path $CsvFile -NoTypeInformation -Append
    
        $editionTarImageUrl = "https://${StorageAccount}.blob.core.windows.net/${storageContainer}/${fileNameWithoutSuffix}.tar"
        $latestTarImageUrl = "https://${StorageAccount}.blob.core.windows.net/${storageContainer}/nanoserver_${product}.tar"
        azcopy copy "${fileNameWithoutSuffix}.tar" "${editionTarImageUrl}"
        azcopy copy "${fileNameWithoutSuffix}.tar" "${latestTarImageUrl}"
        Log -text "Container image ${fileNameWithoutSuffix}.tar save to storage ${storageContainer}"
    }
}

Pop-Location
