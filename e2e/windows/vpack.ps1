param(
    [string]$arg1, #GitBranch
    [string]$arg2, #VPackName
    [string]$arg3, #VPackFolder
    [string]$arg4, #UserName
    [string]$arg5  #PAT
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
                Write-Output $_.Exception.InnerException.Message
                Write-Output "Retry $cnt : $ScriptBlock"
                Start-Sleep $Delay
            }
        } while ($cnt -lt $Maximum)

        # Throw an error after $Maximum unsuccessful invocations. Doesn't need
        # a condition, since the function returns upon successful invocation.
        throw "All retries failed. $ErrorMessage"
    }
}

cd C:\

# Pull AgentBaker Windows scripts
New-Item -Path . -Name $arg3 -ItemType Directory
Invoke-WebRequest -UseBasicParsing "https://github.com/Azure/AgentBaker/blob/$arg1/vhdbuilder/packer/configure-windows-vhd.ps1" -OutFile $arg3/configure-windows-vhd.ps1
Invoke-WebRequest -UseBasicParsing "https://github.com/Azure/AgentBaker/blob/$arg1/vhdbuilder/packer/generate-windows-vhd-configuration.ps1" -OutFile $arg3/generate-windows-vhd-configuration.ps1
Invoke-WebRequest -UseBasicParsing "https://github.com/Azure/AgentBaker/blob/$arg1/vhdbuilder/packer/test/windows-vhd-content-test.ps1" -OutFile $arg3/windows-vhd-content-test.ps1

# Install VPack
Invoke-WebRequest -UseBasicParsing "https://dist.nuget.org/win-x86-commandline/latest/nuget.exe" -OutFile nuget.exe 
Set-Content -Path .\packages.config -Value '<?xml version="1.0" encoding="utf-8"?> <configuration>  <packageSources>  <clear />  </packageSources> </configuration>' 
[xml]$doc = Get-Content .\packages.config
$doc.Save("C:\packages.config")
.\nuget.exe sources add -name "ES.Stream.AS.Packaging" -source "https://pkgs.dev.azure.com/microsoft/_packaging/ES.Stream.AS.Packaging/nuget/v3/index.json" -username "$arg4" -password "$arg5" -configfile .\packages.config
.\nuget.exe install VPack -Configfile .\packages.config

# Add PAT for VPack
Install-PackageProvider -Name NuGet -MinimumVersion 2.8.5.201 -Force
Install-Module CredentialManager -force -Repository PSGallery
$targets = "vpack:https://microsoft.artifacts.visualstudio.com", "vpack:https://microsoft.vsblob.visualstudio.com", "vcas-cms:https://microsoft.artifacts.visualstudio.com", "vcas-cms:https://microsoft.vsblob.visualstudio.com"
$targets | % {New-StoredCredential -Target $_ -UserName $arg4 -Password $arg5 -Persist LOCALMACHINE }

# Verion will increase automatically
./VPack*/content/VPack.exe push /name:$arg2  /srcdir:$arg3
