#!/usr/bin/env bash
set -o nounset
set -o pipefail
set -o errexit

# Build-time provisioner for AKS AIManager custom VHDs.
#
# Consumes a per-model, date-versioned manifest from
# vhdbuilder/release-notes/AKSAIManager/<model-slug>/<version>.json and bakes the
# content for ONE (gpu, os) variant into the OS disk:
#   - the variant's dependency packages (name + version), each baked at its
#     declared bakePath (the list may be empty for models that need nothing extra)
#   - HuggingFace model weights under /opt/kaito/models/<model.slug>
#   - /opt/kaito/manifest.json describing what was baked (read by KAITO at runtime)
#
# The node boots this VHD with an ephemeral OS disk on local NVMe, so the baked
# payload ends up on fast local storage. This is the reusable baking logic; the
# internal KAITO custom-VHD capture pipeline invokes it after selecting a variant.

AIMANAGER_MANIFEST="${AIMANAGER_MANIFEST:-/home/packer/aimanager-model.json}"
AIMANAGER_GPU="${AIMANAGER_GPU:-}"
AIMANAGER_OS_SKU="${AIMANAGER_OS_SKU:-}"
KAITO_ROOT="${KAITO_ROOT:-/opt/kaito}"
KAITO_MANIFEST_PATH="${KAITO_MANIFEST_PATH:-${KAITO_ROOT}/manifest.json}"
# HF_TOKEN is consumed only for gated repos and is never written into the image.

# Function definitions used in this file.
# Functions defined until "${__SOURCED__:+return}" are sourced and unit-tested in
# spec/parts/vhdbuilder/packer/install-aimanager-content_spec.sh.
# -------------------------------------------------------------------------------------------------
isUbuntu() {
    grep -q '^ID=ubuntu' /etc/os-release 2>/dev/null
}

retrycmd() {
    local retries="$1"
    local wait_sleep="$2"
    shift 2
    local i
    for i in $(seq 1 "$retries"); do
        "$@" && return 0
        if [ "$i" -eq "$retries" ]; then
            return 1
        fi
        sleep "$wait_sleep"
    done
}

# Echoes the first variant object matching (gpu, os), or nothing.
select_variant() {
    local manifest="$1"
    local gpu="$2"
    local os_sku="$3"
    jq -c --arg g "$gpu" --arg o "$os_sku" '
        [ .variants[] | select(.gpuSku == $g and .osSku == $o) ] | first // empty' "$manifest"
}

# Enable an apt source by installing its repository keyring .deb (e.g. the NVIDIA CUDA repo).
setup_apt_repo_from_keyring() {
    local keyring_url="$1"
    local tmp_deb
    tmp_deb=$(mktemp --suffix=.deb)
    echo "enabling apt repo via keyring ${keyring_url}"
    retrycmd 5 10 curl -fsSL -o "$tmp_deb" "$keyring_url"
    dpkg -i "$tmp_deb"
    rm -f "$tmp_deb"
    retrycmd 5 10 apt-get update
}

install_dependencies() {
    local variant="$1"
    local count
    count=$(echo "$variant" | jq '.dependencies | length')
    if [ "$count" -eq 0 ]; then
        echo "no dependencies to bake for this variant"
        return 0
    fi
    if ! isUbuntu; then
        echo "AIManager MVP installs dependencies via apt (Ubuntu only)" >&2
        return 1
    fi

    export DEBIAN_FRONTEND=noninteractive
    retrycmd 5 10 apt-get update

    local i name version repo_keyring_url source_path bake_path
    for i in $(seq 0 $((count - 1))); do
        name=$(echo "$variant" | jq -r ".dependencies[$i].name")
        version=$(echo "$variant" | jq -r ".dependencies[$i].version")
        repo_keyring_url=$(echo "$variant" | jq -r ".dependencies[$i].repoKeyringUrl // empty")
        source_path=$(echo "$variant" | jq -r ".dependencies[$i].sourcePath // empty")
        bake_path=$(echo "$variant" | jq -r ".dependencies[$i].bakePath")

        # Enable the package's apt source first when a repo keyring is declared.
        if [ -n "$repo_keyring_url" ]; then
            setup_apt_repo_from_keyring "$repo_keyring_url"
        fi

        echo "installing dependency ${name} (version ${version}) baked at ${bake_path}"
        retrycmd 5 10 apt-get install -y "$name"

        # Relocate to the declared bake path when the package installs elsewhere.
        if [ -n "$source_path" ] && [ "$source_path" != "$bake_path" ]; then
            mkdir -p "$bake_path"
            cp -a "${source_path}/." "$bake_path/"
        fi
    done
}

download_weights() {
    local manifest="$1"
    local hf_repo hf_rev gated baked_path
    hf_repo=$(jq -r '.model.hfRepo' "$manifest")
    hf_rev=$(jq -r '.model.hfRevision' "$manifest")
    gated=$(jq -r '.model.gated' "$manifest")
    baked_path=$(jq -r '.model.bakePath' "$manifest")

    mkdir -p "$baked_path"
    # hf_transfer gives high-throughput multipart downloads for large weight files.
    export HF_HUB_ENABLE_HF_TRANSFER=1

    echo "downloading weights ${hf_repo}@${hf_rev} into ${baked_path}"
    if [ "$gated" = "true" ]; then
        if [ -z "${HF_TOKEN:-}" ]; then
            echo "repo ${hf_repo} is gated but HF_TOKEN is not set" >&2
            return 1
        fi
        retrycmd 5 30 hf download "$hf_repo" --revision "$hf_rev" --local-dir "$baked_path" --token "$HF_TOKEN"
    else
        retrycmd 5 30 hf download "$hf_repo" --revision "$hf_rev" --local-dir "$baked_path"
    fi
    # World-readable so the inference runtime can load weights as any UID.
    chmod -R a+rX "$baked_path"
}

write_kaito_manifest() {
    local manifest="$1"
    local variant="$2"
    local slug hf_repo hf_rev model_dir deps
    slug=$(jq -r '.model.slug' "$manifest")
    hf_repo=$(jq -r '.model.hfRepo' "$manifest")
    hf_rev=$(jq -r '.model.hfRevision' "$manifest")
    model_dir=$(jq -r '.model.bakePath' "$manifest")
    deps=$(echo "$variant" | jq -c '[.dependencies[] | {name, version, bakePath}]')

    mkdir -p "$(dirname "$KAITO_MANIFEST_PATH")"
    jq -n \
        --arg slug "$slug" \
        --arg repo "$hf_repo" \
        --arg rev "$hf_rev" \
        --arg model_dir "$model_dir" \
        --argjson deps "$deps" \
        '{
           model: $slug,
           hfRepo: $repo,
           hfRevision: $rev,
           modelDir: $model_dir,
           dependencies: $deps
         }' >"$KAITO_MANIFEST_PATH"
    chmod a+r "$KAITO_MANIFEST_PATH"
    echo "wrote KAITO manifest to ${KAITO_MANIFEST_PATH}"
}

# Fail-closed validation: never publish a partial image.
validate_baked_content() {
    local manifest="$1"
    local variant="$2"
    local baked_path count i name bake_path expected actual
    baked_path=$(jq -r '.model.bakePath' "$manifest")

    # Each declared dependency must be present at its bake path.
    count=$(echo "$variant" | jq '.dependencies | length')
    for i in $(seq 0 $((count - 1))); do
        name=$(echo "$variant" | jq -r ".dependencies[$i].name")
        bake_path=$(echo "$variant" | jq -r ".dependencies[$i].bakePath")
        if [ ! -e "$bake_path" ]; then
            echo "dependency ${name} missing at ${bake_path}" >&2
            return 1
        fi
    done

    if [ ! -f "$KAITO_MANIFEST_PATH" ]; then
        echo "missing ${KAITO_MANIFEST_PATH}" >&2
        return 1
    fi
    if [ ! -d "$baked_path" ]; then
        echo "missing model dir ${baked_path}" >&2
        return 1
    fi

    actual=$(find "$baked_path" -type f | wc -l | tr -d ' ')
    if [ "$actual" -eq 0 ]; then
        echo "no model files found under ${baked_path}" >&2
        return 1
    fi
    expected=$(jq -r '.model.expectedFileCount // empty' "$manifest")
    if [ -n "$expected" ] && [ "$actual" -ne "$expected" ]; then
        echo "model file count mismatch under ${baked_path}: expected ${expected}, found ${actual}" >&2
        return 1
    fi
    echo "validation passed: ${count} dependencies present, ${actual} model files under ${baked_path}"
}

main() {
    if [ -z "$AIMANAGER_GPU" ] || [ -z "$AIMANAGER_OS_SKU" ]; then
        echo "AIMANAGER_GPU and AIMANAGER_OS_SKU must both be set" >&2
        exit 1
    fi
    if [ ! -f "$AIMANAGER_MANIFEST" ]; then
        echo "AIManager manifest not found at ${AIMANAGER_MANIFEST}" >&2
        exit 1
    fi

    local variant
    variant=$(select_variant "$AIMANAGER_MANIFEST" "$AIMANAGER_GPU" "$AIMANAGER_OS_SKU")
    if [ -z "$variant" ]; then
        echo "no variant matches gpu=${AIMANAGER_GPU} os=${AIMANAGER_OS_SKU} in ${AIMANAGER_MANIFEST}" >&2
        exit 1
    fi
    echo "baking variant gpu=${AIMANAGER_GPU} os=${AIMANAGER_OS_SKU} from $(jq -r '.model.slug' "$AIMANAGER_MANIFEST")"

    install_dependencies "$variant"
    download_weights "$AIMANAGER_MANIFEST"
    write_kaito_manifest "$AIMANAGER_MANIFEST" "$variant"
    validate_baked_content "$AIMANAGER_MANIFEST" "$variant"
    echo "AIManager content baked successfully"
}

${__SOURCED__:+return}
# --------------------------------------- Main Execution starts here --------------------------------------------------

main "$@"
