package amdgpu

#AMDGPUDriver: {
	repositoryURL:          string & =~"^https://repo[.]radeon[.]com/amdgpu/[0-9.]+/ubuntu$"
	signingKeyURL:          string & =~"^https://repo[.]radeon[.]com/[^?[:space:]]+$"
	signingKeyFingerprint:  string & =~"^[A-F0-9]{40}$"
	distribution:           "noble"
	component:              "main"
	packageVersion:         string & =~"^[0-9]+:[0-9][0-9A-Za-z.+~-]+$"
	firmwarePackageVersion: string & =~"^[0-9]+:[0-9][0-9A-Za-z.+~-]+$"
	moduleVersion:          string & =~"^[0-9][0-9.]+$"
	dkmsVersion:            string & =~"^[0-9][0-9A-Za-z.+~-]+$"
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
}

#Components: {
	AMDGPUDriver:      #AMDGPUDriver
	AMDGPUDiagnostics: #AMDGPUDiagnostics
}

#Components
