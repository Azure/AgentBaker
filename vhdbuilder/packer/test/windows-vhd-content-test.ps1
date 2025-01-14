<#
    .SYNOPSIS
        verify the content of Windows image built
    .DESCRIPTION
        This script is used to verify the content of Windows image built
#>

param (
    $windowsSKU,
    $skipValidateReofferUpdate
)

Set-PSDebug -Trace 1

# We use parameters for test script so we set environment variables before importing c:\windows-vhd-configuration.ps1 to reuse it
$env:WindowsSKU=$windowsSKU

. c:\windows-vhd-configuration.ps1

filter Timestamp { "$(Get-Date -Format o): $_" }

function Write-ErrorWithTimestamp($Message) {
    $msg = $message | Timestamp
    Write-Error $msg
}

function Write-OutputWithTimestamp($Message) {
    $msg = $message | Timestamp
    Write-Output $msg
}

# We do not create static public IP for test VM but we need the public IP
# when we want to check some issues in infra. Let me use this solution to
# get it. We can create a static public IP when creating test VM if this
# does not work
$testVMPublicIPAddress=$(curl.exe -s -4 icanhazip.com)
Write-OutputWithTimestamp "Public IP address of the Test VM is $testVMPublicIPAddress"

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
        Write-OutputWithTimestamp "Starting Job $JobName"
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
    Write-OutputWithTimestamp "Downloading file $URL"
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
                    # It's too slow to download the file from the China Cloud. So we only compare the file size.
                    $localFileSize = (Get-Item $dest).length
                    $remoteFileSize = (Invoke-WebRequest $mcURL -UseBasicParsing -Method Head).Headers.'Content-Length'
                    if ($localFileSize -ne $remoteFileSize) {
                        $isIgnore=$False
                        foreach($excludePackage in $global:excludeHashComparisionListInAzureChinaCloud) {
                            if ($mcURL.Contains($excludePackage)) {
                                $isIgnore=$true
                                break
                            }
                        }
                        if ($isIgnore) {
                            continue
                        }

                        Write-ErrorWithTimestamp "$mcURL is valid but the file size is different. Expect $localFileSize but remote file size in AzureChinaCloud is $remoteFileSize"
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

    $dir = "c:\akse-cache\private-packages"
    if (Test-Path $dir) {
        $mappingFile = "c:\akse-cache\private-packages\mapping.json"
        if (Test-Path $mappingFile) {
            $urls = @{}
            (ConvertFrom-Json ((Get-Content $mappingFile -ErrorAction Stop) | Out-String)).psobject.properties | Foreach { $urls[$_.Value] = $False }
            $privatePackages = Get-ChildItem -Path $dir -File -Filter "*.zip"
            foreach($privatePackage in $privatePackages) {
                $isFound = $False
                foreach ($url in $urls.Keys) {
                    if ($url.Contains($privatePackage.Name)) {
                        $urls[$url] = $True
                        $isFound = $True
                        break
                    }
                }

                if (-not $isFound) {
                    Write-ErrorWithTimestamp "URL for $($privatePackage.Name) is not found in $mappingFile"
                    exit 1
                }
            }

            foreach ($url in $urls.Keys) {
                if (-not $urls[$url]) {
                    Write-ErrorWithTimestamp "URL for $url is not cached in $dir"
                    exit 1
                }
            }
        } else {
            Write-ErrorWithTimestamp "File $mappingFile does not exist but $dir exists"
            exit 1
        }
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
    } else {
        Write-OutputWithTimestamp "$lostPatched is(are) installed"
    }
}

function Test-ImagesPulled {
    Write-Output "Test-ImagesPulled."
    $targetImagesToPull = $imagesToPull

    Start-Job-To-Expected-State -JobName containerd -ScriptBlock { containerd.exe }
    # NOTE:
    # 1. listing images with -q set is expected to return only image names/references, but in practise
    #    we got additional digest info. The following command works as a workaround to return only image names instad.
    #    https://github.com/containerd/containerd/blob/master/cmd/ctr/commands/images/images.go#L89
    # 2. As select-string with nomatch pattern returns additional line breaks, qurying MatchInfo's Line property keeps
    #    only image reference as a workaround
    $pulledImages = (ctr.exe -n k8s.io image ls -q | Select-String -notmatch "sha256:.*" | % { $_.Line } )

    $result = (Compare-Object $targetImagesToPull $pulledImages)
    if($result) {
        Write-ErrorWithTimestamp "images to pull do not equal images cached $(($result).InputObject) ."
        exit 1
    } else {
        Write-OutputWithTimestamp "images to pull do equal images cached."
    }
}

function Validate-WindowsFixInFeatureManagement {
    Param(
      [Parameter(Mandatory = $true)][string]
      $Name,
      [Parameter(Mandatory = $false)][string]
      $Value = "1"
    )
    
    $result=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Policies\Microsoft\FeatureManagement\Overrides" -Name $Name)
    if ($result.$Name -ne $Value) {
        Write-ErrorWithTimestamp "The registry for $Name in FeatureManagement\Overrides is not added"
        exit 1
    } else {
        Write-OutputWithTimestamp "The registry for $Name in FeatureManagement\Overrides was added"
    }
}

function Validate-WindowsFixInHnsState {
    Param(
      [Parameter(Mandatory = $true)][string]
      $Name,
      [Parameter(Mandatory = $false)][string]
      $Value = "1"
    )
    
    $result=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name $Name)
    if ($result.$Name -ne $Value) {
        Write-ErrorWithTimestamp "The registry for $Name in hns\State is not added"
        exit 1
    } else {
        Write-OutputWithTimestamp "The registry for $Name in hns\State was added"
    }
}

function Validate-WindowsFixInVfpExtParameters {
    Param(
      [Parameter(Mandatory = $true)][string]
      $Name,
      [Parameter(Mandatory = $false)][string]
      $Value = "1"
    )
    
    $result=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\VfpExt\Parameters" -Name $Name)
    if ($result.$Name -ne $Value) {
        Write-ErrorWithTimestamp "The registry for $Name in VfpExt\Parameters is not added"
        exit 1
    } else {
        Write-OutputWithTimestamp "The registry for $Name in VfpExt\Parameters was added"
    }
}

function Validate-WindowsFixInPath {
    Param(
      [Parameter(Mandatory = $true)][string]
      $Path,
      [Parameter(Mandatory = $true)][string]
      $Name,
      [Parameter(Mandatory = $false)][string]
      $Value = "1"
    )
    
    $result=(Get-ItemProperty -Path $Path -Name $Name)
    if ($result.$Name -ne $Value) {
        Write-ErrorWithTimestamp "The registry for $Name in $Path is not added"
        exit 1
    } else {
        Write-OutputWithTimestamp "The registry for $Name in $Path was added"
    }
}

function Test-RegistryAdded {
    if ($skipValidateReofferUpdate -eq $true) {
        Write-OutputWithTimestamp "Skip validating ReofferUpdate"
    } else {
        # Check whether the registry ReofferUpdate is added. ReofferUpdate indicates that the OS is not updated to the latest version.
        $result=(Get-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Update\TargetingInfo\Installed\Server.OS.amd64" -Name ReofferUpdate -ErrorAction Ignore)
        if ($result -and $result.ReofferUpdate -eq 1) {
            Write-ErrorWithTimestamp "The registry ReofferUpdate is added. The value is 1."
            exit 1
        }
        Write-OutputWithTimestamp "The registry for ReofferUpdate is \"$result\" ."
    }

    Validate-WindowsFixInHnsState -Name EnableCompartmentNamespace

    if ($env:WindowsSKU -Like '2019*') {
        $result=(Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\hns\State" -Name HNSControlFlag)
        if (($result.HNSControlFlag -band 0x10) -ne 0x10) {
            Write-ErrorWithTimestamp "The registry for the two HNS fixes is not added"
            exit 1
        } else {
            Write-OutputWithTimestamp "The registry for the two HNS fixes was added"
        }

        Validate-WindowsFixInPath -Path "HKLM:\SYSTEM\CurrentControlSet\Services\wcifs" -Name WcifsSOPCountDisabled -Value 0
        Validate-WindowsFixInHnsState -Name HnsPolicyUpdateChange
        Validate-WindowsFixInHnsState -Name HnsNatAllowRuleUpdateChange
        Validate-WindowsFixInFeatureManagement -Name 3105872524
        Validate-WindowsFixInVfpExtParameters -Name VfpEvenPodDistributionIsEnabled
        Validate-WindowsFixInFeatureManagement -Name 3230913164
        Validate-WindowsFixInVfpExtParameters -Name VfpNotReuseTcpOneWayFlowIsEnabled
        Validate-WindowsFixInHnsState -Name CleanupReservedPorts
        Validate-WindowsFixInFeatureManagement -Name 652313229
        Validate-WindowsFixInFeatureManagement -Name 2059235981
        Validate-WindowsFixInFeatureManagement -Name 3767762061
        Validate-WindowsFixInFeatureManagement -Name 1102009996

        Validate-WindowsFixInFeatureManagement -Name 2290715789
        Validate-WindowsFixInFeatureManagement -Name 3152880268

        Validate-WindowsFixInFeatureManagement -Name 1605443213
    }

    if ($env:WindowsSKU -Like '2022*') {
        Validate-WindowsFixInFeatureManagement -Name 2629306509
        Validate-WindowsFixInHnsState -Name HnsPolicyUpdateChange
        Validate-WindowsFixInHnsState -Name HnsNatAllowRuleUpdateChange
        Validate-WindowsFixInFeatureManagement -Name 3508525708
        Validate-WindowsFixInHnsState -Name HnsAclUpdateChange
        Validate-WindowsFixInHnsState -Name HnsNpmRefresh
        Validate-WindowsFixInFeatureManagement -Name 1995963020
        Validate-WindowsFixInFeatureManagement -Name 189519500
        Validate-WindowsFixInVfpExtParameters -Name VfpEvenPodDistributionIsEnabled
        Validate-WindowsFixInFeatureManagement -Name 3398685324
        Validate-WindowsFixInHnsState -Name HnsNodeToClusterIpv6
        Validate-WindowsFixInHnsState -Name HNSNpmIpsetLimitChange
        Validate-WindowsFixInHnsState -Name HNSLbNatDupRuleChange
        Validate-WindowsFixInVfpExtParameters -Name VfpIpv6DipsPrintingIsEnabled
        Validate-WindowsFixInHnsState -Name HNSUpdatePolicyForEndpointChange
        Validate-WindowsFixInHnsState -Name HNSFixExtensionUponRehydration
        Validate-WindowsFixInFeatureManagement -Name 87798413
        Validate-WindowsFixInFeatureManagement -Name 4289201804
        Validate-WindowsFixInFeatureManagement -Name 1355135117
        Validate-WindowsFixInHnsState -Name RemoveSourcePortPreservationForRest
        Validate-WindowsFixInFeatureManagement -Name 2214038156
        Validate-WindowsFixInFeatureManagement -Name 1673770637
        Validate-WindowsFixInVfpExtParameters -Name VfpNotReuseTcpOneWayFlowIsEnabled
        Validate-WindowsFixInHnsState -Name FwPerfImprovementChange
        Validate-WindowsFixInHnsState -Name CleanupReservedPorts
        Validate-WindowsFixInFeatureManagement -Name 527922829
        Validate-WindowsFixInPath -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Windows Containers" -Name DeltaHivePolicy -Value 2
        Validate-WindowsFixInFeatureManagement -Name 2193453709
        Validate-WindowsFixInFeatureManagement -Name 3331554445
        Validate-WindowsFixInHnsState -Name OverrideReceiveRoutingForLocalAddressesIpv4
        Validate-WindowsFixInHnsState -Name OverrideReceiveRoutingForLocalAddressesIpv6
        Validate-WindowsFixInFeatureManagement -Name 1327590028
        Validate-WindowsFixInFeatureManagement -Name 1114842764
        Validate-WindowsFixInHnsState -Name HnsPreallocatePortRange
        Validate-WindowsFixInFeatureManagement -Name 4154935436
        Validate-WindowsFixInFeatureManagement -Name 124082829

        Validate-WindowsFixInFeatureManagement -Name 3744292492
        Validate-WindowsFixInFeatureManagement -Name 3838270605
        Validate-WindowsFixInFeatureManagement -Name 851795084
        Validate-WindowsFixInFeatureManagement -Name 26691724
        Validate-WindowsFixInFeatureManagement -Name 3834988172
        Validate-WindowsFixInFeatureManagement -Name 1535854221
        Validate-WindowsFixInFeatureManagement -Name 3632636556
        Validate-WindowsFixInFeatureManagement -Name 1552261773
        Validate-WindowsFixInFeatureManagement -Name 4186914956
        Validate-WindowsFixInFeatureManagement -Name 3173070476
        Validate-WindowsFixInFeatureManagement -Name 3958450316

        Validate-WindowsFixInFeatureManagement -Name 2540111500
        Validate-WindowsFixInFeatureManagement -Name 50261647
        Validate-WindowsFixInFeatureManagement -Name 1475968140

        Validate-WindowsFixInFeatureManagement -Name 747051149

        Validate-WindowsFixInFeatureManagement -Name 260097166

        Validate-WindowsFixInFeatureManagement -Name 4288867982

        # 2024-11B
        Validate-WindowsFixInFeatureManagement -Name 1825620622
        Validate-WindowsFixInFeatureManagement -Name 684111502
        Validate-WindowsFixInFeatureManagement -Name 1455863438
    }

    if ($env:WindowsSKU -Like '23H2*') {
        Validate-WindowsFixInHnsState -Name PortExclusionChange -Value 0

        Validate-WindowsFixInFeatureManagement -Name 1800977551

        # 2024-11B
        Validate-WindowsFixInFeatureManagement -Name 3197800078
        Validate-WindowsFixInFeatureManagement -Name 340036751
        Validate-WindowsFixInFeatureManagement -Name 2020509326
    }
}

function Test-DefenderSignature {
    $mpPreference = Get-MpPreference
    if (-not ($mpPreference -and ($mpPreference.SignatureFallbackOrder -eq "MicrosoftUpdateServer|MMPC") -and [string]::IsNullOrEmpty($mpPreference.SignatureDefinitionUpdateFileSharesSources))) {
        Write-ErrorWithTimestamp "The Windows Defender has wrong Signature. SignatureFallbackOrder: $($mpPreference.SignatureFallbackOrder). SignatureDefinitionUpdateFileSharesSources: $($mpPreference.SignatureDefinitionUpdateFileSharesSources)"
        exit 1
    } else {
        Write-OutputWithTimestamp "The Windows Defender has correct Signature"
    }
}

function Test-ExcludeUDPSourcePort {
    # Checking whether the UDP source port 65330 is excluded
    $result = $(netsh int ipv4 show excludedportrange udp | findstr.exe 65330)
    if (-not $result) {
        Write-ErrorWithTimestamp "The UDP source port 65330 is not excluded."
        exit 1
    } else {
        Write-OutputWithTimestamp "The UDP source port 65330 is excluded."
    }
}

function Test-WindowsDefenderPlatformUpdate {
    $currentDefenderProductVersion = (Get-MpComputerStatus).AMProductVersion
    $doc = New-Object xml
    $doc.Load("$global:defenderUpdateInfoUrl")
    $latestDefenderProductVersion = $doc.versions.platform
 
    if ($latestDefenderProductVersion -gt $currentDefenderProductVersion) {
        Write-ErrorWithTimestamp "Update failed. Current MPVersion: $currentDefenderProductVersion, Expected Version: $latestDefenderProductVersion"
        exit 1
    } else {
        Write-OutputWithTimestamp "Defender update succeeded."
    }
}

function Test-ToolsToCacheOnVHD {
    $toolsDir = "c:\aks-tools"
    $toolsList = @("DU\du.exe", "DU\du64.exe", "DU\du64a.exe")

    foreach ($tool in $toolsList) {
        $toolPath = Join-Path -Path $toolsDir -ChildPath $tool
        if (!(Test-Path -Path $toolPath)) {
            Write-ErrorWithTimestamp "Failed to get tool: $toolPath"
            exit 1
        } else {
            Write-OutputWithTimestamp "Got tool: $toolPath"
        }
    }
}

function Test-ExpandVolumeTask {
    $osDrive = ((Get-WmiObject Win32_OperatingSystem -ErrorAction Stop).SystemDrive).TrimEnd(":")
    $osDisk = Get-Partition -DriveLetter $osDrive | Get-Disk
    $osDiskSize = $osDisk.Size 
    $osDiskAllocatedSize = $osDisk.AllocatedSize
    if ($osDiskSize -ne $osDiskAllocatedSize) {
        Write-ErrorWithTimestamp "The OS disk size $osDiskSize is not equal to the allocated size $osDiskAllocatedSize"
        exit 1
    } else {
        Write-OutputWithTimestamp "The OS disk size $osDiskSize is equal to the allocated size"
    }
}

function Test-SSHDConfig {
    # user must be the name in `TEST_VM_ADMIN_USERNAME="azureuser"` in vhdbuilder/packer/test/run-test.sh
    $result=$(sshd -T -C user=azureuser)
    if ($result -Match 'chacha20-poly1305@openssh.com') {
        Write-ErrorWithTimestamp "C:\programdata\ssh\sshd_config is not updated for CVE-2023-48795"
        exit 1
    } else {
        Write-OutputWithTimestamp "C:\programdata\ssh\sshd_config is updated for CVE-2023-48795"
    }

    if ($result -Match '.*-etm@openssh.com') {
        Write-ErrorWithTimestamp "C:\programdata\ssh\sshd_config is not updated for CVE-2023-48795"
        exit 1
    } else {
        Write-OutputWithTimestamp "C:\programdata\ssh\sshd_config is updated for CVE-2023-48795"
    }

    $ConfigPath = "C:\programdata\ssh\sshd_config"
    $sshdConfig = Get-Content $ConfigPath
    if ($sshdConfig.Contains("#LoginGraceTime") -or (-not $sshdConfig.Contains("LoginGraceTime 0"))) {
        Write-ErrorWithTimestamp "C:\programdata\ssh\sshd_config is not updated for CVE-2006-5051"
        exit 1
    } else {
        Write-OutputWithTimestamp "C:\programdata\ssh\sshd_config is updated for CVE-2006-5051"
    }
}

Write-OutputWithTimestamp "Starting Tests"

Write-OutputWithTimestamp "Test: FilesToCacheOnVHD"
Test-FilesToCacheOnVHD

Write-OutputWithTimestamp "Test: PatchInstalled"
Test-PatchInstalled

Write-OutputWithTimestamp "Test: ImagesPulled"
Test-ImagesPulled

Write-OutputWithTimestamp "Test: RegistryAdded"
Test-RegistryAdded

Write-OutputWithTimestamp "Test: DefenderSignature"
Test-DefenderSignature

Write-OutputWithTimestamp "Test: ExcludeUDPSourcePort"
Test-ExcludeUDPSourcePort

Write-OutputWithTimestamp "Test: WindowsDefenderPlatformUpdate"
Test-WindowsDefenderPlatformUpdate

Write-OutputWithTimestamp "Test: ToolsToCacheOnVHD"
Test-ToolsToCacheOnVHD

Write-OutputWithTimestamp "Test: ExpandVolumeTask"
Test-ExpandVolumeTask

Remove-Item -Path c:\windows-vhd-configuration.ps1
