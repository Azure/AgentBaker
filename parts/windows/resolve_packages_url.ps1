function ResolvePackagesSourceUrl {
  param (
    [string]$url
    [int]$maxAttempts = 5
  )

  $attempt = 0
  $PACKAGES_DOWNLOAD_BASE_URL=""

  while ($attempt -lt $maxAttempts) 
  {
    try 
    {
      $response = Invoke-WebRequest -Uri $url -Method Head -ErrorAction Stop

      if ($response.StatusCode -eq 200) 
      {
        $PACKAGES_DOWNLOAD_BASE_URL="packages.aks.azure.com"
        break
      }
    }
    catch 
    {
      $attempt++
      Start-Sleep -Seconds 1
    }
  }

  if (-not $PACKAGES_DOWNLOAD_BASE_URL) 
  {
    $PACKAGES_DOWNLOAD_BASE_URL="acs-mirror.azureedge.net"
  }

  Write-Host "Packages source URL is $PACKAGES_DOWNLOAD_BASE_URL"
}

function ReplaceBaseUrl {
  param (
    [string]$url
    [string]$packagesUrl
  )

  if ($url.Contains("acs-mirror.azureedge.net"))
  {
    $url = $url.Replace("acs-mirror.azureedge.net", $packagesUrl)
  }
}