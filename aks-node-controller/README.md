# AKS Node Controller

This directory contains files related to AKS Node Controller go binary.

## Overview

AKS Node Controller is a go binary that is responsible for bootstrapping AKS nodes. The controller has two primary functions: 1. Parse the bootstrap config and kickstart bootstrapping and 2. Monitor the completion status. 

## Usage

AKS Node Controller currently relies on two Azure mechanisms to bootstrap the node: [`Custom Script Extension (CSE)`](https://learn.microsoft.com/en-us/azure/virtual-machines/extensions/custom-script-linux) and [`Custom Data`](https://learn.microsoft.com/en-us/azure/virtual-machines/custom-data}). The bootstrapper should use `GetNodeBootstrappingForScriptless` which takes the bootstrapping config of type `AKSNodeConfig` as input and returns the corresponding `CustomData` and `CSE`. For guidance on populating the config, please refer to this [doc](https://github.com/Azure/AgentBaker/tree/dev/pkg/proto/aksnodeconfig/v1).

1. Custom Data: Contains bootstrap configuration of type [`AKS Node Config`](https://github.com/Azure/AgentBaker/tree/dev/pkg/proto/aksnodeconfig/v1) in json format which is placed on the node through cloud-init.

2. CSE: Script used to poll bootstrap status and return exit status once complete. 

```go
builder := aksnodeconfigv1.NewNBContractBuilder()
builder.ApplyConfiguration(aksNodeConfig)
nodeBootstrapping, err = builder.GetNodeBootstrapping()

model := armcompute.VirtualMachineScaleSet{
    Properties: &armcompute.VirtualMachineScaleSetProperties{
        VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
            OSProfile: &armcompute.VirtualMachineScaleSetOSProfile{
                CustomData:         &nodeBootstrapping.CustomData,
                ...
            }
        },
        VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
            Extensions: []*armcompute.VirtualMachineScaleSetExtension{
                {
                    Name: to.Ptr("vmssCSE"),
                    Properties: &armcompute.VirtualMachineScaleSetExtensionProperties{
                        Publisher:               to.Ptr("Microsoft.Azure.Extensions"),
                        Type:                    to.Ptr("CustomScript"),
                        TypeHandlerVersion:      to.Ptr("2.0"),
                        AutoUpgradeMinorVersion: to.Ptr(true),
                        Settings:                map[string]interface{}{},
                        ProtectedSettings: map[string]interface{}{
                            "commandToExecute": nodeBootstrapping.CSE,
                        },
                    },
                },
            }
        },
        ...
    }
}
```

### Extracting Provision Status

The provision status can be extracted from the CSE response. CSE takes the stdout from the bootstrap scripts which contains information in the form `datamodel.CSEStatus`. You can find an example of how to parse the output [here](https://github.com/Azure/AgentBaker/blob/dev/e2e/scenario_helpers_test.go#L163).


### Provisioning Flow

The binary is triggered by a systemd unit, `aks-node-controller.service` which runs on the node. This systemd unit waits for the bootstrapping config to be placed on the node through customdata and then runs the go binary to start the bootstrapping process.

1. aks-node-controller.service: systemd unit that is triggered once cloud-init is complete (guaranteeing that config is present on disk) and then kickstarts bootstrapping.
2. aks-node-controller binary: two modes
        - provision: parses the node config and triggers bootstrap process
        - provision-wait: waits for provision.complete to be present and reads provision.json which is returned by CSE through capturing stdout