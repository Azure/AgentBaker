# Prefetch

Provides functionality to generate scripts used for prefetch optimization.

## ACL release support

ACL image builds must restore first-time provisioning after build-time boots.
Otherwise a new VM can skip Ignition, the service that installs its initial
configuration, and miss files needed for AKS provisioning.

The release pipeline selects [`acl.json`](templates/acl.json) and a
commit-pinned [`acl-prefetch-driver.sh`](../scripts/acl-prefetch-driver.sh) for
non-FIPS AMD64 Gen2 Trusted Launch ACL images when `RUN_PREFETCH_OPTIMIZATION`
is enabled. ARM64, FIPS, CVM and imagecustomizer builds remain excluded until
their final-image validation is complete. Other operating systems retain their
existing template and workload URL.

**Dependency and merge order:** first build and publish an ACL base image with
`acl-image-prepare` from `coreos-init` 0.0.1-5 or later, then update the normal
ACL base-image pin and validate the AgentBaker release build. AIB fails explicitly
if this command is absent; it does not silently skip preparation or fall back
to the previous prototype. No new base-image version is invented by this change.

### Supported Image Builder integration

Microsoft documents
[custom Linux deprovisioning](https://learn.microsoft.com/en-us/azure/virtual-machines/linux/image-builder-json#overriding-the-commands):
create `/tmp/DeprovisioningScript.sh` during customization. Image Builder runs
it at the end of customization to generalize the image for reuse.

The ACL template stages the pinned driver and installs that override. The
deprovisioning path:

1. Checks that the ACL-owned `acl-image-prepare` command is installed.
2. Runs the existing AgentBaker cleanup.
3. Calls `acl-image-prepare` to restore first-time provisioning.
4. Preserves normal Azure deprovisioning with `waagent -force -deprovision+user`.
5. Removes the cleanup/staged driver scripts and flushes writes.

ACL owns selection and restoration of its signed boot assets. The command does
not erase the OS, remove preloaded containers, or replace `waagent`. It must not
be run on normal customer reboots.

The [2025-10-01 API schema](https://github.com/Azure/azure-rest-api-specs/blob/main/specification/imagebuilder/resource-manager/Microsoft.VirtualMachineImages/ImageBuilder/stable/2025-10-01/imagebuilder.json)
exposes boot and workload optimization, but no separate post-optimization
finalizer. The documented workload script contract does not guarantee that
filesystem edits persist in the distributed image. Generalization is therefore
responsible for correctness; the workload hook is responsible only for profiling.

### Workload profiling

Both boot and workload optimization remain enabled. The workload hook starts
containerd and executes the generated prefetch script, which reads selected
container files and runs the existing CSE preloader. It does not rerun cleanup
or change first-boot state.

The generated workload script remains available in the captured image because
the profiling VM needs it. It is not automatically executed on customer boot.
Keeping this small script does not disable optimization or leave cleanup
unexecuted. Its presence is intentional; its removal is not used as evidence
that profiling succeeded. The cleanup helper is removed during deprovisioning,
where changes are part of image preparation.

### Release validation

Test the final distributed image, not just the temporary build VM:

- Boot with unique Ignition CustomData and verify the supplied payload arrived.
- Verify that the first-boot marker and signed addon were consumed.
- Reboot and verify a new boot ID without repeating first-time setup.
- Verify real AKS scriptless provisioning and the exact installed image version.
- Compare cluster operation timings against the same source without optimization.

The ACL repository supplies first-boot/reboot acceptance tests. The initial
three-pair pilot verified the previous capture-preparation implementation on
AMD64 Trusted Launch. It does **not** validate the new packaged helper or the
deprovisioning override. Those require a new base image and final-image tests
before merging the release integration.

The pilot favored prefetch for create, scale-out and image-update paired medians,
but start was slower in all three pairs, including one late-registering node.
That remains an investigation and release-readiness item, not an established
ACL or prefetch root cause. Three pairs do not support p95 or significance claims.

### Reproduction and source images

The [optimizer](scripts/optimize.sh) retains optional
`IMAGE_BUILDER_TEMPLATE_PATH` and `PREFETCH_SOURCE_IMAGE_VERSION_ID` overrides
for targeted validation. Use immutable source/output image versions and a new
output name for each changed candidate; an existing output can be reused by the
optimizer's idempotency check.

The standard release path handles its normal source conversion. A separately
prepared NVMe-capable gallery source was needed for the D4s_v6 pilot because
the legacy TL-to-managed-image conversion lost controller metadata. That
pilot-specific hardware and source workaround are not release template defaults.
Do not disable final-node Trusted Launch or Secure Boot to work around an image
builder limitation.

### Earlier prototype

An earlier attempt restored first-boot state only inside the workload hook.
Although AIB reported success, the distributed image lacked that restoration.
Moving preparation into customization produced a working artifact. This proves
the late-only attempt was insufficient, not why the service discarded or failed
to apply its changes. The release design uses AIB's documented deprovisioning
override rather than depending on undocumented profiling-disk behavior.
