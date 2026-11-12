package main

import "runtime"

// Holds the OS-specific locations. Some options are intentionally non-configurable
// to avoid customization by users; it will help us avoid introducing breaking changes in the future.
type osConfig struct {
	logPath                   string
	provisionJSONFilePath     string
	provisionCompleteFilePath string
	readyReportMarkerPath     string
	eventsDirPath             string
}

func linuxConfig() osConfig {
	return osConfig{
		logPath:                   "/var/log/azure/aks-node-controller.log",
		provisionJSONFilePath:     "/var/log/azure/aks/provision.json",
		provisionCompleteFilePath: "/opt/azure/containers/provision.complete",
		readyReportMarkerPath:     "/var/lib/waagent/experimental_skip_ready_report",
		eventsDirPath:             "/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events",
	}
}

func windowsConfig() osConfig {
	return osConfig{
		logPath:                   `C:\AzureData\aks-node-controller\aks-node-controller.log`,
		provisionJSONFilePath:     `C:\AzureData\aks-node-controller\provision.json`,
		provisionCompleteFilePath: `C:\AzureData\aks-node-controller\provision.complete`,
		// The Ready-report handoff is a Linux waagent feature.
		readyReportMarkerPath: "",
		eventsDirPath:         `C:\WindowsAzure\Logs\Plugins\Microsoft.Compute.CustomScriptExtension\Events\`,
	}
}

func configForOS(goos string) osConfig {
	if goos == "windows" {
		return windowsConfig()
	}
	return linuxConfig()
}

var osCfg = configForOS(runtime.GOOS) //nolint:gochecknoglobals // read-only OS-specific paths resolved once at startup
