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

function GetPackagesFromComponentsJson
{

    Param(
        [Parameter(Mandatory = $true)][Object]
        $componentsJsonContent
    )
    $output = @{}

    foreach ($package in $componentsJsonContent.Packages)
    {
        $downloadLocation = $package.windowsDownloadLocation
        if ($downloadLocation -eq $null -or $downloadLocation -eq "" ) {
            $downloadLocation = $package.downloadLocation
        }

        $thisList = $output[$downloadLocation]
        if ($thisList -eq $null) {
            $thisList = New-Object System.Collections.ArrayList
        }
        $downloadUrl = $package.downloadUris.default.current.windowsDownloadUrl
        $items = $package.downloadUris.default.current.versionsV2

        # no specific windows download url means fall back to regular windows spots.
        if ($downloadUrl -eq $null -or $downloadUrl -eq "" ) {
            $downloadUrl = $package.downloadUris.windows.current.downloadUrl
            $items = $package.downloadUris.windows.current.versionsV2
        }

        foreach ($windowsVersion in $items)
        {
            $url = $downloadUrl.replace("[version]", $windowsVersion.latestVersion)
            $thisList += $url

            if (-not [string]::IsNullOrEmpty($windowsVersion.previousLatestVersion))
            {
                $url = $downloadUrl.replace("[version]", $windowsVersion.previousLatestVersion)
                $thisList += $url
            }
        }

        if ($thisList.Length -gt 0)
        {
            $output[$downloadLocation] = $thisList
        }
    }

    return $output
}