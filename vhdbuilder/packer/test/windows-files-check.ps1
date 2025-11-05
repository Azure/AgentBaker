<#
    .SYNOPSIS
        verify the signature of binaries in windows vhd cached packages
    .DESCRIPTION
        This script is used to verify the signature of binaries in windows vhd cached packages
#>

param (
    $windowsSKU
)

# We use parameters for test script so we set environment variables before importing c:\k\windows-vhd-configuration.ps1 to reuse it
$env:WindowsSKU = $windowsSKU

. vhdbuilder/packer/windows/windows-vhd-configuration.ps1

# We skip the signature validation of following scripts for known issues
# Some scripts in aks-windows-cse-scripts-v0.0.31.zip and aks-windows-cse-scripts-v0.0.32.zip are not signed, and this issue is fixed in aks-windows-cse-scripts-v0.0.33.zip
$SkipMapForSignature = @{
    "aks-windows-cse-scripts-v0.0.31.zip"      = @();
    "aks-windows-cse-scripts-v0.0.32.zip"      = @();
    "azure-vnet-cni-windows-amd64-v1.6.21.zip" = @();
    "azure-vnet-cni-windows-amd64-v1.5.38.zip" = @();
}

$SkipSignatureCheckForBinaries = @{
    # win-bridge.exe is not signed in these k8s packages, and it will be removed from k8s package in the future
    "win-bridge.exe"                      = $True;
    # aks-secure-tls-bootstrap-client.exe should be signed once it has been onboarded to Dalec and published via Upstream,
    # though for now we allow-list it as to not block secure TLS bootstrapping development
    # NOTE: this is okay since the binary is cleaned up during node provisioning when secure TLS bootstrapping is disabled (which is currently the default in production)
    # TODO(cameissner): remove this once the binary is properly signed
    "aks-secure-tls-bootstrap-client.exe" = $True;
}

# MisMatchFiles is used to record files whose file sizes are different on Global and MoonCake
$MisMatchFiles = @{}

# ProxyLocationNotFoundInMooncakeFiles is used to record files who proxy location is not correctly defined MoonCake.
# proxy location not found in Mooncake will lead to 404 error when downloading files from Mooncake.
$ProxyLocationNotFoundInMooncakeFiles = @{}

# NotSignedResult is used to record unsigned files that we think should be signed
$NotSignedResult = @{}

# AllNotSignedFiles is used to record all unsigned files in vhd cache and we exclude files in SkipMapForSignature
$AllNotSignedFiles = @{}

function Start-Job-To-Expected-State {
    [CmdletBinding()]
    Param(
        [Parameter(Position = 0, Mandatory = $true)]
        [string]$JobName,

        [Parameter(Position = 1, Mandatory = $true)]
        [scriptblock]$ScriptBlock,

        [Parameter(Position = 2, Mandatory = $false)]
        [string]$ExpectedState = 'Running',

        [Parameter(Position = 3, Mandatory = $false)]
        [int]$MaxRetryCount = 10,

        [Parameter(Position = 4, Mandatory = $false)]
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

function DownloadFile {
    param (
        $URL,
        $Dest,
        $retryCount = 5,
        $retryDelay = 0,
        [Switch]$redactUrl = $false
    )

    Write-Host "Downloading $URL"
    try {
        # We don't retry as that could hide flakey upstream servers that will cause issues in prod.
        Invoke-WebRequest -Uri $URL -OutFile $Dest -UseBasicParsing -ErrorAction Stop
    }
    catch {
        $logURL = $URL
        if ($redactUrl) {
            $logURL = $logURL.Split("?")[0]
        }
        throw "Curl exited with '$LASTEXITCODE' while attemping to download '$logURL' due to $($_.Exception.Message)"
    }
}

function Test-ValidateAllSignature {
    foreach ($dir in $map.Keys) {
        Test-ValidateSinglePackageSignature $dir
    }

    if ($AllNotSignedFiles.Count -ne 0) {
        $AllNotSignedFiles = (Write-Output $AllNotSignedFiles | ConvertTo-Json -Depth 9)
        Write-Output "All not signed file in cached packages are: $AllNotSignedFiles"
    }

    if ($NotSignedResult.Count -ne 0) {
        $NotSignedResult = (Write-Output $NotSignedResult | ConvertTo-Json -Depth 9)
        Write-Error "All not signed binaries are: $NotSignedResult"
        exit 1
    }
}

function Test-ValidateSinglePackageSignature {
    param (
        $dir
    )

    foreach ($URL in $map[$dir]) {
        $fileName = [IO.Path]::GetFileName($URL)
        $dest = [IO.Path]::Combine($dir, $fileName)

        $installDir = "c:\SignatureCheck"
        if (!(Test-Path $installDir)) {
            New-Item -ItemType Directory $installDir -Force | Out-Null
        }
        if ($fileName.endswith(".zip")) {
            try {
                Expand-Archive -path $dest -DestinationPath $installDir -Force -ErrorAction Stop
            }
            catch {
                Write-Error "Failed to expand archive ${dest} to ${installDir}: $($_.Exception.Message)"
                throw "Expand-Archive failed for ${dest}: $($_.Exception.Message)"
            }
        }
        elseif ($fileName.endswith(".tar.gz")) {
            tar -xzf $dest -C $installDir
            if ($LASTEXITCODE -ne 0) {
                throw "Failed to extract the '$dest' archive with tar. Exit code: $LASTEXITCODE"
            }
        }
        else {
            Write-Error "Unknown package suffix"
            exit 1
        }

        # Check signature for 4 types of files and record unsigned files
        $includeList = @("*.exe", "*.ps1", "*.psm1", "*.dll")
        $NotSignedList = (Get-ChildItem -Path $installDir -Recurse -File -Include $includeList | ForEach-object { Get-AuthenticodeSignature $_.FullName } | Where-Object { $_.status -ne "Valid" })
        if ($NotSignedList.Count -ne 0) {
            foreach ($NotSignedFile in $NotSignedList) {
                $NotSignedFileName = [IO.Path]::GetFileName($NotSignedFile.Path)

                if ($SkipSignatureCheckForBinaries.ContainsKey($NotSignedFileName)) {
                    Write-Output "$NotSignedFileName is in the ignore list. Ignoring signature validation failure"
                    continue
                }

                if (
                    (
                        $SkipMapForSignature.ContainsKey($fileName)
                    ) -and (
                        ( $SkipMapForSignature[$fileName].Length -eq 0 ) -or
                        ( $SkipMapForSignature[$fileName].Contains($NotSignedFileName) )
                    )
                ) {
                    Write-Output "$filename is in the ignore list. Ignoring signature validation failure on $NotSignedFileName"
                    continue
                }

                if (!$NotSignedResult.ContainsKey($dir)) {
                    $NotSignedResult[$dir] = @{}
                }
                if (!$NotSignedResult[$dir].ContainsKey($fileName)) {
                    $NotSignedResult[$dir][$fileName] = @()
                }
                $NotSignedResult[$dir][$fileName] += @($NotSignedFileName)

                Get-AuthenticodeSignature $NotSignedFile.Path | ConvertTo-Json -Depth 1 | Write-Host

                Write-Host "$filename in $dir from URL $URL has unsigned file $NotSignedFileName"
            }
        }

        # Check signature for all types of files except some known types and record unsigned files
        $excludeList = @("*.man", "*.reg", "*.md", "*.toml", "*.cmd", "*.template", "*.txt", "*.wprp", "*.yaml", "*.json", "NOTICE", "*.config", "*.conflist")
        $AllNotSignedList = (Get-ChildItem -Path $installDir -Recurse -File -Exclude $excludeList | ForEach-object { Get-AuthenticodeSignature $_.FullName } | Where-Object { $_.status -ne "Valid" })
        foreach ($NotSignedFile in $AllNotSignedList) {
            $NotSignedFileName = [IO.Path]::GetFileName($NotSignedFile.Path)
            if (($SkipMapForSignature.ContainsKey($fileName) -and ($SkipMapForSignature[$fileName].Length -ne 0) -and !$SkipMapForSignature[$fileName].Contains($NotSignedFileName)) -or !$SkipMapForSignature.ContainsKey($fileName)) {
                if (!$AllNotSignedFiles.ContainsKey($dir)) {
                    $AllNotSignedFiles[$dir] = @{}
                }
                if (!$AllNotSignedFiles[$dir].ContainsKey($fileName)) {
                    $AllNotSignedFiles[$dir][$fileName] = @()
                }
                $AllNotSignedFiles[$dir][$fileName] += @($NotSignedFileName)
            }
        }

        Remove-Item -Path $installDir -Force -Recurse
    }
}

function Test-CompareSingleDir {
    param (
        $dir
    )

    if (!(Test-Path $dir)) {
        New-Item -ItemType Directory $dir -Force | Out-Null
    }

    foreach ($URL in $map[$dir]) {

        # root paths like cri-tools can be ignored since they are only cached in VHD and won't be referenced in control plane.
        $rootPathExceptions = @("cri-tools")
        # When proxy location is not correctly defined in MoonCake, we will get 404 error when downloading files from MoonCake.
        # This valiation should including files in excludeHashComparisionListInAzureChinaCloud.
        if ($URL.StartsWith("https://acs-mirror.azureedge.net/") -and ($rootPathExceptions -notcontains $URL)) {
            $supportedProxyLocations = @(
                "aks",
                "kubernetes",
                "azure-cni",
                "cni-plugins",
                "csi-proxy",
                "aks-engine",
                "containerd",
                "calico-node",
                "ccgakvplugin",
                "cloud-provider-azure"
            )
            $proxyLocation = $URL.Split('/')[3]

            if ($supportedProxyLocations -notcontains $proxyLocation) {
                $ProxyLocationNotFoundInMooncakeFiles[$URL] = $URL
            }
        }

        $fileName = [IO.Path]::GetFileName($URL)
        $dest = [IO.Path]::Combine($dir, $fileName)

        DownloadFile -URL $URL -Dest $dest -redactUrl
        $globalFileSize = (Get-Item $dest).length

        $isIgnore = $False
        foreach ($excludePackage in $global:excludeHashComparisionListInAzureChinaCloud) {
            if ($URL.Contains($excludePackage)) {
                $isIgnore = $true
                break
            }
        }
        if ($isIgnore) {
            continue
        }

        if ($URL.StartsWith("https://acs-mirror.azureedge.net/")) {
            $mcURL = $URL.replace("https://acs-mirror.azureedge.net/", "https://kubernetesartifacts.blob.core.chinacloudapi.cn/")

            Write-Output "Downloading mooncake file: $mcURL"

            $ProgressPreference = 'SilentlyContinue'
            $mooncakeFileSize = (Invoke-WebRequest $mcURL -UseBasicParsing -Method Head -ErrorAction Stop).Headers.'Content-Length'
            $ProgressPreference = 'Continue'

            if ($globalFileSize -ne $mooncakeFileSize) {
                $MisMatchFiles[$URL] = $mcURL
            }
        }
    }
}

# Compare Files on Global and MoonCake
function Test-CompareFiles {
    foreach ($dir in $map.Keys) {
        Test-CompareSingleDir $dir
    }

    if ($ProxyLocationNotFoundInMooncakeFiles.Count -ne 0) {
        $ProxyLocationNotFoundInMooncakeFiles = (echo $ProxyLocationNotFoundInMooncakeFiles | ConvertTo-Json -Compress)
        Write-Error "The proxy location of the following files are not defined in mooncake, please use root path 'aks', or contact 'andyzhangx' for help: $ProxyLocationNotFoundInMooncakeFiles"
        exit 1
    }

    if ($MisMatchFiles.Count -ne 0) {
        $MisMatchFiles = (echo $MisMatchFiles | ConvertTo-Json -Compress)
        Write-Error "The following files have different sizes on global and mooncake: $MisMatchFiles"
        exit 1
    }
}

function Retry-Command {
    [CmdletBinding()]
    Param(
        [Parameter(Position = 0, Mandatory = $true)]
        [scriptblock]$ScriptBlock,

        [Parameter(Position = 1, Mandatory = $true)]
        [string]$ErrorMessage,

        [Parameter(Position = 2, Mandatory = $false)]
        [int]$Maximum = 5,

        [Parameter(Position = 3, Mandatory = $false)]
        [int]$Delay = 10
    )

    Begin {
        $cnt = 0
    }

    Process {
        do {
            $cnt++
            try {
                $ScriptBlock.Invoke()
                if ($LASTEXITCODE) {
                    throw "Retry $cnt : $ErrorMessage"
                }
                return
            }
            catch {
                Write-Error $_.Exception.InnerException.Message -ErrorAction Continue
                if ($_.Exception.InnerException.Message.Contains("There is not enough space on the disk. (0x70)")) {
                    throw "Exit retry since there is not enough space on the disk"
                }
                Start-Sleep $Delay
            }
        } while ($cnt -lt $Maximum)

        # Throw an error after $Maximum unsuccessful invocations. Doesn't need
        # a condition, since the function returns upon successful invocation.
        throw 'All retries failed. $ErrorMessage'
    }
}

function Install-Containerd {
    $containerdFileName = [IO.Path]::GetFileName($global:defaultContainerdPackageUrl)
    $dest = [IO.Path]::Combine("c:\akse-cache\containerd\", $containerdFileName)

    $installDir = "c:\program files\containerd"
    if (!(Test-Path $installDir)) {
        New-Item -ItemType Directory $installDir -Force | Out-Null
    }
    if ($containerdFilename.endswith(".zip")) {
        Expand-Archive -path $dest -DestinationPath $installDir -Force
    }
    else {
        tar -xzf $dest -C $installDir
        mv -Force $installDir\bin\* $installDir
        Remove-Item -Path $installDir\bin -Force -Recurse
    }

    $newPaths = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine) + ";$installDir"
    [Environment]::SetEnvironmentVariable("Path", $newPaths, [EnvironmentVariableTarget]::Machine)
    $env:Path += ";$installDir"

    $containerdConfigPath = [Io.Path]::Combine($installDir, "config.toml")
    # enabling discard_unpacked_layers allows GC to remove layers from the content store after
    # successfully unpacking these layers to the snapshotter to reduce the disk space caching Windows containerd images
    (containerd config default)  | % { $_ -replace "discard_unpacked_layers = false", "discard_unpacked_layers = true" }  | Out-File  -FilePath $containerdConfigPath -Encoding ascii

    Get-Content $containerdConfigPath

    Start-Job -Name containerd -ScriptBlock { containerd.exe }
}

Test-CompareFiles
Test-ValidateAllSignature
