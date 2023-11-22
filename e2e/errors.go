package e2e_test

import "strings"

const (
	vmExtensionProvisioningErrorCode = "VMExtensionProvisioningError"
	resourceNotFoundErrorCode        = "ResourceNotFound"
	notFoundErrorCode                = "404 Not Found"
)

func isVMExtensionProvisioningError(err error) bool {
	return errorHasSubstring(err, vmExtensionProvisioningErrorCode)
}

func isResourceNotFoundError(err error) bool {
	return errorHasSubstring(err, resourceNotFoundErrorCode)
}

func isNotFoundError(err error) bool {
	return errorHasSubstring(err, notFoundErrorCode)
}

func errorHasSubstring(err error, substring string) bool {
	return err != nil && strings.Contains(err.Error(), substring)
}
