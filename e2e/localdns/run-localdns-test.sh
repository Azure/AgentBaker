#!/bin/bash
# Helper script to run localdns e2e tests
# Usage: ./run-localdns-test.sh [test-name]
# Examples:
#   ./run-localdns-test.sh                          # Run the test specified in .env
#   ./run-localdns-test.sh Test_Ubuntu2204_LocalDNSHostsPlugin
#   ./run-localdns-test.sh Test_Ubuntu2404_LocalDNSHostsPlugin

set -euo pipefail

cd "$(dirname "$0")"

# Load .env file if it exists
if [ -f .env ]; then
    echo "Loading configuration from .env..."
    set -a
    source .env
    set +a
fi

# If test name is provided as argument, override TAGS_TO_RUN
if [ $# -gt 0 ]; then
    export TAGS_TO_RUN="name=$1"
    echo "Running specific test: $1"
else
    echo "Running test(s) from .env: ${TAGS_TO_RUN:-all tests}"
fi

# Set defaults
: "${TIMEOUT:=90m}"
: "${PARALLEL:=100}"

echo ""
echo "=========================================="
echo "LocalDNS E2E Test Configuration"
echo "=========================================="
echo "Subscription: ${SUBSCRIPTION_ID:-not set}"
echo "Location: ${E2E_LOCATION:-westus3}"
echo "Tags to run: ${TAGS_TO_RUN:-all}"
echo "Tags to skip: ${TAGS_TO_SKIP:-none}"
echo "Keep VMSS: ${KEEP_VMSS:-false}"
echo "Timeout: ${TIMEOUT}"
echo "Parallel: ${PARALLEL}"
echo "=========================================="
echo ""

# Run the tests
echo "Starting e2e tests..."
go test -parallel $PARALLEL -timeout $TIMEOUT -v -count 1

echo ""
echo "=========================================="
echo "Test run completed!"
echo "=========================================="
if [ "${KEEP_VMSS:-false}" = "true" ]; then
    echo ""
    echo "⚠️  REMINDER: KEEP_VMSS=true is set!"
    echo "⚠️  VMs were NOT deleted. Check scenario-logs/ for SSH keys."
    echo "⚠️  Don't forget to clean up resources to avoid costs:"
    echo ""
    echo "    az group list --query \"[?starts_with(name, 'abe2e-')].name\" -o tsv"
    echo "    az group delete --name <resource-group-name> --yes --no-wait"
    echo ""
fi
