#!/bin/bash

# Test runner script for vhdbuilder/packer ShellSpec tests
# This script runs the ShellSpec tests for the produce_ua_token function

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Running ShellSpec tests for vhdbuilder/packer...${NC}"
echo ""

# Check if shellspec is available
if ! command -v shellspec &>/dev/null; then
	echo -e "${RED}Error: shellspec command not found${NC}"
	echo "Please install ShellSpec: https://github.com/shellspec/shellspec"
	echo "Or install via: curl -fsSL https://git.io/shellspec | sh"
	exit 1
fi

# Change to the directory containing the tests
cd "$(dirname "$0")"

# Run the tests
echo "Running produce_ua_token function tests..."
if shellspec --shell bash --no-warning-as-failure spec/produce_ua_token_spec.sh; then
	echo -e "${GREEN}✓ produce_ua_token tests passed!${NC}"
	echo ""
	echo "Running ensure_sig_image_name_linux function tests..."
else
	echo ""
	echo -e "${RED}✗ produce_ua_token tests failed!${NC}"
	exit 1
fi


if shellspec --shell bash --no-warning-as-failure spec/ensure_sig_image_name_linux_spec.sh; then
	echo ""
	echo "Running prepare_windows_vhd function tests..."
else
	echo ""
	echo -e "${RED}✗ ensure_sig_image_name_linux tests failed!${NC}"
	exit 1
fi


# if shellspec --shell bash --no-warning-as-failure spec/prepare_windows_vhd_spec.sh; then
# 	echo ""
# 	echo -e "${GREEN}✓ All tests passed!${NC}"
# 	exit 0
# else
# 	echo ""
# 	echo -e "${RED}✗ prepare_windows_vhd tests failed!${NC}"
# 	exit 1
# fi