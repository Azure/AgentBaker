######################################################################
#                                                                    #
# Based on work from Anthony Nandaa
# https://microsoft.visualstudio.com/WCCT/_git/windows-container-tools?path=%2Fimages%2Fimport_layers%2FImport-Layers.ps1  #
# Windows Containers Core Team, Microsoft                            #
#                                                                    #
######################################################################
#Assume ProductMap key is ws2019, ws2022
param (
    [Parameter(Mandatory = $true)]
    [hashtable]$ProductMap,
    [Parameter(Mandatory = $true)]
    [string]$StroageAccount,
    [Parameter(Mandatory = $true)]
    [string]$ContainerRegistry,
    [Parameter(Mandatory = $true)]
    [string]$CsvFile
)

$stdoutFile = [System.IO.Path]::GetTempFileName()
$stderrFile = [System.IO.Path]::GetTempFileName()


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

function ValidateWorkingDir {
    param (
        [string]$WorkingDir
    )

    Write-Host "Working Dir: '$WorkingDir'"
    # make sure the WORKDIR has all the needed files:
    # manifest.json and the rest
    # based on the manifest.json, see if all the layers
    # and the config file is present.

    # check for manifest.json file
    $manifestFiles = Get-ChildItem -Path $WorkingDir -Filter "*manifest.json" -File
    if ($manifestFiles.Count -ne 1) {
        throw "Validation: Only one `manifest.json` file should be present in the working directory."
    }

    $version = ($manifestFiles[0].Name -split '_|\.' | Select-Object -Index 5, 6, 7) -join '.'

    $jsonFilePath = $manifestFiles[0].FullName
    # Read JSON file content
    $jsonData = Get-Content -Raw -Path $jsonFilePath | ConvertFrom-Json
    # Iterate through layers and display size info
    # Use version info to piggyback layered size
    foreach ($layer in $jsonData.layers) {
        Write-Host "Layer Digest: $($layer.digest)"
        Write-Host "Size: $($layer.size) bytes"
        Write-Host "--------------------------------------"
        $version = $version + "_" + $($layer.size)
    }

    return $version
}

function ValidateManifest {
    param (
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [Parameter(Mandatory = $true)]
        [hashtable]$sha256Map
    )
    # just checks that the config file and the layer tarballs
    # are present in the WorkingDir
    
    # validate the manifest.json file
    Write-Host "[*] Validating the manifest.json file" -ForegroundColor Yellow

    $jsonFiles = Get-ChildItem -Path $Path -Filter "*manifest.json" -File
    $jsonContent = Get-Content -Path $jsonFiles[0].FullName -Raw
    
    $manifest = $jsonContent | ConvertFrom-Json
    $configDigest = $($manifest.config.digest).Split(":")[1]
    
    # check config file
    if (-not $sha256Map.ContainsKey($configDigest)) {
        throw "Validation: config file referenced in the manifest is not present: sha256:$configDigest"
    }
    else {
        Write-Host "config file present"
    }

    # check layers
    $err = $false
    foreach ($layer in $manifest.layers) {
        $layerDigest = $layer.digest.Split(":")[1]
        if (-not $sha256Map.ContainsKey($layerDigest)) {
            $err = $true
            Write-Warning "layer with sha256:$layerDigest missing"
        }
    }
    if ($err) {
        throw "Some layer tarballs missing"
    }
    else {
        Write-Host "all layer tarballs present"
    }
}

function GetSha256Map {
    param (
        [Parameter(Mandatory = $true)]
        [string]$DirectoryPath
    )
    
    # Create a hashtable to store the SHA256 -> FilePath map
    $sha256Map = @{}
    
    $files = Get-ChildItem -Path $DirectoryPath -File
    
    # Initialize progress bar
    $totalFiles = $files.Count
    $counter = 0
    
    foreach ($file in $files) {
        $counter++
        $percentComplete = [math]::Round(($counter / $totalFiles) * 100, 2)
        
        # Update progress bar
        Write-Progress -Activity "Calculating SHA256 hashes" -Status "Processing $counter of $totalFiles files" -PercentComplete $percentComplete
        
        try {
            $fileHash = Get-FileHash -Path $file.FullName -Algorithm SHA256
            $sha256Map[$fileHash.Hash.ToLower()] = $file.FullName
        } catch {
            Write-Warning "Failed to compute SHA256 for file: $($file.FullName). Error: $_"
        }
    }
    
    # Clear the progress bar after completion
    Write-Progress -Activity "Calculating SHA256 hashes" -Status "Completed" -PercentComplete 100
    
    return $sha256Map
}

function CreateRegistryTree {

    # do some garbage collection
    Write-Host "Cleaning up registry directories"
    $path = "$($env:root)/Import_Layers/"
    if (Test-Path -Path $path) {
        Remove-Item $path -Recurse -Force 
    }
    Write-Host "Cleaning complete" -ForegroundColor Green

    Write-Host "Creating the registry directory tree." -ForegroundColor Green
    $RegRoot = "$($env:root)/Import_Layers/$(GetRandomString 8)"

    Write-Host "=== Registry Path: $RegRoot"

    # re-create a content store similar to that of remote container regitry
    $dockerRegPath = "$RegRoot/data/docker/registry/v2"
    $blobPath = "$dockerRegPath/blobs/sha256"
    $repoPath = "$dockerRegPath/repositories"

    $paths = @($dockerRegPath, $blobPath, $repoPath)
    foreach ($path in $paths) {
        if (-not (Test-Path -Path $path)) {
            mkdir $path -Force > $null
        }
    }
    Write-Host "Registry data directories created"

    return $RegRoot
}

function GetRandomString {
    param (
        [int]$length = 5
    )

    $chars = @()
    $chars += [char[]](65..90)  # A-Z
    $chars += [char[]](97..122) # a-z

    $randomString = -join (1..$length | ForEach-Object { $chars[$(Get-Random -Minimum 0 -Maximum $chars.Length)] })
    return $randomString
}

function CreateBlobs {
    param (
        [Parameter(Mandatory = $true)]
        [string]$RegRoot,
        [Parameter(Mandatory = $true)]
        [hashtable]$Sha256Map
    )

    Write-Host "Creating SHA256 blobs" -ForegroundColor Green
    
    $Sha256Map.GetEnumerator() | ForEach-Object {
        [string]$digest = $_.Key
        [string]$src = $_.Value

        $relPath = $digest.Substring(0,2) + "/" + $digest
        $fullPath = "$RegRoot/data/docker/registry/v2/blobs/sha256/$relPath"
        # skip if already present
        if (Test-Path -Path $fullPath -PathType Leaf) {
            Write-Host "=== sha256:$digest already present"
        }
        else {
            mkdir $fullPath -Force > $null
            $blob = "$fullPath/data"
            Copy-Item $src $blob
            Write-Host "=== Created: $blob"
        }
    }
}

function CreateRepo {
    param (
        [Parameter(Mandatory = $true)]
        [string]$WorkingDir,
        [Parameter(Mandatory = $true)]
        [string]$RepoTag,
        [Parameter(Mandatory = $true)]
        [string]$RegRoot,
        [Parameter(Mandatory = $true)]
        [hashtable]$Sha256Map
    )
    # assuming repo tag is clean (no spaces, special chars, etc.)
    # simple clean-up
    $RepoTag = $RepoTag -replace '[^a-zA-Z0-9]', ''
    $RepoTag = $RepoTag.ToLower()

    $repoDir = "$RegRoot/data/docker/registry/v2/repositories/$RepoTag"
    # remove previous links if present
    if (Test-Path -Path $repoDir) {
        Remove-Item -Recurse -Force $repoDir
    }

    # setup _manifests
    # get the SHA256 for the manifest.json file
    Write-Host "Creating repository _manifests and _layers links" -ForegroundColor Green
    Write-Host "=== Image Tag (after clean-up): $RepoTag"
    Write-Host "=== Repo Path: $repoDir"

    $jsonFiles = Get-ChildItem -Path $WorkingDir -Filter "*manifest.json" -File
    $manifestSHA = (Get-FileHash -Path $jsonFiles[0].FullName -Algorithm SHA256).Hash.ToLower()
    
    Write-Host "=== Manifest File: $($jsonFiles[0].FullName)"
    Write-Host "=== Manifest SHA256: $manifestSHA"

    $indexPath = "$repoDir/_manifests/tags/latest/index/sha256/$manifestSHA"
    $currentPath = "$repoDir/_manifests/tags/latest/current"
    $revPath = "$repoDir/_manifests/revisions/sha256/$manifestSHA"
    mkdir $indexPath > $null
    mkdir $currentPath > $null
    mkdir $revPath > $null
    "sha256:$manifestSHA" | Out-File "$indexPath/link" -NoNewline  -Encoding UTF8
    "sha256:$manifestSHA" | Out-File "$currentPath/link" -NoNewline  -Encoding UTF8
    "sha256:$manifestSHA" | Out-File "$revPath/link" -NoNewline  -Encoding UTF8

    # setup _layers
    # we create layers for all blobs, except the manifest
    # this will typically be for the base layer, delta layer 
    # and config (which is treated as a layer too by distribution/distribution)
    $Sha256Map.GetEnumerator() | ForEach-Object {
        [string]$digest = $_.Key
        if ($digest -ne $manifestSHA) {
            mkdir "$repoDir/_layers/sha256/$digest" > $null
            "sha256:$digest" | Out-File "$repoDir/_layers/sha256/$digest/link" -NoNewline -Encoding UTF8
        }
    }
    Write-Host "manifest links created successfully" -ForegroundColor Green

    return $RepoTag
}

function GetAvailablePort {
    param (
        [int]$startingPort = 5050
    )

    $port = Get-Random -Minimum $startingPort -Maximum $($startingPort + 1000)
    # TODO: to work on a script that gets any available port from 5050
    return $port
}

function StartRegistryServer {
    param (
        [Parameter(Mandatory = $true)]
        [string]$RegRoot
    )

    Write-Host "Starting the registry.exe server ..."
    
    Push-Location .
    Set-Location $RegRoot

    $port = GetAvailablePort

    if (Test-Path -Path .\config) {
        Remove-Item -Recurse -Force .\config
    }

    mkdir config > $null
    $regConfigPath = ".\config\config.yml"
    Set-Content $regConfigPath @"
version: 0.1
log:
  level: debug
  fields:
    service: registry
    environment: development
storage:
  filesystem:
    rootdirectory: "$($RegRoot -replace '\\', '/')/data"
http:
  addr: :$port
  secret: d3addr0p
"@

    Write-Host "=== Registry Config Path: $regConfigPath"

    # Hardcoded for now, as this is from go install on agent pool machine
    $exePath = "$env:GOBIN\registry.exe"

    $command = "serve $regConfigPath"
    $serverProcess = Start-Process -FilePath $exePath -ArgumentList $command -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdoutFile -RedirectStandardError $stderrFile

    # go back to where you started
    Pop-Location

    return @($serverProcess, $port)
}

function RunDockerPull {
    param (
        [Parameter(Mandatory = $true)]
        [System.Diagnostics.Process]$ServerProcess,
        [Parameter(Mandatory = $true)]
        [int]$Port,
        [Parameter(Mandatory = $true)]
        [hashtable]$TagMap,
        [Parameter(Mandatory = $true)]
        [hashtable]$VersionMap
    )
    foreach($key in $TagMap.Keys) {
        Write-Host "Working on product: $key, RepsTag: $($TagMap[$key])"
        $RepoTag = $TagMap[$key]
        $dockerImageTag = "localhost:$Port/$RepoTag"
        Write-Host "Pulling composed image into Docker"
        Write-Host "Image Tag: $dockerImageTag"
        docker pull $dockerImageTag

        Write-Host "Image pulled sucessfully"
        # Write-Host "Validate image with a basic 'docker run'"
        # docker run --rm -d $dockerImageTag

        Write-Host "Image composed successfully!"

        # tagging with the acr name
        $tag = $Key.Substring(2,4)
        docker tag $dockerImageTag "${ContainerRegistry}.azurecr.io/windows/servercore:ltsc$tag"
        docker push "${ContainerRegistry}.azurecr.io/windows/servercore:ltsc$tag"
                Write-Host "Container image pushed servercore:ltsc$tag"

        $versionWithDateWithSize = $VersionMap[$key]
        $version = ($versionWithDateWithSize -split '_|\.' | Select-Object -Index 0, 1) -join '.'
        $buildTime = $versionWithDateWithSize -split '_|\.' | Select-Object -Index 2
        docker tag $dockerImageTag "${ContainerRegistry}.azurecr.io/windows/servercore:$version"
        docker push "${ContainerRegistry}.azurecr.io/windows/servercore:$version"
        Write-Host "Container image pushed servercore:$version"

        # 1809 applied to nano server 2019 only
        # if ($key -eq "ws2019") {
        #     $officialTag = "1809"
        # } else {
        #     $officialTag = "ltsc2022"
        # }

        #tag with offical tag and save it to tar file, this is going to be used by agentbaker
        docker tag $dockerImageTag "mcr.microsoft.com/windows/servercore:ltsc$tag"
        docker save -o "servercore_$tag.tar" "mcr.microsoft.com/windows/servercore:ltsc$tag"

        $expandedSize = (Get-Item "servercore_$tag.tar").Length
        $formattedDate = Convert-CustomTimestamp $buildTime
        $timeStamp = Get-Date -Format "yyyy-MM-ddTHH:mm:ss.0000000Z"
        $baseSize = $versionWithDateWithSize -split '_|\.' | Select-Object -Index 3
        $deltaSize = $versionWithDateWithSize -split '_|\.' | Select-Object -Index 4
        # Define data as an array of objects
        $data = @(
            [PSCustomObject]@{ TimeStamp=$timeStamp; WindowsSku=$key; Flavor="servercore";  Version=$version; Size=0; ExpandedSize = $expandedSize; BaseSize= $baseSize; DeltaSize = $deltaSize; BuildTime=$formattedDate;  }
        )
        # Export to CSV file
        $data | Export-Csv -Path $CsvFile -NoTypeInformation -Append

        $editionTarImageUrl = "https://${StroageAccount}.blob.core.windows.net/${Key}-container/servercore_$tag.tar"
        azcopy copy "servercore_$tag.tar" "${editionTarImageUrl}"
        Write-Host "Container image servercore_$tag.tar save to storage ${Key}-container"
    }
}

function StopRegistryServer {
    param (
        [Parameter(Mandatory = $true)]
        [System.Diagnostics.Process]$ServerProcess
    )

    # stop the server process
    Stop-Process -Id $serverProcess.Id -Force

    # Wait for the process to exit
    $serverProcess.WaitForExit()
    # Read the output from the temporary files
    $output = Get-Content $stdoutFile
    $errorOutput = Get-Content $stderrFile
        # Display the output
    Write-Host "Standard Output of registry:"
    Write-Host $output

    Write-Host "Standard Error of registry:"
    Write-Host $errorOutput

    # Clean up temporary files
    #Remove-Item $stdoutFile
    #Remove-Item $stderrFile
    Write-Host "=== Registry server terminated"
}

# main script
$RegRoot = CreateRegistryTree

$tagMap = @{}
$versionMap = @{}
foreach ($key in $ProductMap.Keys) {
    Write-Output "Working on product: $key, WorkingDir: $($ProductMap[$key])"
    $WorkingDir = $ProductMap[$key]
    
    # validation
    $versionMap[$key] = ValidateWorkingDir $WorkingDir
    [hashtable]$sha256Map = GetSha256Map $WorkingDir
    ValidateManifest $WorkingDir $sha256Map
    
    CreateBlobs -RegRoot $RegRoot -Sha256Map $sha256Map
    # create repo for the new image
    $repoTag = $(GetRandomString 8)
    $repoTagCleaned = CreateRepo -WorkingDir $WorkingDir -RepoTag "$repoTag" -RegRoot $RegRoot -Sha256Map $sha256Map

    $tagMap[$key] = $repoTagCleaned
}

# start serving the image
$serverProcess, $port = StartRegistryServer -RegRoot $RegRoot
# pause for 3 seconds to allow server to initialize
Start-Sleep -Seconds 3

# pull the image from the registry
RunDockerPull -ServerProcess $serverProcess -Port $port -TagMap $tagMap -VersionMap $versionMap

Write-Host "Stop registry server!"
StopRegistryServer -ServerProcess $serverProcess
