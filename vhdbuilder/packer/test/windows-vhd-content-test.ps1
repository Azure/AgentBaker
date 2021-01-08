<#
    .SYNOPSIS
        verify the content of Windows image built
    .DESCRIPTION
        This script is used to verify the content of Windows image built
#>

param (
    $containerRuntime,
    $WindowsSKU
)

# TODO(qinhao): we can share the variables from configure-windows-vhd.ps1
$global:containerdPackageUrl = "https://acs-mirror.azureedge.net/containerd/ms/0.0.11-1/binaries/containerd-windows-0.0.11-1.zip"

function Compare-AllowedSecurityProtocols
{

    $allowedProtocols = @()
    $insecureProtocols = @([System.Net.SecurityProtocolType]::SystemDefault, [System.Net.SecurityProtocolType]::Ssl3)

    foreach ($protocol in [System.Enum]::GetValues([System.Net.SecurityProtocolType]))
    {
        if ($insecureProtocols -notcontains $protocol)
        {
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
    # TODO(qinhao): share this map variable with `configure-windows-vhd.ps1`
    $map = @{
        "c:\akse-cache\" = @(
            "https://github.com/Azure/AgentBaker/raw/master/vhdbuilder/scripts/windows/collect-windows-logs.ps1",
            "https://github.com/Microsoft/SDN/raw/master/Kubernetes/flannel/l2bridge/cni/win-bridge.exe",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/collectlogs.ps1",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/dumpVfpPolicies.ps1",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/portReservationTest.ps1",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/starthnstrace.cmd",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/startpacketcapture.cmd",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/debug/stoppacketcapture.cmd",
            "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/debug/VFP.psm1",
            "https://github.com/microsoft/SDN/raw/master/Kubernetes/windows/helper.psm1",
            "https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/hns.psm1",
            "https://globalcdn.nuget.org/packages/microsoft.applicationinsights.2.11.0.nupkg",
            "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.2.zip",
            "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.3.zip",
            "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.4.zip"
        );
        "c:\akse-cache\containerd\" = @(
            $global:containerdPackageUrl
        );
        "c:\akse-cache\csi-proxy\"    = @(
            "https://acs-mirror.azureedge.net/csi-proxy/v0.2.2/binaries/csi-proxy-v0.2.2.tar.gz"
        );
        "c:\akse-cache\win-k8s\" = @(
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.13-azs/windowszip/v1.16.13-azs-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.14-azs/windowszip/v1.16.14-azs-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.9-azs/windowszip/v1.17.9-azs-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.11-azs/windowszip/v1.17.11-azs-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.10-hotfix.20200817/windowszip/v1.16.10-hotfix.20200817-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.12/windowszip/v1.16.12-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.13/windowszip/v1.16.13-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.13-hotfix.20200714/windowszip/v1.16.13-hotfix.20200714-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.13-hotfix.20200817/windowszip/v1.16.13-hotfix.20200817-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.13-hotfix.20200824/windowszip/v1.16.13-hotfix.20200824-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.13-hotfix.20200917/windowszip/v1.16.13-hotfix.20200917-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.14/windowszip/v1.16.14-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.16.15-hotfix.20200903/windowszip/v1.16.15-hotfix.20200903-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.7-hotfix.20200817/windowszip/v1.17.7-hotfix.20200817-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.8/windowszip/v1.17.8-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.9/windowszip/v1.17.9-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.9-hotfix.20200714/windowszip/v1.17.9-hotfix.20200714-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.9-hotfix.20200817/windowszip/v1.17.9-hotfix.20200817-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.9-hotfix.20200824/windowszip/v1.17.9-hotfix.20200824-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.11/windowszip/v1.17.11-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.11-hotfix.20200901/windowszip/v1.17.11-hotfix.20200901-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.12/windowszip/v1.17.12-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.17.13/windowszip/v1.17.13-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.4-hotfix.20200626/windowszip/v1.18.4-hotfix.20200626-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.5/windowszip/v1.18.5-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.6/windowszip/v1.18.6-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.6-hotfix.20200723/windowszip/v1.18.6-hotfix.20200723-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.8/windowszip/v1.18.8-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.8-hotfix.20200924/windowszip/v1.18.8-hotfix.20200924-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.9/windowszip/v1.18.9-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.18.10/windowszip/v1.18.10-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.0/windowszip/v1.19.0-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.1/windowszip/v1.19.1-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.1-hotfix.20200923/windowszip/v1.19.1-hotfix.20200923-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.2/windowszip/v1.19.2-1int.zip",
            "https://acs-mirror.azureedge.net/kubernetes/v1.19.3/windowszip/v1.19.3-1int.zip"
        );
        "c:\akse-cache\win-vnet-cni\" = @(
            "https://acs-mirror.azureedge.net/azure-cni/v1.1.6/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.1.6.zip",
            "https://acs-mirror.azureedge.net/azure-cni/v1.1.8/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.1.8.zip",
            "https://acs-mirror.azureedge.net/azure-cni/v1.2.0/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.2.0.zip",
            "https://acs-mirror.azureedge.net/azure-cni/v1.2.0_hotfix/binaries/azure-vnet-cni-singletenancy-windows-amd64-v1.2.0_hotfix.zip"
        )
    }

    $emptyFiles = @()
    $missingPaths = @()
    foreach ($dir in $map.Keys)
    {
        if(!(Test-Path $dir))
        {
            Write-Error "Directory $dir does not exit"
            $missingPaths = $missingPaths + $dir
            continue
        }

        foreach ($URL in $map[$dir])
        {
            $fileName = [IO.Path]::GetFileName($URL)
            $dest = [IO.Path]::Combine($dir, $fileName)

            if(![System.IO.File]::Exists($dest))
            {
                Write-Error "File $dest does not exist"
                $emptyFiles = $emptyFiles + $dest
                continue
            }

            # NOTE(qinhao): tried to download all the files and compare file MD5 but as it takes
            #               too long(hours) for the whole process, so check the file size temporarily
            #               until we have a better way to validate these cached files
            if ((Get-Item $dest).length -eq 0kb) {
                Write-Error "File $dest is with size 0kb"
                $emptyFiles = $emptyFiles + $dest
            }

            Write-Output "$dest is cached as expected"
        }
    }
    if ($emptyFiles.count -gt 0 -Or $missingPaths.count -gt 0)
    {
        Write-Error "cache files base paths $missingPaths or(and) cached files $emptyFiles do not exist"
        exit 1
    }

}

function Test-PatchInstalled
{
    # patchIDs contains a list of hotfixes patched in "configure-windows-vhd.ps1", like "kb4558998"
    $patchIDs = @()
    $hotfix = Get-HotFix
    $currenHotfixes = @()
    foreach($hotfixID in $hotfix.HotFixID)
    {
        $currenHotfixes += $hotfixID
    }

    $lostPatched = @($patchIDs | Where-Object {$currenHotfixes -notcontains $_})
    if($lostPatched.count -ne 0)
    {
        Write-Error "$lostPatched is(are) not installed"
        exit 1
    }
    Write-Output "All pathced $patchIDs are installed"
}

function Test-ImagesPulled
{
    param (
        $containerRuntime,
        $WindowsSKU
    )
    $imagesToPull = @()
    switch ($WindowsSKU) {
        '2019' {
            $imagesToPull = @(
                "mcr.microsoft.com/windows/servercore:ltsc2019",
                "mcr.microsoft.com/windows/nanoserver:1809",
                "mcr.microsoft.com/oss/kubernetes/pause:1.4.0",
                "mcr.microsoft.com/oss/kubernetes-csi/livenessprobe:v2.0.1-alpha.1-windows-1809-amd64",
                "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v1.2.1-alpha.1-windows-1809-amd64",
                "mcr.microsoft.com/oss/kubernetes-csi/csi-node-driver-registrar:v2.0.1")
            Write-Output "Pulling images for windows server 2019"
        }
        '2004' {
            $imagesToPull = @(
                "mcr.microsoft.com/windows/servercore:2004",
                "mcr.microsoft.com/windows/nanoserver:2004",
                "mcr.microsoft.com/oss/kubernetes/pause:1.4.0-windows-2004-amd64")
            Write-Output "Pulling images for windows server core 2004"
        }
        default {
            Write-Output "No valid windows SKU is specified $WindowsSKU"
            exit 1
        }
    }
    if ($containerRuntime -eq 'containerd') {
        $pulledImages = ctr.exe -n k8s.io -q
    }
    elseif ($containerRuntime -eq 'docker') {
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

Compare-AllowedSecurityProtocols
Test-FilesToCacheOnVHD
Test-PatchInstalled
Test-ImagesPulled  -containerRuntime $containerRuntime -WindowsSKU $WindowsSKU