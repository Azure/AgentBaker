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

# Tests for produce_ua_token Function

This directory contains ShellSpec tests for the `produce_ua_token` function in `produce-packer-settings-functions.sh`.

## Overview

The `produce_ua_token` function is responsible for determining whether Ubuntu Advantage (UA) tokens are required for building specific Ubuntu SKUs that need Extended Security Maintenance (ESM).

## Test Structure

```
vhdbuilder/packer/
├── spec/
│   ├── spec_helper.sh                 # Test configuration and setup
│   └── produce_ua_token_spec.sh       # Main test file
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
```

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
