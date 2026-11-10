package main

// Windows equivalents of const_unix.go. The controller uses its own directory under
// C:\AzureData because the legacy Windows CSE already writes a provision.complete
// there with different contents (the CSE result rather than an empty sentinel).
const (
	logPath                   = `C:\AzureData\aks-node-controller\aks-node-controller.log`
	provisionJSONFilePath     = `C:\AzureData\aks-node-controller\provision.json`
	provisionCompleteFilePath = `C:\AzureData\aks-node-controller\provision.complete`
	// The Ready-report handoff is a Linux waagent feature; an empty path disables it.
	readyReportMarkerPath = ""
	// Matches $global:EventsLoggingDir in parts/windows/windowscsehelper.ps1.
	eventsDirPath = `C:\WindowsAzure\Logs\Plugins\Microsoft.Compute.CustomScriptExtension\Events\`
)
