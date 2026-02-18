[CmdletBinding()]
param(
    [string]
    [Parameter(Mandatory=$true)]
    $tenantId,
    [string]
    [Parameter(Mandatory=$true)]
    $subscriptionId,
    [string]
    [Parameter(Mandatory=$true)]
    $managedIdentityClientId,
    [string]
    [Parameter(Mandatory=$true)]
    $aadAppId,
    [string]
    [Parameter(Mandatory=$true)]
    $outputDirectory
)

Set-Variable MEDIA_SEEKER_OATH2_TOKEN_ENDPOINT "https://login.microsoftonline.com/72f988bf-86f1-41af-91ab-2d7cd011db47/oauth2/token" -Option Constant
Set-Variable MEDIA_SEEKER_BASE_URL -Value "https://mediaseeker.smf.windowsazure.com" -Option Constant
Set-Variable MEDIA_SEEKER_RESOURCE -Value "ace37fbf-105a-4751-8ac3-3f0b37fc1982" -Option Constant
Set-Variable DATA_VERSION -Value "1.0" -Option Constant

$products = @("WS23H2", "WS2022","RS5")
#$products = ("WS2022")

# RS5 Gen1 only, while WS23H2 and WS2022 have both Gen1 and Gen2
# AKS plan to support 23h2 gen2 only, we may need to adjust the test acoordingly later

$drops= @{
  #"WS23H2" = @("vhd_server_serverdatacenter_en-us_vl_30gb_azuregallery_fixed", "vhd_server_serverdatacenter_en-us_vl_30gb_gen2_azuregallery_fixed")
  # The fixed vhd does not get published to mediaseeker, so we use the dynamic ones instead
  # 23H2 builds not ready yet, for test only by using "vhd_server_serverdatacentercore_en-us_vl", will need to update after builds is available
  "WS23H2" = @("vhd_server_serverdatacentercore_en-us_vl_30gb_azuregallery", "vhd_server_serverdatacentercore_en-us_vl_30gb_gen2_azuregallery")
  "WS2022" = @("vhd_server_serverdatacentercore_en-us_vl_30gb_azuregallery", "vhd_server_serverdatacentercore_en-us_vl_30gb_gen2_azuregallery")
  "RS5" = @("vhd_server_serverdatacentercore_en-us_vl_30gb_azuregallery")
}

# function Get-Token {
#   param
#   (
#       $client_id,
#       $client_secret
#   )
#   # Authentication
#   $AuthNBody = @{
#                   grant_type = 'client_credentials';
#                   client_id = $client_id;
#                   resource = $MEDIA_SEEKER_RESOURCE;
#                   client_secret = $client_secret
#   }

#   $authN = Invoke-RestMethod -Uri $MEDIA_SEEKER_OATH2_TOKEN_ENDPOINT -Method Post -ContentType 'application/x-www-form-urlencoded' -Body $AuthNBody

#   $authZHeader = @{"Authorization" = "Bearer " + $authN.access_token}

#   return $authZHeader
# }

function Get-Date-From-Build-Name {
  param
  (
      $buildName
  )

  $unformattedDate = $buildName.Split(".") | Select-Object -Last 1
  $date = [datetime]::parseexact($unformattedDate, 'yyMMdd-HHmm', $null)

  return $date
}

function Get-LatestBuilds {
  param
(
  $product,
  $authZHeader,
  [bool]$includeNonSignedoff = $false
)

  [System.Uri]$mediaSeekerBaseURL = New-Object -TypeName System.Uri -ArgumentList $MEDIA_SEEKER_BASE_URL

  if ($includeNonSignedoff) {
    # For our test scenerio, the problem of non-live build is difficult to find the corresponding openssh server.
    #$subURL = "/api/builds/search?product={0}&includeNonLive=true&includeNonSignedoffIntent=true" -f $product
    $subURL = "/api/builds/search?product={0}&includeNonLive=true&includeNonSignedoffIntent=true" -f $product
  }
  else {
    $subURL = "/api/builds/search?product={0}&includeNonLive=true" -f $product
  }


  [System.Uri]$fullURL = New-Object -TypeName System.Uri -ArgumentList @($mediaSeekerBaseURL, $subURL)
  $allBuilds = Invoke-RestMethod -Uri $fullURL -Method Get -ContentType 'application/json' -Headers $authZHeader

  Write-Host "fullURL", $fullURL
    
  $allBuilds = $allBuilds | Where-Object {$null -ne $_.release}

  $last7daysBuilds = @()
  $todaysDate = Get-Date
  foreach ($build in $allBuilds) {
    $buildDate = Get-Date-From-Build-Name $build.buildName
    # Extend from 7 days to 14 days for test
    #write-host "compare date", $buildDate.Date, $todaysDate.Date.AddDays(-14)
    if ($buildDate.Date -ge ($todaysDate.Date.AddDays(-14))) {
        write-host "buildDate", $buildDate, "buildName", $build.buildName
        $build | Add-Member -MemberType NoteProperty -Name buildDate -Value $buildDate
        $build | Add-Member -MemberType NoteProperty -Name dataVersion -Value $DATA_VERSION
        $last7daysBuilds += $build
    }
  }

  #Write-Host $last7daysBuilds

  # Looks that the response has already been sorted, sort it again just in case
  $last7daysBuilds = $last7daysBuilds | Sort-Object -Property buildDate -Descending

  #Write-Host "after sort", $last7daysBuilds
  $latestBuilds = @()
  foreach ($build in $last7daysBuilds) {
    $dropName = $drops[$product][0]
    $gen1URL = "/api/media/search?buildGuid={0}&flavor=amd64fre&dropName=$dropName" -f $build.buildGuid
    [System.Uri]$fullURL = New-Object -TypeName System.Uri -ArgumentList @($mediaSeekerBaseURL, $gen1URL)
    $gen1Media = Invoke-RestMethod -Uri $fullURL -Method Get -ContentType 'application/json' -Headers $authZHeader

    if ($gen1Media.Length -eq 0) {
        Write-Host "No media found for $fullURL"
        continue
    }

    Write-Host "Found media, full gen1 URL", $fullURL

    $mediasWithoutPath = @($gen1Media)

    #if (("WS2022") -contains $product) {
    if (("WS23H2", "WS2022") -contains $product) {
      $dropName = $drops[$product][1]
      if ($product -eq "WS23H2") {
        $gen2URL = "/api/media/search?buildGuid={0}&flavor=amd64fre&dropName=$dropName" -f $build.buildGuid
      }
      elseif ($product -eq "WS2022") {
        $gen2URL = "/api/media/search?buildGuid={0}&flavor=amd64fre&dropName=$dropName" -f $build.buildGuid
      }
      [System.Uri]$fullURL = New-Object -TypeName System.Uri -ArgumentList @($mediaSeekerBaseURL, $gen2URL)
      $gen2Media = Invoke-RestMethod -Uri $fullURL -Method Get -ContentType 'application/json' -Headers $authZHeader

      if ($gen2Media.Length -eq 0) {
        continue
      }

      Write-Host "Found media, full gen2 URL", $fullURL

      $mediasWithoutPath += $gen2Media
    }

    Write-Host "mediaWithoutPath", $mediasWithoutPath

    $mediasWithPath = @()
    foreach($mediaWithoutPath in $mediasWithoutPath) {
        $mediaWithPath = @()
        foreach ($media in $mediaWithoutPath) {
            $subURL = "/api/media/{0}" -f $media.dropId
            [System.Uri]$fullURL = New-Object -TypeName System.Uri -ArgumentList @($mediaSeekerBaseURL, $subURL)
            
            Write-Host "fullURL", $fullURL

            $mediaWithPath += Invoke-RestMethod -Uri $fullURL -Method Get -ContentType 'application/json' -Headers $authZHeader | Select-Object -Property dropName, dropId, sharePath, cloudDropName
        }

        $mediasWithPath += $mediaWithPath
    }

    $build | Add-Member -MemberType NoteProperty -Name mediaInfo -Value $mediasWithPath
    $latestBuilds += $build

    #For now, we care only about the latest build
    break
  }

  return $latestBuilds
}

#
# MAIN
#

# $audience = "api://AzureADTokenExchange"
# $federatedToken=az account get-access-token --resource=$audience --query accessToken --output tsv

# az login --service-principal -u $aadAppId --federated-token $federatedToken --tenant $tenantId

$token=az account get-access-token --resource=$MEDIA_SEEKER_RESOURCE --query accessToken --output tsv

#$token = az account get-access-token --resource=$MEDIA_SEEKER_RESOURCE --query accessToken --output tsv
$authZHeader = @{"Authorization" = "Bearer " + $token}

# Connect-AzAccount  -TenantId $tenantId -Subscription $subscriptionId -Identity -AccountId $managedIdentityClientId

# $federatedToken = Get-AzAccessToken -ResourceUrl $audience

# Connect-AzAccount -TenantId $tenantId -Subscription $subscriptionId -ApplicationId $aadAppId -FederatedToken $federatedToken.Token

# $token = Get-AzAccessToken -ResourceUrl $MEDIA_SEEKER_RESOURCE

# $authZHeader = @{"Authorization" = "Bearer " + $token.Token}

#Write-Host $authZHeader 

$todaysBuilds = @()
foreach ($product in $products) {
  Write-Host ("QUERYING PRODUCT - {0}" -f $product)
  # get current day latest serviced media info filtered by product
  $productBuilds = Get-LatestBuilds $product $authZHeader
  if ($productBuilds.Count -eq 0)
  {
    #Write-Host "No builds found for $product, continue with non-sighed off builds"
    Write-Host "No builds found for $product, continue with non-live builds"
    $productBuilds = Get-LatestBuilds $product $authZHeader $true
  }

  $todaysBuilds += $productBuilds
}

$buildsJson = ($todaysBuilds | ConvertTo-Json -Depth 4 -Compress)

if (Test-Path $outputDirectory) {
    Remove-Item $outputDirectory -Recurse -Force
} 

New-Item -ItemType Directory -Path $outputDirectory | Out-Null
Write-Host "created directory, $outputDirectory"

Set-Content -Path "${outputDirectory}\builds.json" -Value $buildsJson