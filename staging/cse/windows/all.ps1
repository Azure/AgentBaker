if (-not $WINDOWS_SCRIPTS_DIRECTORY) {
    $WINDOWS_SCRIPTS_DIRECTORY = 'c:\AzureData\windows'
}

# Dot-source cse scripts with functions that are bundled on the VHD
. $WINDOWS_SCRIPTS_DIRECTORY\helpers.ps1
. $WINDOWS_SCRIPTS_DIRECTORY\azurecnifunc.ps1
. $WINDOWS_SCRIPTS_DIRECTORY\calicofunc.ps1
. $WINDOWS_SCRIPTS_DIRECTORY\configfunc.ps1
. $WINDOWS_SCRIPTS_DIRECTORY\containerdfunc.ps1
. $WINDOWS_SCRIPTS_DIRECTORY\kubeletfunc.ps1
. $WINDOWS_SCRIPTS_DIRECTORY\kubernetesfunc.ps1
. $WINDOWS_SCRIPTS_DIRECTORY\nvidiagpudriverfunc.ps1
. $WINDOWS_SCRIPTS_DIRECTORY\securetlsbootstrapfunc.ps1
. $WINDOWS_SCRIPTS_DIRECTORY\windowsciliumnetworkingfunc.ps1
. $WINDOWS_SCRIPTS_DIRECTORY\networkisolatedclusterfunc.ps1
