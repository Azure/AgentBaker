function GetComponentsFromComponentsJson
{
    Param(
        [Parameter(Mandatory = $true)][Object]
        $componentsJsonContent
    )

    $output = New-Object System.Collections.ArrayList

    foreach ($containerImage in $componentsJsonContent.ContainerImages)
    {
        $versions = $containerImage.windowsVersions
        if ($versions -eq $null) {
            $versions = $containerImage.multiArchVersionsV2
        }

        $downloadUrl = $containerImage.windowsDownloadUrl
        if ($downloadUrl -eq $null) {
            $downloadUrl = $containerImage.downloadUrl
        }

        foreach ($windowsVersion in $versions)
        {
            $skuMatch = $windowsVersion.windowsSkuMatch
            if ($skuMatch -eq $null -or $windowsSku -eq $null -or $windowsSku -Like $skuMatch)
            {
                $url = $downloadUrl.replace("*", $windowsVersion.latestVersion)
                $output += $url

                if (-not [string]::IsNullOrEmpty($windowsVersion.previousLatestVersion))
                {
                    $url = $downloadUrl.replace("*", $windowsVersion.previousLatestVersion)
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
    $output = @{ }

    foreach ($package in $componentsJsonContent.Packages)
    {
        $downloadLocation = $package.windowsDownloadLocation
        if ($downloadLocation -eq $null -or $downloadLocation -eq "")
        {
            continue
        }

        $thisList = $output[$downloadLocation]
        if ($thisList -eq $null)
        {
            $thisList = New-Object System.Collections.ArrayList
        }

        $downloadUrls = $package.downloadURIs.windows
        if ($downloadUrls -eq $null)
        {
            $part = $package.downloadURIs.default.current
        }
        else
        {
            $part = $downloadUrls.default
            switch -Regex ($windowsSku)
            {
                "2019-containerd" {
                    $part = $downloadUrls.ws2019
                }
                "2022-containerd*" {
                    $part = $downloadUrls.ws2022
                }
                "23H2*" {
                    $part = $downloadUrls.ws32h2
                }
            }

            if ($part -eq $null)
            {
                $part = $downloadUrls.default
            }
        }

        $downloadUrl = $part.windowsDownloadUrl
        $items = $part.versionsV2

        # no specific windows download url means fall back to regular windows spots.
        if ($downloadUrl -eq $null -or $downloadUrl -eq "")
        {
            $downloadUrl = $part.downloadUrl
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

function GetDefaultContainerDFromComponentsJson
{
    Param(
        [Parameter(Mandatory = $true)][Object]
        $componentsJsonContent
    )

    $packages = GetPackagesFromComponentsJson($componentsJsonContent)
    $containerDPackages = $packages["c:\akse-cache\containerd\"]
    return $containerDPackages[0]
}