// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

var (
	Ubuntu1604OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "UbuntuServer",
		ImageSku:       "16.04-LTS",
		ImagePublisher: "Canonical",
		ImageVersion:   "latest",
	}

	Ubuntu1804OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "UbuntuServer",
		ImageSku:       "18.04-LTS",
		ImagePublisher: "Canonical",
		ImageVersion:   "latest",
	}

	Ubuntu1804Gen2OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "aks",
		ImageSku:       "aks-ubuntu-1804-gen2-2021-q3",
		ImagePublisher: "microsoft-aks",
		ImageVersion:   "2021.09.28",
	}

	RHELOSImageConfig = AzureOSImageConfig{
		ImageOffer:     "RHEL",
		ImageSku:       "7.3",
		ImagePublisher: "RedHat",
		ImageVersion:   "latest",
	}

	AKSUbuntu1604OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "aks",
		ImageSku:       "aks-ubuntu-1604-2021-q3",
		ImagePublisher: "microsoft-aks",
		ImageVersion:   "2021.09.28",
	}

	AKSUbuntu1804OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "aks",
		ImageSku:       "aks-ubuntu-1804-2021-q3",
		ImagePublisher: "microsoft-aks",
		ImageVersion:   "2021.09.28",
	}

	AKSWindowsServer2019OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "aks-windows",
		ImageSku:       "aks-2019-datacenter-core-smalldisk-2107",
		ImagePublisher: "microsoft-aks",
		ImageVersion:   "17763.2061.210714",
	}

	ACC1604OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "confidential-compute-preview",
		ImageSku:       "16.04-LTS",
		ImagePublisher: "Canonical",
		ImageVersion:   "latest",
	}

	AKSUbuntuContainerd1804OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "aks-aez",
		ImageSku:       "aks-ubuntu-containerd-1804-2021-q2",
		ImagePublisher: "microsoft-aks",
		ImageVersion:   "2021.04.27",
	}

	AKSUbuntuContainerd1804Gen2OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "aks-aez",
		ImageSku:       "aks-ubuntu-containerd-1804-gen2-2021-q2",
		ImagePublisher: "microsoft-aks",
		ImageVersion:   "2021.05.01",
	}

	AzureCloudToOSImageMap = map[string]map[Distro]AzureOSImageConfig{
		AzureChinaCloud: {
			Ubuntu:            Ubuntu1604OSImageConfig,
			Ubuntu1804:        Ubuntu1804OSImageConfig,
			Ubuntu1804Gen2:    Ubuntu1804Gen2OSImageConfig,
			RHEL:              RHELOSImageConfig,
			AKSUbuntu1604:     AKSUbuntu1604OSImageConfig,
			AKS1604Deprecated: AKSUbuntu1604OSImageConfig, // for back-compat
			AKSUbuntu1804:     AKSUbuntu1804OSImageConfig,
			AKS1804Deprecated: AKSUbuntu1804OSImageConfig, // for back-compat
			AKSWindows2019PIR: AKSWindowsServer2019OSImageConfig,
		},
		AzureGermanCloud: {
			Ubuntu:            Ubuntu1604OSImageConfig,
			Ubuntu1804:        Ubuntu1804OSImageConfig,
			Ubuntu1804Gen2:    Ubuntu1804Gen2OSImageConfig,
			RHEL:              RHELOSImageConfig,
			AKSUbuntu1604:     Ubuntu1604OSImageConfig,
			AKS1604Deprecated: Ubuntu1604OSImageConfig, // for back-compat
			AKSUbuntu1804:     Ubuntu1604OSImageConfig, // workaround for https://github.com/Azure/aks-engine/issues/761
			AKS1804Deprecated: Ubuntu1604OSImageConfig, // for back-compat
			AKSWindows2019PIR: AKSWindowsServer2019OSImageConfig,
		},
		AzureUSGovernmentCloud: {
			Ubuntu:            Ubuntu1604OSImageConfig,
			Ubuntu1804:        Ubuntu1804OSImageConfig,
			Ubuntu1804Gen2:    Ubuntu1804Gen2OSImageConfig,
			RHEL:              RHELOSImageConfig,
			AKSUbuntu1604:     AKSUbuntu1604OSImageConfig,
			AKS1604Deprecated: AKSUbuntu1604OSImageConfig, // for back-compat
			AKSUbuntu1804:     AKSUbuntu1804OSImageConfig,
			AKS1804Deprecated: AKSUbuntu1804OSImageConfig, // for back-compat
			AKSWindows2019PIR: AKSWindowsServer2019OSImageConfig,
		},
		AzurePublicCloud: {
			Ubuntu:                      Ubuntu1604OSImageConfig,
			Ubuntu1804:                  Ubuntu1804OSImageConfig,
			Ubuntu1804Gen2:              Ubuntu1804Gen2OSImageConfig,
			RHEL:                        RHELOSImageConfig,
			AKSUbuntu1604:               AKSUbuntu1604OSImageConfig,
			AKS1604Deprecated:           AKSUbuntu1604OSImageConfig, // for back-compat
			AKSUbuntu1804:               AKSUbuntu1804OSImageConfig,
			AKS1804Deprecated:           AKSUbuntu1804OSImageConfig, // for back-compat
			AKSUbuntuContainerd1804:     AKSUbuntuContainerd1804OSImageConfig,
			AKSUbuntuContainerd1804Gen2: AKSUbuntuContainerd1804Gen2OSImageConfig,
			AKSWindows2019PIR:           AKSWindowsServer2019OSImageConfig,
		},
	}
)
