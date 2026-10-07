package staging

import "embed"

//go:embed cse/windows/kubeletconfig.ps1 cse/windows/provisioningscripts/kubeletstart.ps1
var WindowsKubeletScripts embed.FS
