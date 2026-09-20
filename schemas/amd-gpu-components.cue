package amdgpu

import (
	"regexp"
	"strings"
)

#AMDGPUDriver: {
	repositoryURL:          string & =~"^https://repo[.]radeon[.]com/amdgpu/[0-9]+([.][0-9]+){1,3}/ubuntu$"
	signingKeyURL:          string & =~"^https://repo[.]radeon[.]com/[^?[:space:]]+$"
	signingKeyFingerprint:  string & =~"^[A-F0-9]{40}$"
	distribution:           "noble"
	component:              "main"
	packageVersion:         string & =~"^[0-9]+:[0-9]+[.][0-9]+[.][0-9]+[.][0-9]+-[0-9]+[.]24[.]04$"
	firmwarePackageVersion: string & =~"^[0-9]+:[0-9][0-9A-Za-z.+~-]+$"
	moduleVersion:          string & =~"^[0-9][0-9.]+$"
	dkmsVersion:            string & =~"^[0-9][0-9A-Za-z.+~-]+$"

	// These fields describe one driver release, not independent update targets.
	// Reject partial Renovate proposals until a reviewer supplies coherent pins.
	_package:               regexp.FindSubmatch("^([0-9]+):([0-9]+[.][0-9]+[.][0-9]+)[.]([0-9]+)-([0-9]+[.]24[.]04)$", packageVersion)
	_release:               strings.TrimSuffix(strings.TrimPrefix(repositoryURL, "https://repo.radeon.com/amdgpu/"), "/ubuntu")
	_releaseParts:          strings.Split(_release, ".")
	_firmwareRelease:       _release + strings.Repeat(".0", 4-len(_releaseParts))
	moduleVersion:          "\(_package[2]).\(_package[3])"
	dkmsVersion:            "\(_package[2])-\(_package[4])"
	firmwarePackageVersion: "\(_package[1]):\(_firmwareRelease).\(_package[3])-\(_package[4])"
}

#AMDGPUDiagnostics: {
	repositoryURL:         "https://stable.repo.amd.com/rocm/core/packages/ubuntu2404/"
	signingKeyURL:         "https://stable.repo.amd.com/rocm/gpg/packages.gpg"
	signingKeyFingerprint: string & =~"^[A-F0-9]{40}$"
	distribution:          "stable"
	component:             "main"
	amdsmiPackage:         string & =~"^amdrocm-amdsmi[0-9]+[.][0-9]+$"
	amdsmiVersion:         string & =~"^[0-9][0-9A-Za-z.+~-]+$"
	sysdepsPackage:        string & =~"^amdrocm-sysdeps[0-9]+[.][0-9]+$"
	sysdepsVersion:        string & =~"^[0-9][0-9A-Za-z.+~-]+$"
	cliPath:               string & =~"^/opt/rocm/core-[0-9]+[.][0-9]+/bin/amd-smi$"

	// Keep package names, versions and the installed CLI in the same ROCm line.
	_core:          strings.TrimPrefix(amdsmiPackage, "amdrocm-amdsmi")
	sysdepsPackage: "amdrocm-sysdeps\(_core)"
	amdsmiVersion:  =~"^\(regexp.QuoteMeta(_core))[.]"
	sysdepsVersion: =~"^\(regexp.QuoteMeta(_core))[.]"
	cliPath:        "/opt/rocm/core-\(_core)/bin/amd-smi"
}

#Components: {
	AMDGPUDriver:      #AMDGPUDriver
	AMDGPUDiagnostics: #AMDGPUDiagnostics
}

#Components
