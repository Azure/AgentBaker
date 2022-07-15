#!/bin/bash

set -euxo pipefail

: "${SUBSCRIPTION_ID:=8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8}" #Azure Container Service - Test Subscription
: "${RESOURCE_GROUP_NAME:=agentbaker-e2e-tests}"
: "${LOCATION:=eastus}"
: "${CLUSTER_NAME:=agentbaker-e2e-test-cluster}"

SCENARIO_NAME=$1
VM_SKU=$2

export SUBSCRIPTION_ID
export RESOURCE_GROUP_NAME
export LOCATION
export CLUSTER_NAME
export SCENARIO_NAME
export VM_SKU

bash ./e2e-starter.sh

bash ./e2e-scenario.sh