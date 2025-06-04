# Check all the condtions being met in order to start the deployment

param(
    [string]$buildNumber
)

function LoginToAzure {
    $apiVersion = "2020-06-01"
    $resource = "api://AzureADTokenExchange"
    $endpoint = "{0}?resource={1}&api-version={2}" -f $env:IDENTITY_ENDPOINT,$resource,$apiVersion
    $secretFile = ""
    try
    {
        Invoke-WebRequest -Method GET -Uri $endpoint -Headers @{Metadata='True'} -UseBasicParsing
    }
    catch
    {
        $wwwAuthHeaders = $_.Exception.Response.Headers
        #Write-Host "headers: " $wwwAuthHeaders
        foreach ($header in $wwwAuthHeaders) {
            if ($header.Key -eq "WWW-Authenticate") {
                $wwwAuthHeader = $header.Value
                if ($wwwAuthHeader -match "Basic realm=.+")
                {
                    $secretFile = ($wwwAuthHeader -split "Basic realm=")[1]
                }
            }
        }
    }
    Write-Host "Secret file path: " $secretFile`n
    $secret = cat -Raw $secretFile
    $response = Invoke-WebRequest -Method GET -Uri $endpoint -Headers @{Metadata='True'; Authorization="Basic $secret"} -UseBasicParsing
    if ($response)
    {
        $token = (ConvertFrom-Json -InputObject $response.Content).access_token
        #Write-Host "Access token: " $token
    }
    
    az login --service-principal -u $aadAppId --federated-token $token --tenant $aadTenant

    $env:AZCOPY_AUTO_LOGIN_TYPE = "AZCLI"
}

#Define the enviroment variables that will be used in the scripts
$global:subscriptionName="CoreOS_DPLAT_WCCT_Demo"
$global:buildPath = "\\winbuilds\release\rs_sparc_ctr"
$global:containerBaseOsPkgsPath = "amd64fre/containerbaseospkgs"

$global:aadAppId = "aa0caef4-31da-4426-9908-f75af27075c8"
$global:aadTenant = "72f988bf-86f1-41af-91ab-2d7cd011db47"
$global:registryName = "containerrollingregistry"
$global:keyVaultName = "containerrollingKV"
$global:fromSparcUrl="fromsparcsasurl"

# This will need to be synced with the CAPZ scripts, please note it is case sensitive
$global:capzKubernetesVersion = "v1.27.0"
$global:capzContainerdVersion = "1.7.6"

$global:editionDirectories = @(
    # TODO: Add the other editions later
    "cbaseospkg_nanoserver_en-us",
    #"cbaseospkg_serverdatacentercore_ltsc_en-us_vl",
    "cbaseospkg_serverdatacentercore_ltsc_en-us_vl"
)

$global:resourceGroup = "containerrolling"
$global:location = "eastus"
$global:galleryName = "windowsservercore"
$global:sku = "containerrolling"
$global:imageDefName = "rs_sparc_ctr_v1"
$global:storageAccountName = "containerrollingstorage"

#$runningDir = "D:\github\windows-testing-dev"
$global:runningDir = $PSScriptRoot
#TODO: to make the script to run everywhere such as agentpool, will need to make this version file to be globally access instead of being saved locally.
$global:versionFile = "version.txt"
# what is the difference between en-us and en-us_vl?
$hostImagePath = "amd64fre/vhd/vhd_server_serverdatacentercore_en-us_vl"
$cabPath="amd64fre\FeaturesOnDemand\neutral\cabs"

#clean up the logs directory
$logsPath = Join-Path $runningDir -ChildPath "logs"

# Check if the "logs" directory exists
if (Test-Path $logsPath -PathType Container) {
    # Get all items within the "logs" directory and remove them
    #Get-ChildItem -Path $logsPath | Remove-Item -Force -Recurse  -ErrorAction SilentlyContinue
} else {
    # Create the "logs" directory
    New-Item -Path $logsPath -ItemType Directory -Force | Out-Null
}

$timeStamp = Get-Date -Format "ddMMyyHHmmss"
$logFile = "log_$($timeStamp).log"
$global:logFileFullName = Join-Path $logsPath -ChildPath $logFile
"" | Out-File -FilePath $logFileFullName

$global:LOGPATH = $logFileFullName

$latestBuilds = @()
# Check if the build number is provided
if ([string]::IsNullOrEmpty($buildNumber)) {
    $message = "Build number is not provided."
    Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
# Pick the first 2 build to try to avoid the scenerio where the latest build image are still being built
    $latestBuilds = Get-ChildItem -Path $buildPath | Sort-Object -Property LastWriteTime -Descending | Select-Object -First 2
} else {
    $latestBuilds = Get-ChildItem -Path $buildPath | Sort-Object -Property LastWriteTime -Descending | Where-Object { $_.Name -eq $buildNumber }
}

$buildFound = $False

if ($null -ne $latestBuilds) {
    foreach ($latestBuild in $latestBuilds) {
        $global:currentBuildName = $latestBuild.Name
           # Extract the OS version from the given string
        $global:osVersion =  $global:currentBuildName.Split('.')[0..1] -join '.'

        # check if container edition image exists
        # Construct the full path to the subdirectory
        $global:containerBaseImagePath = Join-Path -Path $latestBuild.FullName -ChildPath $containerBaseOsPkgsPath
        # check if the server edtion exists or not, iterate through the editionDirectories
        $notFound = $False
        foreach ($editionDirectory in $editionDirectories) {
            $edtionBaseImagePath = Join-Path -Path $containerBaseImagePath -ChildPath $editionDirectory

            if (Test-Path -Path $edtionBaseImagePath -PathType Container) {
                $message = "container edition image exists: $edtionBaseImagePath"
                Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
            } else {
                $message = "container edition image does not exist: $edtionBaseImagePath"
                Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
                $notFound = $True
                break
            }
        }
        # Continue to search the next build if the edition images does not exist
        if ($notFound) {
            continue
        }

        # Check if the host image exists
        $hostImageFullPath = Join-Path -Path $latestBuild.FullName -ChildPath $hostImagePath
        if (Test-Path -Path $hostImageFullPath -PathType Container) {
            $message = "hostimage exists, path: $hostImageFullPath"
            Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
        } else {
            $message = "Hostimage path: $hostImageFullPath does not exist"
            Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
            continue
        }

        $global:vhdFile = Get-ChildItem -Path $hostImageFullPath -File -Filter "*.vhd" | Sort-Object Length -Descending | Select-Object -First 1
        if ($null -ne $vhdFile) {
            $message = "Found the vhd file in the: $vhdFile.FullName"
            Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
        } else {
            $message = "vhd file in path: $hostImageFullPath does not exist"
            Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
            continue
        }

        # Check if the cab file exists
        $cabFullPath = Join-Path -Path $latestBuild.FullName -ChildPath $cabPath
        if (Test-Path -Path $cabFullPath -PathType Container) {
            $message = "cab file exists, path: $cabFullPath"
            Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
        } else {
            $message = "Cab file path: $cabFullPath does not exist"
            Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
            continue
        }

        $global:cabFile = Get-ChildItem -Path $cabFullPath -File -Filter "OpenSSH-Server-Package*~amd64~~.cab" | Select-Object -First 1
        if ($null -ne $cabFile) {
            $message = "Found the cab file in the: $($cabFile.FullName)"
            Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
        } else {
            $message = "Could not find the cab file in $cabFullPath"
            Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
            continue
        }

        $global:localHostImagePath = Join-Path $runningDir -ChildPath "hostImages"
        # Set this variable to be global to be used by the uploadVhd.ps1
        $global:pathToVhdDirectory = Join-Path -Path $localHostImagePath -ChildPath $currentBuildName
        
        $buildFound = $True

        $message = "Latest build:  $currentBuildName, start the processing"
        Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
        break
    }

} else {
    $message = "No builds found in the specified directory."
    Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append   
    return -1
}

if ($False -eq $buildFound) {
    $message = "No valid builds found in the specified directory." 
    Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
    exit -1
}

# log in to azure
$message = "Log into Azure"
Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append   

#login in to azure to set the context
# az login --service-principal --username $aadAppId --tenant $aadTenant --password $certFile
LoginToAzure

az account set --subscription $subscriptionName
#az acr login --name $registryName

# if ($LastExitCode -eq 0) {
#     $message = "login to azure successfully."
#     Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
# }
# else {
#     $message = "Error happaned when try to login to azure: $_"
#     Write-Host $message; $message | Out-File -FilePath $logFileFullName -Append
#     exit -1
# }

