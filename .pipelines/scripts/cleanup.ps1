$StorageAccount="wcctagentbakerstorage"

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

# The following are to clean up 2019/2022 container base image
$baseImageStorages = @("ws2019-container", "ws2022-container")
foreach ($container in $baseImageStorages) {
    Log -text "Start to clean up the old images from storage container: $container"
    $containerFiles = az storage blob list --account-name $StorageAccount --auth-mode login -c $container --query "[?starts_with(name, 'CBaseOs') && ends_with(name, 'tar.gz')].{name:name, timeCreated:properties.creationTime}" | ConvertFrom-Json | Sort-Object -Property timeCreated -Descending
    if ($null -ne $containerFiles) {
        if ($containerFiles -isnot [System.Collections.ArrayList]) {
            $containerFiles = [System.Collections.ArrayList]@($containerFiles)
        }

        # Keep the last 10 images
        $containersToKeep = $containerFiles[0..9]
        # Delete all other images
        foreach ($containerFile in $containerFiles) {
            if ($containerFile -notin $containersToKeep) {
                # Remove the old image from storage
                $fliesToBeDeleted = @()
                $containerFileName = ${containerFile}.name
                $fliesToBeDeleted += $containerFileName
                $cfgFileName = "${containerFileName}.config.json"
                $fliesToBeDeleted += $cfgFileName
                $manifestFileName = "${containerFileName}.manifest.json"
                $fliesToBeDeleted += $manifestFileName
                $unzippedContainerFileName = $containerFileName -replace '\.tar\.gz$', '.tar'
                $fliesToBeDeleted += $unzippedContainerFileName

                foreach ($fileToBeDeleted in $fliesToBeDeleted) {
                    Log -text "Remove the old containerfile : ${fileToBeDeleted}"

                    az storage blob delete --account-name $StorageAccount --auth-mode login -c $container -n $fileToBeDeleted
                    if ($LastExitCode -eq 0) {
                        Log -text "Image: $fileToBeDeleted has been removed from storage successfully."
                    } else {
                        Log -text "Error happaned when trying to remove Image: $fileToBeDeleted from storage."
                    }
                }
            }
        }
    }
}

Log -text "Start to clean up the old rs_sparc_ctr_serverdatacentercore- image from resource group"
# we need to keep the "rs_sparc_ctr_serverdatacentercore", so the last "-" is important to be added in the query pattern
$sparcImages = az image list --resource-group "wcct-agentbaker-test" --query "[?starts_with(name, 'rs_sparc_ctr_serverdatacentercore-')].{name:name, timeCreated:tags.TimeStamp}" | ConvertFrom-Json | Sort-Object -Property timeCreated -Descending
if ($null -ne $sparcImages) {
    if ($sparcImages -isnot [System.Collections.ArrayList]) {
        $sparcImages = [System.Collections.ArrayList]@($sparcImages)
    }

    # Keep the last 10 vhd
    $sparcImagesToKeep = $sparcImages[0..9]
    # Delete all other vhds
    foreach ($sparcImage in $sparcImages) {
        if ($sparcImage -notin $sparcImagesToKeep) {
            # Remove the old image from storage
            $imageName = ${sparcImage}.name
            Log -text "Remove the old sparc 1es image : ${imageName}"

            az image delete --name $imageName --resource-group "wcct-agentbaker-test"
            if ($LastExitCode -eq 0) {
                Log -text "1es Image: $imageName has been removed from resource group successfully."
            } else {
                Log -text "Error happaned when trying to remove Image: $imageName from resourcegroup."
            }
        }
    }
}

$StorageContainer="from-sparc"
Log -text "Start to clean up the old openssh cab files from storage container: $StorageContainer"
$opensshFiles = az storage blob list --account-name $StorageAccount --auth-mode login -c $StorageContainer --query "[?ends_with(name, '.cab')].{name:name, timeCreated:properties.creationTime}" | ConvertFrom-Json | Sort-Object -Property timeCreated -Descending
if ($null -ne $opensshFiles) {
    if ($opensshFiles -isnot [System.Collections.ArrayList]) {
        $opensshFiles = [System.Collections.ArrayList]@($opensshFiles)
    }

    # Keep the last 10 openssh
    $opensshsToKeep = $opensshFiles[0..9]
    # Delete all other opensshs
    foreach ($opensshFile in $opensshFiles) {
        if ($opensshFile -notin $opensshsToKeep) {
            # Remove the old image from stroage
            $opensshFileName = ${opensshFile}.name
            Log -text "Remove the old opensshfile : ${opensshFileName}"

            az storage blob delete --account-name $StorageAccount --auth-mode login -c $StorageContainer -n $opensshFileName
            if ($LastExitCode -eq 0) {
                Log -text "Image: $opensshFileName has been removed from storage successfully."
            } else {
                Log -text "Error happaned when trying to remove Image: $opensshFileName from storage."
            }
        }
    }
}

Log -text "Start to clean up 2025 container base images from storage container"
$editions=@("servercore", "nanoserver")
foreach ($edition in $editions) {
    Log -text "working on edition: $edition, removing from stroage container: $StorageContainer"

    # Start to do some clean ups, this will include clean up the storage and the ACR
    # Remove old content from storage
    # The ConvertFrom-Json will work with PWSH but maynot with old version of PS
    $editionZipFiles = az storage blob list --account-name $StorageAccount --auth-mode login -c $StorageContainer --prefix $edition --query "[?ends_with(name, 'tar.gz')].{name:name, timeCreated:properties.creationTime}" | ConvertFrom-Json | Sort-Object -Property timeCreated -Descending
    if ($null -ne $editionZipFiles) {
        if ($editionZipFiles -isnot [System.Collections.ArrayList]) {
            $editionZipFiles = [System.Collections.ArrayList]@($editionZipFiles)
        }

        # Keep the last 10 images, plus the duplicate latest tar
        $zipsToKeep = $editionZipFiles[0..10]
        # Delete all other images
        foreach ($zipFile in $editionZipFiles) {
            if ($zipFile -notin $zipsToKeep) {
                # Remove the old image from ACR
                Log -text "Remove the old zipfile : ${zipFile}"

                $zipFileName = ${zipFile}.name
                $fileNameWithoutZip = [System.IO.Path]::GetFileNameWithoutExtension($zipFileName)
                az storage blob delete --account-name $StorageAccount --auth-mode login -c $StorageContainer -n "$fileNameWithoutZip"
                if ($LastExitCode -eq 0) {
                    Log -text "Image: ${fileNameWithoutZip} has been removed from storage successfully."
                } else {
                    Log -text "Error happaned when trying to remove Image: ${fileNameWithoutZip} from storage, probably it does not exist"
                }
                az storage blob delete --account-name $StorageAccount --auth-mode login -c $StorageContainer -n $zipFileName
                if ($LastExitCode -eq 0) {
                    Log -text "Image: $zipFileName has been removed from storage successfully."
                } else {
                    Log -text "Error happaned when trying to remove Image: $zipFileName from storage."
                }
            }
        }
    }
}

Log -text "Start to clean up the old images from storage container: vhds"
# Clean up the vhds and ssh-vhds container
$vhdFiles = az storage blob list --account-name $StorageAccount --auth-mode login -c "vhds" --query "[?starts_with(name, 'rs_sparc_ctr_serverdatacentercore')].{name:name, timeCreated:properties.creationTime}" | ConvertFrom-Json | Sort-Object -Property timeCreated -Descending
if ($null -ne $vhdFiles) {
    if ($vhdFiles -isnot [System.Collections.ArrayList]) {
        $vhdFiles = [System.Collections.ArrayList]@($vhdFiles)
    }

    # Keep the last 7 vhd
    $vhdsToKeep = $vhdFiles[0..6]
    # Delete all other vhds
    foreach ($vhdFile in $vhdFiles) {
        if ($vhdFile -notin $vhdsToKeep) {
            # Remove the old image from storage
            $vhdFileName = ${vhdFile}.name
            Log -text "Remove the old opensshfile : ${vhdFileName}"

            az storage blob delete --account-name $StorageAccount --auth-mode login -c "vhds" -n $vhdFileName
            if ($LastExitCode -eq 0) {
                Log -text "vhd: $vhdFileName has been removed from storage successfully."
            } else {
                Log -text "Error happaned when trying to remove Image: $vhdFileName from storage."
            }
        }
    }
}

Log -text "Start to clean up the old images from storage container: ssh-vhds"
$sshvhdFiles = az storage blob list --account-name $StorageAccount --auth-mode login -c "ssh-vhds" --query "[?starts_with(name, 'rs_sparc_ctr_serverdatacentercore')].{name:name, timeCreated:properties.creationTime}" | ConvertFrom-Json | Sort-Object -Property timeCreated -Descending
if ($null -ne $sshvhdFiles) {
    if ($sshvhdFiles -isnot [System.Collections.ArrayList]) {
        $sshvhdFiles = [System.Collections.ArrayList]@($sshvhdFiles)
    }

    # Keep the last 7 ssh-vhd
    $sshvhdsToKeep = $sshvhdFiles[0..6]
    # Delete all other ssh-vhds
    foreach ($sshvhdFile in $sshvhdFiles) {
        if ($sshvhdFile -notin $sshvhdsToKeep) {
            # Remove the old image from storage
            $sshvhdFileName = ${sshvhdFile}.name
            Log -text "Remove the old sshvhdfile : ${sshvhdFileName}"

            az storage blob delete --account-name $StorageAccount --auth-mode login -c "ssh-vhds" -n $sshvhdFileName
            if ($LastExitCode -eq 0) {
                Log -text "ssh-vhd: $sshvhdFileName has been removed from storage successfully."
            } else {
                Log -text "Error happaned when trying to remove Image: $sshvhdFileName from storage."
            }
        }
    }
}



# The following are the containers for us to get the vhd image from WSD
$validationContainers = @("ws2019", "ws2022", "ws2022-gen2", "ws23h2", "ws23h2-gen2")
Log -text "Start to clean up the old images from storage container: $validationContainers"

foreach ($container in $validationContainers) {
    Log -text "Start to clean up the old images from storage container: $container"
    $containerFiles = az storage blob list --account-name $StorageAccount --auth-mode login -c $container --query "[?ends_with(name, 'vhd')].{name:name, timeCreated:properties.creationTime}" | ConvertFrom-Json | Sort-Object -Property timeCreated -Descending
    if ($null -ne $containerFiles) {
        if ($containerFiles -isnot [System.Collections.ArrayList]) {
            $containerFiles = [System.Collections.ArrayList]@($containerFiles)
        }

        # Keep the last 7 images
        $containersToKeep = $containerFiles[0..6]
        # Delete all other images
        foreach ($containerFile in $containerFiles) {
            if ($containerFile -notin $containersToKeep) {
                # Remove the old image from storage
                $containerFileName = ${containerFile}.name
                Log -text "Remove the old containerfile : ${containerFileName}"

                az storage blob delete --account-name $StorageAccount --auth-mode login -c $container -n $containerFileName
                if ($LastExitCode -eq 0) {
                    Log -text "Image: $containerFileName has been removed from storage successfully."
                } else {
                    Log -text "Error happaned when trying to remove Image: $containerFileName from storage."
                }
            }
        }
    }
}

#TODO: the validation container is used to store the container base image from MSINT, can be disabled as this route is now broken
Log -text "Start to clean up the old images from storage container: validation"
$containerFiles = az storage blob list --account-name $StorageAccount --auth-mode login -c validation --query "[?ends_with(name, 'tar.gz')].{name:name, timeCreated:properties.creationTime}" | ConvertFrom-Json | Sort-Object -Property timeCreated -Descending
if ($null -ne $containerFiles) {
    if ($containerFiles -isnot [System.Collections.ArrayList]) {
        $containerFiles = [System.Collections.ArrayList]@($containerFiles)
    }

    # Keep the last 10 images
    $containersToKeep = $containerFiles[0..9]
    # Delete all other images
    foreach ($containerFile in $containerFiles) {
        if ($containerFile -notin $containersToKeep) {
            # Remove the old image from storage
            $containerFileName = ${containerFile}.name
            Log -text "Remove the old containerfile : ${containerFileName}"

            az storage blob delete --account-name $StorageAccount --auth-mode login -c validation -n $containerFileName
            if ($LastExitCode -eq 0) {
                Log -text "Image: $containerFileName has been removed from storage successfully."
            } else {
                Log -text "Error happaned when trying to remove Image: $containerFileName from storage."
            }
        }
    }
}