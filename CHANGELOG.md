# Change Log

## 2026-07-28 - GPU E2E CDI refresh recovery

- **Goal:** Fix the shared NVIDIA CDI startup race behind the Ubuntu A100 and NC GPU E2E failures in build 174132278.
- **Important findings:** `nvidia-cdi-refresh.service` exhausts its start limit before NVIDIA userspace libraries are staged; H100 was skipped because UAENorth quota was fully consumed.
- **Failed attempts:** An `ExecStartPre` driver wait would deadlock the toolkit package install. Repository-wide shell validation is blocked by existing SC3014 warnings on `main`.
- **Files changed:** `parts/linux/cloud-init/artifacts/cse_config.sh`, `spec/parts/linux/cloud-init/artifacts/cse_config_spec.sh`, and `CHANGELOG.md`.
- **Next step:** Refresh PR #8736 and rerun GPU E2E; handle the H100 quota increase separately.
