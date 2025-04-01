function ResolvePackagesSourceURL {
  param (
    [string]$url
    [int]maxAttempts = 5
  )

  $attempt = 0
  $PACKAGES_DOWNLOAD_BASE_URL=""

  while ($attempt -lt $maxAttempts) {
    try {
      $response = Invoke-WebRequest -Uri $url -Method Head -ErrorAction Stop

      if ($response = 200) {
        $PACKAGES_DOWNLOAD_BASE_URL="packages.aks.azure.com"
        break
      }
    }
    catch {
      $attempt++
      Start-Sleep -Seconds 1
    }
  }

}