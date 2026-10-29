# Prefetch

Provides functionality to generate scripts used for prefetch optimization.

## ACL first-boot experiment

ACL remains excluded from release prefetch optimization. The opt-in
[`acl-experiment.json`](templates/acl-experiment.json) template rearms first boot
at customization capture without changing that release gate. Its native file
customizer stages the pinned
[`acl-prefetch-driver.sh`](../scripts/acl-prefetch-driver.sh), then invokes
`--prepare-only`: run the existing VHD cleanup, verify the selected UKI's addon,
restore the legacy first-boot marker, and flush writes. This leaves the baked
cleanup and workload scripts available for optimization.
The workload hook warms the baked workload and repeats the reset defensively.
Do not rely on workload-script disk mutations being captured.
It deliberately fails when required scripts or first-boot state are missing.
The initial experiment targets UKI-based AMD64 ACL Trusted Launch images.
The experimental builder uses `Standard_D4s_v6`, matching the pilot nodes;
both D4s_v5 and D4ds_v5 were rejected by eastus2 capacity preflight.
The AIB service rejects an empty customizer list. A live late-reset-only
prototype completed optimization but its distributed image had no first-boot
argument, did not receive fresh Ignition CustomData, and still contained both
scripts the workload hook was intended to remove. That is why restoration is
required in the customization capture path, not only in the workload hook.
The final-image gate below remains mandatory.

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

For the NVMe-only v6 builder, prepare a generalized gallery source from the
same exported VHD with `DiskControllerTypes=SCSI,NVMe` and
`SecurityType=TrustedLaunchSupported`. Point `PREFETCH_SOURCE_IMAGE_VERSION_ID`
at that exact version and set `ENABLE_TRUSTED_LAUNCH=False` **for this prepared
AIB source only**, so the optimizer uses `SharedImageVersion` rather than the
legacy TL-to-managed-image conversion. That conversion lost controller
metadata and failed v6 VM creation in the live experiment. The AIB source
allows a standard Gen2 build VM; this does not change the required Trusted
Launch/Secure Boot settings of either final benchmark image or node.
Keep the final control/treatment image definition `SecurityType=TrustedLaunch`.
The experimental template limits each image build to 60 minutes.

**The hook's capture semantics are experimental.** A successful AIB run does not
prove that its disk mutations survive distribution. Before cluster performance
testing, boot the final output with unique Ignition CustomData, verify the
payload arrived and both first-boot triggers were consumed, then reboot and
verify that Ignition did not replay. Inspect the final VHD offline if it fails.
Do not re-enable the release gate until these checks pass. Use the same
source/AKS payload, architecture, security settings and disk type for both arms.