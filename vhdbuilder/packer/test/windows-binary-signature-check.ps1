<#
    .SYNOPSIS
        verify the signature of binaries in windows vhd cached packages
    .DESCRIPTION
        This script is used to verify the signature of binaries in windows vhd cached packages
#>

param (
    $windowsSKU
)

# We use parameters for test script so we set environment variables before importing c:\windows-vhd-configuration.ps1 to reuse it
$env:WindowsSKU=$windowsSKU

. c:\windows-vhd-configuration.ps1

# We skip the signature validation of following scripts for known issues
# Some scripts in aks-windows-cse-scripts-v0.0.31.zip and aks-windows-cse-scripts-v0.0.32.zip are not signed, and this issue is fixed in aks-windows-cse-scripts-v0.0.33.zip
# win-bridge.exe is not signed in these k8s packages, and it will be removed from k8s package in the future
$SkipMapForSignature=@{
    "aks-windows-cse-scripts-v0.0.31.zip"=@();
    "aks-windows-cse-scripts-v0.0.32.zip"=@();
    "v1.24.9-hotfix.20230728-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.24.10-hotfix.20230728-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.24.15-hotfix.20230728-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.25.5-hotfix.20230728-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.25.6-hotfix.20230728-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.25.11-hotfix.20230728-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.25.15-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.26.0-hotfix.20230728-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.26.3-hotfix.20230728-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.26.6-hotfix.20230728-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.26.10-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.27.1-hotfix.20230728-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.27.3-hotfix.20230728-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.27.7-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.28.0-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.28.1-1int.zip"=@(
        "win-bridge.exe"
    );
    "v1.28.3-1int.zip"=@(
        "win-bridge.exe"
    )
}

# NotSignedResult is used to record unsigned files that we think should be signed
$NotSignedResult=@{}

# AllNotSignedFiles is used to record all unsigned files in vhd cache and we exclude files in SkipMapForSignature
$AllNotSignedFiles=@{}

function DownloadFileWithRetry {
    param (
        $URL,
        $Dest,
        $retryCount = 5,
        $retryDelay = 0,
        [Switch]$redactUrl = $false
    )
    curl.exe -f --retry $retryCount --retry-delay $retryDelay -L $URL -o $Dest
    if ($LASTEXITCODE) {
        $logURL = $URL
        if ($redactUrl) {
            $logURL = $logURL.Split("?")[0]
        }
        throw "Curl exited with '$LASTEXITCODE' while attemping to download '$logURL'"
    }
}

function Test-ValidateAllSignature {
    foreach ($dir in $map.Keys) {
        Test-ValidateSinglePackageSignature $dir
    }

    if ($AllNotSignedFiles.Count -ne 0) {
        $AllNotSignedFiles = (echo $AllNotSignedFiles | ConvertTo-Json -Compress)
        Write-Output "All not signed file in cached packages are: $AllNotSignedFiles"
    }

    if ($NotSignedResult.Count -ne 0) {
        $NotSignedResult = (echo $NotSignedResult | ConvertTo-Json -Compress)
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
        if (!(Test-Path $dir)) {
            New-Item -ItemType Directory $dir -Force | Out-Null
        }

        DownloadFileWithRetry -URL $URL -Dest $dest -redactUrl

        $installDir="c:\SignatureCheck"
        if (!(Test-Path $installDir)) {
            New-Item -ItemType Directory $installDir -Force | Out-Null
        }
        if ($fileName.endswith(".zip")) {
            Expand-Archive -path $dest -DestinationPath $installDir -Force
        } elseif ($fileName.endswith(".tar.gz")) {
            tar -xzf $dest -C $installDir
        } else {
            Write-Error "Unknown package suffix"
            exit 1
        }

        # Check signature for 4 types of files and record unsigned files
        $includeList = @("*.exe", "*.ps1", "*.psm1", "*.dll")
        $NotSignedList = (Get-ChildItem -Path $installDir -Recurse -File -Include $includeList | ForEach-object {Get-AuthenticodeSignature $_.FullName} | Where-Object {$_.status -ne "Valid"})
        if ($NotSignedList.Count -ne 0) {
            foreach ($NotSignedFile in $NotSignedList) {
                $NotSignedFileName = [IO.Path]::GetFileName($NotSignedFile.Path)
                if (($SkipMapForSignature.ContainsKey($fileName) -and ($SkipMapForSignature[$fileName].Length -ne 0) -and !$SkipMapForSignature[$fileName].Contains($NotSignedFileName)) -or !$SkipMapForSignature.ContainsKey($fileName)) {
                    if (!$NotSignedResult.ContainsKey($dir)) {
                        $NotSignedResult[$dir]=@{}
                    }
                    if (!$NotSignedResult[$dir].ContainsKey($fileName)) {
                        $NotSignedResult[$dir][$fileName]=@()
                    }
                    $NotSignedResult[$dir][$fileName]+=@($NotSignedFileName)
                }
            }
        }

        # Check signature for all types of files except some known types and record unsigned files
        $excludeList = @("*.man", "*.reg", "*.md", "*.toml", "*.cmd", "*.template", "*.txt", "*.wprp", "*.yaml", "*.json", "NOTICE", "*.config", "*.conflist")
        $AllNotSignedList = (Get-ChildItem -Path $installDir -Recurse -File -Exclude $excludeList | ForEach-object {Get-AuthenticodeSignature $_.FullName} | Where-Object {$_.status -ne "Valid"})
        foreach ($NotSignedFile in $AllNotSignedList) {
            $NotSignedFileName = [IO.Path]::GetFileName($NotSignedFile.Path)
            if (($SkipMapForSignature.ContainsKey($fileName) -and ($SkipMapForSignature[$fileName].Length -ne 0) -and !$SkipMapForSignature[$fileName].Contains($NotSignedFileName)) -or !$SkipMapForSignature.ContainsKey($fileName)) {
                if (!$AllNotSignedFiles.ContainsKey($dir)) {
                    $AllNotSignedFiles[$dir]=@{}
                }
                if (!$AllNotSignedFiles[$dir].ContainsKey($fileName)) {
                    $AllNotSignedFiles[$dir][$fileName]=@()
                }
                $AllNotSignedFiles[$dir][$fileName]+=@($NotSignedFileName)
            }
        }

        Remove-Item -Path $installDir -Force -Recurse
    }
}

Test-ValidateAllSignature