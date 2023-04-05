package main

import "strings"

const (
	vmExtensionProvisioningErrorCode = "VMExtensionProvisioningError"
	resourceNotFoundErrorCode        = "ResourceNotFound"
)

func isVMExtensionProvisioningError(err error) bool {
	return errorHasSubstring(err, vmExtensionProvisioningErrorCode)
}

func isResourceNotFoundError(err error) bool {
	return errorHasSubstring(err, resourceNotFoundErrorCode)
}

func errorHasSubstring(err error, substring string) bool {
	return err != nil && strings.Contains(err.Error(), substring)
}
