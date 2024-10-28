package main

import (
	"context"
	"fmt"
	"production-vms/config"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

// create a main_test.go in order to test this function

func createProductionVM(ctx context.Context, imageResourceID, nicID, vmName string) error {
	fmt.Printf("Creating VM %s in resource group %s\n", vmName, config.ResourceGroupName)
	vmParameters := armcompute.VirtualMachine{
		Location: to.Ptr(config.Config.Location),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes("Standard_D8pds_v5")),
			},
			StorageProfile: &armcompute.StorageProfile{
				// Use the managed image reference
				ImageReference: &armcompute.ImageReference{
					ID: to.Ptr(imageResourceID),
				},
				OSDisk: &armcompute.OSDisk{
					Name:         to.Ptr("myVM-osdisk"),
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr(vmName),
				AdminUsername: to.Ptr("azureuser"),
				AdminPassword: to.Ptr("YourPassword123!"),
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: to.Ptr(nicID),
					},
				},
			},
		},
	}
	fmt.Printf("Finished creating the params, about to create the VM\n")

	pollerResp, err := config.Azure.VirtualMachinesClient.BeginCreateOrUpdate(ctx, config.ResourceGroupName, vmName, vmParameters, nil)
	if err != nil {
		return fmt.Errorf("cannot create VM: %v", err)
	}
	_, err = pollerResp.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("cannot get the VM to create or update due to: %v", err)
	}

	fmt.Printf("VM %s created successfully\n", vmName)
	return nil
}

func createResourceGroup(ctx context.Context) error {
	resourceGroupParams := armresources.ResourceGroup{
		Location: &config.Config.Location,
	}
	_, err := config.Azure.ResourceGroupsClient.CreateOrUpdate(ctx, config.ResourceGroupName, resourceGroupParams, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group: %v", err)
	}

	return nil
}

func createVnet(ctx context.Context, vnetName string) error {
	fmt.Printf("Checking if VNet %s exists in resource group %s\n", vnetName, config.ResourceGroupName)
	_, err := config.Azure.VNetClient.Get(ctx, config.ResourceGroupName, vnetName, nil)
	if err == nil {
		fmt.Printf("VNet %s already exists in resource group %s\n", vnetName, config.ResourceGroupName)
		return nil
	}

	fmt.Printf("Creating VNet %s in resource group %s\n", vnetName, config.ResourceGroupName)
	vnetParams := armnetwork.VirtualNetwork{
		Location: &config.Config.Location,
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{
					to.Ptr("10.0.0.0/16"), // Adjust your address space as needed
				},
			},
		},
	}
	vnetPoller, err := config.Azure.VNetClient.BeginCreateOrUpdate(ctx, config.ResourceGroupName, vnetName, vnetParams, nil)
	if err != nil {
		return fmt.Errorf("failed to begin VNet creation: %v", err)
	}
	vnetResp, err := vnetPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create VNet: %v", err)
	}
	fmt.Printf("VNet %s created in resource group %s\n", *vnetResp.ID, config.ResourceGroupName)
	return nil
}

func createSubnet(ctx context.Context, vnetName, subnetName string) (string, error) {
	fmt.Printf("Checking if subnet %s exists in VNet %s\n", subnetName, vnetName)
	subnetResp, err := config.Azure.SubNetClient.Get(ctx, config.ResourceGroupName, vnetName, subnetName, nil)
	if err == nil {
		fmt.Printf("Subnet %s already exists in VNet %s\n", subnetName, vnetName)
		return *subnetResp.ID, nil
	}

	fmt.Printf("Creating subnet %s in VNet %s\n", subnetName, vnetName)
	subnetParams := armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr("10.0.1.0/24"), // Adjust your subnet prefix as needed
		},
	}

	subnetPoller, err := config.Azure.SubNetClient.BeginCreateOrUpdate(ctx, config.ResourceGroupName, vnetName, subnetName, subnetParams, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin subnet creation: %v", err)
	}
	subResp, err := subnetPoller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create subnet: %v", err)
	}
	fmt.Printf("Subnet %s created successfully\n", subnetName)
	return *subResp.ID, nil
}

func createNetworkInterface(ctx context.Context, nicName, subnetID string) (string, error) {
	fmt.Printf("Creating NIC %s in subnet %s\n", nicName, subnetID)
	nicParams := armnetwork.Interface{
		Location: &config.Config.Location,
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet: &armnetwork.Subnet{ID: &subnetID},
					},
				},
			},
		},
	}
	poller, err := config.Azure.NetworkInterfacesClient.BeginCreateOrUpdate(ctx, config.ResourceGroupName, nicName, nicParams, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin NIC creation: %v", err)
	}
	nicResp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create NIC: %v", err)
	}
	fmt.Printf("NIC %s created successfully\n", nicName)
	return *nicResp.ID, nil
}
