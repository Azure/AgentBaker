#!/usr/bin/env bash

# if anything goes wrong, then abort.
set -euo pipefail

# This script runs the AgentBaker e2e tests for a VHD. It uses the following environment variables:
# * E2E_SUBSCRIPTION_ID: this variable contains the subscription to run the e2e tests in
# * DefaultWorkingDirectory: this variable contains the default working directory. Likely "." is sufficient
# * VHD_BUILD_ID - the build identifier for the pipeline. This is optional and if it is missing then the latest build from
#   the main branch is used.
# * IGNORE_SCENARIOS_WITH_MISSING_VHD: a true/false flag that indicates if the build should fail if the VHD is missing.
# * BUILD_SRC_DIR: the src directory for the repository. Probably the same as DefaultWorkingDirectory.

# In addition, the e2e test framework reads a whole lot of environment variables.
# These are defined in: e2e/config/config.go

az account set -s "${E2E_SUBSCRIPTION_ID}"
echo "Using subscription ${E2E_SUBSCRIPTION_ID} for e2e tests"

# Map E2E_SUBSCRIPTION_ID to SUBSCRIPTION_ID which the Go test framework reads
export SUBSCRIPTION_ID="${E2E_SUBSCRIPTION_ID}"

# -----------------------------------------------------------------------------
# RCV1P testing-subscription override block
# -----------------------------------------------------------------------------
# Context:
#   AgentBaker's RCV1P (root-cert v1 platform-injection) end-to-end tests need
#   to run in a subscription where Microsoft.Compute/PlatformSettingsOverride
#   is registered. Two distinct environments expose that capability:
#
#     1. The MSFT-tenant default E2E subscription (variable group: ab-e2e-tme).
#        PlatformSettingsOverride is registered AND every VMSS is auto-tagged
#        with the RCV1P opt-in tag by an Azure Policy at create time. The
#        framework therefore does NOT stamp tags itself; it relies on the
#        platform to inject them.
#
#     2. The dedicated RCV1P testing subscription (id pinned in the
#        ab-e2e-tme variable group as E2E_SUBSCRIPTION_ID_RCV1P).
#        PlatformSettingsOverride is registered but NO auto-tagging policy
#        is attached, so the framework MUST stamp tags itself.
#
#   The aks-rp orchestrator runs both flows against the same AgentBaker
#   pipeline (e2e-tme.yaml) and switches between them via the --subscription-id
#   parameter on the pipeline. That parameter ultimately lands in
#   E2E_SUBSCRIPTION_ID below.
#
# Why this block exists:
#   When the orchestrator points e2e-tme.yaml at the RCV1P sub, the variables
#   inherited from ab-e2e-tme (which target the default sub) are wrong:
#
#     a) BLOB_STORAGE_ACCOUNT_PREFIX=abe2etme yields the account name
#        "abe2etme<region>", which is already owned by the default sub.
#        Storage account names are globally unique, so trying to (re)create
#        it under the RCV1P sub fails with StorageAccountAlreadyTaken and
#        every Linux RCV1P scenario aborts before provisioning.
#
#     b) The Go test framework reads RCV1P_TAGS_AUTO_INJECTED to decide
#        whether to stamp opt-in tags on VMSSes it creates. The default
#        (true) is correct for the MSFT-tenant flow but wrong for the
#        RCV1P sub, where the framework must do the stamping itself.
#
#     c) The RCV1P stage of the Linux orchestrator only publishes Linux
#        VHDs to the gallery, but the test selector still picks up
#        Test_RCV1P_Windows*. Without IGNORE_SCENARIOS_WITH_MISSING_VHD,
#        those Windows scenarios fail hard with "image does not exist in
#        gallery" instead of skipping.
#
# How detection works:
#   E2E_SUBSCRIPTION_ID is the active subscription this run will use (set
#   from the variable group, possibly overridden by the orchestrator's
#   --subscription-id). E2E_SUBSCRIPTION_ID_RCV1P is a constant defined in
#   the ab-e2e-tme variable group identifying the RCV1P sub. We compare
#   them; no subscription GUID is hardcoded in the script.
#
#   For pipelines whose variable group does not define
#   E2E_SUBSCRIPTION_ID_RCV1P (e.g. non-TME variable groups), the first
#   condition is empty and the block is a no-op -- the default behavior
#   (auto-injection on, shared storage account, missing-VHD = failure)
#   is preserved.
#
# What gets overridden:
#   * BLOB_STORAGE_ACCOUNT_PREFIX = "abe2etmercv1p"
#       The framework computes the storage account name as
#       <prefix><default-location> (see e2e/config/config.go:BlobStorageAccount).
#       "abe2etmercv1p" is unique globally, so the storage account is created
#       inside the RCV1P sub on first run and reused thereafter.
#   * RCV1P_TAGS_AUTO_INJECTED = "false"
#       Tells the framework to stamp opt-in tags on each VMSS it creates,
#       and lets Test_RCV1P_*_NotOptedIn actually run (they self-skip when
#       this flag is true because a not-opted-in VMSS is impossible under
#       auto-injection).
#   * IGNORE_SCENARIOS_WITH_MISSING_VHD = "true"
#       Surfaces missing VHDs as SKIP, not FAIL (see e2e/test_helpers.go).
#       Needed because the Linux RCV1P orchestrator run does not produce
#       Windows VHDs; the Windows RCV1P scenarios would otherwise fail.
#
# Long-term plan:
#   Replace this runtime override with a dedicated RCV1P pipeline
#   (e2e-rcv1p.yaml already exists and wires the correct variable group
#   ab-e2e-tme-rcv1p and rcv1pTagsAutoInjected=false). That requires an
#   aks-rp orchestrator change to queue e2e-rcv1p.yaml instead of
#   e2e-tme.yaml. Until then, this block keeps the single-pipeline flow
#   working from inside AgentBaker only.
# -----------------------------------------------------------------------------
if [ -n "${E2E_SUBSCRIPTION_ID_RCV1P:-}" ] && [ "${E2E_SUBSCRIPTION_ID}" = "${E2E_SUBSCRIPTION_ID_RCV1P}" ]; then
  echo "Active subscription matches E2E_SUBSCRIPTION_ID_RCV1P; applying RCV1P-specific overrides"
  # See "What gets overridden" in the comment block above for the rationale
  # behind each of these three settings.
  export BLOB_STORAGE_ACCOUNT_PREFIX="abe2etmercv1p"
  export RCV1P_TAGS_AUTO_INJECTED="false"
  export IGNORE_SCENARIOS_WITH_MISSING_VHD="true"
  echo "  BLOB_STORAGE_ACCOUNT_PREFIX=${BLOB_STORAGE_ACCOUNT_PREFIX}"
  echo "  RCV1P_TAGS_AUTO_INJECTED=${RCV1P_TAGS_AUTO_INJECTED}"
  echo "  IGNORE_SCENARIOS_WITH_MISSING_VHD=${IGNORE_SCENARIOS_WITH_MISSING_VHD}"
fi

# Setup go
export GOPATH="$(go env GOPATH)"
go version

# specify the logging directory so logs go to the right place
export LOGGING_DIR="scenario-logs-$(date +%s)"
echo "setting logging dir to $LOGGING_DIR"
# tell DevOps to set the variable so later pipeline steps can use it.

echo "##vso[task.setvariable variable=LOGGING_DIR]$LOGGING_DIR"
# make sure the logging directory exists
mkdir -p "${DefaultWorkingDirectory}/e2e/${LOGGING_DIR}"

# default any unbound required variables if necessary
VHD_BUILD_ID="${VHD_BUILD_ID:-}"
IGNORE_SCENARIOS_WITH_MISSING_VHD="${IGNORE_SCENARIOS_WITH_MISSING_VHD:-}"
LOGGING_DIR="${LOGGING_DIR:-}"
E2E_SUBSCRIPTION_ID="${E2E_SUBSCRIPTION_ID:-}"
ENABLE_SECURE_TLS_BOOTSTRAPPING="${ENABLE_SECURE_TLS_BOOTSTRAPPING:-true}"
TAGS_TO_SKIP="${TAGS_TO_SKIP:-}"
TAGS_TO_RUN="${TAGS_TO_RUN:-}"
E2E_GO_TEST_TIMEOUT="${E2E_GO_TEST_TIMEOUT:-80m}"
GALLERY_NAME="${GALLERY_NAME:-}"
SIG_GALLERY_NAME="${SIG_GALLERY_NAME:-}"

# echo some variables so that we have a chance of debugging if things fail due to a pipeline issue
echo "VHD_BUILD_ID: ${VHD_BUILD_ID}"
echo "IGNORE_SCENARIOS_WITH_MISSING_VHD: ${IGNORE_SCENARIOS_WITH_MISSING_VHD}"
echo "LOGGING_DIR: ${LOGGING_DIR}"
echo "E2E_SUBSCRIPTION_ID: ${E2E_SUBSCRIPTION_ID}"
echo "ENABLE_SECURE_TLS_BOOTSTRAPPING: ${ENABLE_SECURE_TLS_BOOTSTRAPPING}"
echo "TAGS_TO_SKIP: ${TAGS_TO_SKIP}"
echo "TAGS_TO_RUN: ${TAGS_TO_RUN}"
echo "GALLERY_NAME: ${GALLERY_NAME}"
echo "SIG_GALLERY_NAME: ${SIG_GALLERY_NAME}"
echo "E2E_GO_TEST_TIMEOUT: ${E2E_GO_TEST_TIMEOUT}"

# set variables that the go program expects if we are running a specific build
if [ -n "${VHD_BUILD_ID}" ]; then
  echo "VHD_BUILD_ID is specified (${VHD_BUILD_ID}). Running tests using VHDs from that build"
  export SIG_VERSION_TAG_NAME=buildId
  export SIG_VERSION_TAG_VALUE=$VHD_BUILD_ID
else
  echo "VHD_BUILD_ID is not specified. Running tests with default SIG version tag selectors."
fi

if [ -n "${SIG_GALLERY_NAME}" ]; then
  echo "SIG_GALLERY_NAME is specified (${SIG_GALLERY_NAME}). Updating GALLERY_NAME to $SIG_GALLERY_NAME"
  export GALLERY_NAME=$SIG_GALLERY_NAME
fi

az extension add --name bastion

# this software is used to take the output of "go test" and produce a junit report that we can upload to the pipeline
# and see fancy test results.
cd e2e
mkdir -p bin
architecture=$(uname -m)

case "$architecture" in
  x86_64 | amd64) architecture="amd64" ;;
  aarch64 | arm64) architecture="arm64" ;;
  *)
    echo "Unsupported architecture: $architecture"
    exit 1
    ;;
esac

gotestsum_version="1.13.0"
gotestsum_archive="gotestsum_${gotestsum_version}_linux_${architecture}.tar.gz"
gotestsum_url="https://github.com/gotestyourself/gotestsum/releases/download/v${gotestsum_version}/${gotestsum_archive}"

temp_file="$(mktemp)"
curl -fsSL "$gotestsum_url" -o "$temp_file"
tar -xzf "$temp_file" -C bin
chmod +x bin/gotestsum
rm -f "$temp_file"

# gotestsum configure to only show logs for failed tests, json file for detailed logs
# Run the tests! Yey!
test_exit_code=0
./bin/gotestsum --format testdox --junitfile "${BUILD_SRC_DIR}/e2e/report.xml" --jsonfile "${BUILD_SRC_DIR}/e2e/test-log.json" -- -parallel 60 -timeout "${E2E_GO_TEST_TIMEOUT}" || test_exit_code=$?

# Upload test results as Azure DevOps artifacts
echo "##vso[artifact.upload containerfolder=test-results;artifactname=e2e-test-log]${BUILD_SRC_DIR}/e2e/test-log.json"

exit $test_exit_code
