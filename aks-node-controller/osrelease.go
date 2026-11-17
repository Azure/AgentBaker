package main

import "strings"

const (
	defaultOSReleasePath           = "/etc/os-release"
	osReleaseIDAzureContainerLinux = "azurecontainerlinux"
	osReleaseIDAzureLinux          = "azurelinux"
	osReleaseIDFlatcar             = "flatcar"
	osReleaseIDUbuntu              = "ubuntu"
	osVariantIDOSGuard             = "osguard"
)

func parseNodeCustomDataOSRelease(data []byte) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found {
			values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return values
}
