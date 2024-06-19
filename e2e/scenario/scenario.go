package scenario

// This function is called internally by the scenario package to get each e2e scenario's respective config as one long slice.
// To add a sceneario, implement a new method on the Template type in a separate file that returns a *Scenario and add
// its return value to the slice returned by this function.
func AllScenarios() []*Scenario {
	return []*Scenario{
		// block for ubuntu 1804
		ubuntu1804(),
		ubuntu1804gpu(),
		ubuntu1804_azurecni(),
		ubuntu1804gpu_azurecni(),
		ubuntu1804ChronyRestarts(),

		// block for ubuntu 2204
		ubuntu2204(),
		ubuntu2204ARM64(),
		ubuntu2204CustomSysctls(),
		ubuntu2204Wasm(),
		ubuntu2204gpuNoDriver(),
		ubuntu2204CustomCATrust(),
		ubuntu2204ArtifactStreaming(),
		ubuntu2204privatekubepkg(),
		ubuntu2204AirGap(),
		ubuntu2204ContainerdURL(),
		ubuntu2204ContainerdVersion(),
		ubuntu2204ChronyRestarts(),

		// block for mariner v2
		marinerv2(),
		marinerv2ARM64(),
		marinerv2gpu(),
		marinerv2CustomSysctls(),
		marinerv2Wasm(),
		marinerv2_azurecni(),
		marinerv2gpu_azurecni(),
		marinerv2AirGap(),
		marinerv2ARM64AirGap(),
		marinerv2ChronyRestarts(),

		// block for azurelinux v2
		azurelinuxv2(),
		azurelinuxv2ARM64(),
		azurelinuxv2gpu(),
		azurelinuxv2CustomSysctls(),
		azurelinuxv2Wasm(),
		azurelinuxv2_azurecni(),
		azurelinuxv2gpu_azurecni(),
		azurelinuxv2ARM64AirGap(),
		azurelinuxv2AirGap(),
		azurelinuxv2ChronyRestarts(),

		// block for gpu scenarios
		ubuntu2204gpu("ubuntu2204-gpu-ncv3", "Standard_NC6s_v3"),
		ubuntu2204gpu("ubuntu2204-gpu-a100", "Standard_NC24ads_A100_v4"),
		ubuntu2204gpu("ubuntu2204-gpu-a10", "Standard_NV6ads_A10_v5"),
	}
}
