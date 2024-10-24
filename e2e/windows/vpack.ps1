param(
    [string]$arg1, #GitBranch
    [string]$arg2, #VPackName
    [string]$arg3  #VPackFolder
)

function Retry-Command {
    [CmdletBinding()]
    Param(
        [Parameter(Position=0, Mandatory=$true)]
        [scriptblock]$ScriptBlock,

        [Parameter(Position=1, Mandatory=$true)]
        [string]$ErrorMessage,

        [Parameter(Position=2, Mandatory=$false)]
        [int]$Maximum = 5,

        [Parameter(Position=3, Mandatory=$false)]
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
            } catch {
                Write-Log $_.Exception.InnerException.Message
                Write-Log "Retry $cnt : $ScriptBlock"
                Start-Sleep $Delay
            }
        } while ($cnt -lt $Maximum)

        # Throw an error after $Maximum unsuccessful invocations. Doesn't need
        # a condition, since the function returns upon successful invocation.
        throw 'All retries failed. $ErrorMessage'
    }
}

# Pull AgentBaker Repo
git clone "https://github.com/Azure/AgentBaker.git"
git checkout $arg1
New-Item -Path . -Name $arg3 -ItemType Directory
Copy-Item "AgentBaker/vhdbuilder/packer/configure-windows-vhd.ps1" -Destination $arg3
Copy-Item "AgentBaker/vhdbuilder/packer/generate-windows-vhd-configuration.ps1" -Destination $arg3
Copy-Item "AgentBaker/vhdbuilder/packer/test/windows-vhd-content-test.ps1" -Destination $arg3

# Install VPack
# Reference: https://www.osgwiki.com/wiki/VPack.exe#(Installation)
$downloadPath = "$env:TEMP\pkgmanbootstrap.exe"
Retry-Command -ScriptBlock {
    & (New-Object System.Net.WebClient).DownloadFile("https://aka.ms/packagemanagerbootstrap", "$downloadPath") 
} -ErrorMessage "Failed to download ES Package Manager tool"

Retry-Command -ScriptBlock {
    & $downloadPath --AcceptEula
} -ErrorMessage "Failed to install ES Package Manager tool"

Remove-Item -Path $downloadPath

Retry-Command -ScriptBlock {
    & $env:LOCALAPPDATA\EspmClient\ESPackageManagerClient.exe install AS.VPack.Selector
} -ErrorMessage "Failed to install VPack"

$vpackPath = "$env:USERPROFILE\AppData\Local\VPack"
$regexAddPath = [regex]::Escape($vpackPath)
$arrPath = $env:Path -split ';' | Where-Object {$_ -notMatch "^$regexAddPath\\?"}
$env:Path = ($arrPath + $vpackPath) -join ';'

# $vpackName = "aks-win-test-1"
# $vpackFolder = "AKS-Windows-VHD-Script"

VPack.exe push /name:$arg2  /srcdir:$arg3

# $vpackVersion = (VPack.exe list /Name:$vpackName /OnlyLatest:True) -split '-'

# VPack.exe pull /name:$vpackName  /destdir:$vpackFolder  /vr:$vpackVersion
