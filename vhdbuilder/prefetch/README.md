# Prefetch

Provides functionality to generate scripts used for prefetch optimization.

## ACL first-boot experiment

ACL remains excluded from release prefetch optimization. The opt-in
[`acl-experiment.json`](templates/acl-experiment.json) template tests a late
first-boot reset without changing that release gate. It omits the normal
customizer cleanup, preserving the baked cleanup script for the
[`acl-prefetch-driver.sh`](../scripts/acl-prefetch-driver.sh) workload hook.
The hook warms the baked workload, runs the existing VHD cleanup, verifies the
selected UKI's addon, restores the legacy first-boot marker, and flushes writes.
It deliberately fails when required scripts or first-boot state are missing.
The initial experiment targets UKI-based AMD64 ACL Trusted Launch images.
The experimental builder uses `Standard_D4s_v6`, matching the pilot nodes;
both D4s_v5 and D4ds_v5 were rejected by eastus2 capacity preflight.
The customizer only checks prerequisites: the AIB service rejects an empty
customizer list. Its boot can consume first-boot state, which the late workload
hook must restore; the final-image gate below remains mandatory.

Run the existing [optimizer](scripts/optimize.sh) with its normal required
environment and these overrides:

```bash
export IMAGE_BUILDER_TEMPLATE_PATH="$PWD/vhdbuilder/prefetch/templates/acl-experiment.json"
export PREFETCH_SOURCE_IMAGE_VERSION_ID="/subscriptions/.../resourceGroups/.../providers/Microsoft.Compute/galleries/.../images/.../versions/<exact-version>"
export WORKLOAD_OPTIMIZATION_SCRIPT_URL="https://raw.githubusercontent.com/Azure/AgentBaker/<commit-sha>/vhdbuilder/scripts/acl-prefetch-driver.sh"
```

Use an experiment-owned identity, resource groups, gallery and storage, not
release destinations. `PREFETCH_SOURCE_IMAGE_VERSION_ID` permits reading the
pinned source from another subscription while creating optimization resources
in `SUBSCRIPTION_ID`. The identity needs access to its source and destinations.
Keep the original source as the unoptimized control; the treatment must be
imported from AIB's final VHD output. Record both immutable image IDs and the
source build/commit. Never select `latest` for either arm.

**The hook's capture semantics are experimental.** A successful AIB run does not
prove that its disk mutations survive distribution. Before cluster performance
testing, boot the final output with unique Ignition CustomData, verify the
payload arrived and both first-boot triggers were consumed, then reboot and
verify that Ignition did not replay. Inspect the final VHD offline if it fails.
Do not re-enable the release gate until these checks pass. Use the same
source/AKS payload, architecture, security settings and disk type for both arms.