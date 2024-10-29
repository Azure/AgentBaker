package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/Azure/agentbaker/production-vms/config"
)

// to run locally
// 		production-vms % go run . --json-dir testdata

type Options struct {
	JsonDir string
}

func parseFlags() Options {
	var opts Options
	flag.StringVar(&opts.JsonDir, "json-dir", "", "Directory containing JSON files")
	flag.Parse()
	return opts
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), config.Config.TestTimeout)
	defer cancel()

	opts := parseFlags()
	if opts.JsonDir == "" {
		log.Fatalf("missing required flag: json-dir")
	}

	vhdData, err := extractVHDInformation(&opts.JsonDir)
	if err != nil {
		log.Fatalf("failed to extract all VHD Data: %v", err)
	}
	for _, vhd := range vhdData {
		log.Printf("VHD: %s\n", *vhd)
	}

	subnetID, err := setUpAzureResources(ctx)
	if err != nil {
		log.Fatalf("failed to set up azure resources: %v", err)
	}

	for _, vhd := range vhdData {
		err = createProductionVM(ctx, vhd, subnetID)
		if err != nil {
			log.Fatalf("failed to create production VM: %v", err)
		}
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
	return subNetID, nil
}
