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

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-ErrorWithTimestamp($Message) {
    $msg = $message | Timestamp
    Write-Error $msg
}
# We do not create static public IP for test VM but we need the public IP
# when we want to check some issues in infra. Let me use this solution to
# get it. We can create a static public IP when creating test VM if this
# does not work
$testVMPublicIPAddress=$(curl.exe -s -4 icanhazip.com)
Write-Output "Public IP address of the Test VM is $testVMPublicIPAddress"

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

        Write-ErrorWithTimestamp "Cannot start $JobName"
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
            Write-ErrorWithTimestamp "Directory $dir does not exit"
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
                Write-ErrorWithTimestamp "File $dest does not exist"
                $invalidFiles = $invalidFiles + $dest
                continue
            }

            $fileName = [IO.Path]::GetFileName($URL.Split("?")[0])
            $tmpDest = [IO.Path]::Combine([System.IO.Path]::GetTempPath(), $fileName)
            DownloadFileWithRetry -URL $URL -Dest $tmpDest -redactUrl
            $remoteFileHash = (Get-FileHash  -Algorithm SHA256 -Path $tmpDest).Hash
            $localFileHash = (Get-FileHash  -Algorithm SHA256 -Path $dest).Hash
            Remove-Item -Path $tmpDest

            # We have to ignore them since sizes on disk are same but the sizes are different. We are investigating this issue
            $excludeHashComparisionListInGlobal = @()
            if ($localFileHash -ne $remoteFileHash) {
                $isIgnore=$False
                foreach($excludePackage in $excludeHashComparisionListInGlobal) {
                    if ($URL.Contains($excludePackage)) {
                        $isIgnore=$true
                        break
                    }
                }
                if (-not $isIgnore) {
                    Write-ErrorWithTimestamp "$dest : Local file hash is $localFileHash but remote file hash in global is $remoteFileHash"
                    $invalidFiles = $invalidFiles + $dest
                    continue
                }
            }

            if ($URL.StartsWith("https://acs-mirror.azureedge.net/")) {
                $mcURL = $URL.replace("https://acs-mirror.azureedge.net/", "https://kubernetesartifacts.blob.core.chinacloudapi.cn/")
                try {
                    DownloadFileWithRetry -URL $mcURL -Dest $tmpDest -redactUrl
                    $remoteFileHash = (Get-FileHash  -Algorithm SHA256 -Path $tmpDest).Hash
                    Remove-Item -Path $tmpDest
                    if ($localFileHash -ne $remoteFileHash) {
                        $excludeHashComparisionListInAzureChinaCloud = @(
                            "calico-windows",
                            "azure-vnet-cni-singletenancy-windows-amd64",
                            "azure-vnet-cni-singletenancy-swift-windows-amd64",
                            "azure-vnet-cni-singletenancy-windows-amd64-v1.4.35.zip",
                            "azure-vnet-cni-singletenancy-overlay-windows-amd64-v1.4.35.zip"
                        )

                        $isIgnore=$False
                        foreach($excludePackage in $excludeHashComparisionListInAzureChinaCloud) {
                            if ($mcURL.Contains($excludePackage)) {
                                $isIgnore=$true
                                break
                            }
                        }
                        if ($isIgnore) {
                            continue
                        }

                        Write-ErrorWithTimestamp "$mcURL is valid but the file hash is different. Expect $localFileHash but remote file hash in AzureChinaCloud is $remoteFileHash"
                        $invalidFiles = $mcURL
                        continue
                    }
                } catch {
                    Write-ErrorWithTimestamp "$mcURL is invalid"
                    $invalidFiles = $mcURL
                    continue
                }
            }
        }
    }
    if ($invalidFiles.count -gt 0 -Or $missingPaths.count -gt 0) {
        Write-ErrorWithTimestamp "cache files base paths $missingPaths or(and) cached files $invalidFiles are invalid"
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
        Write-ErrorWithTimestamp "$lostPatched is(are) not installed"
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
        Write-ErrorWithTimestamp "unsupported container runtime $containerRuntime"
    }

    if(Compare-Object $imagesToPull $pulledImages) {
        Write-ErrorWithTimestamp "images to pull do not equal images cached $imagesToPull != $pulledImages"
        exit 1
    }
}

function Test-RegistryAdded {
    if ($containerRuntime -eq 'containerd') {
        $result=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name EnableCompartmentNamespace)
        if ($result.EnableCompartmentNamespace -ne 1) {
            Write-ErrorWithTimestamp "The registry for SMB Resolution Fix for containerD is not added"
            exit 1
        }
    }
    if ($env:WindowsSKU -Like '2019*') {
        $result=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag)
        if (($result.HNSControlFlag -band 0x50) -ne 0x50) {
            Write-ErrorWithTimestamp "The registry for the two HNS fixes is not added"
            exit 1
        }
        $result=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\wcifs" -Name WcifsSOPCountDisabled)
        if ($result.WcifsSOPCountDisabled -ne 0) {
            Write-ErrorWithTimestamp "The registry for the WCIFS fix in 2022-10B is not added"
            exit 1
        }
    }
    if ($env:WindowsSKU -Like '2022*') {
        $result=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name 2629306509)
        if ($result.2629306509 -ne 1) {
            Write-ErrorWithTimestamp "The registry for the WCIFS fix in 2022-10B is not added"
            exit 1
        }
    }
}

function Test-DefenderSignature {
    $mpPreference = Get-MpPreference
    if (-not ($mpPreference -and ($mpPreference.SignatureFallbackOrder -eq "MicrosoftUpdateServer|MMPC") -and [string]::IsNullOrEmpty($mpPreference.SignatureDefinitionUpdateFileSharesSources))) {
        Write-ErrorWithTimestamp "The Windows Defender has wrong Signature. SignatureFallbackOrder: $($mpPreference.SignatureFallbackOrder). SignatureDefinitionUpdateFileSharesSources: $($mpPreference.SignatureDefinitionUpdateFileSharesSources)"
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
        Write-ErrorWithTimestamp "Azure extensions are not expected. Details: $($compareResult | Out-String)"
        exit 1
    }
}

function Test-DockerCat {
    if ($containerRuntime -eq 'docker') {
        $dockerVersion = (docker version --format '{{.Server.Version}}')
        if ($dockerVersion -eq "20.10.9") {
            $catFilePath = "C:\Windows\System32\CatRoot\{F750E6C3-38EE-11D1-85E5-00C04FC295EE}\docker-20-10-9.cat"
            if (!(Test-Path $catFilePath)) {
                Write-ErrorWithTimestamp "$catFilePath does not exist"
                exit 1
            }
        }
    }
}

function Test-ExcludeUDPSourcePort {
    # Checking whether the UDP source port 65330 is excluded
    $result = $(netsh int ipv4 show excludedportrange udp | findstr.exe 65330)
    if (-not $result) {
        Write-ErrorWithTimestamp "The UDP source port 65330 is not excluded."
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
