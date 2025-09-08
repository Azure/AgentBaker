
function SafeReplaceString {
    Param(
        [Parameter(Mandatory = $true)][string]
        $stringToReplace
    )

    # this is a security guard - ensure that only allow-listed variables can be replaced by removing all others.
    # We run in a sub-shell so clearing all other variables doesn't impact the shell running this code.
    $stringToReplace = &{
        Clear-Variable -Name * -Exclude version,CPU_ARCH,stringToReplace -ErrorAction SilentlyContinue
        $executionContext.InvokeCommand.ExpandString($stringToReplace)
    }

    return $stringToReplace
}


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
        if ($versions -eq $null)
        {
            $versions = $containerImage.multiArchVersionsV2
        }

        $downloadUrl = $containerImage.windowsDownloadUrl
        if ($downloadUrl -eq $null)
        {
            $downloadUrl = $containerImage.downloadUrl
        }

        foreach ($windowsVersion in $versions)
        {
            $skuMatch = $windowsVersion.windowsSkuMatch
            if ($skuMatch -eq $null -or $windowsSku -eq $null -or $windowsSku -Like $skuMatch)
            {
                $version = $windowsVersion.latestVersion
                $url = SafeReplaceString($downloadUrl)
                $url = $url.replace("*", $windowsVersion.latestVersion)
                $output += $url

                if (-not [string]::IsNullOrEmpty($windowsVersion.previousLatestVersion))
                {
                    $version = $windowsVersion.previousLatestVersion
                    $url = SafeReplaceString($downloadUrl)
                    $url = $url.replace("*", $windowsVersion.previousLatestVersion)
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
                    break
                }
                "2022-containerd*" {
                    $part = $downloadUrls.ws2022
                    break
                }
                "23H2*" {
                    $part = $downloadUrls.ws32h2
                    break
                }
                "2025*" {
                    $part = $downloadUrls.ws2025
                    break
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
            $version = $windowsVersion.latestVersion
            $url = SafeReplaceString($downloadUrl)
            $thisList += $url

            if (-not [string]::IsNullOrEmpty($windowsVersion.previousLatestVersion))
            {
                $version = $windowsVersion.previousLatestVersion
                $url = SafeReplaceString($downloadUrl)
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

function GetOCIArtifactsFromComponentsJson
{

    Param(
        [Parameter(Mandatory = $true)][Object]
        $componentsJsonContent
    )
    $output = @{ }

    foreach ($ociArtifact in $componentsJsonContent.OCIArtifacts)
    {
        $registry = $ociArtifact.registry
        $downloadLocation = $ociArtifact.windowsDownloadLocation
        if ($downloadLocation -eq $null -or $downloadLocation -eq "")
        {
            continue
        }

        $thisList = $output[$downloadLocation]
        if ($thisList -eq $null)
        {
            $thisList = New-Object System.Collections.ArrayList
        }

        $versions = $ociArtifact.windowsVersions
        if ($versions -eq $null)
        {
            continue
        }

        foreach ($windowsVersion in $versions)
        {
            $skuMatch = $windowsVersion.windowsSkuMatch
            if ([string]::IsNullOrEmpty($skuMatch) -or $windowsSku -Like $skuMatch)
            {
                $version = $windowsVersion.latestVersion
                $artifactName = SafeReplaceString($registry)
                $artifactName = $artifactName.replace("*", $version)
                $thisList += $artifactName
            }

            if (-not [string]::IsNullOrEmpty($windowsVersion.previousLatestVersion))
            {
                $version = $windowsVersion.previousLatestVersion
                $artifactName = SafeReplaceString($registry)
                $artifactName = $artifactName.replace("*", $version)
                $thisList += $artifactName
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


function GetRegKeysToApply
{
    Param(
        [Parameter(Mandatory = $true)][Object]
        $windowsSettingsContent
    )
    $output = New-Object System.Collections.ArrayList

    foreach ($key in $windowsSettingsContent.WindowsRegistryKeys)
    {
        if ($windowsSku -Like $key.WindowsSkuMatch)
        {
            $output += $key
        }
    }

    return $output;
}

function GetKeyMapForReleaseNotes
{
    Param(
        [Parameter(Mandatory = $true)][Object]
        $windowsSettingsContent
    )

    $output = @{ }

    foreach ($key in $windowsSettingsContent.WindowsRegistryKeys)
    {
        if ($windowsSku -Like $key.WindowsSkuMatch)
        {
            $path = $key.Path
            $name = $key.Name
            $arr = $output[$path]
            if ($output[$path] -eq $null)
            {
                $output[$path] = New-Object System.Collections.ArrayList
            }
            $output[$path] += $name
        }
    }

    return $output;
}

function LogReleaseNotesForWindowsRegistryKeys
{
    Param(
        [Parameter(Mandatory = $true)][Object]
        $windowsSettingsContent
    )

    $logLines = New-Object System.Collections.ArrayList
    $releaseNotesToSet = GetKeyMapForReleaseNotes $windowsSettingsContent

    foreach ($key in $releaseNotesToSet.Keys)
    {
        $logLines += ("`t{0}" -f $key)
        $names = $releaseNotesToSet[$key]
        foreach ($name in $names)
        {
            $value = (Get-ItemProperty -Path $key -Name $name).$name
            $logLines += ("`t`t{0} : {1}" -f $name, $value)
        }
    }

    return $logLines
}

function GetPatchInfo
{
    Param(
        [Parameter(Mandatory = $true)][Object]
        $windowsSku,

        [Parameter(Mandatory = $true)][Object]
        $windowsSettingsContent
    )

    $output = New-Object System.Collections.ArrayList

    $baseVersionBlock = $windowsSettingsContent.WindowsBaseVersions."$windowsSku";

    if ($baseVersionBlock -eq $null) {
        return $output
    }

    $patchData = $baseVersionBlock.patches_to_apply

    # I'd much rather have two functions here - one to return the ids and one to return the urls. But annoyingly
    # powershell converts an array of strings of size 1 into a string. Which is super dumb. And means we can't trust
    # the return value of the function to be an array. It's OK for some of the functions above as they'll always be
    # returning lots of items. But there is usually only one patch to apply.
    return $patchData
}

function GetWindowsBaseVersion
{
    Param(
        [Parameter(Mandatory = $true)][String]
        $windowsSku,

        [Parameter(Mandatory = $true)][Object]
        $windowsSettingsContent
    )


    $baseVersionBlock = $windowsSettingsContent.WindowsBaseVersions."$windowsSku";

    if ($baseVersionBlock -eq $null) {
        return ""
    }

    return $baseVersionBlock.base_image_version
}


function GetWindowsBaseVersions {
    Param(
        [Parameter(Mandatory = $true)][Object]
        $windowsSettingsContent
    )

    return $windowsSettingsContent.WindowsBaseVersions.PSObject.Properties.Name
}

function GetDefenderUpdateUrl {
    Param(
        [Parameter(Mandatory = $true)][Object]
        $windowsSettingsContent
    )

    return $windowsSettingsContent.WindowsDefenderInfo.DefenderUpdateUrl
}


function GetDefenderUpdateInfoUrl {
    Param(
        [Parameter(Mandatory = $true)][Object]
        $windowsSettingsContent
    )

    return $windowsSettingsContent.WindowsDefenderInfo.DefenderUpdateInfoUrl
}

function GetAllCachedThings {
    Param(
        [Parameter(Mandatory = $true)][Object]
        $componentsJsonContent,
        [Parameter(Mandatory = $true)][Object]
        $windowsSettingsContent
    )

    $items = GetComponentsFromComponentsJson $componentsJsonContent
    $packages = GetPackagesFromComponentsJson $componentsJsonContent
    $ociArtifacts = GetOCIArtifactsFromComponentsJson $componentsJsonContent
    $regKeys = GetRegKeysToApply $windowsSettingsContent
    $baseVersion =  GetWindowsBaseVersion -windowsSku $windowsSku -windowsSettingsContent $windowsSettingsContent

    $items += "Windows ${windowsSku} base version: ${baseVersion}"

    foreach ($packageName in $packages.keys) {
        foreach ($package in $packages[$packageName]) {
            $items += $packageName + ": " + $package
        }
    }

    foreach ($ociArtifactName in $ociArtifacts.keys) {
        foreach ($ociArtifact in $ociArtifacts[$ociArtifactName]) {
            $items += $ociArtifactName + ": " + $ociArtifact
        }
    }

    foreach ($regKey in $regKeys) {
        $items += $regKey.Path + "\" + $regKey.Name + "=" + $regKey.Value
    }

    return ($items | Sort-Object -Unique )
}