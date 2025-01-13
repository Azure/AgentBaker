package e2e

const (
	buildIDTagKey    = "buildID"
	defaultNamespace = "default"
)

// cse output parsing consts
const (
	extensionErrorCodeRegex   = `ProvisioningState/failed/(\d+)`
	linuxExtensionExitCodeStr = `Enable failed: failed to execute command: command terminated with exit status=(\d+)`
)
