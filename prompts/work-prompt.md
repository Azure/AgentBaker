# Work Prompt for Claude

## Task: Add `nvidiaManagementMode` NBC Variable for Managed GPU Experience

### Working Branch

**Branch**: `calvinshum/managed-gpu/nvidia-management-mode` (you should already be on this branch)

### Background

The RP (Resource Provider) side changes are complete. We need to add an NBC (Node Bootstrap Configuration) variable called `nvidiaManagementMode` (as a bool) on the AgentBaker side. This variable will be used as another way to enable the managed GPU experience feature.

**Key Points:**

- The VMSS tag `EnableManagedGPUExperience` will still be present
- `nvidiaManagementMode` is **not** an additional gate - if **either** the VMSS tag OR `nvidiaManagementMode` wants to enable the feature, we enable it
- On the RP side, it's only set to `false` (unmanaged) when explicitly set to "Unmanaged"
- Special case handling (when `nvidiaManagementMode=false` but VMSS tag is on) is handled on the RP side, not here

### Reference Implementation

Use Ganesh's commit as a reference for how to properly add an NBC variable:

- **Commit**: <https://github.com/Azure/AgentBaker/commit/d2bfaa34fee2e2542b978e125e8ea61c4a85c162>
- **PR**: #7210 - "Managed GPU experience AFEC enablement flag"
- **Note**: The `ManagedGPUExperienceAFECEnabled` flag added in that PR is actually unused (the AFEC flag is not validated in this repo), but it's a good template for adding an NBC var

### Files Modified in Reference (Ganesh's commit)

1. `aks-node-controller/proto/aksnodeconfig/v1/gpu_config.proto` - Add proto field
2. `aks-node-controller/pkg/gen/aksnodeconfig/v1/gpu_config.pb.go` - **Auto-generated** (run `make generate`)
3. `aks-node-controller/parser/parser.go` - Map proto field to CSE env var
4. `pkg/agent/datamodel/types.go` - Add field to `NodeBootstrappingConfiguration`
5. `pkg/agent/baker.go` - Add template function for CSE
6. `pkg/agent/baker_test.go` - Add tests
7. `parts/linux/cloud-init/artifacts/cse_cmd.sh` - Use template function
8. `e2e/scenario_gpu_managed_experience_test.go` - Update e2e tests
9. `pkg/agent/testdata/**` - **Auto-generated** (run `make generate`)

---

## Plan: Three Commits

### Commit 1: NBC Infrastructure Changes

Add the `nvidiaManagementMode` bool variable following the same pattern as `ManagedGPUExperienceAFECEnabled`:

**There are TWO NBC paths that need to be updated:**

#### Path 1: Scriptless NBC (aks-node-controller)

1. **`aks-node-controller/proto/aksnodeconfig/v1/gpu_config.proto`**
   - Add new field: `bool nvidia_management_mode = 7;` (next available field number)
   - Add appropriate comment

2. **`aks-node-controller/parser/parser.go`**
   - Add CSE env mapping: `"NVIDIA_MANAGEMENT_MODE": fmt.Sprintf("%v", config.GetGpuConfig().GetNvidiaManagementMode())`

#### Path 2: Legacy NBC (pkg/agent)

3. **`pkg/agent/datamodel/types.go`**
   - Add field to `NodeBootstrappingConfiguration`: `NvidiaManagementMode bool`

4. **`pkg/agent/baker.go`**
   - Add template function:

     ```go
     "IsNvidiaManagementModeEnabled": func() bool {
         return config.NvidiaManagementMode
     },
     ```

5. **`parts/linux/cloud-init/artifacts/cse_cmd.sh`**
   - Add: `NVIDIA_MANAGEMENT_MODE="{{IsNvidiaManagementModeEnabled}}"`

#### Tests

6. **`pkg/agent/baker_test.go`**
   - Add test entries similar to the `ManagedGPUExperienceAFECEnabled` tests (search for "ManagedGPUExperienceAFEC" in the file to see the pattern)

7. **`e2e/scenario_gpu_managed_experience_test.go`**
   - Update existing tests to set `nbc.NvidiaManagementMode = true`

---

### Commit 2: Generate Testdata (Manual Step)

**⚠️ STOP AND ASK USER TO RUN THESE COMMANDS MANUALLY:**

After Commit 1 is complete, Claude should ask the user to run the following commands:

```bash
# Generate protobuf Go code and update all testdata
make generate
```

This will update:

- `aks-node-controller/pkg/gen/aksnodeconfig/v1/gpu_config.pb.go` (from proto)
- All testdata files in `pkg/agent/testdata/`

Once the user confirms the commands have been run successfully, commit the generated files as a separate commit.

---

### Commit 3: Use the Variable in cse_helpers.sh

Simple change to enable managed GPU experience when either flag is set:

**File to modify:**

**`parts/linux/cloud-init/artifacts/cse_helpers.sh`** (around line 706)

Current logic in `enableManagedGPUExperience()` only checks VMSS tag:

```bash
enableManagedGPUExperience() {
    set -x
    body=$(curl -fsSL -H "Metadata: true" --noproxy "*" "http://169.254.169.254/metadata/instance?api-version=2021-02-01")
    ret=$?
    if [ "$ret" -ne 0 ]; then
      return $ret
    fi
    should_enforce=$(echo "$body" | jq -r '.compute.tagsList[] | select(.name == "EnableManagedGPUExperience") | .value')
    echo "${should_enforce,,}"
}
```

**New logic** - enable if EITHER the VMSS tag OR `NVIDIA_MANAGEMENT_MODE` is true:

```bash
enableManagedGPUExperience() {
    set -x

    # Check if nvidiaManagementMode is enabled via NBC
    if [ "${NVIDIA_MANAGEMENT_MODE,,}" == "true" ]; then
        echo "true"
        return 0
    fi

    # Fall back to VMSS tag check
    body=$(curl -fsSL -H "Metadata: true" --noproxy "*" "http://169.254.169.254/metadata/instance?api-version=2021-02-01")
    ret=$?
    if [ "$ret" -ne 0 ]; then
      return $ret
    fi
    should_enforce=$(echo "$body" | jq -r '.compute.tagsList[] | select(.name == "EnableManagedGPUExperience") | .value')
    echo "${should_enforce,,}"
}
```

**⚠️ STOP AND ASK USER TO RUN `make generate`** after this commit to regenerate testdata for the parts/ changes.

---

## Important Reminders

- **Testdata is auto-generated**: Don't manually edit files in `pkg/agent/testdata/` - run `make generate`
- **Proto files generate Go code**: After editing `.proto`, run `make generate`
- The variable is a **bool**, not a string enum (on RP side it's "Managed"/"Unmanaged", but AB receives a bool)
- This is **OR logic**: if either `nvidiaManagementMode` OR the VMSS tag wants to enable, we enable
