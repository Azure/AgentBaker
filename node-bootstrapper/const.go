package main

// Some options are intentionally non-configurable to avoid customization by users
// it will help us to avoid introducing any breaking changes in the future.
const (
	logFile               = "/var/log/azure/node-bootstrapper.log"
	bootstrapService      = "bootstrap.service"
	provisionJSONFilePath = "/var/log/azure/aks/provision.json"
)
