#!/bin/bash
set -euo pipefail
# Don't echo commands to the console, as this will cause Azure DevOps to do odd things with setvariable.
set +x

# This script downloads the Windows CSE ZIP produced by the signing pipeline.

: "${CSE_PUBLISH_DIR:?CSE_PUBLISH_DIR must be set}"
: "${CSE_FILE_NAME:?CSE_FILE_NAME must be set}"
: "${CSE_SIGNING_BUILD_ID:?CSE_SIGNING_BUILD_ID must be set}"
: "${SYSTEM_COLLECTIONURI:?SYSTEM_COLLECTIONURI must be set}"
: "${SYSTEM_TEAMPROJECTID:?SYSTEM_TEAMPROJECTID must be set}"
: "${AZURE_DEVOPS_EXT_PAT:?AZURE_DEVOPS_EXT_PAT must be set}"

signed_artifacts_dir="$(mktemp -d)"
trap 'rm -rf -- "$signed_artifacts_dir"' EXIT

echo "Downloading signed Windows CSE package from run $CSE_SIGNING_BUILD_ID"
az pipelines runs artifact download \
  --organization "$SYSTEM_COLLECTIONURI" \
  --project "$SYSTEM_TEAMPROJECTID" \
  --run-id "$CSE_SIGNING_BUILD_ID" \
  --artifact-name windows-cse-package \
  --path "$signed_artifacts_dir" \
  --only-show-errors

signed_zip="$signed_artifacts_dir/aks-windows-cse-scripts-current.zip"
if [ ! -s "$signed_zip" ]; then
  echo "Missing or empty signed Windows CSE ZIP in run $CSE_SIGNING_BUILD_ID" >&2
  exit 1
fi

mkdir -p "$CSE_PUBLISH_DIR"
cp "$signed_zip" "$CSE_PUBLISH_DIR/$CSE_FILE_NAME"