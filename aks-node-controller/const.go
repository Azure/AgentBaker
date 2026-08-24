package main

// Some options are intentionally non-configurable to avoid customization by users
// it will help us to avoid introducing any breaking changes in the future.
const (
	logPath                   = "/var/log/azure/aks-node-controller.log"
	provisionJSONFilePath     = "/var/log/azure/aks/provision.json"
	provisionCompleteFilePath = "/opt/azure/containers/provision.complete"
	// preProvisionCompleteFilePath is the completion marker for a pre-provision (image bake) run.
	// It is volatile: /run is a tmpfs cleared on every boot, so it cannot be captured into the
	// image and a node created from that image cannot inherit the bake's completion signal.
	preProvisionCompleteFilePath = "/run/azure/pre-provision.complete"
)
