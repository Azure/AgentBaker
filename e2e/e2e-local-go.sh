#!/bin/bash

set -euxo pipefail

: "${SUBSCRIPTION_ID:=8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8}" #Azure Container Service - Test Subscription
: "${RESOURCE_GROUP_NAME:=agentbaker-e2e-tests}"
: "${LOCATION:=eastus}"
: "${CLUSTER_NAME:=agentbaker-e2e-test-cluster}"
: "${AZURE_TENANT_ID:=72f988bf-86f1-41af-91ab-2d7cd011db47}"

export SUBSCRIPTION_ID
export RESOURCE_GROUP_NAME
export LOCATION
export CLUSTER_NAME
export AZURE_TENANT_ID

go version
go test -timeout 20m -v -run Test_All ./...
