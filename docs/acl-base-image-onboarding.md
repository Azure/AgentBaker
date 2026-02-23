# ACL Base Image — AgentBaker Integration Guide

This document explains how ACL base images flow through AgentBaker into AKS nodes. It covers how to identify which image AgentBaker uses, how to reproduce issues, how to validate fixes, and how to run the test pipeline.

---

## Table of Contents

1. [How ACL images become AKS nodes](#1-how-acl-images-become-aks-nodes)
2. [Which image is AgentBaker using?](#2-which-image-is-agentbaker-using)
3. [Reproducing issues](#3-reproducing-issues)
4. [Validating a fix](#4-validating-a-fix)
5. [Changing the ACL base image version in AgentBaker](#5-changing-the-acl-base-image-version-in-agentbaker)
6. [Running the AgentBaker pipeline](#6-running-the-agentbaker-pipeline)

---

## 1. How ACL images become AKS nodes

There are three stages between an ACL base image and a running AKS node. Understanding this pipeline clarifies where your changes take effect and how to test them.

```
┌──────────────────┐      ┌───────────────────────┐      ┌─────────────────────┐
│  ACL Base Image  │─────▶│  AgentBaker VHD Build  │─────▶│  AKS Node (live VM) │
│  (acl-scripts)   │      │  (Packer)              │      │  (CSE provisioning) │
└──────────────────┘      └───────────────────────┘      └─────────────────────┘
```

### Stage 1: ACL base image

Built from `acl-scripts`. The output is a raw ACL image uploaded to a Shared Image Gallery (SIG).

### Stage 2: AgentBaker VHD build (Packer)

AgentBaker's Packer build starts from the ACL base image and layers AKS-specific tooling on top:

1. **Applies Ignition config** — the Butane config at `vhdbuilder/packer/flatcar-customdata.yaml` (compiled to Ignition JSON) configures early-boot services for CSE file delivery.
2. **Runs provisioning scripts** — `pre-install-dependencies.sh` → `install-dependencies.sh` → `post-install-dependencies.sh` install kubelet, containerd configs, CIS hardening, etc.
3. **Runs VHD content tests** — `vhdbuilder/packer/test/linux-vhd-content-test.sh` validates the VHD (service health, file permissions, CIS compliance, etc.).
4. **Publishes VHD** — The resulting VHD is published to the `AKSFlatcar` gallery as image definitions `flatcargen2` (x86)

This stage may contain **workarounds** for missing base image functionality. As issues are fixed in the base image, these workarounds become removable.

### Stage 3: AKS node provisioning (CSE)

When AKS creates a new node:
1. The VHD from Stage 2 is used as the VM's OS disk.
2. A Custom Script Extension (CSE) runs bootstrap scripts that configure kubelet, networking, and cluster-specific settings.
3. On Flatcar, Ignition delivers CSE files via a tar archive extracted by `ignition-file-extract.service`.

---

## 2. Which image is AgentBaker using?

### ACL base image (input to the VHD build)

The ACL base image is specified in the Packer template. To find the current image:

**x86_64** — look at `vhdbuilder/packer/vhd-image-builder-flatcar.json`, variables section:

```json
"sig_source_subscription_id": "...",
"sig_source_resource_group": "...",
"sig_source_gallery_name": "...",
"sig_source_image_name": "...",
"sig_source_image_version": "..."
```

These five fields fully identify the SIG image used as the build base. The `sig_source_image_version` is the version of the ACL base image.

### How ACL is distinguished from vanilla Flatcar

AgentBaker reads `/etc/os-release` to detect the OS. ACL sets `ID=acl`, vanilla Flatcar sets `ID=flatcar`.

Helper functions in `parts/linux/cloud-init/artifacts/cse_helpers.sh`:
- `isFlatcar()` → returns true for **both** ACL and Flatcar (shared code paths)
- `isACL()` → returns true only for ACL (ACL-specific overrides)

The pipeline uses `OS_SKU=Flatcar` for both ACL and vanilla Flatcar. The distinction is in the **source image**, not the SKU.

---

## 3. Reproducing issues

Each task description also has a link to the AgentBaker error for more context.

### Option A: Boot a bare ACL VM

The most direct way to reproduce a base image issue is to boot a VM from the ACL base image directly (without AgentBaker layering).

### Option B: Revert a workaround and run the pipeline

To confirm that a specific AgentBaker workaround is covering for a base image issue, revert the workaround commit on your branch and run the VHD build or E2E pipeline. The resulting failure shows exactly what the base image is missing.

1. Identify the workaround commit (should be in the Task description).
2. Create a new branch off the ACL integration branch (e.g. `git checkout -b revert-test aadagarwal/acl-v20260127`).
3. `git revert <commit>` on that branch.
4. Run the pipeline ([Section 6](#6-running-the-agentbaker-pipeline)) from that branch.
5. The pipeline failure pinpoints which test or provisioning step depends on the workaround.

This is useful when you want to reproduce an issue without manually setting up a VM.

---

## 4. Validating a fix

Validation is two-stage: first confirm the fix works on a bare ACL image, then confirm AgentBaker's pipeline passes on top of it.

### Stage 1: Validate on a bare ACL VM

1. Build a new ACL image from your modified `acl-scripts`.
2. Upload the image to a SIG.
3. Boot a VM from the new image.
4. Verify the fix — check that the expected files/configs/services are present and working. Reboot and verify again to confirm persistence (especially for `/var` directories that depend on `tmpfiles.d`).

### Stage 2: Validate through AgentBaker

1. **Update the ACL base image version in AgentBaker** — see [Section 5](#5-changing-the-acl-base-image-version-in-agentbaker).
2. **Trigger the pipeline** — this builds the full AKS VHD from your new base image, runs VHD content tests, and then runs E2E tests. See [Section 6](#6-running-the-agentbaker-pipeline).

If your fix properly addresses the issue, any corresponding AgentBaker workaround should be safely removable. The typical workflow for confirming this:

1. Update the base image version in AgentBaker.
2. Remove the workaround code.
3. Run the pipeline.
4. If everything passes, the fix is validated end-to-end.

### Where AgentBaker workarounds live

ACL-specific workarounds in AgentBaker are concentrated in a few files. Search for `isACL`, `isFlatcar`, `OS_SKU.*Flatcar`, or `ACL` to find them:

| File | What kind of workarounds |
|---|---|
| `vhdbuilder/packer/pre-install-dependencies.sh` | Missing packages/files installed inline (udev rules, directories) |
| `vhdbuilder/packer/install-dependencies.sh` | Service disabling/overriding (iptables, resolved) |
| `vhdbuilder/scripts/linux/flatcar/tool_installs_flatcar.sh` | Service configuration overrides (chrony) |
| `parts/linux/cloud-init/flatcar.yml` | Ignition workarounds (explicit enable symlinks instead of presets) |
| `vhdbuilder/packer/test/linux-vhd-content-test.sh` | Test skips/adjustments for known base image limitations |
| `e2e/validators.go` | Failure allowlists for known-broken systemd units |

---

## 5. Changing the ACL base image version in AgentBaker

> **Before you start** — complete these prerequisites or the pipeline will fail:
>
> 1. **Get write access to the AgentBaker repo.**
>    You need write permissions to push a branch. If you don't have access, request it at https://repos.opensource.microsoft.com/orgs/Azure/teams/agentbakerwrite.
>
> 2. **Grant gallery access to the AgentBaker service principal.**
>    The service principal `nodesig-test-agent-identity` (subscription `c4c3550e-a965-4993-a50c-628fd38cd3e1`) needs the **Compute Gallery Image Reader** role on your gallery. Use [Elevating Permissions (PIM)](https://dev.azure.com/mariner-org/mariner/_wiki/wikis/mariner.wiki/6414/Elevating-Permissions-(PIM)) to assign it. Without this you get a 403 `AuthorizationFailed` error.
>
> 2. **Replicate your image to westus3.**
>    The pipeline builds in westus3. If your SIG image version is not replicated there you get a 400 `InvalidTemplateDeployment` error. Replicate via **Azure Compute Gallery → Image version → Update → Target regions**.

### x86_64

Edit `vhdbuilder/packer/vhd-image-builder-flatcar.json`, variables section (lines 24–28):

```json
"sig_source_subscription_id": "<subscription>",
"sig_source_resource_group": "<resource-group>",
"sig_source_gallery_name": "<gallery-name>",
"sig_source_image_name": "<image-definition>",
"sig_source_image_version": "<version>"
```

To update just the version: change `sig_source_image_version`. If the new image is in a different gallery, update the other four fields too.

---

## 6. Running the AgentBaker pipeline

The pipeline takes the ACL base image, produces an AKS-ready VHD, runs VHD content tests, and then runs E2E tests that validate the VHD can successfully bootstrap AKS nodes.

**Pipeline definition**: `.pipelines/.vsts-vhd-builder-release.yaml`

> If you cannot trigger the pipeline, you may need to request the [AKS Ext Cont entitlement](https://coreidentity.microsoft.com/manage/Entitlement/entitlement/tmaksextcont-jawg).

### How to trigger

1. Go to the pipeline in Azure DevOps - https://msazure.visualstudio.com/CloudNativeCompute/_build?definitionId=172842&_a=summary.
2. Click **Run pipeline**.
3. Select your AgentBaker branch (with the updated base image version).
4. Under parameters, enable `buildflatcargen2`. Disable other OS builds to save time.

### What happens during the build

1. **Packer provisions a VM** from the ACL base image using Ignition config.
2. **Provisioning scripts run** in sequence: `pre-install-dependencies.sh` → reboot → `install-dependencies.sh` → reboot → `post-install-dependencies.sh` → `cis.sh` → `cleanup-vhd.sh`.
3. **VHD content tests run** — a test VM is created from the built VHD and `linux-vhd-content-test.sh` executes on it via `az vm run-command invoke`. Failures appear as `testname:Error: message`.
4. **VHD is captured** and published to the SIG with tags including `buildId` and `branch`.
5. **E2E tests run** — for each scenario, the framework creates a VMSS with a single VM using the VHD, applies CSE and Ignition config to bootstrap the node, waits for kubelet to report `NodeReady`, and runs validators via SSH (through Azure Bastion) that check systemd unit health, file existence, service status, and more. Any unexpected systemd service failure causes a test failure.

### Build output

The build produces a SIG image version tagged with the pipeline's `buildId`.

### VHD content tests

VHD content tests run **inside the VHD after the Packer build** (step 3 above), not during E2E. They validate that the VHD has the expected files, services, and configurations before it's published.

**Test file**: `vhdbuilder/packer/test/linux-vhd-content-test.sh`

**Execution**: The pipeline creates a VM from the built VHD and runs the test script via `az vm run-command invoke`. Orchestrated by `vhdbuilder/packer/test/run-test.sh`. Failures are reported as `testname:Error: message` on stderr. Tests use a simple pass/fail pattern: call `err $test "message"` to report a failure.

### E2E scenarios

Defined in `e2e/scenario_test.go` (functions prefixed with `Test_Flatcar`):

| Scenario | What it tests |
|---|---|
| `Test_Flatcar` | Basic node bootstrap, validates essential files |
| `Test_Flatcar_Scriptless` | aks-node-controller self-contained mode |
| `Test_Flatcar_AzureCNI` | Azure CNI networking |
| `Test_Flatcar_AzureCNI_ChronyRestarts` | Chrony restart behavior |
| `Test_Flatcar_CustomCATrust` | Custom CA certificate injection |
| `Test_Flatcar_DisableSSH` | SSH disable feature |
| `Test_Flatcar_SecureTLSBootstrapping_BootstrapToken_Fallback` | TLS bootstrap fallback |

---
