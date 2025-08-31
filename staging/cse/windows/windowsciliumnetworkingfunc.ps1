
<#
.SYNOPSIS
Enables and installs Windows Cilium Networking on the system.

.DESCRIPTION
This function installs Windows Cilium Networking if it's enabled globally. It composes installation script arguments,
handles optional scenario configuration from JSON, and executes the installation script. The function also manages
reboot requirements and error handling during the installation process. When not enabled, the feature is left installed
but disabled to retain eBPF for Windows Guest Proxy Agent (GPA).

.PARAMETER None
This function does not accept any parameters. It relies on global variables for configuration.

.OUTPUTS
None. The function writes log messages and may set global variables for reboot status and exit codes.

.EXAMPLE
Enable-WindowsCiliumNetworking
Installs Windows Cilium Networking using the global configuration settings.

.NOTES
- Requires $global:EnableWindowsCiliumNetworking to be set to $true
- Uses $global:WindowsCiliumNetworkingConfiguration for JSON configuration
- May set $global:RebootNeeded to $true if a restart is required
- Sets exit code $global:WINDOWS_CSE_ERROR_WINDOWS_CILIUM_NETWORKING_INSTALL_FAILED on failure
- Depends on external install.ps1 script in $global:WindowsCiliumScriptsDirectory
- Leaves feature installed but disabled to preserve eBPF for Windows Guest Proxy Agent (GPA)
- Relies on node prep removing source NuGet package from aks-cache following installation

#>
function Enable-WindowsCiliumNetworking
{
    if (!$global:EnableWindowsCiliumNetworking) {
        Write-Log "Windows Cilium Networking is disabled, skipping installation"
        return
    }

    # Compose install script arguments
    $isRebootNeeded = $false
    $installArgs = @{
        RebootNeededOut = ([ref]$isRebootNeeded)
    }

    # Add scenario configuration if specified
    if (![string]::IsNullOrEmpty($global:WindowsCiliumNetworkingConfiguration)) {
        if (!(Test-Json -Json $global:WindowsCiliumNetworkingConfiguration)) {
            Write-Log "Windows Cilium Networking configuration is not valid JSON. Proceeding with default configuration."
        } else {
            $installArgs['ScenarioConfig'] = $windowsCiliumBaseConfigurationPath
        }
    }

    # Invoke install script
    try {
        $wcnInstallScript = Join-Path -Path $global:WindowsCiliumScriptsDirectory -ChildPath 'install.ps1'
        & $wcnInstallScript @installArgs

        Write-Log "Windows Cilium Networking installation completed successfully$(if ($isRebootNeeded) { ' (restart required)' })."
        if ($isRebootNeeded) {
            $global:RebootNeeded = $true
        }
    }
    catch {
        Write-Log "Failed to install Windows Cilium Networking: $_"
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_WINDOWS_CILIUM_NETWORKING_INSTALL_FAILED -ErrorMessage "$_"
    }
}
