package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type VHD struct {
	name       string
	resourceId string
	ImageArch  string
}

func extractVHDInformation(jsonDir *string) ([]VHD, error) {
	var vhdData []VHD

	err := filepath.Walk(*jsonDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(path) == ".json" {
			fmt.Println("Found JSON file:", path)

			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer file.Close()

			var data map[string]interface{}
			if err := json.NewDecoder(file).Decode(&data); err != nil {
				return fmt.Errorf("failed to decode JSON: %w", err)
			}

			curVHD := VHD{}
			if vhdID, ok := data["captured_sig_resource_id"].(string); ok {
				curVHD.resourceId = vhdID
			}
			if imageArch, ok := data["image_architecture"].(string); ok {
				curVHD.ImageArch = imageArch
			}
			curVHD.name = generateVMName(curVHD.resourceId)
			vhdData = append(vhdData, curVHD)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return vhdData, nil
}

/*
takes - /subscriptions/8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8/resourceGroups/aksvhdtestbuildrg/providers/Microsoft.Compute/galleries/PackerSigGalleryEastUS/images/AzureLinuxV2gen2/versions/1.1730016408.31319
returns - testVM-AzureLinuxV2gen2-1.1730016408.31319
*/
func generateVMName(resourceID string) string {
	// todo(alburgess) - also put the date in the name for readability 
	parts := strings.Split(resourceID, "/")
	imageName := ""
	version := ""
	for i, part := range parts {
		if part == "images" && i+2 < len(parts) {
			imageName = parts[i+1] // Image name is the element after "images"
			version = parts[i+3]   // Version is the element after "versions"
			break
		}
	}
	vmName := fmt.Sprintf("testVM-%s-%s", imageName, version)
	return vmName
}
