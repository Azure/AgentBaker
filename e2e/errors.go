package e2e

import "strings"

const (
	vmExtensionProvisioningErrorCode = "VMExtensionProvisioningError"
)

func isVMExtensionProvisioningError(err error) bool {
	return errorHasSubstring(err, vmExtensionProvisioningErrorCode)
}

func errorHasSubstring(err error, substring string) bool {
	return err != nil && strings.Contains(err.Error(), substring)
}
