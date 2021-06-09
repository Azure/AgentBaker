

# Set the service telemetry GUID. This is used with Windows Analytics https://docs.microsoft.com/en-us/sccm/core/clients/manage/monitor-windows-analytics
function Set-TelemetrySetting
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $WindowsTelemetryGUID
    )
    Set-ItemProperty -Path "HKLM:\Software\Microsoft\Windows\CurrentVersion\Policies\DataCollection" -Name "CommercialId" -Value $WindowsTelemetryGUID -Force
}

# Resize the system partition to the max available size. Azure can resize a managed disk, but the VM still needs to extend the partition boundary
function Resize-OSDrive
{
    $osDrive = ((Get-WmiObject Win32_OperatingSystem).SystemDrive).TrimEnd(":")
    $size = (Get-Partition -DriveLetter $osDrive).Size
    $maxSize = (Get-PartitionSupportedSize -DriveLetter $osDrive).SizeMax
    if ($size -lt $maxSize)
    {
        Resize-Partition -DriveLetter $osDrive -Size $maxSize
    }
}

# https://docs.microsoft.com/en-us/powershell/module/storage/new-partition
function Initialize-DataDisks
{
    Get-Disk | Where-Object PartitionStyle -eq 'raw' | Initialize-Disk -PartitionStyle MBR -PassThru | New-Partition -UseMaximumSize -AssignDriveLetter | Format-Volume -FileSystem NTFS -Force
}

# Set the Internet Explorer to use the latest rendering mode on all sites
# https://docs.microsoft.com/en-us/windows-hardware/customize/desktop/unattend/microsoft-windows-ie-internetexplorer-intranetcompatibilitymode
# (This only affects installations with UI)
function Set-Explorer
{
    New-Item -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer"
    New-Item -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\BrowserEmulation"
    New-ItemProperty -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\BrowserEmulation" -Name IntranetCompatibilityMode -Value 0 -Type DWord
    New-Item -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\Main"
    New-ItemProperty -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\Main" -Name "Start Page" -Type String -Value http://bing.com
}

function Install-Docker
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $DockerVersion
    )

    # DOCKER_API_VERSION needs to be set for Docker versions older than 18.09.0 EE
    # due to https://github.com/kubernetes/kubernetes/issues/69996
    # this issue was fixed by https://github.com/kubernetes/kubernetes/issues/69996#issuecomment-438499024
    # Note: to get a list of all versions, use this snippet
    # $versions = (curl.exe -L "https://go.microsoft.com/fwlink/?LinkID=825636&clcid=0x409" | ConvertFrom-Json).Versions | Get-Member -Type NoteProperty | Select-Object Name
    # Docker version to API version decoder: https://docs.docker.com/develop/sdk/#api-version-matrix

    switch ($DockerVersion.Substring(0,5))
    {
        "17.06" {
            Write-Log "Docker 17.06 found, setting DOCKER_API_VERSION to 1.30"
            [System.Environment]::SetEnvironmentVariable('DOCKER_API_VERSION', '1.30', [System.EnvironmentVariableTarget]::Machine)
        }

        "18.03" {
            Write-Log "Docker 18.03 found, setting DOCKER_API_VERSION to 1.37"
            [System.Environment]::SetEnvironmentVariable('DOCKER_API_VERSION', '1.37', [System.EnvironmentVariableTarget]::Machine)
        }

        default {
            Write-Log "Docker version $DockerVersion found, clearing DOCKER_API_VERSION"
            [System.Environment]::SetEnvironmentVariable('DOCKER_API_VERSION', $null, [System.EnvironmentVariableTarget]::Machine)
        }
    }

    try {
        $installDocker = $true
        $dockerService = Get-Service | ? Name -like 'docker'
        if ($dockerService.Count -eq 0) {
            Write-Log "Docker is not installed. Install docker version($DockerVersion)."
        }
        else {
            $dockerServerVersion = docker version --format '{{.Server.Version}}'
            Write-Log "Docker service is installed with docker version($dockerServerVersion)."
            if ($dockerServerVersion -eq $DockerVersion) {
                $installDocker = $false
                Write-Log "Same version docker installed will skip installing docker version($dockerServerVersion)."
            }
            else {
                Write-Log "Same version docker is not installed. Will install docker version($DockerVersion)."
            }
        }

        if ($installDocker) {
            Find-Package -Name Docker -ProviderName DockerMsftProvider -RequiredVersion $DockerVersion -ErrorAction Stop
            Write-Log "Found version $DockerVersion. Installing..."
            Install-Package -Name Docker -ProviderName DockerMsftProvider -Update -Force -RequiredVersion $DockerVersion
            net start docker
            Write-Log "Installed version $DockerVersion"
        }
    } catch {
        Write-Log "Error while installing package: $_.Exception.Message"
        $currentDockerVersion = (Get-Package -Name Docker -ProviderName DockerMsftProvider).Version
        Write-Log "Not able to install docker version. Using default version $currentDockerVersion"
    }
}

function Set-DockerLogFileOptions {
    Write-Log "Updating log file options in docker config"
    $dockerConfigPath = "C:\ProgramData\docker\config\daemon.json"

    if (-not (Test-Path $dockerConfigPath)) {
        "{}" | Out-File $dockerConfigPath
    }

    $dockerConfig = Get-Content $dockerConfigPath | ConvertFrom-Json
    $dockerConfig | Add-Member -Name "log-driver" -Value "json-file" -MemberType NoteProperty
    $logOpts = @{ "max-size" = "50m"; "max-file" = "5" }
    $dockerConfig | Add-Member -Name "log-opts" -Value $logOpts -MemberType NoteProperty
    $dockerConfig = $dockerConfig | ConvertTo-Json -Depth 10

    Write-Log "New docker config:"
    Write-Log $dockerConfig

    # daemon.json MUST be encoded as UTF8-no-BOM!
    Remove-Item $dockerConfigPath
    $fileEncoding = New-Object System.Text.UTF8Encoding $false
    [IO.File]::WriteAllLInes($dockerConfigPath, $dockerConfig, $fileEncoding)

    Restart-Service docker
}

# Pagefile adjustments
function Adjust-PageFileSize()
{
    wmic pagefileset set InitialSize=8096,MaximumSize=8096
}

function Adjust-DynamicPortRange()
{
    # Kube-proxy reserves 63 ports per service which limits clusters with Windows nodes
    # to ~225 services if default port reservations are used.
    # https://docs.microsoft.com/en-us/virtualization/windowscontainers/kubernetes/common-problems#load-balancers-are-plumbed-inconsistently-across-the-cluster-nodes
    # Kube-proxy load balancing should be set to DSR mode when it releases with future versions of the OS

    Invoke-Executable -Executable "netsh.exe" -ArgList @("int", "ipv4", "set", "dynamicportrange", "tcp", "16385", "49151")
}

# TODO: should this be in this PR?
# Service start actions. These should be split up later and included in each install step
function Update-ServiceFailureActions
{
    Param(
        [Parameter(Mandatory = $true)][string]
        $ContainerRuntime
    )
    sc.exe failure "kubelet" actions= restart/60000/restart/60000/restart/60000 reset= 900
    sc.exe failure "kubeproxy" actions= restart/60000/restart/60000/restart/60000 reset= 900
    sc.exe failure $ContainerRuntime actions= restart/60000/restart/60000/restart/60000 reset= 900
}

function Add-SystemPathEntry
{
    Param(
        [Parameter(Mandatory = $true)][string]
        $Directory
    )
    # update the path variable if it doesn't have the needed paths
    $path = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine)
    $updated = $false
    if(-not ($path -match $Directory.Replace("\","\\")+"(;|$)"))
    {
        $path += ";"+$Directory
        $updated = $true
    }
    if($updated)
    {
        Write-Log "Updating path, added $Directory"
        [Environment]::SetEnvironmentVariable("Path", $path, [EnvironmentVariableTarget]::Machine)
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path","User")
    }
}

function Enable-FIPSMode
{
    Param(
        [Parameter(Mandatory = $true)][bool]
        $FipsEnabled
    )

    if ( $FipsEnabled ) {
        Write-Log "Set the registry to enable fips-mode"
        Set-ItemProperty -Path "HKLM:\System\CurrentControlSet\Control\Lsa\FipsAlgorithmPolicy" -Name "Enabled" -Value 1 -Type DWORD -Force
    }
    else
    {
        Write-Log "Leave FipsAlgorithmPolicy as it is."
    }
}

function Enable-Privilege {
    param($Privilege)
    $Definition = @'
  using System;
  using System.Runtime.InteropServices;
  public class AdjPriv {
    [DllImport("advapi32.dll", ExactSpelling = true, SetLastError = true)]
    internal static extern bool AdjustTokenPrivileges(IntPtr htok, bool disall,
      ref TokPriv1Luid newst, int len, IntPtr prev, IntPtr rele);
    [DllImport("advapi32.dll", ExactSpelling = true, SetLastError = true)]
    internal static extern bool OpenProcessToken(IntPtr h, int acc, ref IntPtr phtok);
    [DllImport("advapi32.dll", SetLastError = true)]
    internal static extern bool LookupPrivilegeValue(string host, string name,
      ref long pluid);
    [StructLayout(LayoutKind.Sequential, Pack = 1)]
    internal struct TokPriv1Luid {
      public int Count;
      public long Luid;
      public int Attr;
    }
    internal const int SE_PRIVILEGE_ENABLED = 0x00000002;
    internal const int TOKEN_QUERY = 0x00000008;
    internal const int TOKEN_ADJUST_PRIVILEGES = 0x00000020;
    public static bool EnablePrivilege(long processHandle, string privilege) {
      bool retVal;
      TokPriv1Luid tp;
      IntPtr hproc = new IntPtr(processHandle);
      IntPtr htok = IntPtr.Zero;
      retVal = OpenProcessToken(hproc, TOKEN_ADJUST_PRIVILEGES | TOKEN_QUERY,
        ref htok);
      tp.Count = 1;
      tp.Luid = 0;
      tp.Attr = SE_PRIVILEGE_ENABLED;
      retVal = LookupPrivilegeValue(null, privilege, ref tp.Luid);
      retVal = AdjustTokenPrivileges(htok, false, ref tp, 0, IntPtr.Zero,
        IntPtr.Zero);
      return retVal;
    }
  }
'@
    $ProcessHandle = (Get-Process -id $pid).Handle
    $type = Add-Type $definition -PassThru
    $type[0]::EnablePrivilege($processHandle, $Privilege)
}

function Install-GmsaPlugin {
    Param(
        [Parameter(Mandatory=$true)]
        [String] $GmsaPackageUrl
    )

    if ( $GmsaPackageUrl -eq "" ) {
        Write-Log "GmsaPackageUrl is not set so skip installing GMSA plugin."
        return
    }

    $tempInstallPackageFoler = $env:TEMP
    $tempPluginZipFile = [Io.path]::Combine($ENV:TEMP, "gmsa.zip")

    Write-Log "Getting the GMSA plugin package"
    DownloadFileOverHttp -Url $GmsaPackageUrl -DestinationPath $tempPluginZipFile
    Expand-Archive -Path $tempPluginZipFile -DestinationPath $tempInstallPackageFoler -Force
    if($LASTEXITCODE) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_EXPAND_ARCHIVE -ErrorMessage "Failed to extract the '$tempPluginZipFile' archive."
    }
    Remove-Item -Path $tempPluginZipFile -Force

    $tempInstallPackageFoler = [Io.path]::Combine($tempInstallPackageFoler, "CCGPlugin")

    # Copy the plugin DLL file.
    Write-Log "Installing the GMSA plugin"
    Copy-Item -Force -Path "$tempInstallPackageFoler\CCGAKVPlugin.dll" -Destination "${env:SystemRoot}\System32\"

    # Enable the logging manifest.
    Write-Log "Importing the CCGEvents manifest file"
    wevtutil.exe im "$tempInstallPackageFoler\CCGEvents.man"
    if($LASTEXITCODE) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_IMPORT_CCGEVENTS -ErrorMessage "Failed to import the CCGEvents.man manifest file."
    }

    # Enable the PowerShell privilege to set the registry permissions.
    Write-Log "Enabling the PowerShell privilege"
    $enablePrivilegeResponse = Retry-Command -Command "Enable-Privilege" -Args @{Privilege="SeTakeOwnershipPrivilege"} -Retries 5 -RetryDelaySeconds 5
    if(!$enablePrivilegeResponse) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_ENABLE_POWERSHELL_PRIVILEGE -ErrorMessage "Failed to enable the PowerShell privilege."
    }

    # Set the registry permissions.
    Write-Log "Setting the registry permissions"
    try {
        $ccgKeyPath = "System\CurrentControlSet\Control\CCG\COMClasses"
        $owner = [System.Security.Principal.NTAccount]"BUILTIN\Administrators"

        $key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
            $ccgKeyPath,
            [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree,
            [System.Security.AccessControl.RegistryRights]::TakeOwnership)
        $acl = $key.GetAccessControl()
        $acl.SetOwner($owner)
        $key.SetAccessControl($acl)
        
        $key = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey(
            $ccgKeyPath,
            [Microsoft.Win32.RegistryKeyPermissionCheck]::ReadWriteSubTree,
            [System.Security.AccessControl.RegistryRights]::ChangePermissions)
        $acl = $key.GetAccessControl()
        $rule = New-Object System.Security.AccessControl.RegistryAccessRule(
            $owner,
            [System.Security.AccessControl.RegistryRights]::FullControl,
            [System.Security.AccessControl.AccessControlType]::Allow)
        $acl.SetAccessRule($rule)
        $key.SetAccessControl($acl)
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_PERMISSION -ErrorMessage "Failed to set the registry permissions."
    }
  
    # Set the appropriate registry values.
    # Ignore errors, because it fails even though the registry changes are applied.
    # We will validate everything below anyway.
    try {
        Write-Log "Setting the appropriate GMSA plugin registry values"
        reg.exe import "$tempInstallPackageFoler\registerplugin.reg" 2>$null 1>$null
        Write-Log "Setted GMSA plugin registry values successfully"
    } catch {
        Write-Log "Error: $_"
        $LASTEXITCODE = $null
        Write-Log "Failed to set GMSA plugin registry values"
    }

    Write-Log "Removing $tempInstallPackageFoler"
    Remove-Item -Path $tempInstallPackageFoler -Force -Recurse

    # Validate that we have the proper registry values
    $tests = @(
        @{
            "Path" = "HKLM:\SOFTWARE\Classes\Interface\{6ECDA518-2010-4437-8BC3-46E752B7B172}"
            "Validate" = {
                $default = $_.'(default)'
                $expected = "ICcgDomainAuthCredentials"
                if($default -ne $expected) {
                    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_REGISTRY_ICCG_DOMAIN_AUTH_CREDENTIALS -ErrorMessage "Default value at '$($_.PSPath)' is '$default', expected '$expected'."
                }
            }
        },
        @{
            "Path" = "HKLM:\SOFTWARE\Classes\Interface\{6ECDA518-2010-4437-8BC3-46E752B7B172}\ProxyStubClsid32"
            "Validate" = {
                $default = $_.'(default)'
                $expected = "{A6FF50C0-56C0-71CA-5732-BED303A59628}"
                if($default -ne $expected) {
                    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_REGISTRY_PROXY_STUB_CLSID32 -ErrorMessage "Default value at '$($_.PSPath)' is '$default', expected '$expected'."
                }
            }
        }
        @{
            "Path" = "HKLM:\Software\CLASSES\Appid\{557110E1-88BC-4583-8281-6AAC6F708584}"
            "Validate" = {
                $expected = @{
                    "access" = [Byte[]] 1, 0, 4, 128, 68, 0, 0, 0, 84, 0, 0, 0, 0, 0, 0, 0, 20, 0, 0, 0, 2, 0, 48, 0, 2, 0, 0, 0, 0, 0, 20, 0, 11, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 5, 18, 0, 0, 0, 0, 0, 20, 0, 11, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 5, 11, 0, 0, 0, 1, 2, 0, 0, 0, 0, 0, 5, 32, 0, 0, 0, 32, 2, 0, 0, 1, 2, 0, 0, 0, 0, 0, 5, 32, 0, 0, 0, 32, 2, 0, 0
                    "launch" = [Byte[]] 1, 0, 4, 128, 68, 0, 0, 0, 84, 0, 0, 0, 0, 0, 0, 0, 20, 0, 0, 0, 2, 0, 48, 0, 2, 0, 0, 0, 0, 0, 20, 0, 11, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 5, 18, 0, 0, 0, 0, 0, 20, 0, 11, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 5, 11, 0, 0, 0, 1, 2, 0, 0, 0, 0, 0, 5, 32, 0, 0, 0, 32, 2, 0, 0, 1, 2, 0, 0, 0, 0, 0, 5, 32, 0, 0, 0, 32, 2, 0, 0
                }
                $diff = Compare-Object $_.AccessPermission $expected["access"]
                if($diff) {
                    Write-Log $diff
                    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_REGISTRY_ACCESS_PERMISSION -ErrorMessage "The 'AccessPermission' at '$($_.PSPath)' is not the expected one."
                }
                $diff = Compare-Object $_.LaunchPermission $expected["launch"]
                if($diff) {
                    Write-Log $diff
                    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_REGISTRY_LAUNCH_PERMISSION -ErrorMessage "The 'LaunchPermission' at '$($_.PSPath)' is not the expected one."
                }
                if($_.DllSurrogate -ne "") {
                    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_REGISTRY_DLLSURROGATE -ErrorMessage "Expected 'DDllSurrogate' at '$($_.PSPath)' to be empty value."
                }
            }
        }
        @{
            "Path" = "HKLM:\SOFTWARE\CLASSES\CLSID\{CCC2A336-D7F3-4818-A213-272B7924213E}"
            "Validate" = {
                $actual = $_.AppID
                $expected = "{557110E1-88BC-4583-8281-6AAC6F708584}"
                if($actual -ne $expected) {
                    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_REGISTRY_APPID -ErrorMessage "The 'AppID' at '$($_.PSPath) is '$actual'. Expected '$expected'."
                }
            }
        }
        @{
            "Path" = "HKLM:\SOFTWARE\CLASSES\CLSID\{CCC2A336-D7F3-4818-A213-272B7924213E}\InprocServer32"
            "Validate" = {
                $default = $_.'(default)'
                $expected = "C:\Windows\System32\CCGAKVPlugin.dll"
                if($default -ne $expected) {
                    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_REGISTRY_INPROCSERVER32 -ErrorMessage "Default value at '$($_.PSPath)' is '$default', expected '$expected'."
                }
                $threadingModel = $_.ThreadingModel
                $expected = "Both"
                if($threadingModel -ne $expected) {
                    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_REGISTRY_THREADING_MODEL -ErrorMessage "The 'ThreadingModel' at '$($_.PSPath)' is '$threadingModel', expected '$expected'."
                }
            }
        }
        @{
            "Path" = "HKLM:\SYSTEM\CurrentControlSet\Control\CCG\COMClasses\{CCC2A336-D7F3-4818-A213-272B7924213E}"
            "Validate" = {
                $default = $_.'(default)'
                if($default -ne "") {
                    Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_REGISTRY_CCG_COMCLASSES -ErrorMessage "The default value at '$($_.PSPath)' is not empty value."
                }
            }
        }
    )

    Write-Log "Validating the GMSA plugin registry changes"
    foreach($t in $tests) {
        $item = Get-ItemProperty $t["Path"]
        Write-Log "Validating $($t["Path"])"
        $item | ForEach-Object $t["Validate"]
    }

    Write-Log "Successfully installed the GMSA plugin"
}
