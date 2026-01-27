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
$bomList = @()

# starting containerd for printing containerD info, the same way as we pre-pull containerD images in configure-windows-vhd.ps1
Start-Job -Name containerd -ScriptBlock { containerd.exe }
$imageList=$(ctr.exe -n k8s.io image ls | select -Skip 1)
foreach($image in $imageList) {
    $splitResult=($image -split '\s+')
    if ($splitResult[0].StartsWith("sha256:")) {
        # Get repoDigests from sha256
        # Example:
        # sha256:1fb25eb608ab558cf66b7546fd598b2dc81b6678a380fcdb188da780e107f4ab
        # application/vnd.docker.distribution.manifest.list.v2+json
        # sha256:dfd3ce22e4b6e987ff2bfb3efe5e4912512fce35660be2ae5faa91e6f4da9748
        # 1.3
        # GiB
        # windows/amd64
        # io.cri-containerd.image=managed
        $repoDigests=$splitResult[0]
        $id=$splitResult[2]

        $isExist=$false
        foreach($bom in $bomList) {
            if ($bom.id -eq $id) {
                $bom.repoDigests += $repoDigests
                $isExist=$true
                break
            }
        }
        if (-not $isExist) {
            # This should never happen
            # We need to handle id and repoTags in the first loop and then handle repoDigests in the second loop if this occurs
            throw "Cannot find image id $id in bomList"
        }
    } else {
        # Get id and repoTags
        # Example:
        # mcr.azure.cn/windows/servercore:ltsc2022
        # application/vnd.docker.distribution.manifest.list.v2+json
        # sha256:dfd3ce22e4b6e987ff2bfb3efe5e4912512fce35660be2ae5faa91e6f4da9748
        # 1.3
        # GiB
        # windows/amd64
        # io.cri-containerd.image=managed
        #
        # mcr.microsoft.com/windows/servercore@sha256:dfd3ce22e4b6e987ff2bfb3efe5e4912512fce35660be2ae5faa91e6f4da9748
        # application/vnd.docker.distribution.manifest.list.v2+json
        # sha256:dfd3ce22e4b6e987ff2bfb3efe5e4912512fce35660be2ae5faa91e6f4da9748
        # 1.3
        # GiB
        # windows/amd64
        # io.cri-containerd.image=managed
        #
        # mcr.microsoft.com/windows/servercore@sha256:dfd3ce22e4b6e987ff2bfb3efe5e4912512fce35660be2ae5faa91e6f4da9748
        # application/vnd.docker.distribution.manifest.list.v2+json
        # sha256:dfd3ce22e4b6e987ff2bfb3efe5e4912512fce35660be2ae5faa91e6f4da9748
        # 1.3
        # GiB
        # windows/amd64
        # io.cri-containerd.image=managed
        $repoTags=$splitResult[0]
        $id=$splitResult[2]

        # Ignore repoTags when it contains id
        if (-not $repoTags.Contains($id)) {
            $isExist=$false
            foreach($bom in $bomList) {
                if ($bom.id -eq $id) {
                    $bom.repoTags += $repoTags
                    $isExist=$true
                    break
                }
            }

            if (-not $isExist) {
                $bom=[pscustomobject]@{
                    id=$id;
                    repoTags=@($repoTags);
                    repoDigests=@();
                }
                $bomList+=$bom
            }
        }
    }
}

Stop-Job  -Name containerd
Remove-Job -Name containerd

$imageBom=$(echo $bomList | ConvertTo-Json)

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
