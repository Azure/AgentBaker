package main

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

func create_produciton_vm(location, imageResourceID, vmName string) {
	// implement create_produciton_vm

	vmParameters := armcompute.VirtualMachine{
		Location: to.Ptr(location),
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
						ID: to.Ptr("<your-network-interface-id>"), // Replace with your network interface ID
					},
				},
			},
		},
	}
}
