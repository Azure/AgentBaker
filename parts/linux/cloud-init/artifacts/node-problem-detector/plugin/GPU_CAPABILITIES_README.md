# GPU VM Capabilities

This file (`gpu_vms_capabilities.json`) defines GPU capabilities for Azure NVIDIA GPU VM SKUs used by NPD health checks.

## Fields

| Field | Description | Source |
|-------|-------------|--------|
| `GPUs` | Number of physical GPUs | Azure SKU API |
| `RdmaEnabled` | InfiniBand/RDMA networking | Azure SKU API |
| `NVLinkEnabled` | NVLink GPU interconnect | Manual (NVIDIA specs) |

## Verifying the File

```bash
./scripts/verify_gpu_capabilities.sh [location]
```

The script checks:
- GPU counts and RDMA flags match Azure SKU API
- Reports SKUs missing from either the file or API

**Note:** Some SKUs may be regional or in preview. SKUs in the file but not in the API for a given region are expected (e.g., `NC*ads_A10_v4`, `NCC40ads_H100_v5`, `ND128*_GB200_v6`). Internal SKUs (`*_internal_*`) are excluded.

## Adding New SKUs

```bash
# Get SKU capabilities
az vm list-skus --location eastus --size <SKU> -o json | \
  jq '.[0].capabilities[] | select(.name | test("GPU|Rdma"))'
```

1. Add entry with GPU count and RdmaEnabled from API
2. Set NVLinkEnabled based on NVIDIA specs (see table below)

## NVLink-Enabled SKUs

| SKU Pattern | GPU | NVLink |
|-------------|-----|--------|
| ND40rs_v2 | V100 | 2.0 |
| ND96*_A100_v4 | A100 | 3.0 |
| ND96*_H100_v5 | H100 | 4.0 |
| ND128*_GB200_v6 | GB200 | 5.0 |
| NC*ads_H100_v5 | H100 NVL | Yes |
