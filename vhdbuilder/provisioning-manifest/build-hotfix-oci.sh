#!/bin/bash
# build-hotfix-oci.sh — Build and push a provisioning script hotfix as an OCI artifact.
#
# Usage:
#   ./build-hotfix-oci.sh \
#     --sku ubuntu-2204 \
#     --affected-version 202602.10.0 \
#     --description "Fix for CVE-2026-XXXX in provision_source.sh" \
#     --files "parts/linux/cloud-init/artifacts/cse_helpers.sh" \
#     [--registry hotfixscriptpoc.azurecr.io] \
#     [--dry-run]
#
# The hotfix tag is auto-generated: <affected-version>-hotfix
# The affected-version should match the IMAGE_VERSION (VHD SIG version)
# stamped during VHD build (e.g. 202602.10.0).
# Re-running with the same version overwrites the existing tag.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MANIFEST_FILE="${SCRIPT_DIR}/manifest.json"

# Defaults
REGISTRY="hotfixscriptpoc.azurecr.io"
DRY_RUN=false
SKU=""
AFFECTED_VERSION=""
DESCRIPTION=""
FILES=""

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Build and push a provisioning script hotfix as an OCI artifact.

Required:
  --sku <sku>                 Target OS SKU (e.g., ubuntu-2204)
  --affected-version <ver>    Baked VHD image version to hotfix (e.g., 202602.10.0)
  --description <desc>        Human-readable description of the hotfix
  --files <paths>             Comma-separated source paths of changed files
                              (relative to repo root, e.g., parts/linux/cloud-init/artifacts/cse_helpers.sh)

Optional:
  --registry <registry>       Target container registry (default: hotfixscriptpoc.azurecr.io)
  --dry-run                   Build artifact locally but do not push to registry

Examples:
  # Build and push a hotfix for a single file
  $(basename "$0") \\
    --sku ubuntu-2204 \\
    --affected-version 202602.10.0 \\
    --description "Fix CVE-2026-XXXX in provision_source.sh" \\
    --files "parts/linux/cloud-init/artifacts/cse_helpers.sh"

  # Dry-run (no push) with multiple files
  $(basename "$0") \\
    --sku ubuntu-2204 \\
    --affected-version 202602.10.0 \\
    --description "Fix provisioning regression" \\
    --files "parts/linux/cloud-init/artifacts/cse_helpers.sh,parts/linux/cloud-init/artifacts/cse_install.sh" \\
    --dry-run
EOF
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --sku) SKU="$2"; shift 2 ;;
        --affected-version) AFFECTED_VERSION="$2"; shift 2 ;;
        --description) DESCRIPTION="$2"; shift 2 ;;
        --files) FILES="$2"; shift 2 ;;
        --registry) REGISTRY="$2"; shift 2 ;;
        --dry-run) DRY_RUN=true; shift ;;
        -h|--help) usage ;;
        *) echo "ERROR: Unknown option: $1"; usage ;;
    esac
done

# Validate required arguments
if [[ -z "$SKU" || -z "$AFFECTED_VERSION" || -z "$DESCRIPTION" || -z "$FILES" ]]; then
    echo "ERROR: Missing required arguments."
    usage
fi

# Validate manifest exists
if [[ ! -f "$MANIFEST_FILE" ]]; then
    echo "ERROR: Manifest not found at ${MANIFEST_FILE}"
    exit 1
fi

# Validate SKU exists in manifest
if ! jq -e ".skus[\"${SKU}\"]" "$MANIFEST_FILE" > /dev/null 2>&1; then
    echo "ERROR: SKU '${SKU}' not found in manifest."
    echo "Available SKUs: $(jq -r '.skus | keys[]' "$MANIFEST_FILE" | tr '\n' ', ')"
    exit 1
fi

# Validate oras is available
if ! command -v oras &>/dev/null; then
    echo "ERROR: 'oras' CLI not found. Install from https://oras.land/"
    exit 1
fi

HOTFIX_TAG="${AFFECTED_VERSION}-hotfix"
REPOSITORY=$(jq -r ".skus[\"${SKU}\"].repository" "$MANIFEST_FILE")
ARTIFACT_REF="${REGISTRY}/${REPOSITORY}:${HOTFIX_TAG}"
ARTIFACT_TYPE="application/vnd.aks.provisioning-scripts.hotfix.v1"
SOURCE_COMMIT=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo "unknown")

STAGING_DIR=$(mktemp -d)
OUTPUT_DIR=$(mktemp -d)
trap 'rm -rf "$STAGING_DIR" "$OUTPUT_DIR"' EXIT

echo "=== Provisioning Script Hotfix Builder ==="
echo "SKU:              ${SKU}"
echo "Affected Version: ${AFFECTED_VERSION}"
echo "Hotfix Tag:       ${HOTFIX_TAG}"
echo "Registry:         ${REGISTRY}"
echo "Repository:       ${REPOSITORY}"
echo "Artifact Ref:     ${ARTIFACT_REF}"
echo "Dry Run:          ${DRY_RUN}"
echo "Source Commit:    ${SOURCE_COMMIT}"
echo ""

# Split files into array
IFS=',' read -ra FILE_LIST <<< "$FILES"

# Build file inventory: validate each file exists and map to destination
declare -A FILE_MAP
METADATA_FILES_JSON="["
FIRST=true

for src_file in "${FILE_LIST[@]}"; do
    src_file=$(echo "$src_file" | xargs) # trim whitespace
    src_path="${REPO_ROOT}/${src_file}"

    if [[ ! -f "$src_path" ]]; then
        echo "ERROR: Source file not found: ${src_path}"
        exit 1
    fi

    # Look up destination in manifest
    dest=$(jq -r ".skus[\"${SKU}\"].scripts[] | select(.source == \"${src_file}\") | .destination" "$MANIFEST_FILE")
    if [[ -z "$dest" || "$dest" == "null" ]]; then
        echo "ERROR: Source file '${src_file}' not found in manifest for SKU '${SKU}'."
        echo "Valid source files for ${SKU}:"
        jq -r ".skus[\"${SKU}\"].scripts[].source" "$MANIFEST_FILE"
        exit 1
    fi

    perms=$(jq -r ".skus[\"${SKU}\"].scripts[] | select(.source == \"${src_file}\") | .permissions" "$MANIFEST_FILE")
    FILE_MAP["$src_file"]="$dest"

    # Copy file to staging at destination path
    staging_dest="${STAGING_DIR}${dest}"
    mkdir -p "$(dirname "$staging_dest")"
    cp "$src_path" "$staging_dest"
    chmod "$perms" "$staging_dest"

    # Compute SHA256
    file_sha256=$(sha256sum "$src_path" | awk '{print $1}')

    echo "  ✓ ${src_file} → ${dest} (sha256: ${file_sha256:0:16}...)"

    # Build JSON entry
    if [[ "$FIRST" == "true" ]]; then
        FIRST=false
    else
        METADATA_FILES_JSON+=","
    fi
    METADATA_FILES_JSON+="{\"source\":\"${src_file}\",\"destination\":\"${dest}\",\"sha256\":\"${file_sha256}\"}"
done
METADATA_FILES_JSON+="]"

echo ""
echo "Creating tarball..."

TARBALL_NAME="provisioning-scripts-${HOTFIX_TAG}.tar.gz"
TARBALL_PATH="${OUTPUT_DIR}/${TARBALL_NAME}"

# Create tarball from the staging directory (paths are absolute from /)
tar -czf "$TARBALL_PATH" -C "$STAGING_DIR" .

TARBALL_SHA256=$(sha256sum "$TARBALL_PATH" | awk '{print $1}')
TARBALL_SIZE=$(stat --format='%s' "$TARBALL_PATH" 2>/dev/null || stat -f '%z' "$TARBALL_PATH")

echo "  Tarball: ${TARBALL_NAME} (${TARBALL_SIZE} bytes, sha256: ${TARBALL_SHA256:0:16}...)"

# Generate metadata
METADATA_PATH="${OUTPUT_DIR}/hotfix-metadata.json"
cat > "$METADATA_PATH" <<EOF
{
  "hotfixId": "${HOTFIX_TAG}",
  "affectedVersion": "${AFFECTED_VERSION}",
  "sku": "${SKU}",
  "description": "${DESCRIPTION}",
  "sourceCommit": "${SOURCE_COMMIT}",
  "createdAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "tarballSha256": "${TARBALL_SHA256}",
  "files": ${METADATA_FILES_JSON}
}
EOF

echo ""
echo "Metadata:"
jq . "$METADATA_PATH"

if [[ "$DRY_RUN" == "true" ]]; then
    echo ""
    echo "=== DRY RUN — artifact built but not pushed ==="
    echo "Tarball:  ${TARBALL_PATH}"
    echo "Metadata: ${METADATA_PATH}"

    # Copy artifacts to Build.StagingDirectory if set (for ADO pipeline)
    if [[ -n "${BUILD_STAGINGDIRECTORY:-}" ]]; then
        mkdir -p "${BUILD_STAGINGDIRECTORY}/hotfix-artifact"
        cp "$TARBALL_PATH" "${BUILD_STAGINGDIRECTORY}/hotfix-artifact/"
        cp "$METADATA_PATH" "${BUILD_STAGINGDIRECTORY}/hotfix-artifact/"
        echo "Artifacts copied to ${BUILD_STAGINGDIRECTORY}/hotfix-artifact/"
    fi
    exit 0
fi

echo ""
echo "Pushing to ${ARTIFACT_REF}..."

pushd "$OUTPUT_DIR" > /dev/null
oras push "$ARTIFACT_REF" \
    --artifact-type "$ARTIFACT_TYPE" \
    --annotation "aks.provisioning-scripts.affected-version=${AFFECTED_VERSION}" \
    --annotation "aks.provisioning-scripts.hotfix-id=${HOTFIX_TAG}" \
    --annotation "aks.provisioning-scripts.sku=${SKU}" \
    --annotation "aks.provisioning-scripts.description=${DESCRIPTION}" \
    --annotation "aks.provisioning-scripts.source-commit=${SOURCE_COMMIT}" \
    "${TARBALL_NAME}:application/gzip" \
    "hotfix-metadata.json:application/vnd.aks.provisioning-scripts.metadata+json"
popd > /dev/null

echo ""
echo "=== Hotfix published successfully ==="
echo "Artifact: ${ARTIFACT_REF}"
echo ""
echo "Nodes running VHD version '${AFFECTED_VERSION}' will automatically"
echo "detect and apply this hotfix at next provisioning."
