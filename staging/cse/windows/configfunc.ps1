

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

    Write-Log "Docker version $DockerVersion found, clearing DOCKER_API_VERSION"
    [System.Environment]::SetEnvironmentVariable('DOCKER_API_VERSION', $null, [System.EnvironmentVariableTarget]::Machine)

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
    #
    # The fix which reduces dynamic port usage is still needed for DSR mode
    # Update the range to avoid that it conflicts with NodePort range (30000 - 32767)
    if ($global:EnableIncreaseDynamicPortRange) {
        # UDP port 65330 is excluded in vhdbuilder/packer/configure-windows-vhd.ps1 since it may fail when it is set in provisioning nodes
        Invoke-Executable -Executable "netsh.exe" -ArgList @("int", "ipv4", "set", "dynamicportrange", "tcp", "16385", "49151") -ExitCode $global:WINDOWS_CSE_ERROR_SET_TCP_DYNAMIC_PORT_RANGE
        Invoke-Executable -Executable "netsh.exe" -ArgList @("int", "ipv4", "add", "excludedportrange", "tcp", "30000", "2768") -ExitCode $global:WINDOWS_CSE_ERROR_SET_TCP_EXCLUDE_PORT_RANGE
        Invoke-Executable -Executable "netsh.exe" -ArgList @("int", "ipv4", "set", "dynamicportrange", "udp", "16385", "49151") -ExitCode $global:WINDOWS_CSE_ERROR_SET_UDP_DYNAMIC_PORT_RANGE
        Invoke-Executable -Executable "netsh.exe" -ArgList @("int", "ipv4", "add", "excludedportrange", "udp", "30000", "2768") -ExitCode $global:WINDOWS_CSE_ERROR_SET_UDP_EXCLUDE_PORT_RANGE
    } else {
        Invoke-Executable -Executable "netsh.exe" -ArgList @("int", "ipv4", "set", "dynamicportrange", "tcp", "33000", "32536") -ExitCode $global:WINDOWS_CSE_ERROR_SET_TCP_DYNAMIC_PORT_RANGE
    }
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

    $tempInstallPackageFoler = [Io.path]::Combine($env:TEMP, "CCGAKVPlugin")
    $tempPluginZipFile = [Io.path]::Combine($ENV:TEMP, "gmsa.zip")

    Write-Log "Getting the GMSA plugin package"
    DownloadFileOverHttp -Url $GmsaPackageUrl -DestinationPath $tempPluginZipFile -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_GMSA_PACKAGE
    Expand-Archive -Path $tempPluginZipFile -DestinationPath $tempInstallPackageFoler -Force
    if ($LASTEXITCODE) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_EXPAND_ARCHIVE -ErrorMessage "Failed to extract the '$tempPluginZipFile' archive."
    }
    Remove-Item -Path $tempPluginZipFile -Force

    # Copy the plugin DLL file.
    Write-Log "Installing the GMSA plugin"
    Copy-Item -Force -Path "$tempInstallPackageFoler\CCGAKVPlugin.dll" -Destination "${env:SystemRoot}\System32\"

    # Enable the PowerShell privilege to set the registry permissions.
    Write-Log "Enabling the PowerShell privilege"
    $enablePrivilegeResponse=$false
    for($i = 0; $i -lt 10; $i++) {
        Write-Log "Retry $i : Trying to enable the PowerShell privilege"
        $enablePrivilegeResponse = Enable-Privilege -Privilege "SeTakeOwnershipPrivilege"
        if ($enablePrivilegeResponse) {
            break
        }
        Start-Sleep 1
    }
    if(!$enablePrivilegeResponse) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_ENABLE_POWERSHELL_PRIVILEGE -ErrorMessage "Failed to enable the PowerShell privilege."
    }

    # Set the registry permissions.
    Write-Log "Setting GMSA plugin registry permissions"
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
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_PERMISSION -ErrorMessage "Failed to set GMSA plugin registry permissions. $_"
    }
  
    # Set the appropriate registry values.
    try {
        Write-Log "Setting the appropriate GMSA plugin registry values"
        reg.exe import "$tempInstallPackageFoler\registerplugin.reg" 2>$null 1>$null
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_SET_REGISTRY_VALUES -ErrorMessage  "Failed to set GMSA plugin registry values. $_"
    }

    # Enable the logging manifest.
    Write-Log "Importing the CCGEvents manifest file"
    wevtutil.exe im "$tempInstallPackageFoler\CCGEvents.man"
    if ($LASTEXITCODE) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_IMPORT_CCGEVENTS -ErrorMessage "Failed to import the CCGEvents.man manifest file. $LASTEXITCODE"
    }

    # Enable the CCGAKVPlugin logging manifest.
    # Introduced since v1.1.3
    if (Test-Path -Path "$tempInstallPackageFoler\CCGAKVPluginEvents.man" -PathType Leaf) {
        Write-Log "Importing the CCGAKVPluginEvents manifest file"
        wevtutil.exe im "$tempInstallPackageFoler\CCGAKVPluginEvents.man"
        if ($LASTEXITCODE) {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_GMSA_IMPORT_CCGAKVPPLUGINEVENTS -ErrorMessage "Failed to import the CCGAKVPluginEvents.man manifest file. $LASTEXITCODE"
        }
    } else {
        Write-Log "CCGAKVPluginEvents.man does not exist in the package"
    }

    Write-Log "Removing $tempInstallPackageFoler"
    Remove-Item -Path $tempInstallPackageFoler -Force -Recurse

    Write-Log "Successfully installed the GMSA plugin"
}

function Install-OpenSSH {
    Param(
        [Parameter(Mandatory = $true)][string[]] 
        $SSHKeys
    )

    $adminpath = "c:\ProgramData\ssh"
    $adminfile = "administrators_authorized_keys"

    $sshdService = Get-Service | ? Name -like 'sshd'
    if ($sshdService.Count -eq 0)
    {
        Write-Log "Installing OpenSSH"
        $isAvailable = Get-WindowsCapability -Online | ? Name -like 'OpenSSH*'

        if (!$isAvailable) {
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_OPENSSH_NOT_INSTALLED -ErrorMessage "OpenSSH is not available on this machine"
        }

        Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
    }
    else
    {
        Write-Log "OpenSSH Server service detected - skipping online install..."
    }

    Start-Service sshd

    if (!(Test-Path "$adminpath")) {
        Write-Log "Created new file and text content added"
        New-Item -path $adminpath -name $adminfile -type "file" -value ""
    }

    Write-Log "$adminpath found."
    Write-Log "Adding keys to: $adminpath\$adminfile ..."
    $SSHKeys | foreach-object {
        Add-Content $adminpath\$adminfile $_
    }

    Write-Log "Setting required permissions..."
    icacls $adminpath\$adminfile /remove "NT AUTHORITY\Authenticated Users"
    icacls $adminpath\$adminfile /inheritance:r
    icacls $adminpath\$adminfile /grant SYSTEM:`(F`)
    icacls $adminpath\$adminfile /grant BUILTIN\Administrators:`(F`)

    Write-Log "Restarting sshd service..."
    Restart-Service sshd
    # OPTIONAL but recommended:
    Set-Service -Name sshd -StartupType 'Automatic'

    # Confirm the Firewall rule is configured. It should be created automatically by setup. 
    $firewall = Get-NetFirewallRule -Name *ssh*

    if (!$firewall) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_OPENSSH_FIREWALL_NOT_CONFIGURED -ErrorMessage "OpenSSH's firewall is not configured properly"
    }
    Write-Log "OpenSSH installed and configured successfully"
}

function New-CsiProxyService {
    Param(
        [Parameter(Mandatory = $true)][string]
        $CsiProxyPackageUrl,
        [Parameter(Mandatory = $true)][string]
        $KubeDir
    )

    $tempdir = New-TemporaryDirectory
    $binaryPackage = "$tempdir\csiproxy.tar"

    DownloadFileOverHttp -Url $CsiProxyPackageUrl -DestinationPath $binaryPackage -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_CSI_PROXY_PACKAGE

    tar -xzf $binaryPackage -C $tempdir
    cp "$tempdir\bin\csi-proxy.exe" "$KubeDir\csi-proxy.exe"

    del $tempdir -Recurse

    & "$KubeDir\nssm.exe" install csi-proxy "$KubeDir\csi-proxy.exe" | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppDirectory "$KubeDir" | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppRestartDelay 5000 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy Description csi-proxy | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy Start SERVICE_DEMAND_START | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy ObjectName LocalSystem | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppThrottle 1500 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppStdout "$KubeDir\csi-proxy.log" | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppStderr "$KubeDir\csi-proxy.err.log" | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppStdoutCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppStderrCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppRotateFiles 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppRotateOnline 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppRotateSeconds 86400 | RemoveNulls
    & "$KubeDir\nssm.exe" set csi-proxy AppRotateBytes 10485760 | RemoveNulls
}

function New-HostsConfigService {
    $HostsConfigParameters = [io.path]::Combine($KubeDir, "hostsconfigagent.ps1")

    & "$KubeDir\nssm.exe" install hosts-config-agent C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppDirectory "$KubeDir" | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppParameters $HostsConfigParameters | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppRestartDelay 5000 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent Description hosts-config-agent | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent Start SERVICE_DEMAND_START | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent ObjectName LocalSystem | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent Type SERVICE_WIN32_OWN_PROCESS | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppThrottle 1500 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppStdout "$KubeDir\hosts-config-agent.log" | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppStderr "$KubeDir\hosts-config-agent.err.log" | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppStdoutCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppStderrCreationDisposition 4 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppRotateFiles 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppRotateOnline 1 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppRotateSeconds 86400 | RemoveNulls
    & "$KubeDir\nssm.exe" set hosts-config-agent AppRotateBytes 10485760 | RemoveNulls
}

function CopyFileFromCache {
    Param(
        [Parameter(Mandatory = $true)][string]
        $DestinationFolder,
        [Parameter(Mandatory = $true)][string]
        $FileName
    )
    $search = @()
    if (Test-Path $global:CacheDir) {
        $search = [IO.Directory]::GetFiles($global:CacheDir, $FileName, [IO.SearchOption]::AllDirectories)
    }

    if ($search.Count -ne 0) {
        $DestinationPath=[io.path]::Combine($DestinationFolder, $FileName)
        Write-Log "Using cached version of $FileName - Copying file from $($search[0]) to $DestinationPath"
        Copy-Item -Path $search[0] -Destination $DestinationPath -Force
    }
    else {
        Write-Log "WARNING: $FileName is missed in cached folder $global:CacheDir"
    }
}
