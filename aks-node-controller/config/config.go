// Package config holds the fixed node locations and OS identification that aks-node-controller
// commands share. Some locations are intentionally not configurable, so that users cannot customize
// them and we can change them without breaking users.
package config

const (
	// ComponentsFilePath is where the VHD build places components.json (see
	// vhdbuilder/packer/install-dependencies.sh). All scriptless VHDs are expected to have
	// this file baked in, so a missing or malformed file indicates a real build/provisioning
	// bug rather than an expected legacy-VHD gap.
	ComponentsFilePath = "/opt/azure/components.json"
	NBCCmdPath         = "/opt/azure/containers/aks-node-controller-nbc-cmd.sh"
)

// OS holds the OS-specific locations.
type OS struct {
	LogPath                   string
	ProvisionJSONFilePath     string
	ProvisionCompleteFilePath string
	ReadyReportMarkerPath     string
	EventsDirPath             string
}

func Linux() OS {
	return OS{
		LogPath:                   "/var/log/azure/aks-node-controller.log",
		ProvisionJSONFilePath:     "/var/log/azure/aks/provision.json",
		ProvisionCompleteFilePath: "/opt/azure/containers/provision.complete",
		ReadyReportMarkerPath:     "/var/lib/waagent/experimental_skip_ready_report",
		EventsDirPath:             "/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events",
	}
}

func Windows() OS {
	return OS{
		LogPath:                   `C:\AzureData\aks-node-controller\aks-node-controller.log`,
		ProvisionJSONFilePath:     `C:\AzureData\aks-node-controller\provision.json`,
		ProvisionCompleteFilePath: `C:\AzureData\aks-node-controller\provision.complete`,
		// The Ready-report handoff is a Linux waagent feature.
		ReadyReportMarkerPath: "",
		EventsDirPath:         `C:\WindowsAzure\Logs\Plugins\Microsoft.Compute.CustomScriptExtension\Events\`,
	}
}

func ForOS(goos string) OS {
	if goos == "windows" {
		return Windows()
	}
	return Linux()
}
