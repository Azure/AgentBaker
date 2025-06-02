$ContainerRegistry="containerrollingregistry"

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

Log -text "Start to clean up 2025 container base images from ACR"
$editions=@("servercore", "nanoserver")
foreach ($edition in $editions) {
    Log -text "working on edition: $edition, removing from container registry: $ContainerRegistry"
    # Remove the old image from ACR
    $imageTags = az acr repository show-tags -n ${ContainerRegistry} --repository ${edition} --orderby time_desc | ConvertFrom-Json

    if ($null -ne $imageTags) {
        if ($imageTags -isnot [System.Collections.ArrayList]) {
            $imageTags = [System.Collections.ArrayList]@($imageTags)
        }

        # Keep the last 10 images, plus the duplicate latest tag
        $tagsToKeep = $imageTags[0..10]
        # Delete all other images
        foreach ($tag in $imageTags) {
            if ($tag -notin $tagsToKeep) {
                # Remove the old image from ACR
                Log -text "Remove the old image: ${edition}:$tag"
                az acr repository delete -n "$ContainerRegistry.azurecr.io" --image "${edition}:$tag" --yes
                if ($LastExitCode -eq 0) {
                    Log -text "Image: ${edition}:$tag has been removed from ACR successfully."
                } else {
                    Log -text "Error happaned when trying gto remove Image: ${edition}:$tag from ACR"
                    return -1
                }
            }
        }
    }
}

# Remove the 2019/2022 container base image saved in ACR
$editions=@("servercore", "nanoserver")
foreach ($edition in $editions) {
    Log -text "working on edition: windows/$edition, removing from container registry: $ContainerRegistry"
    # Remove the old 2019 image from ACR
    $imageTags = az acr repository show-tags -n ${ContainerRegistry} --repository windows/${edition} --orderby time_desc | ConvertFrom-Json | Where-Object { $_ -like "17763*" }

    if ($null -ne $imageTags) {
        if ($imageTags -isnot [System.Collections.ArrayList]) {
            $imageTags = [System.Collections.ArrayList]@($imageTags)
        }

        # Keep the last 10 images, plus the duplicate latest tag
        $tagsToKeep = $imageTags[0..10]
        # Delete all other images
        foreach ($tag in $imageTags) {
            if ($tag -notin $tagsToKeep) {
                # Remove the old image from ACR
                Log -text "Remove the old image: ${edition}:$tag"
                az acr repository delete -n "$ContainerRegistry.azurecr.io" --image "windows/${edition}:$tag" --yes
                if ($LastExitCode -eq 0) {
                    Log -text "Image: windows/${edition}:$tag has been removed from ACR successfully."
                } else {
                    Log -text "Error happaned when trying gto remove Image: windows/${edition}:$tag from ACR"
                    return -1
                }
            }
        }
    }
    # Remove the old 2022 image from ACR
    $imageTags = az acr repository show-tags -n ${ContainerRegistry} --repository windows/${edition} --orderby time_desc | ConvertFrom-Json | Where-Object { $_ -like "20348*" }

    if ($null -ne $imageTags) {
        if ($imageTags -isnot [System.Collections.ArrayList]) {
            $imageTags = [System.Collections.ArrayList]@($imageTags)
        }

        # Keep the last 10 images, plus the duplicate latest tag
        $tagsToKeep = $imageTags[0..10]
        # Delete all other images
        foreach ($tag in $imageTags) {
            if ($tag -notin $tagsToKeep) {
                # Remove the old image from ACR
                Log -text "Remove the old image: ${edition}:$tag"
                az acr repository delete -n "$ContainerRegistry.azurecr.io" --image "windows/${edition}:$tag" --yes
                if ($LastExitCode -eq 0) {
                    Log -text "Image: windows/${edition}:$tag has been removed from ACR successfully."
                } else {
                    Log -text "Error happaned when trying gto remove Image: windows/${edition}:$tag from ACR"
                    return -1
                }
            }
        }
    }
}