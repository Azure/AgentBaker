<#
    .SYNOPSIS
        Produces a JSON image BOM for a Windows VHD

    .DESCRIPTION
        Produces a JSON image BOM for a Windows VHD
#>
$windowsSKU = $env:WindowsSKU
$buildDate = $env:BuildDate

$ErrorActionPreference = "Stop"

$imageBomJsonFilePath = "c:\image-bom.json"

# Keyed by image id so repoTags/repoDigests lookups are O(1) instead of scanning an array;
# [ordered] preserves first-seen order to match the previous array-based output.
$bomMap = [ordered]@{}

. c:/k/containerd_helpers.ps1

$imageList = Invoke-WithContainerd -ScriptBlock {
    $images = Invoke-Ctr -Arguments @("-n", "k8s.io", "image", "ls") -FailureMessage "Failed to list containerd images."
    return $images | Select-Object -Skip 1
}
foreach($image in $imageList) {
    $splitResult=($image -split '\s+')
    # Each line of `ctr image ls` is: REF TYPE DIGEST SIZE UNIT PLATFORMS LABELS
    # Example (repoTags line):
    #   mcr.azure.cn/windows/servercore:ltsc2022  application/vnd.docker.distribution.manifest.list.v2+json  sha256:dfd3...  1.3  GiB  windows/amd64  io.cri-containerd.image=managed
    # Example (repoDigests line):
    #   sha256:1fb25eb...  application/vnd.docker.distribution.manifest.list.v2+json  sha256:dfd3...  1.3  GiB  windows/amd64  io.cri-containerd.image=managed
    $id=$splitResult[2]

    if ($splitResult[0].StartsWith("sha256:")) {
        # Get repoDigests from sha256
        $repoDigests=$splitResult[0]

        if (-not $bomMap.Contains($id)) {
            # This should never happen
            # We need to handle id and repoTags in the first loop and then handle repoDigests in the second loop if this occurs
            throw "Cannot find image id $id in bomList"
        }
        $bomMap[$id].repoDigests += $repoDigests
    } else {
        # Get id and repoTags
        $repoTags=$splitResult[0]

        # Ignore repoTags when it contains id
        if ($repoTags.Contains($id)) {
            continue
        }

        if ($bomMap.Contains($id)) {
            $bomMap[$id].repoTags += $repoTags
        } else {
            $bomMap[$id] = [pscustomobject]@{
                id=$id;
                repoTags=@($repoTags);
                repoDigests=@();
            }
        }
    }
}

$imageBom=$(echo @($bomMap.Values) | ConvertTo-Json)

$systemInfo = Get-ItemProperty -Path 'HKLM:SOFTWARE\Microsoft\Windows NT\CurrentVersion'
$aksWindowsImageVersion="$($systemInfo.CurrentBuildNumber).$($systemInfo.UBR).$buildDate"

$listResult = @"
{
        "sku": "windows-$windowsSKU",
        "imageVersion": "$aksWindowsImageVersion",
        "imageBom": $imageBom
}
"@

echo $listResult | ConvertFrom-Json | ConvertTo-Json -Depth 3 | set-content $imageBomJsonFilePath

# Ensure proper encoding is set for JSON image BOM
[IO.File]::ReadAllText($imageBomJsonFilePath) | Out-File -Encoding utf8 $imageBomJsonFilePath
