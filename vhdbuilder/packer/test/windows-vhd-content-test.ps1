<#
    .SYNOPSIS
        verify the content of Windows image built
    .DESCRIPTION
        This script is used to verify the content of Windows image built
#>

param (
    $containerRuntime,
    $windowsSKU
)

# We use parameters for test script so we set environment variables before importing c:\windows-vhd-configuration.ps1 to reuse it
$env:ContainerRuntime=$containerRuntime
$env:WindowsSKU=$windowsSKU

. c:\windows-vhd-configuration.ps1

function Start-Job-To-Expected-State {
    [CmdletBinding()]
    Param(
        [Parameter(Position=0, Mandatory=$true)]
        [string]$JobName,

        [Parameter(Position=1, Mandatory=$true)]
        [scriptblock]$ScriptBlock,

        [Parameter(Position=2, Mandatory=$false)]
        [string]$ExpectedState = 'Running',

        [Parameter(Position=3, Mandatory=$false)]
        [int]$MaxRetryCount = 10,

        [Parameter(Position=4, Mandatory=$false)]
        [int]$DelaySecond = 10
    )

    Begin {
        $cnt = 0
    }

    Process {
        Start-Job -Name $JobName -ScriptBlock $ScriptBlock

        do {
            Start-Sleep $DelaySecond
            $job = (Get-Job -Name $JobName)
            if ($job -and ($job.State -Match $ExpectedState)) { return }
            $cnt++
        } while ($cnt -lt $MaxRetryCount)

        Write-Error "Cannot start $JobName"
        exit 1
    }
}

function DownloadFileWithRetry {
    param (
        $URL,
        $Dest,
        $retryCount = 5,
        $retryDelay = 0,
        [Switch]$redactUrl = $false
    )
    curl.exe -s -f --retry $retryCount --retry-delay $retryDelay -L $URL -o $Dest
    if ($LASTEXITCODE) {
        $logURL = $URL
        if ($redactUrl) {
            $logURL = $logURL.Split("?")[0]
        }
        throw "Curl exited with '$LASTEXITCODE' while attemping to download '$logURL'"
    }
}

function Test-FilesToCacheOnVHD
{
    $invalidFiles = @()
    $missingPaths = @()
    foreach ($dir in $map.Keys) {
        $fakeDir = $dir
        if ($dir.StartsWith("c:\akse-cache\win-k8s")) {
            $dir = "c:\akse-cache\win-k8s\"
        }
        if(!(Test-Path $dir)) {
            Write-Error "Directory $dir does not exit"
            $missingPaths = $missingPaths + $dir
            continue
        }

        foreach ($URL in $map[$fakeDir]) {
            $fileName = [IO.Path]::GetFileName($URL)
            $dest = [IO.Path]::Combine($dir, $fileName)

            # Do not validate containerd package on docker VHD
            if ($containerRuntime -ne 'containerd' -And $dir -eq "c:\akse-cache\containerd\") {
                continue
            }

            # Windows containerD supports Windows containerD, starting from Kubernetes 1.20
            if ($containerRuntime -eq "containerd" -And $fakeDir -eq "c:\akse-cache\win-k8s\") {
                $k8sMajorVersion = $fileName.split(".",3)[0]
                $k8sMinorVersion = $fileName.split(".",3)[1]
                # Skip to validate $URL for containerD is supported from Kubernets 1.20
                if ($k8sMinorVersion -lt "20" -And $k8sMajorVersion -eq "v1") {
                    continue
                }
            }

            if(![System.IO.File]::Exists($dest)) {
                Write-Error "File $dest does not exist"
                $invalidFiles = $invalidFiles + $dest
                continue
            }

            $fileName = [IO.Path]::GetFileName($URL.Split("?")[0])
            $tmpDest = [IO.Path]::Combine([System.IO.Path]::GetTempPath(), $fileName)
            DownloadFileWithRetry -URL $URL -Dest $tmpDest -redactUrl
            $remoteFileHash = (Get-FileHash  -Algorithm SHA256 -Path $tmpDest).Hash
            $localFileHash = (Get-FileHash  -Algorithm SHA256 -Path $dest).Hash
            Remove-Item -Path $tmpDest
            if ($localFileHash -ne $remoteFileHash) {
                Write-Error "$dest : Local file hash is $localFileHash but remote file hash in global is $remoteFileHash"
                $invalidFiles = $invalidFiles + $dest
                continue
            }

            if ($URL.StartsWith("https://acs-mirror.azureedge.net/")) {
                $mcURL = $URL.replace("https://acs-mirror.azureedge.net/", "https://kubernetesartifacts.blob.core.chinacloudapi.cn/")
                try {
                    DownloadFileWithRetry -URL $mcURL -Dest $tmpDest -redactUrl
                    $remoteFileHash = (Get-FileHash  -Algorithm SHA256 -Path $tmpDest).Hash
                    Remove-Item -Path $tmpDest
                    if ($localFileHash -ne $remoteFileHash) {
                        $excludeSizeComparisionList = @(
                            "calico-windows",
                            "azure-vnet-cni-singletenancy-windows-amd64",
                            "azure-vnet-cni-singletenancy-swift-windows-amd64",
                            "azure-vnet-cni-singletenancy-windows-amd64-v1.4.35.zip",
                            "azure-vnet-cni-singletenancy-overlay-windows-amd64-v1.4.35.zip"
                        )

                        $isIgnore=$False
                        foreach($excludePackage in $excludeSizeComparisionList) {
                            if ($mcURL.Contains($excludePackage)) {
                                $isIgnore=$true
                                break
                            }
                        }
                        if ($isIgnore) {
                            continue
                        }

                        Write-Error "$mcURL is valid but the file hash is different. Expect $localFileHash but remote file hash in AzureChinaCloud is $remoteFileHash"
                        $invalidFiles = $mcURL
                        continue
                    }
                } catch {
                    Write-Error "$mcURL is invalid"
                    $invalidFiles = $mcURL
                    continue
                }
            }
        }
    }
    if ($invalidFiles.count -gt 0 -Or $missingPaths.count -gt 0) {
        Write-Error "cache files base paths $missingPaths or(and) cached files $invalidFiles are invalid"
        exit 1
    }

}

function Test-PatchInstalled {
    $hotfix = Get-HotFix
    $currenHotfixes = @()
    foreach($hotfixID in $hotfix.HotFixID) {
        $currenHotfixes += $hotfixID
    }

    $lostPatched = @($patchIDs | Where-Object {$currenHotfixes -notcontains $_})
    if($lostPatched.count -ne 0) {
        Write-Error "$lostPatched is(are) not installed"
        exit 1
    }
}

function Test-ImagesPulled {
    if ($containerRuntime -eq 'containerd') {
        Start-Job-To-Expected-State -JobName containerd -ScriptBlock { containerd.exe }
        # NOTE:
        # 1. listing images with -q set is expected to return only image names/references, but in practise
        #    we got additional digest info. The following command works as a workaround to return only image names instad.
        #    https://github.com/containerd/containerd/blob/master/cmd/ctr/commands/images/images.go#L89
        # 2. As select-string with nomatch pattern returns additional line breaks, qurying MatchInfo's Line property keeps
        #    only image reference as a workaround
        $pulledImages = (ctr.exe -n k8s.io image ls -q | Select-String -notmatch "sha256:.*" | % { $_.Line } )
    }
    elseif ($containerRuntime -eq 'docker') {
        Start-Service docker
        $pulledImages = docker images --format "{{.Repository}}:{{.Tag}}"
    }
    else {
        Write-Error "unsupported container runtime $containerRuntime"
    }

    if(Compare-Object $imagesToPull $pulledImages) {
        Write-Error "images to pull do not equal images cached $imagesToPull != $pulledImages"
        exit 1
    }
}

function Test-RegistryAdded {
    if ($containerRuntime -eq 'containerd') {
        $result=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name EnableCompartmentNamespace)
        if ($result.EnableCompartmentNamespace -ne 1) {
            Write-Error "The registry for SMB Resolution Fix for containerD is not added"
            exit 1
        }
    }
}

function Test-DefenderSignature {
    $mpPreference = Get-MpPreference
    if (-not ($mpPreference -and ($mpPreference.SignatureFallbackOrder -eq "MicrosoftUpdateServer|MMPC") -and [string]::IsNullOrEmpty($mpPreference.SignatureDefinitionUpdateFileSharesSources))) {
        Write-Error "The Windows Defender has wrong Signature. SignatureFallbackOrder: $($mpPreference.SignatureFallbackOrder). SignatureDefinitionUpdateFileSharesSources: $($mpPreference.SignatureDefinitionUpdateFileSharesSources)"
        exit 1
    }
}

function Test-AzureExtensions {
    # Expect the Windows VHD without any other extensions unrelated to AKS.
    # This test is called by "az vm run-command" that installs "Microsoft.CPlat.Core.RunCommandWindows".
    # So the expected extensions list is below.
    $expectedExtensions = @(
        "Microsoft.CPlat.Core.RunCommandWindows"
    )
    $actualExtensions = (Get-ChildItem "C:\Packages\Plugins").Name
    $compareResult = (Compare-Object $expectedExtensions $actualExtensions)
    if ($compareResult) {
        Write-Error "Azure extensions are not expected. Details: $($compareResult | Out-String)"
        exit 1
    }
}

function Test-DockerCat {
    if ($containerRuntime -eq 'docker') {
        $dockerVersion = (docker version --format '{{.Server.Version}}')
        if ($dockerVersion -eq "20.10.9") {
            $catFilePath = "C:\Windows\System32\CatRoot\{F750E6C3-38EE-11D1-85E5-00C04FC295EE}\docker-20-10-9.cat"
            if (!(Test-Path $catFilePath)) {
                Write-Error "$catFilePath does not exist"
                exit 1
            }
        }
    }
}

function Test-ExcludeUDPSourcePort {
    # Checking whether the UDP source port 65330 is excluded
    $result = $(netsh int ipv4 show excludedportrange udp | findstr.exe 65330)
    if (-not $result) {
        Write-Error "The UDP source port 65330 is not excluded."
        exit 1
    }
}

Test-FilesToCacheOnVHD
Test-PatchInstalled
Test-ImagesPulled
Test-RegistryAdded
Test-DefenderSignature
Test-AzureExtensions
Test-DockerCat
Test-ExcludeUDPSourcePort
Remove-Item -Path c:\windows-vhd-configuration.ps1
