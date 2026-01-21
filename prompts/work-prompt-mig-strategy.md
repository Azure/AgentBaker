# Work Prompt for Claude

## Task: Add `MigStrategy` NBC Variable for MIG Strategy Configuration

### Working Branch

**Branch**: `calvinshum/managed-gpu/mig-strategy` (create this branch before starting)

### Background

The RP (Resource Provider) side changes are complete. We need to add an NBC (Node Bootstrap Configuration) variable called `MigStrategy` (as a string) on the AgentBaker side. This variable controls the MIG (Multi-Instance GPU) strategy used by the nvidia-device-plugin when exposing MIG devices to Kubernetes.

**Key Points:**

- The RP side defines `migStrategy` as a string enum with values: `"None"`, `"Single"`, `"Mixed"`
- `MigStrategy` is only meaningful when `MIG_NODE=true` (i.e., when `GPUInstanceProfile` is set)
- `GPUInstanceProfile` controls MIG **partitioning** (hardware-level), while `migStrategy` controls how the device plugin **exposes** those partitions to Kubernetes (software-level)
- The fallback/default behavior is `single` - we only use `mixed` when explicitly specified

### Reference Implementation

Use Ganesh's commit as a reference for how to properly add an NBC variable:

- **Commit**: <https://github.com/Azure/AgentBaker/commit/d2bfaa34fee2e2542b978e125e8ea61c4a85c162>
- **PR**: #7210 - "Managed GPU experience AFEC enablement flag"

Also reference the `ENABLE_MANAGED_GPU` variable implementation (from the management-mode work) for a similar pattern.

### Files Modified in Reference (Ganesh's commit)

1. `aks-node-controller/proto/aksnodeconfig/v1/gpu_config.proto` - Add proto field
2. `aks-node-controller/pkg/gen/aksnodeconfig/v1/gpu_config.pb.go` - **Auto-generated** (run `make generate`)
3. `aks-node-controller/parser/parser.go` - Map proto field to CSE env var
4. `pkg/agent/datamodel/types.go` - Add field to `NodeBootstrappingConfiguration`
5. `pkg/agent/baker.go` - Add template function for CSE
6. `pkg/agent/baker_test.go` - Add tests
7. `parts/linux/cloud-init/artifacts/cse_cmd.sh` - Use template function
8. `e2e/scenario_gpu_managed_experience_test.go` - Update e2e tests (if applicable)
9. `pkg/agent/testdata/**` - **Auto-generated** (run `make generate`)

---

## Plan: Three Commits

### Commit 1: NBC Infrastructure Changes

Add the `MigStrategy` string variable following the same pattern as other NBC variables:

**There are TWO NBC paths that need to be updated:**

#### Path 1: Scriptless NBC (aks-node-controller)

1. **`aks-node-controller/proto/aksnodeconfig/v1/gpu_config.proto`**
   - Add new field: `string mig_strategy = X;` (use next available field number)
   - Add appropriate comment explaining the field:

     ```protobuf
     // mig_strategy specifies the MIG strategy for the nvidia-device-plugin.
     // Valid values are "None", "Single", "Mixed". Only meaningful when MIG is enabled
     // (i.e., when gpu_instance_profile is set). Defaults to "Single" if not specified.
     string mig_strategy = X;
     ```

2. **`aks-node-controller/parser/parser.go`**
   - Add CSE env mapping: `"NVIDIA_MIG_STRATEGY": config.GetGpuConfig().GetMigStrategy(),`

#### Path 2: Legacy NBC (pkg/agent)

1. **`pkg/agent/datamodel/types.go`**
   - Add field to `NodeBootstrappingConfiguration`: `MigStrategy string`

2. **`pkg/agent/baker.go`**
   - Add template function:

     ```go
     "GetMigStrategy": func() string {
         return config.MigStrategy
     },
     ```

3. **`parts/linux/cloud-init/artifacts/cse_cmd.sh`**
   - Add: `NVIDIA_MIG_STRATEGY="{{GetMigStrategy}}"`

#### Tests

1. **`pkg/agent/baker_test.go`**
   - Add test entries similar to other GPU-related tests (search for "GPUInstanceProfile" or "EnableManagedGPU" in the file to see the pattern)

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

### Commit 3: Use the Variable in cse_config.sh

Modify the `startNvidiaManagedExpServices()` function to use the new `NVIDIA_MIG_STRATEGY` variable.

**File to modify:** `parts/linux/cloud-init/artifacts/cse_config.sh`

**Current logic** (around line 1242):

```bash
if [ "${MIG_NODE}" = "true" ]; then
    # Configure with MIG strategy for MIG nodes
    tee "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}/10-device-plugin-config.conf" > /dev/null <<'EOF'
[Service]
Environment="MIG_STRATEGY=--mig-strategy single"
ExecStart=
ExecStart=/usr/bin/nvidia-device-plugin $MIG_STRATEGY --pass-device-specs
EOF
else
    # Configure with pass-device-specs for non-MIG nodes
    tee "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}/10-device-plugin-config.conf" > /dev/null <<'EOF'
[Service]
ExecStart=
ExecStart=/usr/bin/nvidia-device-plugin --pass-device-specs
EOF
fi
```

**New logic:**

```bash
if [ "${MIG_NODE}" = "true" ]; then
    # Configure with MIG strategy for MIG nodes.
    # MIG strategy controls how nvidia-device-plugin exposes MIG instances to Kubernetes:
    #   - "single": All MIG devices exposed as generic nvidia.com/gpu resources
    #   - "mixed": MIG devices exposed with specific types like nvidia.com/mig-1g.5gb
    #
    # We only use "mixed" when explicitly specified via NVIDIA_MIG_STRATEGY.
    # Otherwise, we default to "single" which is the safer/simpler option.
    # Note: NVIDIA_MIG_STRATEGY values from RP are "None", "Single", "Mixed".
    # "None" and "Single" both result in using the "single" strategy.
    if [ "${NVIDIA_MIG_STRATEGY}" = "Mixed" ]; then
        MIG_STRATEGY_FLAG="--mig-strategy mixed"
    else
        # Default to "single" for "Single", "None", empty, or any other value
        MIG_STRATEGY_FLAG="--mig-strategy single"
    fi

    tee "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}/10-device-plugin-config.conf" > /dev/null <<EOF
[Service]
ExecStart=
ExecStart=/usr/bin/nvidia-device-plugin ${MIG_STRATEGY_FLAG} --pass-device-specs
EOF
else
    # Configure with pass-device-specs for non-MIG nodes
    tee "${NVIDIA_DEVICE_PLUGIN_OVERRIDE_DIR}/10-device-plugin-config.conf" > /dev/null <<'EOF'
[Service]
ExecStart=
ExecStart=/usr/bin/nvidia-device-plugin --pass-device-specs
EOF
fi
```

**⚠️ STOP AND ASK USER TO RUN `make generate`** after this commit to regenerate testdata for the parts/ changes.

---

## Summary of Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Variable name | `MigStrategy` (string) | Short, clear, matches RP naming convention |
| CSE env var name | `NVIDIA_MIG_STRATEGY` | Prefixed with NVIDIA for clarity |
| Type | String (not bool) | Preserves the three-value enum (None/Single/Mixed) |
| Relationship to MIG_NODE | Dependent | `migStrategy` is only meaningful when `MIG_NODE=true` |
| Default behavior | `single` | Use "mixed" only when explicitly set to "Mixed" |

## Important Reminders

- **Testdata is auto-generated**: Don't manually edit files in `pkg/agent/testdata/` - run `make generate`
- **Proto files generate Go code**: After editing `.proto`, run `make generate`
- The variable is a **string**, not a bool (values: "None", "Single", "Mixed")
- `MigStrategy` is only applied when `MIG_NODE=true` (when `GPUInstanceProfile` is set)
- The default/fallback strategy is always `single`
