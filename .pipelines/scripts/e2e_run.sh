#!/usr/bin/env bash

set -euo pipefail

az login --identity --object-id "${E2E_AGENT_IDENTITY_ID}"
az account set -s "${E2E_SUBSCRIPTION_ID}"
echo "Using subscription ${E2E_SUBSCRIPTION_ID} for e2e tests"

GOPATH="$(go env GOPATH)"

go version

LOGGING_DIR="scenario-logs-$(date +%s)"
echo "setting logging dir to $LOGGING_DIR"
echo "##vso[task.setvariable variable=LOGGING_DIR]$LOGGING_DIR"

mkdir -p "${DefaultWorkingDirectory}/e2e/${LOGGING_DIR}"


echo "VHD_BUILD_ID=$VHD_BUILD_ID"
echo "IGNORE_SCENARIOS_WITH_MISSING_VHD: $IGNORE_SCENARIOS_WITH_MISSING_VHD"
echo "LOGGING_DIR: $LOGGING_DIR"

if [ -n "${VHD_BUILD_ID}" ]; then
  echo "VHD_BUILD_ID is specified (${VHD_BUILD_ID}). Running tests using VHDs from that build"
  export SIG_VERSION_TAG_NAME=buildId
  export SIG_VERSION_TAG_VALUE=$VHD_BUILD_ID
else
  echo "VHD_BUILD_ID is not specified. Running tests with default SIG version tag selectors."
fi

cd e2e
mkdir -p bin
GOBIN=`pwd`/bin/ go install github.com/jstemmer/go-junit-report/v2@latest

# Yes, we build first. That's because the exit code from "go test" below is eaten by the go-junit-report command. So if there are build problems
# then the tests pass. Bah.
go build -mod=readonly ./...
go test -v -parallel 100 -timeout 90m 2>&1 | ./bin/go-junit-report -iocopy -set-exit-code -out "${BUILD_SRC_DIR}/e2e/report.xml"
