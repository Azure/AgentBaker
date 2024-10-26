package datamodel


const Nvidia470CudaDriverVersion = "cuda-470.82.01"

//nolint:gochecknoglobals
var (
	NvidiaCudaDriverVersion string
	NvidiaGridDriverVersion string
	AKSGPUCudaVersionSuffix string
	AKSGPUGridVersionSuffix string
)

type gpuVersion struct {
	RenovateTag   string `json:"renovateTag"`
	LatestVersion string `json:"latestVersion"`
}

type gpuContainerImage struct {
	DownloadURL string     `json:"downloadURL"`
	GPUVersion  gpuVersion `json:"gpuVersion"`
}

type componentsConfig struct {
	GPUContainerImages []gpuContainerImage `json:"GPUContainerImages"`
}

func LoadConfig() error {
	// Read the embedded components.json file
	data, err := parts.Templates.ReadFile("linux/cloud-init/artifacts/components.json")
	if err != nil {
		return fmt.Errorf("failed to read components.json: %w", err)
	}

	var config componentsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to unmarshal components.json: %w", err)
	}

	const driverIndex = 0
	const suffixIndex = 1
	const expectedLength = 2

	for _, image := range config.GPUContainerImages {
		parts := strings.Split(image.GPUVersion.LatestVersion, "-")
		if len(parts) != expectedLength {
			continue
		}
		version, suffix := parts[driverIndex], parts[suffixIndex]

		if strings.Contains(image.DownloadURL, "aks-gpu-cuda") {
			NvidiaCudaDriverVersion = version
			AKSGPUCudaVersionSuffix = suffix
		} else if strings.Contains(image.DownloadURL, "aks-gpu-grid") {
			NvidiaGridDriverVersion = version
			AKSGPUGridVersionSuffix = suffix
		}
	}
	return nil
}

//nolint:gochecknoinits
func init() {
	if err := LoadConfig(); err != nil {
		panic(fmt.Sprintf("Failed to load configuration: %v", err))
	}
}s