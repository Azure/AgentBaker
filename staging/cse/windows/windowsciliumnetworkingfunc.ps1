
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
            $installArgs['ScenarioConfig'] = $global:WindowsCiliumNetworkingConfiguration
        }
    }

    # Invoke install script
    try {
        Invoke-WindowsCiliumNetworkingInstallScript -Arguments $installArgs

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

<#
.SYNOPSIS
Invokes the Windows Cilium Networking installation script.

.DESCRIPTION
A wrapper function for invoking the Windows Cilium Networking installation script.
This function provides a mockable interface for script execution, making unit testing easier.
The script path is determined from the global WindowsCiliumScriptsDirectory variable.

.PARAMETER Arguments
A hashtable of arguments to pass to the installation script using PowerShell splatting.

.OUTPUTS
None. The function executes the Windows Cilium Networking installation script with the provided arguments.

.EXAMPLE
Invoke-WindowsCiliumNetworkingInstallScript -Arguments @{RebootNeededOut = [ref]$isRebootNeeded}

.NOTES
This function can be easily mocked in unit tests using frameworks like Pester.
The installation script path is constructed from $global:WindowsCiliumScriptsDirectory.
#>
function Invoke-WindowsCiliumNetworkingInstallScript {
    param(
        [Parameter(Mandatory = $true)]
        [hashtable]$Arguments
    )
    
    $wcnInstallScript = Join-Path -Path $global:WindowsCiliumScriptsDirectory -ChildPath 'install.ps1'
    & $wcnInstallScript @Arguments
}
