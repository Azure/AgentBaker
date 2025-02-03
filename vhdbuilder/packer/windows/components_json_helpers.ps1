function GetComponentsFromComponentsJson
{
    Param(
        [Parameter(Mandatory = $true)][Object]
        $componentsJsonContent
    )

    $output = New-Object System.Collections.ArrayList

    foreach ($containerImage in $componentsJsonContent.ContainerImages)
    {
        foreach ($windowsVersion in $containerImage.windowsVersions)
        {
            $skuMatch = $windowsVersion.windowsSkuMatch
            if ($skuMatch -eq $null -or $windowsSku -eq $null -or $windowsSku -Like $skuMatch )
            {
                $url = $containerImage.downloadUrl.replace("*", $windowsVersion.latestVersion)
                $output += $url

                if (-not [string]::IsNullOrEmpty($windowsVersion.previousLatestVersion))
                {
                    $url = $containerImage.downloadUrl.replace("*", $windowsVersion.previousLatestVersion)
                    $output += $url
                }
            }
        }
    }

    return $output
}
