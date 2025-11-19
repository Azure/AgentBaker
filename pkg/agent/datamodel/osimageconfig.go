// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

//nolint:gochecknoglobals
var (
	AKSUbuntuContainerd2204Gen2OSImageConfig = AzureOSImageConfig{
		ImageOffer:     "aks",
		ImageSku:       "aks-ubuntu-containerd-22.04-gen2",
		ImagePublisher: "microsoft-aks",
		ImageVersion:   "2025.06.02",
	}

	AzureCloudToOSImageMap = map[string]map[Distro]AzureOSImageConfig{
		AzureChinaCloud: {
			AKSUbuntuContainerd2204Gen2: AKSUbuntuContainerd2204Gen2OSImageConfig,
		},
		AzureGermanCloud: {
			AKSUbuntuContainerd2204Gen2: AKSUbuntuContainerd2204Gen2OSImageConfig,
		},
		AzureUSGovernmentCloud: {
			AKSUbuntuContainerd2204Gen2: AKSUbuntuContainerd2204Gen2OSImageConfig,
		},
		AzurePublicCloud: {
			AKSUbuntuContainerd2204Gen2: AKSUbuntuContainerd2204Gen2OSImageConfig,
		},
		AzureGermanyCloud: {
			Ubuntu: AKSUbuntuContainerd2204Gen2OSImageConfig,
		},
		USNatCloud: {
			AKSUbuntuContainerd2204Gen2: AKSUbuntuContainerd2204Gen2OSImageConfig,
		},
		USSecCloud: {
			AKSUbuntuContainerd2204Gen2: AKSUbuntuContainerd2204Gen2OSImageConfig,
		},
	}
)
