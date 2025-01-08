# Introduction of Windows E2E

## Summary

Windows E2E is used to validate the latest change of windows cse package automatically. We currently support three
versions of windows images: windows 2019 containerd, windows 2022 containerd gen1 and windows 2022 containerd gen2,
windows 23H2 gen1 and windows 23H2 gen2.

## Code Path

```bash
└── .pipelines
    ├── e2e-windows.yaml
    └── templates
        └── e2e-windows-template.yaml
```

- **e2e-windows.yaml**. It is the yaml file used by the Azure pipeline. We can set the parameters that are needed in
  templates/e2e-windows-template.yaml here. By the way, pipeline variables are mostly set in variable
  group `ab-e2e-windows` and only KUBERNETES_VERSION is set on pipeline variables to manually run the pipeline with
  specified kubernetes version.

- **templates/e2e-windows-template.yaml**. It is the basic unit yaml file of pipeline execution. Three jobs are defined
  in this file: Setup_Test_Cluster, Generate_Matrix and Test.
    - Setup_Test_Cluster is used to create test cluster and add windows nodepool. This step will be much faster if test
      resources are already created.
    - Generate_Matrix is used to obtain parameters from matrix.json file. Since only windows e2e uses it now, we can
      specify the parameters in the code and this step will be deprecated in the future.
    - Test is used to deploy a new windows vmss with the latest cse package and try to join the node to the test
      cluster. We also upload the cse logs collected from the deployed vmss to the artifacts of the test pipeline.

```bash
└── windows
    ├── README.md
    ├── e2e-create-windows-nodepool.sh
    ├── e2e-helper.sh
    ├── e2e-scenario.sh
    ├── e2e-starter.sh
    ├── e2e_test.go
    ├── matrix.json
    ├── nodebootstrapping_template.json
    ├── percluster_template.json
    ├── pod-windows-template.yaml
    ├── scenarios
    │   └── windows
    │       └── property-windows-template.json
    ├── upload-cse-logs.ps1
    └── windows_vmss_template.json
```

- **README.md**. It is the brief introduction of Windows AgentBaker E2E.
- **e2e-create-windows-nodepool.sh**. It is the bash file used to add a test windows nodepool to the test cluster.
- **e2e-helper.sh**. It is the help file that provides functions for other bash files.
- **e2e-scenario.sh**. It is the bash file used to deploy a test vmss with the lastest windows cse package. If the
  deployment succeeds, we will continue to deploy a test pod to validate whether the node is successfully joined to the
  test cluster.
- **e2e-starter.sh**. It is the bash file used to create the test cluster. After the cluster is first created, we will
  upload some files(i.e., certificate files) in the linux nodepool to the storage account. In this way, all the
  pipelines that use the same cluster to do the test could download the linux files and obtain needed parameters from
  them.
- **e2e_test.go**. This file is used to generate CustomData file and CSEcmd file for further deployment of test vmss.
- **matrix.json**. This json file is used to set parameters of SCENARIO_NAME and VM_SKU. Since VM_SKU is set in other
  code now, we can delete this file after setting SCENARIO_NAME in the code in the future.
- **nodebootstrapping_template.json**. It is the first part of template used to generate windows CustomData file and
  CSECmd file and it contains the basic properties for the cluster and linux nodepool.
- **percluster_template.json**. It is the second part of template used to generate windows CustomData file and CSECmd
  file and it contains properties that should be obtained from pipeline variables and linux files uploaded to the
  storage account.
- **pod-windows-template.yaml**. This pod yaml file is used to deploy a test pod to validate whether the node is already
  joined to the test cluster.
- **scenarios/windows/property-windows-template.json**. It is the third and last part of template used to generate
  windows CustomData file and CSECmd file and it contains properties that are related to the test windows nodepool.
- **upload-cse-logs.ps1**. This script is used to upload generated cse logs to the stroage account in the deployed
  windows vmss for further download.
- **windows_vmss_template.json**. This json file is the basic template used to deploy a windows vmss. We add necessary
  properties to this file in `e2e-scenario.sh`.

## Generate and Update current windows test images

Currently, we use the latest test images `/subscriptions/$AZURE_E2E_IMAGE_SUBSCRIPTION_ID/resourceGroups/$AZURE_E2E_IMAGE_RESOURCE_GROUP_NAME/providers/Microsoft.Compute/galleries/$AZURE_E2E_IMAGE_GALLERY_NAME/images/windows-$WINDOWS_E2E_IMAGE/versions/latest`.

supported versions of `$WINDOWS_E2E_IMAGE` are

- 2019-containerd, 
- 2022-containerd, 
- 2022-containerd-gen2, 
- 23H2, 
- 23H2-gen2

as is referred by `IMAGE_REFERENCE` in `e2e-scenario.sh`.

To generate new windows test images, we can
use [`[TEST All VHDs] AKS Windows VHD Build - Msft Tenant`](https://msazure.visualstudio.com/CloudNativeCompute/_build?definitionId=210712&_a=summary).
Before running the pipeline, we need to set the values of pipeline variables as follows:

- `SIG_GALLERY_NAME` is AKSWindows
- `SIG_IMAGE_NAME_PREFIX` is windows-e2e-test
- `SIG_IMAGE_VERSION` is the latest date (e.g., 2023.02.07)

## Generate basic template for vmss deployment

We generate basic template for vmss deployment by removing node-related properties from the example template of an
existing vmss and replacing existing vmss name with our target vmss name.

To obtain an example template, there are two ways:

- **From Azure Portal**:
  Enter MC resource group -> Enter the existing vmss -> Click `Export template` on the left bar. Then you can see the
  example template.
- **From az cli**:
  ```
  VMSS_RESOURCE_Id=$(az resource show --resource-group $MC_RESOURCE_GROUP_NAME \
  --name $MC_WIN_VMSS_NAME  \
  --resource-type Microsoft.Compute/virtualMachineScaleSets \
  --query id --output tsv)
  
  az group export --resource-group $MC_RESOURCE_GROUP_NAME  \ 
  --resource-ids $VMSS_RESOURCE_Id \
  --include-parameter-default-value > template.json
  ```