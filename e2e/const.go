package e2e

const (
	buildIDTagKey           = "buildID"
	defaultNamespace        = "default"
	extensionErrorCodeRegex = `ProvisioningState/failed/(\d+)`
	// This is the error pattern we want to parse the exit code, see query https://dataexplorer.azure.com/clusters/aks/databases/AKSprod?query=H4sIAAAAAAAAA13MMQ7CMAxA0Z1TWJkZwhGqkoGBCXEANzWtIXUq2wIqcXjCyvyffmeb5L6K09u77Pxk33YfeM2kBItNkFtDFoPQX9KZzHCiqyyoNmNJqlUDoIx%2FNgkOheCGXGgMbbhqvVP2H9tDXUnRucrp2JLjg%2BAQY%2FwC8d99zowAAAA%3D
	linuxExtensionExitCodeStr = `Enable failed: failed to execute command: command terminated with exit status=(\d+)`
)
