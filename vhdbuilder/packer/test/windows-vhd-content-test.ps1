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

function Compare-AllowedSecurityProtocols {
    $allowedProtocols = @()
    $insecureProtocols = @([System.Net.SecurityProtocolType]::SystemDefault, [System.Net.SecurityProtocolType]::Ssl3)

    foreach ($protocol in [System.Enum]::GetValues([System.Net.SecurityProtocolType])) {
        if ($insecureProtocols -notcontains $protocol) {
            $allowedProtocols += $protocol
        }
    }
    if([System.Net.ServicePointManager]::SecurityProtocol -ne $allowedProtocols) {
        Write-Error "allowedSecurityProtocols '$([System.Net.ServicePointManager]::SecurityProtocol)', expecting '$allowedProtocols'"
        exit 1
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
                Write-Log "Skip to validate $URL for docker VHD"
                continue
            }
            # Windows containerD supports Windows containerD, starting from Kubernetes 1.20
            elseif ($containerRuntime -eq "containerd" -And $fakeDir -eq "c:\akse-cache\win-k8s\") {
                $k8sMajorVersion = $fileName.split(".",3)[0]
                $k8sMinorVersion = $fileName.split(".",3)[1]
                if ($k8sMinorVersion -lt "20" -And $k8sMajorVersion -eq "v1") {
                    Write-Output "Skip to validate $URL for containerD is supported from Kubernets 1.20"
                    continue
                }
            }

            if(![System.IO.File]::Exists($dest)) {
                Write-Error "File $dest does not exist"
                $invalidFiles = $invalidFiles + $dest
                continue
            }
            $remoteFileSize = (Invoke-WebRequest $URL -UseBasicParsing -Method Head).Headers.'Content-Length'
            $localFileSize = (Get-Item $dest).length

            if ($localFileSize -ne $remoteFileSize) {
                Write-Error "$dest : Local file size is $localFileSize but remote file size is $remoteFileSize"
                $invalidFiles = $invalidFiles + $dest
                continue
            }

            Write-Output "$dest is cached as expected"
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
    Write-Output "All pathced $patchIDs are installed"
}

function Test-ImagesPulled {
    if ($containerRuntime -eq 'containerd') {
        Start-Job -Name containerd -ScriptBlock { containerd.exe }
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

    Write-Output "Container runtime: $containerRuntime"
    if(Compare-Object $imagesToPull $pulledImages) {
        Write-Error "images to pull do not equal images cached $imagesToPull != $pulledImages"
        exit 1
    }
    else {
        Write-Output "images are cached as expected"
    }
}

function Test-RegistryAdded {
    Write-Output "Get the registry for the HNS fix in 2021-2C"
    $result=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag)
    if ($result.HNSControlFlag -eq 3) {
        Write-Output "The registry for the HNS fix is added"
    } else {
        Write-Error "The registry for the HNS fix is not added"
        exit 1
    }
    if ($containerRuntime -eq 'containerd') {
        $result=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name EnableCompartmentNamespace)
        if ($result.EnableCompartmentNamespace -eq 1) {
            Write-Output "The registry for SMB Resolution Fix for containerD is added"
        } else {
            Write-Error "The registry for SMB Resolution Fix for containerD is not added"
            exit 1
        }
    }
}

Compare-AllowedSecurityProtocols
Test-FilesToCacheOnVHD
Test-PatchInstalled
Test-ImagesPulled
Test-RegistryAdded
Remove-Item -Path c:\windows-vhd-configuration.ps1
