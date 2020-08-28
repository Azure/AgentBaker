// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

//AzureEnvironmentSpecConfig is the overall configuration differences in different cloud environments.
type AzureEnvironmentSpecConfig struct {
	CloudName            string                        `json:"cloudName,omitempty"`
	DockerSpecConfig     DockerSpecConfig              `json:"dockerSpecConfig,omitempty"`
	KubernetesSpecConfig KubernetesSpecConfig          `json:"kubernetesSpecConfig,omitempty"`
	EndpointConfig       AzureEndpointConfig           `json:"endpointConfig,omitempty"`
	OSImageConfig        map[Distro]AzureOSImageConfig `json:"osImageConfig,omitempty"`
}

//DockerSpecConfig is the configurations of docker
type DockerSpecConfig struct {
	DockerEngineRepo         string `json:"dockerEngineRepo,omitempty"`
	DockerComposeDownloadURL string `json:"dockerComposeDownloadURL,omitempty"`
}

//KubernetesSpecConfig is the kubernetes container images used.
type KubernetesSpecConfig struct {
	AzureTelemetryPID                    string `json:"azureTelemetryPID,omitempty"`
	KubernetesImageBase                  string `json:"kubernetesImageBase,omitempty"`
	TillerImageBase                      string `json:"tillerImageBase,omitempty"`
	ACIConnectorImageBase                string `json:"aciConnectorImageBase,omitempty"`
	MCRKubernetesImageBase               string `json:"mcrKubernetesImageBase,omitempty"`
	NVIDIAImageBase                      string `json:"nvidiaImageBase,omitempty"`
	AzureCNIImageBase                    string `json:"azureCNIImageBase,omitempty"`
	CalicoImageBase                      string `json:"CalicoImageBase,omitempty"`
	EtcdDownloadURLBase                  string `json:"etcdDownloadURLBase,omitempty"`
	KubeBinariesSASURLBase               string `json:"kubeBinariesSASURLBase,omitempty"`
	WindowsTelemetryGUID                 string `json:"windowsTelemetryGUID,omitempty"`
	CNIPluginsDownloadURL                string `json:"cniPluginsDownloadURL,omitempty"`
	VnetCNILinuxPluginsDownloadURL       string `json:"vnetCNILinuxPluginsDownloadURL,omitempty"`
	VnetCNIWindowsPluginsDownloadURL     string `json:"vnetCNIWindowsPluginsDownloadURL,omitempty"`
	ContainerdDownloadURLBase            string `json:"containerdDownloadURLBase,omitempty"`
	CSIProxyDownloadURL                  string `json:"csiProxyDownloadURL,omitempty"`
	WindowsProvisioningScriptsPackageURL string `json:"windowsProvisioningScriptsPackageURL,omitempty"`
	WindowsPauseImageURL                 string `json:"windowsPauseImageURL,omitempty"`
	AlwaysPullWindowsPauseImage          bool   `json:"alwaysPullWindowsPauseImage,omitempty"`
}

//AzureEndpointConfig describes an Azure endpoint
type AzureEndpointConfig struct {
	ResourceManagerVMDNSSuffix string `json:"resourceManagerVMDNSSuffix,omitempty"`
}

//AzureOSImageConfig describes an Azure OS image
type AzureOSImageConfig struct {
	ImageOffer     string `json:"imageOffer,omitempty"`
	ImageSku       string `json:"imageSku,omitempty"`
	ImagePublisher string `json:"imagePublisher,omitempty"`
	ImageVersion   string `json:"imageVersion,omitempty"`
}

// AzureTelemetryPID represents the current telemetry ID
// See more information here https://docs.microsoft.com/en-us/azure/marketplace/azure-partner-customer-usage-attribution
// PID is maintained to keep consistent with Azure Stack Telemetry Terminologies
type AzureTelemetryPID string

const (
	// DefaultAzureStackDeployTelemetryPID tracking ID for Deployment
	DefaultAzureStackDeployTelemetryPID = "pid-1bda96ec-adf4-4eea-bb9a-8462de5475c0"
	// DefaultAzureStackScaleTelemetryPID tracking ID for Scale
	DefaultAzureStackScaleTelemetryPID = "pid-bbbafa53-d6a7-4022-84a2-86fcbaec7030"
	// DefaultAzureStackUpgradeTelemetryPID tracking ID for Upgrade
	DefaultAzureStackUpgradeTelemetryPID = "pid-0d9b5198-7cd7-4252-a890-5658eaf874be"
)

var (
	//DefaultKubernetesSpecConfig is the default Docker image source of Kubernetes
	DefaultKubernetesSpecConfig = KubernetesSpecConfig{
		KubernetesImageBase:                  "k8s.gcr.io/",
		TillerImageBase:                      "gcr.io/kubernetes-helm/",
		ACIConnectorImageBase:                "microsoft/",
		NVIDIAImageBase:                      "nvidia/",
		CalicoImageBase:                      "calico/",
		AzureCNIImageBase:                    "mcr.microsoft.com/containernetworking/",
		MCRKubernetesImageBase:               "mcr.microsoft.com/",
		EtcdDownloadURLBase:                  "mcr.microsoft.com/oss/etcd-io/",
		KubeBinariesSASURLBase:               "https://acs-mirror.azureedge.net/kubernetes/",
		WindowsTelemetryGUID:                 "fb801154-36b9-41bc-89c2-f4d4f05472b0",
		CNIPluginsDownloadURL:                "https://acs-mirror.azureedge.net/cni/cni-plugins-amd64-" + CNIPluginVer + ".tgz",
		VnetCNILinuxPluginsDownloadURL:       "https://acs-mirror.azureedge.net/azure-cni/" + AzureCniPluginVerLinux + "/binaries/azure-vnet-cni-linux-amd64-" + AzureCniPluginVerLinux + ".tgz",
		VnetCNIWindowsPluginsDownloadURL:     "https://acs-mirror.azureedge.net/azure-cni/" + AzureCniPluginVerWindows + "/binaries/azure-vnet-cni-singletenancy-windows-amd64-" + AzureCniPluginVerWindows + ".zip",
		ContainerdDownloadURLBase:            "https://storage.googleapis.com/cri-containerd-release/",
		CSIProxyDownloadURL:                  "https://acs-mirror.azureedge.net/csi-proxy/v0.1.0/binaries/csi-proxy.tar.gz",
		WindowsProvisioningScriptsPackageURL: "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-" + DefaultWindowsProvisioningScriptsPackageVersion + ".zip",
		WindowsPauseImageURL:                 "mcr.microsoft.com/oss/kubernetes/pause:" + WindowsPauseImageVersion,
		AlwaysPullWindowsPauseImage:          DefaultAlwaysPullWindowsPauseImage,
	}

	//DefaultDockerSpecConfig is the default Docker engine repo.
	DefaultDockerSpecConfig = DockerSpecConfig{
		DockerEngineRepo:         "https://aptdocker.azureedge.net/repo",
		DockerComposeDownloadURL: "https://github.com/docker/compose/releases/download",
	}

	// AKSWindowsServer2019OSImageConfig is the AKS image based on Windows Server 2019
	AKSWindowsServer2019OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "aks-windows",
		ImageSku:       "2019-datacenter-core-smalldisk-2007",
		ImagePublisher: "microsoft-aks",
		ImageVersion:   "17763.1339.200717",
	}

	// WindowsServer2019OSImageConfig is the 'vanilla' Windows Server 2019 image
	WindowsServer2019OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "WindowsServer",
		ImageSku:       "2019-Datacenter-Core-with-Containers-smalldisk",
		ImagePublisher: "MicrosoftWindowsServer",
		ImageVersion:   "17763.1339.2007101755",
	}
)
