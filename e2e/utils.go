package e2e_test

import (
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

func strToBoolMap(str string) map[string]bool {
	str = strings.ReplaceAll(str, " ", "")
	if str == "" {
		return nil
	}
	parts := strings.SplitN(str, ",", -1)
	m := make(map[string]bool, len(parts))
	for _, p := range parts {
		m[p] = true
	}
	return m
}

func getPrivateIP(res listVMSSVMNetworkInterfaceResult) (string, error) {
	if len(res.Value) > 0 {
		v := res.Value[0]
		if len(v.Properties.IPConfigurations) > 0 {
			ipconfig := v.Properties.IPConfigurations[0]
			return ipconfig.Properties.PrivateIPAddress, nil
		}
	}
	return "", fmt.Errorf("unable to extract private IP address from listVMSSNetworkInterfaceResult:\n%+v", res)
}

func getVMSSNICConfig(vmss *armcompute.VirtualMachineScaleSet) (*armcompute.VirtualMachineScaleSetNetworkConfiguration, error) {
	if vmss != nil && vmss.Properties != nil &&
		vmss.Properties.VirtualMachineProfile != nil && vmss.Properties.VirtualMachineProfile.NetworkProfile != nil {
		networkProfile := vmss.Properties.VirtualMachineProfile.NetworkProfile
		if len(networkProfile.NetworkInterfaceConfigurations) > 0 {
			return networkProfile.NetworkInterfaceConfigurations[0], nil
		}
	}
	return nil, fmt.Errorf("unable to extract vmss nic info, vmss model or vmss model properties were nil/empty:\n%+v", vmss)
}
