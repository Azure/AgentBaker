package scenario

// These SIG image versions are stored in the ACS test subscription, guarded by resource deletion locks
var DefaultImageVersionIDs = map[string]string{
	"ubuntu1804":         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/1804Gen2/versions/1.1699020623.27601",
	"ubuntu2204":         "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2/versions/1.1699020621.21398",
	"marinerv2":          "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2/versions/1.1699020635.14957",
	"azurelinuxv2":       "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/AzureLinuxV2Gen2/versions/1.1694137195.23451",
	"ubuntu2204-arm64":   "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/2204Gen2Arm64/versions/1.1687293304.23474",
	"marinerv2-arm64":    "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/CBLMarinerV2Gen2Arm64/versions/1.1687293288.20788",
	"azurelinuxv2-arm64": "/subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/AzureLinuxV2Gen2Arm64/versions/1.1694137200.8668",
}
