param(
    [string] $StorageAccount,
    [string] $StorageContainer,
    [string] $ContainerRegistry,
    [string] $CsvFile
)

# $StorageAccount = "wcctagentbakerstorage"
# $StorageContainer = "from-sparc"
# $ContainerRegistry = "containerrollingregistry"

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

docker version

$baseDir = $PSScriptRoot

# login to azure ACR
az acr login --name $ContainerRegistry
if ($LastExitCode -eq 0) {
    Log -text "login to azure ACR $ContainerRegistry successfully."
}
else {
    Log -text "Error happaned when try to login to azure ACR ${ContainerRegistry}: $_"
    return -1
}

# Remove all unused images
Log -text "Cleanup existing container images."

docker image prune -a -f

$editions=@("servercore", "nanoserver")

# clean up the former directories if any
$imagesPath = Join-Path $baseDir -ChildPath "containerImages"
New-Item -Path $imagesPath -ItemType Directory -Force | Out-Null

Set-Location -Path $imagesPath

# $sasExpiryDate = (Get-Date).AddDays(1).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
# $sasToken=$(az storage container generate-sas --account-name $StorageAccount --as-user --auth-mode login --permissions "rwld" --expiry $sasExpiryDate --name $StorageContainer --https-only --output tsv)

foreach ($edition in $editions) {
    #Just in case we missed anything before 
    $editionFiles = az storage blob list --account-name $StorageAccount --auth-mode login --container-name $StorageContainer --prefix $edition --query "[?ends_with(name, 'tar.gz')].[name, properties.lastModified]" --output tsv | Sort-Object { [DateTime]::Parse($_.Split("`t")[1]) } -Descending | Select-Object -First 1

    foreach ($editionFile in $editionFiles) {
        
        $editionFileName = $editionFile.Split("`t")[0]
        $fileNameWithoutSuffix = $editionFileName -replace '\.tar\.gz$', ''
        $tarFileName = $fileNameWithoutSuffix + ".tar"
        $version = ($fileNameWithoutSuffix -split '-|\.' | Select-Object -Index 1, 2) -join '.'
        
        # $fileExists = az storage blob show --account-name $StorageAccount --auth-mode login --container-name $StorageContainer --name $tarFileName
        # if ($null -ne $fileExists) {
        #     Log -text "Container base image exists for $fileNameWithoutSuffix, skip"
        #     continue
        # }

        $edtionZipImageUrl = "https://${StorageAccount}.blob.core.windows.net/${StorageContainer}/${editionFileName}?${sasToken}"
    
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
        docker tag $shortId "${ContainerRegistry}.azurecr.io/${edition}:$version"
        docker push "${ContainerRegistry}.azurecr.io/${edition}:$version"
        Log -text "Container imag epushed ${edition}:$version"
    
        # Also tag it to the lastest
        docker tag $shortId "${ContainerRegistry}.azurecr.io/${edition}:latest"
        docker push "${ContainerRegistry}.azurecr.io/${edition}:latest"
        Log -text "Container image pushed ${edition}:latest"
    
        Remove-Item -Path "${fileNameWithoutSuffix}.tar" -ErrorAction SilentlyContinue
        docker tag $shortId "mcr.microsoft.com/windows/${edition}:ltsc2022"
        docker save -o "${fileNameWithoutSuffix}.tar" "mcr.microsoft.com/windows/${edition}:ltsc2022"

        $expandedSize = (Get-Item "${fileNameWithoutSuffix}.tar").Length
        $timeStamp = Get-Date -Format "yyyy-MM-ddTHH:mm:ss.0000000Z"
        # Define data as an array of objects
        # BuildTime is missing from the image, so we set it to empty string
        $buildTime = ""
        $data = @(
            [PSCustomObject]@{ TimeStamp=$timeStamp; WindowsSku="activebranch"; Flavor=$edition;  Version=$version; Size=$fileSize; ExpandedSize = $expandedSize; BaseSize= 0; DeltaSize = 0; BuildTime=$buildTime }
        )
        # Export to CSV file
        $data | Export-Csv -Path $CsvFile -NoTypeInformation -Append
    
        $editionTarImageUrl = "https://${StorageAccount}.blob.core.windows.net/${StorageContainer}/${fileNameWithoutSuffix}.tar"
    
        azcopy copy "${fileNameWithoutSuffix}.tar" "${editionTarImageUrl}"
    
        Log -text "Container image ${fileNameWithoutSuffix}.tar save to storage ${StorageContainer}"
    }
}
