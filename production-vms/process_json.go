package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type VHD struct {
	name       string
	resourceId string
	ImageArch  string
}

// takes the json from vhd-publishing-info in order to get the VM information that we want
func extractVHDInformation(jsonDir *string) ([]*VHD, error) {
	var vhdData []*VHD

	err := filepath.Walk(*jsonDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if filepath.Ext(path) == ".json" && strings.Contains(path, "vhd-publishing-info") {
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
			curVHD.name, err = generateVMName(curVHD.resourceId)
			if err != nil {
				return fmt.Errorf("failed to generate VM name: %w", err)
			}
			vhdData = append(vhdData, &curVHD)
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
returns - testVM-yyyy-mm-dd-AzureLinuxV2gen2-1.1730016408.31319
*/
func generateVMName(resourceID string) (string, error) {
	currentDate := time.Now().Format("2006-01-02")
	parts := strings.Split(resourceID, "/")
	
	parts_of_resource_id := 13
	if len(parts) < parts_of_resource_id {
		return "", fmt.Errorf("invalid resource ID: %s", resourceID)
	}
	imageName := parts[len(parts)-3]
	version := parts[len(parts)-1]
	vmName := fmt.Sprintf("testVM-%s-%s-%s", currentDate, imageName, version)
	return vmName, nil
}
