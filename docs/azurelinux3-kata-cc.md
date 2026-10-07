# Azure Linux 3 Kata-CC integration (draft)

## Shared AgentBaker / AKS RP contract

| Field | Value |
| --- | --- |
| Distro constant | `AKSAzureLinuxV3Gen2KataCC` |
| Persisted distro | `aks-azurelinux-v3-gen2-kata-cc` |
| Gallery / definition | `AKSAzureLinux/V3kataccgen2` |
| Gallery resource group | `AKS-AzureLinux` (existing Azure Linux gallery configuration) |
| Release build artifact | `azurelinuxv3-gen2-kata-cc` |
| Build OS_VERSION / generation | `V3katacc` / `V2` |
| RP migration toggle | `enable-azurelinux-v3-kata-cc`, default **false** |

The identity is distinct from ordinary AZL3 Kata (`V3katagen2`) and frozen
AZL2 Kata-CC (`V2katagen2`). It uses the existing Azure Linux gallery subscription
selection and Linux image version machinery; no unpublished version is invented.

RP permits selection for new eligible Kata-CC pools and Kubernetes-version
upgrades only. Ordinary updates/scale and dedicated node-image-only upgrades
retain the persisted family. Pools already on AZL3 remain there after toggle
rollback. Existing KataCcIsolationPreview access and nested-SNP requirements remain.

## Integration blockers before building or activation

The opt-in release job is deliberately blocked before building. Bootstrap also
returns an explicit error for this new identity rather than emitting the old
cloud-hypervisor configuration. These guards are integration TODOs, not a working
OpenVMM implementation. Existing jobs and distros keep their behavior.

1. Confirm the signed producer artifact contract with the UVM/signing owners
   (build pipeline 417429, signing pipeline 318279). Do not infer the new file
   layout from existing artifact display names or assume the old COSE/IGVM layout.
2. Replace the job guard with a download pinned to an explicit successful signed
   producer run. Carry matching host package versions from that producer to avoid
   independently resolving latest guest and host inputs.
3. Wire packer staging and `packer_source.sh`, install the matching host packages,
   and update all containerd template variants and scriptless runtime generation
   for the verified OpenVMM configuration/shim contract. Remove the bootstrap
   guard only with rendering tests and a real signed-image provisioning test.
4. Build, publish and replicate `V3kataccgen2`. The definition is staged in the
   existing `GetMaintainedLinuxSIGImageConfigMap` definitions-to-skip list. Remove
   **only this entry** after first publication and verify replication before
   enabling RP selection. The RP toggle does not bypass release verification.
5. Release AgentBakerSvc with the mapping and bootstrap implementation. Update
   the local AgentBaker dependency in RP/sharedlib/nodeprovisioner consumers and
   validate both remote and vendored bootstrap paths before rollout. A remote
   image mapping alone does not update local classification/runtime code.
6. Run signed VHD/E2E validation, including ordinary Kata regression, CC workloads,
   scale after toggle rollback, Kubernetes upgrades and image-only upgrades.

The check-in VHD pipeline does not build this image until those inputs exist.
No final signed payload, publication, E2E success or production enablement is
claimed by this draft.
