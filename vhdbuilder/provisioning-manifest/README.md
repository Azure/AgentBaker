# Provisioning Script Hotfix System

This directory contains the tooling for publishing **hotfixed provisioning scripts** as OCI artifacts. Nodes autonomously detect and pull hotfixes at provisioning time.

## Overview

When a critical bug is discovered in provisioning scripts baked into VHDs, this system allows operators to publish corrected scripts without waiting for the next weekly VHD release. Nodes detect hotfixes via a lightweight registry query during CSE execution.

### How It Works

1. **VHD Build**: Each VHD is stamped with the AgentBaker commit SHA in `/opt/azure/containers/.provisioning-scripts-version`
2. **Hotfix Publish**: An operator builds and pushes corrected scripts as an OCI artifact tagged `<baked-version>-hotfix`
3. **Node Detection**: At provisioning time, `check_for_script_hotfix()` in `cse_start.sh` checks the registry for a matching hotfix tag
4. **Overlay**: If found, the tarball is extracted over the baked scripts before `provision.sh` runs
5. **Fallback**: Any failure is non-fatal — nodes always proceed with baked scripts

## Files

| File | Purpose |
|------|---------|
| `manifest.json` | SKU → script inventory mapping (source paths → VHD destinations) |
| `build-hotfix-oci.sh` | Operator script to build and push hotfix OCI artifacts |
| `README.md` | This document |

## Publishing a Hotfix

### Prerequisites

- `oras` CLI installed ([oras.land](https://oras.land/))
- `jq` installed
- Authenticated to the target registry (`az acr login --name <acr>`)
- The fix committed on a branch checked out from the affected version's tag

### Step-by-Step

1. **Identify affected versions**: Determine which baked VHD versions contain the bug. Check the version stamp format (currently git commit SHA).

2. **Prepare the fix**: Check out the affected version's tag and apply your fix:
   ```bash
   git checkout <affected-version-tag>
   git cherry-pick <fix-commit>
   ```

3. **Build and push** the hotfix:
   ```bash
   ./build-hotfix-oci.sh \
     --sku ubuntu-2204 \
     --affected-version <baked-version> \
     --description "Fix for <issue description>" \
     --files "parts/linux/cloud-init/artifacts/cse_helpers.sh"
   ```

4. **Verify** the artifact was pushed:
   ```bash
   oras repo tags abe2eprivatenonanonwestus3.azurecr.io/aks/provisioning-scripts/ubuntu-2204
   ```

5. **Test** by provisioning a node with the affected VHD version. Check `/var/log/azure/hotfix-check.log` for detection logs and `/opt/azure/containers/.hotfix-applied` for the applied marker.

### Dry Run

Build the artifact locally without pushing:
```bash
./build-hotfix-oci.sh \
  --sku ubuntu-2204 \
  --affected-version <version> \
  --description "Test hotfix" \
  --files "parts/linux/cloud-init/artifacts/cse_helpers.sh" \
  --dry-run
```

## Updating an Existing Hotfix

To add more files to an existing hotfix, re-run `build-hotfix-oci.sh` with **all** affected files. The same tag is overwritten:

```bash
./build-hotfix-oci.sh \
  --sku ubuntu-2204 \
  --affected-version <version> \
  --description "Updated fix: added provision_installs.sh" \
  --files "parts/linux/cloud-init/artifacts/cse_helpers.sh,parts/linux/cloud-init/artifacts/cse_install.sh"
```

## Multi-Version Bug Workflow

If the same bug affects multiple VHD versions, publish a separate hotfix for each:

```bash
# For each affected version
for version in 202601.01.0 202602.10.0; do
  git checkout "${version}" && git cherry-pick <fix-commit>
  ./build-hotfix-oci.sh \
    --sku ubuntu-2204 \
    --affected-version "${version}" \
    --description "Fix CVE-XXXX" \
    --files "parts/linux/cloud-init/artifacts/cse_helpers.sh"
done
```

## Retiring a Hotfix

When all affected VHDs have aged out of the 6-month support window:

1. **Delete the tag** from the source ACR:
   ```bash
   az acr repository delete \
     --name abe2eprivatenonanonwestus3 \
     --image aks/provisioning-scripts/ubuntu-2204:<version>-hotfix
   ```

2. Nodes will stop seeing the tag — no action needed on the node side.

## New VHD Releases

When a new weekly VHD ships with the fix natively:
- The new VHD has a different version stamp
- `check_for_script_hotfix()` won't find a `<new-version>-hotfix` tag
- **No changes needed** — the hotfix coexists harmlessly

## Debugging

### Logs

- **Hotfix check log**: `/var/log/azure/hotfix-check.log`
- **Applied marker**: `/opt/azure/containers/.hotfix-applied`
- **Version stamp**: `/opt/azure/containers/.provisioning-scripts-version`

### Common Issues

| Symptom | Cause | Resolution |
|---------|-------|------------|
| "ORAS not available" | Old VHD without ORAS binary | Expected — no hotfix support on old VHDs |
| "no version stamp" | VHD built before version stamping | Expected — no hotfix support |
| "no hotfix tag found" | No hotfix published for this version | Normal case — no action needed |
| "pull failed" | Registry unreachable or auth issue | Non-fatal — baked scripts used |
| "metadata version mismatch" | Tag points to wrong artifact | Re-push with correct `--affected-version` |

## Emergency: skipValidation

In the ADO pipeline, the `skipValidation` parameter skips e2e tests and publishes directly. Use **only** when:
- The fix is time-critical (active production outage)
- The fix is trivially correct (e.g., a typo fix)
- A human has reviewed the change and accepts the risk

Document the justification in the pipeline run notes.

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `HOTFIX_REGISTRY` | `abe2eprivatenonanonwestus3.azurecr.io` | Override the registry for testing |

## Manifest Schema

The `manifest.json` maps SKUs to their script inventories:

```json
{
  "version": "1.0",
  "skus": {
    "<sku-name>": {
      "description": "...",
      "registry": "<default-registry>",
      "repository": "aks/provisioning-scripts/<sku-name>",
      "scripts": [
        {
          "source": "parts/linux/cloud-init/artifacts/<script>.sh",
          "destination": "/opt/azure/containers/<script>.sh",
          "permissions": "0744"
        }
      ]
    }
  }
}
```

- **source**: Path relative to repo root (what the developer edits)
- **destination**: Absolute path on the VHD (what packer copies it as)
- **permissions**: File permissions set during VHD build
