// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import "log"

const (
	// CustomDataMaxBase64Bytes is the maximum size of CustomData in base64-encoded bytes
	// imposed by the Azure VMSS/VM API (64 KB raw → 87380 base64 characters).
	CustomDataMaxBase64Bytes = 87380
)

// customDataContext holds identifying information for log messages.
type customDataContext struct {
	SubscriptionID string
	ResourceGroup  string
	VMSSName       string
	PoolName       string
}

// recordCustomDataSize logs an error when the generated CustomData exceeds
// the VMSS maximum.
func recordCustomDataSize(customData string, osType string, distro string, ctx customDataContext) {
	size := len(customData)
	if size > CustomDataMaxBase64Bytes {
		log.Printf("ERROR: CustomData exceeds VMSS limit: %d/%d (%.1f%%) os=%s distro=%s subscription=%s resource_group=%s vmss=%s pool=%s",
			size, CustomDataMaxBase64Bytes, float64(size)/float64(CustomDataMaxBase64Bytes)*100, osType, distro,
			ctx.SubscriptionID, ctx.ResourceGroup, ctx.VMSSName, ctx.PoolName)
	}
}
