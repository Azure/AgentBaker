

# Set the service telemetry GUID. This is used with Windows Analytics https://docs.microsoft.com/en-us/sccm/core/clients/manage/monitor-windows-analytics
function Set-TelemetrySetting
{
    Param(
        [Parameter(Mandatory=$true)][string]
        $WindowsTelemetryGUID
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.SetTelemetrySetting" -TaskMessage "Start to apply telemetry data setting. WindowsTelemetryGUID: $global:WindowsTelemetryGUID"
    Set-ItemProperty -Path "HKLM:\Software\Microsoft\Windows\CurrentVersion\Policies\DataCollection" -Name "CommercialId" -Value $WindowsTelemetryGUID -Force
}

# Resize the system partition to the max available size. Azure can resize a managed disk, but the VM still needs to extend the partition boundary
# This approach was recommended by the Windows Storage team to avoid performance delay when calling Get-PartitionSupportedSize
function Resize-OSDrive
{
    Logs-To-Event -TaskName "AKS.WindowsCSE.ResizeOSDrive" -TaskMessage "Start to resize os drive if possible"
    try {
        $osDrive = ((Get-WmiObject Win32_OperatingSystem -ErrorAction Stop).SystemDrive).TrimEnd(":")

        # Ensure the OS volume needs to be expanded, diskpart will fail if the partition is already expanded
        $osDisk = Get-Partition -DriveLetter $osDrive | Get-Disk
        if ($osDisk.Size - $osDisk.AllocatedSize -gt 1GB)
        {
            Write-Log "Expanding the OS volume"
            # Create a diskpart script (text file) that will select the OS volume, extend it and exit.
            $diskpartScriptPath = [String]::Format("{0}\\diskpart_extendOSVol.script", $env:temp)
            [String]::Format("select volume {0}`nextend`nexit", $osDrive) | Out-File -Encoding "UTF8" $diskpartScriptPath
            Invoke-Executable -Executable "diskpart.exe" -ArgList @("/s", $diskpartScriptPath) -ExitCode $global:WINDOWS_CSE_ERROR_RESIZE_OS_DRIVE
            Remove-Item -Path $diskpartScriptPath -Force
        } else {
            Write-Log "No need to expand the OS volume due to ScheduledTask executed before CSE."
        }
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_RESIZE_OS_DRIVE -ErrorMessage "Failed to resize os drive. Error: $_"
    }
}

# https://docs.microsoft.com/en-us/powershell/module/storage/new-partition
function Initialize-DataDisks
{
    Logs-To-Event -TaskName "AKS.WindowsCSE.InitializeDataDisks" -TaskMessage "Start to initialize data disks"
    Get-Disk | Where-Object PartitionStyle -eq 'raw' | Initialize-Disk -PartitionStyle MBR -PassThru | New-Partition -UseMaximumSize -AssignDriveLetter | Format-Volume -FileSystem NTFS -Force
}

# Set the Internet Explorer to use the latest rendering mode on all sites
# https://docs.microsoft.com/en-us/windows-hardware/customize/desktop/unattend/microsoft-windows-ie-internetexplorer-intranetcompatibilitymode
# (This only affects installations with UI)
function Set-Explorer
{
    Logs-To-Event -TaskName "AKS.WindowsCSE.SetExplorer" -TaskMessage "Start to disable Internet Explorer compat mode and set homepage"

    New-Item -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer"
    New-Item -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\BrowserEmulation"
    New-ItemProperty -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\BrowserEmulation" -Name IntranetCompatibilityMode -Value 0 -Type DWord
    New-Item -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\Main"
    New-ItemProperty -Path HKLM:"\\SOFTWARE\\Policies\\Microsoft\\Internet Explorer\\Main" -Name "Start Page" -Type String -Value http://bing.com
}

# Pagefile adjustments
function Adjust-PageFileSize()
{
    Logs-To-Event -TaskName "AKS.WindowsCSE.AdjustPageFileSize" -TaskMessage "Start to adjust pagefile size"

    try {
        $computersys = Get-WmiObject Win32_ComputerSystem -EnableAllPrivileges;
        $computersys.AutomaticManagedPagefile = $False;
        $computersys.Put();

        $pagefile = Get-WmiObject -Query "Select * From Win32_PageFileSetting Where Name like '%pagefile.sys'";
        $pagefile.InitialSize = 8096;
        $pagefile.MaximumSize = 8096;
        $pagefile.Put();
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_ADJUST_PAGEFILE_SIZE -ErrorMessage "Failed to adjust pagefile size. Error: $_"
    }
}

function Adjust-DynamicPortRange()
{
    Logs-To-Event -TaskName "AKS.WindowsCSE.AdjustDynamicPortRange" -TaskMessage "Start to adjust dynamic port range"

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
    Logs-To-Event -TaskName "AKS.WindowsCSE.UpdateServiceFailureActions" -TaskMessage "Start to update service failure actions"

    sc.exe failure "kubelet" actions= restart/60000/restart/60000/restart/60000 reset= 900
    sc.exe failure "kubeproxy" actions= restart/60000/restart/60000/restart/60000 reset= 900
    sc.exe failure "containerd" actions= restart/60000/restart/60000/restart/60000 reset= 900
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

    Logs-To-Event -TaskName "AKS.WindowsCSE.EnableFIPSMode" -TaskMessage "Start to enable FIPS mode: $FipsEnabled."
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
    Logs-To-Event -TaskName "AKS.WindowsCSE.InstallGmsaPlugin" -TaskMessage "Start to install Windows gmsa package. WindowsGmsaPackageUrl: $global:WindowsGmsaPackageUrl"

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
    Logs-To-Event -TaskName "AKS.WindowsCSE.InstallOpenSSH" -TaskMessage "Start to install OpenSSH"

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

function Config-CredentialProvider {
    Param(
        [Parameter(Mandatory=$true)][string]
        $KubeDir,
        [Parameter(Mandatory=$true)][string]
        $CredentialProviderConfPath,
        [Parameter(Mandatory = $false)][string]
        $CustomCloudContainerRegistryDNSSuffix
    )

    Write-Log "Configuring kubelet credential provider"
    $azureConfigFile = [io.path]::Combine("$KubeDir", "azure.json")

    $credentialProviderConfig = @"
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    args:
      - $azureConfigFile
"@

    if (![string]::IsNullOrEmpty($CustomCloudContainerRegistryDNSSuffix)) {
        $credentialProviderConfig = @"
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: acr-credential-provider
    matchImages:
      - "*.azurecr.io"
      - "*.azurecr.cn"
      - "*.azurecr.de"
      - "*.azurecr.us"
      - "*$CustomCloudContainerRegistryDNSSuffix"
    defaultCacheDuration: "10m"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    args:
      - $azureConfigFile
"@
    }
    $credentialProviderConfig | Out-File -encoding ASCII -filepath "$CredentialProviderConfPATH"
}

function Validate-CredentialProviderConfigFlags {
    function get-KubeletFlagValue {
        Param(
            [Parameter(Mandatory=$true)][string]
            $KubeletConfigArg
        )
        $splitResult=($KubeletConfigArg -split "=")
        if ($splitResult.Length -ne 2 -or [string]::IsNullOrEmpty($splitResult[1])){
            Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CREDENTIAL_PROVIDER_CONFIG -ErrorMessage "Failed to get kubelet flag value from flag $KubeletConfigArg"
        }
        return $splitResult[1]
    }
    ForEach ($kubeletConfigArg in $global:KubeletConfigArgs){
        if ($kubeletConfigArg -like "--image-credential-provider-config=*") {
            $global:credentialProviderConfigPath=get-KubeletFlagValue -KubeletConfigArg $kubeletConfigArg
        }
        if ($kubeletConfigArg -like "--image-credential-provider-bin-dir=*") {
            $global:credentialProviderBinDir=get-KubeletFlagValue -KubeletConfigArg $kubeletConfigArg
        }
    }

    # Both flags should be set to enable out of tree credential provider or not set at the same time to disable it.
    if ([string]::IsNullOrEmpty($credentialProviderConfigPath) -xor [string]::IsNullOrEmpty($credentialProviderBinDir)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CREDENTIAL_PROVIDER_CONFIG -ErrorMessage "Not all credential provider flags are configured: --image-credential-provider-config=$credentialProviderConfigPath, --image-credential-provider-bin-dir=$credentialProviderBinDir"
    }
}

function Install-CredentialProvider {
    Param(
        [Parameter(Mandatory=$true)][string]
        $KubeDir,
        [Parameter(Mandatory = $false)][string]
        $CustomCloudContainerRegistryDNSSuffix
    )

    try {
        # Out of tree credential provider is turned on as a must after 1.30, and is optinal in 1.29, for cluster < 1.29, it's not enabled.
        # And only when it's enabled, the credential provider flags are set.
        $global:credentialProviderConfigPath = ""
        $global:credentialProviderBinDir = ""
        Validate-CredentialProviderConfigFlags
        if ([string]::IsNullOrEmpty($global:credentialProviderConfigPath) -and [string]::IsNullOrEmpty($global:credentialProviderBinDir)) {
            Write-Log "Out of tree credential provider is not enabled"
            return
        }

        Logs-To-Event -TaskName "AKS.WindowsCSE.Install-CredentialProvider" -TaskMessage "Start to install out of tree credential provider"

        Write-Log "Create credential provider configuration file"
        Config-CredentialProvider -KubeDir $KubeDir -CredentialProviderConfPath $global:credentialProviderConfigPath -CustomCloudContainerRegistryDNSSuffix $CustomCloudContainerRegistryDNSSuffix

        Write-Log "Download credential provider binary from $global:CredentialProviderURL to $global:credentialProviderBinDir"
        $tempDir = New-TemporaryDirectory
        $credentialproviderbinaryPackage = "$tempDir\credentialprovider.tar.gz"
        DownloadFileOverHttp -Url $global:CredentialProviderURL -DestinationPath $credentialproviderbinaryPackage -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_CREDEDNTIAL_PROVIDER
        tar -xzf $credentialproviderbinaryPackage -C $tempDir
        Create-Directory -FullPath $global:credentialProviderBinDir
        cp "$tempDir\azure-acr-credential-provider.exe" "$global:credentialProviderBinDir\acr-credential-provider.exe"
        # acr-credential-provider.exe cannot be found by kubelet through provider name before the fix https://github.com/kubernetes/kubernetes/pull/120291
        # so we copy the exe file to acr-credential-provider to make all 1.29 release work.
        cp "$global:credentialProviderBinDir\acr-credential-provider.exe" "$global:credentialProviderBinDir\acr-credential-provider"
        del $tempDir -Recurse
    } catch {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_INSTALL_CREDENTIAL_PROVIDER -ErrorMessage "Error installing credential provider. Error: $_"
    }
}

function New-CsiProxyService {
    Param(
        [Parameter(Mandatory = $true)][string]
        $CsiProxyPackageUrl,
        [Parameter(Mandatory = $true)][string]
        $KubeDir
    )

    Logs-To-Event -TaskName "AKS.WindowsCSE.StartCsiProxyService" -TaskMessage "Start Csi proxy service. CsiProxyUrl: $global:CsiProxyUrl"

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
    Logs-To-Event -TaskName "AKS.WindowsCSE.StartHostConfigService" -TaskMessage "Start hosts config agent"

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

function Register-LogCollectorScriptTask {
    Param(
        [Parameter(Mandatory = $true)][int]
        $IntervalInMinutes
    )
    Write-Log "Creating a scheduled task to run loggenerator.ps1"

    $action = New-ScheduledTaskAction -Execute "powershell.exe" -Argument "-File `"c:\k\loggenerator.ps1`""
    $principal = New-ScheduledTaskPrincipal -UserId SYSTEM -LogonType ServiceAccount -RunLevel Highest
    $trigger = New-JobTrigger -Once -At (Get-Date).Date -RepeatIndefinitely -RepetitionInterval (New-TimeSpan -Minutes $IntervalInMinutes)
    $definition = New-ScheduledTask -Action $action -Principal $principal -Trigger $trigger -Description "aks-log-generator-task"
    Register-ScheduledTask -TaskName "aks-log-generator-task" -InputObject $definition
}

function Enable-GuestVMLogs {
    Param(
        [Parameter(Mandatory = $true)][int]
        $IntervalInMinutes
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.EnableGuestVMLogs" -TaskMessage "Start to enable Guest VM Logs. LogGeneratorIntervalInMinutes: $LogGeneratorIntervalInMinutes"

    if ($IntervalInMinutes -le 0) {
        Write-Log "Do not add AKS logs in GuestVMLogs"
        return
    }

    Register-LogCollectorScriptTask -IntervalInMinutes $IntervalInMinutes
}

function Upload-GuestVMLogs {
    Param(
        [Parameter(Mandatory = $true)][int]
        $ExitCode
    )

    try {
        if ($ExitCode -ne 0) {
            # We do not reuse aks-log-generator-task or loggenerator.ps1 since neither may exist
            Write-Log "Start to upload guestvmlogs when failing in node provisioning"

            $aksLogFolder="C:\WindowsAzure\Logs\aks"
            $tempWorkFoler = [Io.path]::Combine($env:TEMP, "guestvmlogs")

            Write-Log "Creating $aksLogFolder"
            # The folder "C:\WindowsAzure\Logs" may not exist
            New-Item -ItemType Directory -Force -Path $aksLogFolder > $null

            Write-Log "Creating SymbolicLink for C:\AzureData\CustomDataSetupScript.log in $aksLogFolder"
            New-Item -ItemType SymbolicLink -Path (Join-Path $aksLogFolder "CustomDataSetupScript.log") -Target "C:\AzureData\CustomDataSetupScript.log" > $null

            # Create a work folder
            Write-Log "Creating $tempWorkFoler"
            Create-Directory -FullPath $tempWorkFoler
            cd $tempWorkFoler

            # Generate logs
            Write-Log "Generating guestvmlogs"
            Invoke-Expression(Get-Childitem -Path "C:\WindowsAzure\" -Filter "CollectGuestLogs.exe" -Recurse | sort LastAccessTime -desc | select -first 1).FullName

            # Get the output
            $logFile=(Get-Childitem -Path $tempWorkFoler  -Filter "*.zip").FullName

            # Upload logs
            Write-Log "Start to uploading $logFile"
            C:\AzureData\windows\sendlogs.ps1 -Path $logFile
        } elseif (Get-ScheduledTask -TaskName 'aks-log-generator-task' -ErrorAction Ignore) {
            Write-Log "Start the scheduled task aks-log-generator-task to upload the CSE log immediately"
            # Upload the full node logs if it succeeds and it is enabled
            Start-ScheduledTask -TaskName 'aks-log-generator-task'
        }
    } catch {
        # This should not impact the node provisioning result
        Write-Log "Failed to upload CustomDataSetupScript.log. $_"
    }
}

# Retag-ImagesForAzureChinaCloud add tags for images for AzureChinaCloud to
# use cached images instead of pulling them from MCR
# This must be run after installing containerd but before New-InfraContainer
function Retag-ImagesForAzureChinaCloud {
    param(
        [Parameter(Mandatory=$true)][string]
        $TargetEnvironment
    )

    Logs-To-Event -TaskName "AKS.WindowsCSE.RetagImagesForAzureChinaCloud" -TaskMessage "Start to retag images for Azure China Cloud"
    
    $isExist=$false
    $imageList=$(ctr.exe -n k8s.io image ls | select -Skip 1)
    foreach ($imageInfo in $imageList) {
        $splitResult=($imageInfo -split '\s+')
        $image=$splitResult[0]
        if ($image -like 'mcr.azk8s.cn*') {
            $isExist=$true
            break
        }
    }
    
    if ($TargetEnvironment -ne "AzureChinaCloud") {
        if ($isExist) {
            Write-Log "Clear existing tags for AzureChinaCloud in $TargetEnvironment"
            foreach ($imageInfo in $imageList) {
                $splitResult=($imageInfo -split '\s+')
                $image=$splitResult[0]
                if ($image -like 'mcr.azk8s.cn*' -and (-not $image.Contains("@sha256:"))) {
                    ctr.exe -n k8s.io image delete $image
                }
            }
        }
        Write-Log "Not retagging images for $TargetEnvironment"
        return
    }

    # Skip if we have already retagged the images in building VHDs
    if ($isExist) {
        Write-Log "Skip because images have already been retagged for AzureChinaCloud"
        return
    }

    Write-Log "Retagging images for AzureChinaCloud"
    foreach ($imageInfo in $imageList) {
        $splitResult=($imageInfo -split '\s+')
        $image=$splitResult[0]
        if ($image -like 'mcr.microsoft.com*' -and (-not $image.Contains("@sha256:"))) {
            Write-Log "Retagging image $image for AzureChinaCloud"
            $retagImageUrl=$image.replace('mcr.microsoft.com', 'mcr.azk8s.cn')
            ctr.exe -n k8s.io image tag $image $retagImageUrl
        }
    }
}
