# Title

**Author(s)**: @<Ganeshkumar Ashokavardhanan>

**EM**: @<Sylvain Boily>
**PM**: @<Sachi Desai> , @<Jack Jiang>

**PRD**: [Default AKS GPU Experience.docx](https://microsoft.sharepoint.com/:w:/r/teams/azurecontainercompute/_layouts/15/Doc.aspx?sourcedoc=%7BB781825F-280D-4787-985D-045133DA7192%7D&file=Default%20AKS%20GPU%20Experience.docx&action=default&mobileredirect=true)

**Design/UX**: [Managed GPU User Experience and Migration Plan.docx](https://microsoft-my.sharepoint.com/:w:/p/aganeshkumar/IQCBzMDS4VM0TqBEoCu0JY21AQOSYgrSNgqS9vTOHkw-UIk?e=QxRrlG)

## Brief description of why this change is needed

This allows to toggle a managed GPU experience (GPU metrics and device plugin) for all the Linux nodepools that use Nvidia GPUs in the cluster. Currently we have a VMSS tag at the nodepool level for enabling the managed GPU experience, however, we want to have the cluster level API instead for a more opinionated and managed experience and better error handling.

Additionally, to support managed Multi-Instance GPUs with managed GPU, we need to obtain the MIG strategy (single or mixed) at the node pool level.

### Future Foundation

This will serve as the foundation for the larger managed GPU experience in the future. Having per vendor specifications underneath the agent pool level will lead to maximum flexibility for node pools, while allowing us seamlessly introduce and segment new capabilities that will arrive as we onboard more GPU SKUs with chips from other vendors.

With `managementMode`, we also want to keep the door open to allowing components to be added/removed in the future (e.g. for the first iteration, we'll install DCGM and device-plugin. In v2, we might want to add in DRA instead of device-plugin).

- For cases where the components being switched around will introduce potentially breaking changes, we will only introduce/GA it in major K8s version updates.
- Each component will be a PRD in its own right, with their own preview/GA timelines.

## REST API proposal

```diff

  "ManagedClusterAgentPoolProfileProperties": {

    "GPUInstanceProfile": {
      "type": "string",
      "enum": [
        "MIG1g",
        "MIG2g",
        "MIG3g",
        "MIG4g",
        "MIG7g"
      ],
      "x-ms-enum": {
        "name": "GPUInstanceProfile ",
        "modelAsString": true
      },
      "description": "GPUInstanceProfile to be used to specify GPU MIG instance profile for supported GPU VM SKU."
    },
    "GPUProfile": {
      "type": "object",
      "description": "GPU settings for the Agent Pool",
      "properties": {
        "driver": {
          "type": "string",
          "description": "Whether to install the GPU drivers. When not specified, the default is Install.",
          "enum": [
            "Install",
            "None"
          ],
          "x-ms-enum": {
            "name": "GPUDriver",
            "modelAsString": true,
            "values": [
              {
                "value": "Install",
                "description": "Install the GPU driver."
              },
              {
                "value": "None",
                "description": "Skip GPU driver installation."
              }
            ]
          }
        },
+      },
+      "nvidia": {
+        "$ref": "#/definitions/NvidiaGPUProfile",
+        "description": "NVIDIA-specific GPU settings."
+      }
    },
+    "NvidiaGPUProfile": {
+       "type": "object",
+       "description": "NVIDIA-specific GPU settings.",
+       "properties": {

+         "managementMode": {
+           "type": "string",
+            "enum": [
+             "Unmanaged",
+             "Managed"
+           ],
+           "x-ms-enum": {
+             "name": "ManagementMode",
+             "modelAsString": true,
+             "values": [
+               {
+                 "value": "Unmanaged",
+                 "description": "Managed GPU experience is disabled for NVIDIA GPUs."
+               },
+               {
+                 "value": "Managed",
+                 "description": "Managed GPU experience is enabled for NVIDIA GPUs."
+               }
+             ]
+           },
+           "description": "The Managed GPU experience installs additional components, such as the DCGM metrics for monitoring, on top of the GPU driver for you. For more details of what is installed, check out aka.ms/aks/managed-gpu.",
+           "default": "Managed"
+          }
+        },

+         "migStrategy": {
+           "type": "string",
+           "description": "Sets the MIG (Multi-Instance GPU) strategy that will be used for managed MIG support. For more information about the different strategies, visit aka.ms/aks/managed-gpu. When not specified, the default is None.",
+           "enum":[
+             "None",
+             "Single",
+             "Mixed"
+           ],
+           "x-ms-enum": {
+             "name": "MIGStrategy",
+             "modelAsString": true,
+             "values": [
+               {
+                 "value": "None",
+                 "description": "Don't set a MIG strategy. If you previously had one set, this will override it and set remove the set MIG strategy."
+               },
+               {
+                 "value": "Single",
+                 "description": "Set the MIG strategy for managed MIG as single."
+               },
+               {
+                 "value": "Mixed",
+                 "description": "Set the MIG strategy for managed MIG as mixed."
+               }
+             ]
+           }
+         }

      }
    }
  }
}
```

If the proposal would add a new API path, make sure it's listed in the [available operations](https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=%2Fresourceprovider%2Fserver%2Fmicrosoft.com%2Fcontainerservice%2Fserver%2Foperations%2Favailableoperations.go&_a=contents&version=GBmaster).

### The two MIGs

We have options that deal with MIG configurations under both `GPUInstanceProfile` and `migStrategy`. One will not be replacing the other.

- The `GPUInstanceProfile` is an existing field that lets you specify MIG partitioning options for NVIDIA GPUs. When users set this today, they currently have to configure the MIG strategy via Helm or daemonset.
- The proposed `migStrategy` field will move the MIG strategy configuration into the AKS API so users don't have to deal with daemonsets themselves.

## Mixing the `driver install` and `managedGPU install` options

We see that we are now introducing both `gpuProfile.driver` and `gpuProfile/nvidia.managedGpu` fields. The former is to install GPU drivers the latter is GPU drivers and additional components on top.

If a user ends up toggling both of them, the proposed interaction is like so:

| | nvidia.managedGpu=managed | nvidia.managedGpu=unmanaged | nvidia-managedGpu undeclared |
| -- | -- | -- | -- |
| driver=install | Install everything (drivers + added components) | Install just GPU drivers | Install everything |
| driver=none | Block this | Do/install nothing | Block this |
| driver undeclared | Install everything | Block this | Install everything |

As a reminder, the current default behavior for `driver` is `Install`. The proposed default for `nvidia/ManagedGpu` is `Managed`.

## Versioning

We won't be introducing separate component versioning into the API for managed GPU at this time. Version control for the components will be dictated by AKS and documented appropriately. For users that have needs tied to specific versions, we allow unmanaged driver installation for them to pick/choose the driver versions that best suits their needs.

## Notes on validation

- Nodepool level fields for gpu metrics and gpu device plugin cannot be set

## Preview and user default exposure behavior

In a [Teams discussion](https://teams.microsoft.com/l/message/19:meeting_YTM2MzdkNDktMmYwYy00MTVlLTgzOWItMGJjZmFhOGVhNWU3@thread.v2/1768266309693?context=%7B%22contextType%22%3A%22chat%22%7D) with the wider team, the question of how we would roll this out will look like. As the intention is to eventually have this behavior be the default, care should be taken to avoid having this feature haphazardly flipped for users.

### ManagedGPUExperience and ManagedGPU

AKS has already introduced a VMSS/nodepool tag for [enabling the managed GPU experience](https://learn.microsoft.com/en-us/azure/aks/aks-managed-gpu-nodes?tabs=add-ubuntu-gpu-node-pool#register-the-managedgpuexperiencepreview-feature-flag-in-your-subscription) which is gated by the feature flag `ManagedGPUExperience`.

For the purposes of this doc for the preview API, we will consider `ManagedGPU` the flag of this "newer" experience.

### Preview Behavior

Once the `ManagedGPU` experience rolls out, we want to be able to reconcile the delta between the two flags mentioned above. We also want to make sure that, thinking forward, we have a smooth way to ensure that the API and default behavior is rolled out smoothly to customers.

| Item/Flag | Details | Intended Preview Behavior |
| -- | -- | -- |
| `ManagedGPUExperience` | The "old" feature flag that gated the VMSS tag for the [managed GPU experience on AKS](https://learn.microsoft.com/en-us/azure/aks/aks-managed-gpu-nodes?tabs=add-ubuntu-gpu-node-pool#register-the-managedgpuexperiencepreview-feature-flag-in-your-subscription). |  If the user solely has this flag toggled, there will be no effect to their cluster |
| [deprecate] `ManagedGPUExperience` + VMSS tag ON | If a user has both the feature flag enabled, and the correct VMSS tag for the managed GPU experience toggled on their AP. | This should enable the preview feature for the managed GPU experience on at the AP level. This is a behavior we'd like to deprecate in favor of the "new" API fields. |
| [new way] `ManagedGPUExperience` + API field controlled by the user (default to false) | User will have to toggle the AFEC flag and explicitly enable the corresponding API fields to have the preview for managed GPU enabled per AP. |

## GA

In hitting GA, the intention for this capability will be for it to be toggled by default, with the user being able to opt out if they please. This should be gated behind a K8s version being released.

As an example, if we hitch the GA to the release of K8s v1.36 on AKS, then the GA will happen alongside the GA of v1.36. Users that have applicable APs of v1.36 (and greater) will in turn have this feature flipped on by default.

Having a major K8s version serve as a boundary between having this feature on by default or not should also help avoid users getting unexpected surprises in the behavior. Such a type of change is not unexpected during major version upgrades.

## Best practices

See [AKS API best practices](https://dev.azure.com/msazure/CloudNativeCompute/_wiki/wikis/CloudNativeCompute.wiki/430963/AKS-API-Best-Practices).
**If your change is breaking**, make sure to also [engage with the ARG team](https://eng.ms/docs/cloud-ai-platform/azure-core/azure-management-and-platforms/control-plane-bburns/azure-resource-graph/azure-resource-graph/write-path/cris/api-version-management/cri-apiversionupgrade-for-partners) to ensure ARG updates to use the new API.

## CLI Proposal

```
az aks create --managed-gpu
az aks nodepool add --node-vm-size <Supported GPU SKU> (drivers installed even when `--gpu-driver` flag is not present)
az aks nodepool add --node-vm-size <GPU SKU> --mig-profile <MIG profile> --mig-strategy <single or mixed>
```

## Terraform Support

N/A since it's preview per note below in the template

- Notice: Currently we pause to support non-GA feature to [Official AKS Terraform provider](https://registry.terraform.io/providers/hashicorp/azurerm/latest). If you want to have TF entry for preview feature, you can add it to [Azure/azapi](https://registry.terraform.io/providers/Azure/azapi/latest) owned by Azure TF team.

## Conclusion

**Approvers**: At least 2 Eng and 1 PM **from AKS API Review alias (aksarb)**

| AKS ARB | Approval Status | Notes |
| -- | -- | -- |
| Matthew Christopher | Approved  | Please make sure to follow aka.ms/aks/api-best-practices when adding support for the enums in the API.  You should be able to follow the examples there. Please also make sure to have tests for the matrix above (driver=install|none|undeclared) |
| Liqian Luo | Approved on 12/01/2025 | Based on [Teams discussions](https://teams.microsoft.com/l/message/19:106ed06b2a3745b1a7ee5c573ab098c6@thread.skype/1762553398856?tenantId=72f988bf-86f1-41af-91ab-2d7cd011db47&groupId=e121dbfd-0ec1-40ea-8af5-26075f6a731b&parentMessageId=1762553398856&teamName=Azure%20Container%20Compute&channelName=AKS%20API%20channel&createdTime=1762553398856 "https://teams.microsoft.com/l/message/19:106ed06b2a3745b1a7ee5c573ab098c6@thread.skype/1762553398856?tenantId=72f988bf-86f1-41af-91ab-2d7cd011db47&groupId=e121dbfd-0ec1-40ea-8af5-26075f6a731b&parentMessageId=1762553398856&teamName=Azure%20Container%20Compute&channelName=AKS%20API%20channel&createdTime=1762553398856")) |
| Yi Zhang |   | |
| Shashank Barsin | Approved on 12/08/2025 |  |
| Ahmed Sabbour |  |  |
| Jorge Palma |  |  |
