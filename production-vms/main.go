package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"production-vms/config"
)

// to run locally
// 		production-vms % go run . --json-dir test-jsons

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), config.Config.TestTimeout)
	defer cancel()

	jsonDir := flag.String("json-dir", "", "Directory containing JSON files")
	flag.Parse()
	if *jsonDir == "" {
		fmt.Println("No JSON directory provided.")
		return
	}
	vhdIDs, err := extract_all_vhd_ids(jsonDir)
	if err != nil {
		log.Fatalf("failed to extract all VHD IDs: %v", err)
	}
	fmt.Printf("Found %d VHD IDs: %s\n", len(vhdIDs), vhdIDs)

	nicID, err := setUpAzureResources(ctx)
	if err != nil {
		log.Fatalf("failed to set up azure resources: %v", err)
	}

	err = createProductionVM(ctx, vhdIDs[0], nicID, "alison-test-production-vm")
	if err != nil {
		log.Fatalf("failed to create production VM: %v", err)
	}
}

func setUpAzureResources(ctx context.Context) (string, error) {
	if err := createResourceGroup(ctx); err != nil {
		return "", fmt.Errorf("failed to create resource group: %v", err)
	}

	vnetName := "alison-test-vnet"
	if err := createVnet(ctx, vnetName); err != nil {
		return "", fmt.Errorf("failed to create virtual network: %v", err)
	}

	subnetName := "alison-test-subnet"
	subNetID, err := createSubnet(ctx, vnetName, subnetName)
	if err != nil {
		return "", fmt.Errorf("failed to create subnet: %v", err)
	}

	nicName := "alison-test-nic"
	nicID, err := createNetworkInterface(ctx, nicName, subNetID)
	if err != nil {
		return "", fmt.Errorf("failed to create network interface: %v", err)
	}
	return nicID, nil
}
