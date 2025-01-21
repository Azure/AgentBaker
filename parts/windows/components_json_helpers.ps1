function GetComponentsFromComponentsJson2
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
            $url = $containerImage.downloadUrl.replace("*", $windowsVersion.latestVersion)
            $output += $url

            if ( -not [string]::IsNullOrEmpty($windowsVersion.previousLatestVersion) )
            {
                $url = $containerImage.downloadUrl.replace("*", $windowsVersion.previousLatestVersion)
                $output += $url
            }
        }
    }

    return $output
}
