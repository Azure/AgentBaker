#!/bin/bash

set -euxo pipefail

: "${SUBSCRIPTION_ID:=8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8}" #Azure Container Service - Test Subscription
: "${RESOURCE_GROUP_NAME:=agentbaker-e2e-tests}"
: "${LOCATION:=eastus}"
: "${CLUSTER_NAME:=agentbaker-e2e-test-cluster}"
: "${SIG_VERSION_ID:=}"

# This will error out if SCENARIO_NAME or VM_SKU are unbound,
# fall back to defaulting logic in e2e-scenario.sh when SIG_VERSION_ID is empty
SCENARIO_NAME=$1
VM_SKU=$2
[ $# -eq 3 ] && SIG_VERSION_ID=$3

export SUBSCRIPTION_ID
export RESOURCE_GROUP_NAME
export LOCATION
export CLUSTER_NAME
export SCENARIO_NAME
export VM_SKU
export SIG_VERSION_ID

bash ./e2e-starter.sh

bash ./e2e-scenario.sh