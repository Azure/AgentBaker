package config

import "strings"

const OSReleasePath = "/etc/os-release"

// os-release ID and VARIANT_ID values that appear in more than one classification path.
const (
	OSReleaseIDAzureContainerLinux = "azurecontainerlinux"
	OSReleaseIDAzureLinux          = "azurelinux"
	OSReleaseIDFlatcar             = "flatcar"
	OSReleaseIDUbuntu              = "ubuntu"
	OSVariantIDOSGuard             = "osguard"
)

// ParseOSRelease parses os-release KEY=value lines. It ignores comments and lines without "=",
// and removes quotes around values.
func ParseOSRelease(data []byte) map[string]string {
	values := make(map[string]string)
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return values
}
