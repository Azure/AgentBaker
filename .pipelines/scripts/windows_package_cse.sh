#!/bin/bash
set -euo pipefail
# Don't echo commands to the console, as this will cause Azure DevOps to do odd things with setvariable.
set +x

# This script packages the CSE for Windows using scripts from the signing pipeline.

: "${CSE_RELEASE_DIR:?CSE_RELEASE_DIR must be set}"
: "${CSE_PUBLISH_DIR:?CSE_PUBLISH_DIR must be set}"
: "${CSE_FILE_NAME:?CSE_FILE_NAME must be set}"
: "${CSE_SIGNING_BUILD_ID:?CSE_SIGNING_BUILD_ID must be set}"
: "${SYSTEM_COLLECTIONURI:?SYSTEM_COLLECTIONURI must be set}"
: "${SYSTEM_TEAMPROJECTID:?SYSTEM_TEAMPROJECTID must be set}"
: "${AZURE_DEVOPS_EXT_PAT:?AZURE_DEVOPS_EXT_PAT must be set}"

signed_artifacts_dir="$(mktemp -d)"
trap 'rm -rf -- "$signed_artifacts_dir"' EXIT

echo "Downloading signed PowerShell scripts from run $CSE_SIGNING_BUILD_ID"
az pipelines runs artifact download \
  --organization "$SYSTEM_COLLECTIONURI" \
  --project "$SYSTEM_TEAMPROJECTID" \
  --run-id "$CSE_SIGNING_BUILD_ID" \
  --artifact-name signed-powershell-scripts \
  --path "$signed_artifacts_dir" \
  --only-show-errors

echo "Creating CSE release pacakge $CSE_PUBLISH_DIR/$CSE_FILE_NAME"
mkdir -p "$CSE_RELEASE_DIR"
cp -r ./staging/cse/windows/* "$CSE_RELEASE_DIR"
rm "$CSE_RELEASE_DIR"/*.tests.ps1
rm "$CSE_RELEASE_DIR"/*.tests.suites -r
rm "$CSE_RELEASE_DIR"/README
rm "$CSE_RELEASE_DIR"/debug/update-scripts.ps1

script_count=0
while IFS= read -r -d '' script; do
  relative_path="${script#"$CSE_RELEASE_DIR"/}"
  signed_script="$signed_artifacts_dir/staging/cse/windows/$relative_path"
  if [ ! -f "$signed_script" ]; then
    echo "Missing signed script in run $CSE_SIGNING_BUILD_ID: $relative_path" >&2
    exit 1
  fi
  cp "$signed_script" "$script"
  script_count=$((script_count + 1))
done < <(find "$CSE_RELEASE_DIR" -type f -name '*.ps1' -print0)
if [ "$script_count" -eq 0 ]; then
  echo "No PowerShell scripts found to package in $CSE_RELEASE_DIR" >&2
  exit 1
fi

mkdir -p "$CSE_PUBLISH_DIR"

pushd "$CSE_RELEASE_DIR" || exit 1
  zip -r "$CSE_PUBLISH_DIR/$CSE_FILE_NAME" ./*
popd || exit 1