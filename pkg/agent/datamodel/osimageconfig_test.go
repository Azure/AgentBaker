// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOSImageConfigForCloudSettings(t *testing.T) {
	clouds := []string{
		AzureBleuCloud,
		AzureChinaCloud,
		AzureGermanCloud,
		AzureGermanyCloud,
		AzurePublicCloud,
		AzureUSGovernmentCloud,
		USNatCloud,
		USSecCloud,
	}
	for _, cloud := range clouds {
		assert.Contains(t, AzureCloudToOSImageMap, cloud)
		assert.NotEmpty(t, AzureCloudToOSImageMap[cloud])
	}
}
