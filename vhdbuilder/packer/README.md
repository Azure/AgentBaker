The workflow becomes complicated as we add more VHDs. Here is a quick description of the current VHD build workflow:

# gen1
MarketplaceImage->(func/packer) -> VHD in classic SA
## gen1 mariner:
external-vhd->(func/init:internal-vhd)->(func/packer) -> VHD in classic SA

# sig:
MarketplaceImage->(func/init: ensure dest gallery/definition)->(func/packer: managed image)->dest SIG

# gen2:
MarketplaceImage->(func/init: ensure dest existing/static gallery/definition)->(func/packer: managed image)->dest SIG->(func/convert: sig->disk) -> VHD in classic SA

## gen2 mariner:
external-vhd-src->(func/init: ensure dest existing/static gallery/definition, internal-vhd->image; )->source SIG->(func/packer: managed image)->dest SIG->(func/convert: sig->disk) -> VHD in classic SA

# Roadmap
Goal1: remove mariner workflow so things will be simplified.

---

# Tests for produce_ua_token and ensure_sig_image_name_linux Functions

This directory contains ShellSpec tests for the `produce_ua_token` and `ensure_sig_image_name_linux` functions in `produce-packer-settings-functions.sh`.

## Overview

### produce_ua_token Function
The `produce_ua_token` function is responsible for determining whether Ubuntu Advantage (UA) tokens are required for building specific Ubuntu SKUs that need Extended Security Maintenance (ESM).

### ensure_sig_image_name_linux Function
The `ensure_sig_image_name_linux` function manages the generation of Shared Image Gallery (SIG) names and image names based on various conditions including OS offers, SKUs, and feature flags.

## Test Structure

```
vhdbuilder/packer/
├── spec/
│   ├── spec_helper.sh                 # Test configuration and setup
│   ├── produce_ua_token_spec.sh       # Tests for produce_ua_token function (28 tests)
│   └── ensure_sig_image_name_linux_spec.sh  # Tests for ensure_sig_image_name_linux function (46 tests)
├── run_tests.sh                       # Test runner script
└── README.md                          # This documentation
```

## Running the Tests

### Prerequisites

1. **ShellSpec Installation**: Ensure ShellSpec is installed on your system.
   ```bash
   # Install ShellSpec
   curl -fsSL https://git.io/shellspec | sh
   ```

2. **Environment**: Run tests from the `vhdbuilder/packer` directory.

### Running All Tests

```bash
# From vhdbuilder/packer directory
./run_tests.sh
```

### Running Tests with ShellSpec Directly

```bash
# From vhdbuilder/packer directory
shellspec --shell bash spec/produce_ua_token_spec.sh

# Run with verbose output
shellspec --shell bash --format tap spec/produce_ua_token_spec.sh

# Run with coverage (if kcov is installed)
shellspec --shell bash --kcov spec/produce_ua_token_spec.sh

# Run specific function tests
shellspec --shell bash spec/ensure_sig_image_name_linux_spec.sh
```

## Test Coverage

### produce_ua_token Function (28 tests)
- **Ubuntu 18.04/20.04 scenarios**: Valid tokens, missing tokens, case handling
- **FIPS scenarios**: Case-insensitive FIPS detection, token requirements
- **Non-Ubuntu/Non-linux scenarios**: Other OS distributions and modes
- **Edge cases**: Environment variables, logging, complex version strings

### ensure_sig_image_name_linux Function (46 tests)
- **SIG_GALLERY_NAME scenarios**: Default generation, provided values
- **IMG_OFFER conditions**: cbl-mariner, azure-linux-3 with case handling
- **OS_SKU conditions**: azurelinuxosguard with case handling
- **FEATURE_FLAGS**: cvm detection, substring matching, priority handling
- **Priority testing**: Condition precedence (cbl-mariner → azure-linux-3 → azurelinuxosguard → cvm)
- **Edge cases**: Unset variables, special characters, complex combinations

## Function Logic

### produce_ua_token Function

```
if MODE == "linuxVhdMode" AND OS_SKU == "ubuntu" (case-insensitive):
    if OS_VERSION in ["18.04", "20.04"] OR ENABLE_FIPS == "true" (case-insensitive):
        if UA_TOKEN is empty:
            output error message to stdout and exit 1
        else:
            keep existing UA_TOKEN and log to stdout
    else:
        UA_TOKEN = "notused"
else:
    UA_TOKEN = "notused"
```

### ensure_sig_image_name_linux Function

**SIG_GALLERY_NAME Logic:**
- If empty/unset: "PackerSigGalleryEastUS"
- If provided: Use provided value

**SIG_IMAGE_NAME Logic:**
- If provided: Use provided value
- If empty/unset: Start with SKU_NAME, then apply first matching condition:
  1. **cbl-mariner**: Add "CBLMariner" or "AzureLinux" prefix based on ENABLE_CGROUPV2
  2. **azure-linux-3**: Add "AzureLinux" prefix
  3. **azurelinuxosguard**: Add "AzureLinuxOSGuard" prefix
  4. **cvm in FEATURE_FLAGS**: Add "Specialized" suffix

## Extending Tests

To add new test cases:

1. **Add to existing describe blocks** in `spec/produce_ua_token_spec.sh`
2. **Create new describe blocks** for new scenarios
3. **Follow the pattern**:
   ```bash
   It 'should describe expected behavior'
     # Set up environment variables
     MODE="..."
     OS_SKU="..."
     # ... other variables

     When call produce_ua_token
     The status should be success/failure
     The variable UA_TOKEN should eq "expected-value"
     The stderr should include "expected-message"
   End
   ```

## Dependencies

- **ShellSpec**: Testing framework
- **bash**: Shell interpreter
- **Source file**: `vhdbuilder/packer/produce-packer-settings-functions.sh`
