#!/bin/bash
set -euxo pipefail

[ -z "${SUBSCRIPTION_ID:-}" ] && echo "SUBSCRIPTION_ID must be set" && exit 1

SKIP_TAG_NAME="gc.skip"
SKIP_TAG_VALUE="true"

DRY_RUN="${DRY_RUN:-}"

STANDARD_DEADLINE=$(( $(date +%s) - 14400 )) # 4 hours ago
WEEK_AGO=$(( $(date +%s) - 604800 )) # 7 days ago

SIG_GALLERY_NAME="PackerSigGalleryEastUS"
SIG_RESOURCE_GROUP="aksvhdtestbuildrg"

function main() {
    az account set -s "$SUBSCRIPTION_ID"

    echo "garbage collecting ephemeral resource groups..."
    cleanup_rgs || echo "resource group cleanup failed, continuing..."

    echo "garbage collecting SIG image versions..."
    cleanup_sig_versions || echo "SIG image version cleanup failed, continuing..."

    echo "garbage collecting managed images..."
    cleanup_managed_images || echo "managed image cleanup failed, continuing..."

    echo "garbage collecting old storage accounts..."
    cleanup_storage_accounts || echo "storage account cleanup failed, continuing..."
}

function cleanup_rgs() {
    groups=$(az group list | jq -r --arg dl $STANDARD_DEADLINE '.[] | select(.name | test("vhd-test*|vhd-scanning*|pkr-Resource-Group*|^image-builder-*")) | select(.tags.now < $dl).name'  | tr -d '\"' || true)
    if [ -z "$groups" ]; then
        echo "no resource groups found for garbage collection"
        return 0
    fi

    for group in $groups; do
        echo "resource group $group is in-scope for garbage collection"
        group_object=$(az group show -g "$group")
        tag_value=$(echo "$group_object" | jq -r --arg skipTagName $SKIP_TAG_NAME '.tags."\($skipTagName)"')

        if [ "${tag_value,,}" = "$SKIP_TAG_VALUE" ]; then
            now=$(echo "$group_object" | jq -r '.tags.now')
            if [ "$now" != "null" ] && [ "$now" -lt "$WEEK_AGO" ]; then
                echo "resource group $group is tagged with $SKIP_TAG_NAME=$SKIP_TAG_VALUE but is more than 7 days old, will attempt to delete..."
                delete_group "$group" || return $?
            fi
            continue
        fi

        echo "will attempt to delete resource group $group"
        delete_group "$group" || return $?
    done
}

function delete_group() {
    local group=$1

    if [ "${DRY_RUN,,}" = "true" ]; then
        echo "DRY_RUN: az group delete -g $group --yes --no-wait"
        return 0
    fi

    if ! az group delete -g "$group" --yes --no-wait; then
        echo "failed to delete resource group: ${group}, continuing..."
    fi
}

function cleanup_sig_versions() {
    local cutoff_date
    cutoff_date=$(date -u -d "7 days ago" +%Y-%m-%dT%H:%M:%S 2>/dev/null || date -u -v-7d +%Y-%m-%dT%H:%M:%S)

    echo "deleting SIG image versions published before ${cutoff_date} from ${SIG_RESOURCE_GROUP}/${SIG_GALLERY_NAME}"

    # collect locks: RG-level locks AND locks on gallery child resources
    local locked_ids
    locked_ids=$(az lock list -g "$SIG_RESOURCE_GROUP" --query "[].id" -o tsv 2>/dev/null || true)
    local gallery_locks
    gallery_locks=$(az lock list --resource-group "$SIG_RESOURCE_GROUP" \
        --resource "$SIG_GALLERY_NAME" --resource-type "Microsoft.Compute/galleries" \
        --query "[].id" -o tsv 2>/dev/null || true)
    locked_ids=$(printf '%s\n%s' "$locked_ids" "$gallery_locks")

    local image_definitions
    image_definitions=$(az sig image-definition list -g "$SIG_RESOURCE_GROUP" -r "$SIG_GALLERY_NAME" --query "[].name" -o tsv 2>/dev/null)
    if [ -z "$image_definitions" ]; then
        echo "no image definitions found"
        return 0
    fi

    local total_deleted=0
    for def in $image_definitions; do
        local version_ids
        version_ids=$(az sig image-version list -g "$SIG_RESOURCE_GROUP" -r "$SIG_GALLERY_NAME" -i "$def" \
            --query "[?publishingProfile.publishedDate < '${cutoff_date}' && provisioningState=='Succeeded'].{name:name, id:id, skip:tags.\"gc.skip\"}" -o json 2>/dev/null)

        local eligible_ids
        eligible_ids=$(echo "$version_ids" | jq -r '.[] | select(.skip != "true") | .id' 2>/dev/null || true)
        if [ -z "$eligible_ids" ]; then
            continue
        fi

        # filter out locked resources
        local delete_ids=""
        local count=0
        for vid in $eligible_ids; do
            if echo "$locked_ids" | grep -q "$vid" 2>/dev/null; then
                echo "skipping locked version: $vid"
                continue
            fi
            delete_ids="${delete_ids} ${vid}"
            count=$((count + 1))
        done

        if [ -z "$delete_ids" ]; then
            continue
        fi

        echo "deleting ${count} versions from ${def}"
        if [ "${DRY_RUN,,}" = "true" ]; then
            echo "DRY_RUN: would delete ${count} versions from ${def}"
            continue
        fi

        # delete in batches of 20 to avoid command line limits
        echo "$delete_ids" | xargs -n 20 | while read -r batch; do
            # shellcheck disable=SC2086
            az resource delete --ids $batch 2>/dev/null || echo "failed to delete some versions from ${def}, continuing..."
        done
        total_deleted=$((total_deleted + count))
    done

    echo "SIG cleanup complete: deleted ${total_deleted} image versions"
}

function cleanup_managed_images() {
    echo "deleting managed images older than 7 days from ${SIG_RESOURCE_GROUP}"

    local cutoff_epoch=$WEEK_AGO
    local delete_ids=""
    local count=0

    # images with tags.now (Linux build images)
    local tagged_ids
    tagged_ids=$(az image list -g "$SIG_RESOURCE_GROUP" \
        --query "[?tags.now < '${cutoff_epoch}'].id" -o tsv 2>/dev/null || true)

    # images without tags (e.g. imported Windows base images) — use timeCreated
    local cutoff_date
    cutoff_date=$(date -u -d "7 days ago" +%Y-%m-%dT%H:%M:%S 2>/dev/null || date -u -v-7d +%Y-%m-%dT%H:%M:%S)
    local untagged_ids
    untagged_ids=$(az image list -g "$SIG_RESOURCE_GROUP" \
        --query "[?tags.now==null && timeCreated < '${cutoff_date}'].id" -o tsv 2>/dev/null || true)

    delete_ids="${tagged_ids} ${untagged_ids}"
    delete_ids=$(echo "$delete_ids" | xargs)  # trim whitespace

    if [ -z "$delete_ids" ]; then
        echo "no managed images eligible for deletion"
        return 0
    fi

    count=$(echo "$delete_ids" | wc -w)
    echo "found ${count} managed images eligible for deletion"

    if [ "${DRY_RUN,,}" = "true" ]; then
        echo "DRY_RUN: would delete ${count} managed images"
        return 0
    fi

    echo "$delete_ids" | xargs -n 20 | while read -r batch; do
        # shellcheck disable=SC2086
        az resource delete --ids $batch 2>/dev/null || echo "failed to delete some managed images, continuing..."
    done

    echo "managed image cleanup complete: deleted ${count} images"
}

function cleanup_storage_accounts() {
    echo "deleting aksimages* storage accounts older than 4 hours from ${SIG_RESOURCE_GROUP}"

    local storage_deadline=$STANDARD_DEADLINE
    local delete_ids
    delete_ids=$(az storage account list -g "$SIG_RESOURCE_GROUP" \
        --query "[?starts_with(name, 'aksimages') && tags.now < '${storage_deadline}'].id" -o tsv 2>/dev/null || true)

    if [ -z "$delete_ids" ]; then
        echo "no storage accounts eligible for deletion"
        return 0
    fi

    local count
    count=$(echo "$delete_ids" | wc -w)
    echo "found ${count} storage accounts eligible for deletion"

    if [ "${DRY_RUN,,}" = "true" ]; then
        echo "DRY_RUN: would delete ${count} storage accounts"
        return 0
    fi

    for sa_id in $delete_ids; do
        az storage account delete --yes --ids "$sa_id" 2>/dev/null || echo "failed to delete storage account ${sa_id}, continuing..."
    done

    echo "storage account cleanup complete"
}

main "$@"
