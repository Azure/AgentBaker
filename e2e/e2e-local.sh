#!/bin/bash

set -euxo pipefail

: "${SUBSCRIPTION_ID:=8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8}" #Azure Container Service - Test Subscription
: "${LOCATION:=eastus}"
: "${AZURE_TENANT_ID:=72f988bf-86f1-41af-91ab-2d7cd011db47}"
: "${TIMEOUT:=45m}"

export SUBSCRIPTION_ID
export LOCATION
export AZURE_TENANT_ID

go version
go test -timeout $TIMEOUT -v -run Test_All ./