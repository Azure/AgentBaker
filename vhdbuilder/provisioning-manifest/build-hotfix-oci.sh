#!/bin/bash
# build-hotfix-oci.sh — Builds a provisioning script hotfix OCI artifact
# and pushes it to a container registry using ORAS.
#
# Each hotfix targets exactly ONE baked VHD version because a script with
# the same filename can differ between VHD versions. If a bug spans
# multiple VHD versions, publish a separate hotfix artifact per version
# (each built from the correct source commit for that version).
#
# There is exactly ONE hotfix tag per version. If you need to add more
# fixed files, rebuild the hotfix with all files included — the tag
# gets overwritten in the registry.
#
# Usage:
#   ./build-hotfix-oci.sh \
#     --sku ubuntu-2204 \
#     --affected-version v0.20260201.0 \
#     --description "Fix for CVE-2026-XXXX in provision_source.sh" \
#     --files "parts/linux/cloud-init/artifacts/cse_helpers.sh" \
#     --registry myacr.azurecr.io
#
# Tag: <affected-version>-hotfix  (e.g., v0.20260201.0-hotfix)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MANIFEST_FILE="${SCRIPT_DIR}/manifest.json"

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Required:
  --sku <sku>               Target OS SKU (e.g., ubuntu-2204)
  --affected-version <ver>  The exact baked VHD version this hotfix targets (e.g., v0.20260201.0)
  --description <text>      Description of the hotfix
  --files <paths>           Comma-separated list of source file paths to include
                            (relative to repo root, e.g., parts/linux/cloud-init/artifacts/cse_helpers.sh)

Optional:
  --registry <registry>     Container registry to push to (default: \$REGISTRY or empty for local-only build)
  --dry-run                 Build artifact locally without pushing
  -h, --help                Show this help message

Tag format: <affected-version>-hotfix  (e.g., v0.20260201.0-hotfix)
To update a hotfix, re-run with all files included — the tag is overwritten.
EOF
    exit 1
}

SKU=""
AFFECTED_VERSION=""
DESCRIPTION=""
FILES=""
REGISTRY="${REGISTRY:-}"
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --sku)              SKU="$2"; shift 2 ;;
        --affected-version) AFFECTED_VERSION="$2"; shift 2 ;;
        --description)      DESCRIPTION="$2"; shift 2 ;;
        --files)            FILES="$2"; shift 2 ;;
        --registry)         REGISTRY="$2"; shift 2 ;;
        --dry-run)          DRY_RUN=true; shift ;;
        -h|--help)          usage ;;
        *)                  echo "Unknown option: $1"; usage ;;
    esac
done

# Validate required parameters
for var in SKU AFFECTED_VERSION DESCRIPTION FILES; do
    if [ -z "${!var}" ]; then
        echo "ERROR: --$(echo "$var" | tr '[:upper:]' '[:lower:]' | tr '_' '-') is required"
        usage
    fi
done

# Single tag per version
HOTFIX_ID="${AFFECTED_VERSION}-hotfix"

if [ ! -f "$MANIFEST_FILE" ]; then
    echo "ERROR: Manifest file not found: $MANIFEST_FILE"
    exit 1
fi

# Validate SKU exists in manifest
if ! jq -e ".skus.\"${SKU}\"" "$MANIFEST_FILE" > /dev/null 2>&1; then
    echo "ERROR: SKU '${SKU}' not found in manifest. Available SKUs:"
    jq -r '.skus | keys[]' "$MANIFEST_FILE"
    exit 1
fi

REPOSITORY=$(jq -r ".skus.\"${SKU}\".repository" "$MANIFEST_FILE")

# Parse file list
IFS=',' read -ra FILE_ARRAY <<< "$FILES"

# Validate all files exist and are in the manifest
echo "=== Validating hotfix files ==="
for file in "${FILE_ARRAY[@]}"; do
    file=$(echo "$file" | xargs) # trim whitespace
    filepath="${REPO_ROOT}/${file}"
    if [ ! -f "$filepath" ]; then
        echo "ERROR: Source file not found: $filepath"
        exit 1
    fi

    dest=$(jq -r --arg src "$file" ".skus.\"${SKU}\".scripts[] | select(.source == \$src) | .destination" "$MANIFEST_FILE")
    if [ -z "$dest" ]; then
        echo "ERROR: Source file '${file}' not found in manifest for SKU '${SKU}'"
        echo "  Available sources:"
        jq -r ".skus.\"${SKU}\".scripts[].source" "$MANIFEST_FILE"
        exit 1
    fi
    echo "  ✓ ${file} → ${dest}"
done

# Create staging directory
STAGING_DIR=$(mktemp -d)
TARBALL_NAME="provisioning-scripts-${HOTFIX_ID}.tar.gz"
trap 'rm -rf "$STAGING_DIR"' EXIT

echo ""
echo "=== Building hotfix artifact ==="
echo "  SKU:              ${SKU}"
echo "  Hotfix ID:        ${HOTFIX_ID}"
echo "  Affected version: ${AFFECTED_VERSION}"
echo "  Description:      ${DESCRIPTION}"
echo ""

# Stage files at their destination paths
FILE_METADATA="[]"
for file in "${FILE_ARRAY[@]}"; do
    file=$(echo "$file" | xargs)
    filepath="${REPO_ROOT}/${file}"

    dest=$(jq -r --arg src "$file" ".skus.\"${SKU}\".scripts[] | select(.source == \$src) | .destination" "$MANIFEST_FILE")
    perms=$(jq -r --arg src "$file" ".skus.\"${SKU}\".scripts[] | select(.source == \$src) | .permissions" "$MANIFEST_FILE")

    # Create destination directory structure in staging (strip leading /)
    dest_relative="${dest#/}"
    dest_dir=$(dirname "${STAGING_DIR}/root/${dest_relative}")
    mkdir -p "$dest_dir"

    cp "$filepath" "${STAGING_DIR}/root/${dest_relative}"
    chmod "$perms" "${STAGING_DIR}/root/${dest_relative}"

    file_sha256=$(sha256sum "$filepath" | awk '{print $1}')
    echo "  Staged: ${dest} (${perms}, sha256:${file_sha256:0:16}...)"

    FILE_METADATA=$(echo "$FILE_METADATA" | jq \
        --arg path "$dest" \
        --arg sha "$file_sha256" \
        --arg perms "$perms" \
        '. + [{"path": $path, "sha256": $sha, "permissions": $perms}]')
done

# Create tarball rooted so extraction with -C / places files correctly
echo ""
echo "=== Creating tarball ==="
(cd "${STAGING_DIR}/root" && tar -czf "${STAGING_DIR}/${TARBALL_NAME}" .)
tarball_size=$(stat --printf="%s" "${STAGING_DIR}/${TARBALL_NAME}" 2>/dev/null || stat -f%z "${STAGING_DIR}/${TARBALL_NAME}")
tarball_sha256=$(sha256sum "${STAGING_DIR}/${TARBALL_NAME}" | awk '{print $1}')
echo "  ${TARBALL_NAME}: ${tarball_size} bytes, sha256:${tarball_sha256:0:16}..."

# Generate hotfix metadata
echo ""
echo "=== Generating metadata ==="
SOURCE_COMMIT=$(cd "$REPO_ROOT" && git rev-parse HEAD 2>/dev/null || echo "unknown")

cat > "${STAGING_DIR}/hotfix-metadata.json" <<EOF
{
  "hotfixId": "${HOTFIX_ID}",
  "sku": "${SKU}",
  "affectedVersion": "${AFFECTED_VERSION}",
  "description": "${DESCRIPTION}",
  "sourceCommit": "${SOURCE_COMMIT}",
  "tarballSha256": "${tarball_sha256}",
  "files": $(echo "$FILE_METADATA" | jq '.')
}
EOF

echo "  hotfix-metadata.json generated"
cat "${STAGING_DIR}/hotfix-metadata.json" | jq .

if [ "$DRY_RUN" = true ]; then
    echo ""
    echo "=== Dry run — skipping push ==="
    echo "  Artifacts staged in: ${STAGING_DIR}"
    echo "  Tarball: ${STAGING_DIR}/${TARBALL_NAME}"
    echo "  Metadata: ${STAGING_DIR}/hotfix-metadata.json"
    # Don't clean up staging dir on dry run so user can inspect
    trap '' EXIT
    exit 0
fi

if [ -z "$REGISTRY" ]; then
    echo ""
    echo "ERROR: --registry is required for push (or use --dry-run)"
    exit 1
fi

# Push to registry using ORAS
echo ""
echo "=== Pushing to registry ==="
ARTIFACT_REF="${REGISTRY}/${REPOSITORY}:${HOTFIX_ID}"

oras push "$ARTIFACT_REF" \
    --artifact-type "application/vnd.aks.provisioning-scripts.hotfix.v1" \
    --annotation "aks.provisioning-scripts.affected-version=${AFFECTED_VERSION}" \
    --annotation "aks.provisioning-scripts.hotfix-id=${HOTFIX_ID}" \
    --annotation "aks.provisioning-scripts.description=${DESCRIPTION}" \
    --annotation "org.opencontainers.image.source=https://github.com/Azure/AgentBaker" \
    --annotation "org.opencontainers.image.revision=${SOURCE_COMMIT}" \
    "${STAGING_DIR}/${TARBALL_NAME}:application/gzip" \
    "${STAGING_DIR}/hotfix-metadata.json:application/vnd.aks.provisioning-scripts.metadata+json"

echo ""
echo "=== Hotfix published successfully ==="
echo "  Artifact: ${ARTIFACT_REF}"
echo "  Affected VHD version: ${AFFECTED_VERSION}"
echo ""
echo "Nodes with baked version ${AFFECTED_VERSION} will auto-detect and apply the hotfix at provisioning time."
