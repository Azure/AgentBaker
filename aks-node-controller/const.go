package main

// Some options are intentionally non-configurable to avoid customization by users
// it will help us to avoid introducing any breaking changes in the future.
const (
	logPath               = "/var/log/azure/aks-node-controller.log"
	provisionJSONFilePath = "/var/log/azure/aks/provision.json"
	// TODO: After 2026-10-01, remove provision.complete writer/waiter compatibility and use provision.json as the sole completion signal.
	provisionCompleteFilePath = "/opt/azure/containers/provision.complete"
)
