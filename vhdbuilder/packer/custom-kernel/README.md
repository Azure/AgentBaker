# Testing a custom Ubuntu kernel

This is a test-only workflow for Ubuntu 26.04 Minimal AMD64 Gen2 VHDs. It
supports a matching `linux-image-<release>` (or
`linux-image-unsigned-<release>`) and `linux-modules-<release>` package pair,
plus optional `linux-modules-extra-<release>` and matching versioned header
packages. All packages must have the same version and use the `*-azure`
kernel flavor. Image/modules packages must be for `amd64`; common headers
may use architecture `all`. ARM64, FIPS, Trusted Launch/Secure Boot, CVM and
production publishing are intentionally not supported.

The feature is off by default. Keep this commit on an experiment branch or
cherry-pick it onto a future AgentBaker branch. Do not promote the resulting
VHD to production: it holds the test packages and pins GRUB to the requested
kernel, which prevents normal kernel updates from becoming the default.

## 1. Prepare private storage and permissions

Use an approved test storage account. Do not enable public access, use account
keys, or put SAS tokens in pipeline variables. The download runs on the
pipeline agent using the existing `VHD_ARM_SERVICE_CONNECTION`; the Packer VM
receives the files over SSH and needs no additional storage permissions.

Example variables (replace these with your approved resources):

```bash
SUBSCRIPTION='<test-subscription-id>'
ACCOUNT='<existing-test-storage-account>'
CONTAINER='custom-kernel-tests'
PREFIX='my-experiment/ubuntu2604-amd64/run-001'
DEB_DIR='/absolute/path/to/canonical-packages'
```

Prerequisites: Bash, Azure CLI signed into the correct tenant, `dpkg-deb`,
and `sha256sum`. Obtain the packages and patch confirmation directly from the
kernel provider. Checksums below preserve the uploaded bytes; they do not
prove that a package contains the requested fix.

The normal Ubuntu dependency step requires `linux-headers-<release>`.
If this is not available in Ubuntu's configured repositories, request that
package and its versioned common-header dependencies from the kernel provider
and put them in the same directory. The installer checks availability before
installing and does not skip the normal header requirement.

The uploader needs **Storage Blob Data Contributor**. To create a container,
it needs that permission at storage-account scope (or have an administrator
create the container first). Azure resource Contributor alone does not grant
blob data access.

An authorized administrator can pre-create the private container and grant the
pipeline service connection's **service principal object ID** reader access:

```bash
az storage container create --subscription "$SUBSCRIPTION" \
  --account-name "$ACCOUNT" --name "$CONTAINER" \
  --public-access off --auth-mode login

ACCOUNT_ID=$(az storage account show --subscription "$SUBSCRIPTION" \
  --name "$ACCOUNT" --query id -o tsv)
CONTAINER_SCOPE="$ACCOUNT_ID/blobServices/default/containers/$CONTAINER"
PIPELINE_PRINCIPAL_OBJECT_ID='<service-connection-principal-object-id>'

az role assignment create --subscription "$SUBSCRIPTION" \
  --assignee-object-id "$PIPELINE_PRINCIPAL_OBJECT_ID" \
  --assignee-principal-type ServicePrincipal \
  --role 'Storage Blob Data Reader' --scope "$CONTAINER_SCOPE"
```

Find that principal through Azure DevOps **Project settings > Service
connections**, using the connection named by `VHD_ARM_SERVICE_CONNECTION`.
Do not confuse its application/client ID with its service principal object
ID, or grant access to the Packer VM's managed identity instead. Role assignment
requires separate RBAC-administration permissions; the upload script does not
grant roles. Allow time for RBAC propagation.

Storage firewalls/private endpoints must permit the pipeline agent's network.
A reader role does not bypass network restrictions. A subscription name or
number alone is not a download endpoint.

## 2. Upload a versioned package set

From the AgentBaker repository root:

```bash
bash vhdbuilder/packer/custom-kernel/manage.sh upload \
  "$SUBSCRIPTION" "$ACCOUNT" "$CONTAINER" "$PREFIX" "$DEB_DIR"
```

The helper validates the package metadata before accessing Azure, creates a
private container if necessary, rejects an existing public container, and uploads
only the `.deb` files plus a `kernel-release` file and `SHA256SUMS`. Files such as Windows
`:Zone.Identifier` sidecars are not uploaded.

Existing blobs are never overwritten. Use a fresh prefix for every attempt,
including after a partial upload; do not mutate a prefix while a build is
using it. The checksum manifest is uploaded last. Keep the printed checksums
with the experiment results.

## 3. Run only the target VHD build

Commit/push the experiment branch to `Azure/AgentBaker` when ready, then select
it in **Run pipeline** for your test Linux VHD build definition.

| Parameter | Value |
| --- | --- |
| `build2604minimalgen2containerd` | `true` |
| All other `build*` parameters | `false` |
| `customKernelStorageAccount` | `$ACCOUNT` |
| `customKernelContainer` | `$CONTAINER` |
| `customKernelBlobPrefix` | `$PREFIX` |
| `useOverrides` | `false`, unless separately needed |

Use `ENVIRONMENT=test`, `GENERATE_PUBLISHING_INFO=False`, and `DRY_RUN=False`
(a dry run deletes the output image). Keep normal content tests, scanning,
kernel validation and temporary-resource cleanup enabled. The custom-kernel
parameters affect only the Ubuntu 26.04 Minimal AMD64 job; leaving other
builds selected still spends resources building those other images.

Custom builds use a separate `${SKU_NAME}-customkernel` gallery image
definition, so they do not advance the normal image definition's latest version.
Different experiments can still share this test-only definition; always record
and consume the exact output version.

The helper rejects production/publishing, ARM64, FIPS, Trusted Launch and
CVM configurations before downloading anything. A nonempty blob prefix enables
the feature; an empty prefix preserves normal kernel behavior. Downloads use
the build's `SUBSCRIPTION_ID`; use a storage account in that subscription.
If a reused agent checkout contains an old ignored
`vhdbuilder/packer/custom-kernel/packages/` directory, clean that generated
directory before retrying; the helper refuses to reuse it.

The pipeline downloads and validates the package set, then Packer:

1. Completes the normal distribution upgrade and kernel replacement.
2. Installs the custom packages with dependency resolution and `--reinstall`
   (including when their version already exists), marks them manual/held,
   and pins the GRUB entry.
3. Uses the existing reboot and fails unless `uname -r` matches the requested
   kernel. This happens before the main dependency-install step.
4. Verifies the running kernel again after the second reboot and post-install
   cleanup. `CUSTOM_KERNEL_VERIFIED=<release>` and the checksums are recorded
   in `/opt/azure/vhd-install.complete` and the downloaded release notes.

Unsigned images require Secure Boot to remain disabled. The host-side installer
also rejects an enabled Secure Boot state reported by `mokutil`.

## 4. Validate the fix, not just the image build

Record the exact output Compute Gallery image-version resource ID from the
build. Use that image explicitly in your AgentBaker E2E configuration instead
of a floating "latest" reference. Normal Linux cleanup retains successful
non-dry-run gallery outputs initially, but the test environment's retention
cleanup can remove older images.

On every test VM, confirm `uname -r` and package versions and retain the full
kernel log. Run the affected scenario repeatedly on fresh VMs and compare with
an unpatched baseline. A green build or ready node alone does not prove that a
kernel warning has been fixed. Do not suppress stack-trace validation.

After the experiment, delete only its exact blob prefix and test image version
through your approved cleanup process; do not delete the shared storage account
or unrelated prefixes. Emptying the prefix parameter disables custom kernels
in future builds, but does not unpin kernels in already-created test VHDs.
Consume a validated, supported fix through the normal release process instead.

## Local checks

These commands use mocked Azure/package-install operations and do not upload
packages, install a kernel or queue a build:

```bash
shellcheck vhdbuilder/packer/custom-kernel/manage.sh \
  spec/vhdbuilder/packer/custom_kernel_spec.sh
shellspec --shell bash spec/vhdbuilder/packer/custom_kernel_spec.sh
```
