function Start-InstallWindowsNextGenNetworking {
    param(
        [Parameter(Mandatory = $true)]
        [AllowEmptyString()]
        [string]$NextGenNetworkingURL
    )

    Logs-To-Event -TaskName "AKS.WindowsCSE.InstallWindowsNextGenNetworking" -TaskMessage "Start to install Windows next-gen networking eBPF using URL $NextGenNetworkingURL"

    # TODO: this is a placeholder for the installation script.
}
