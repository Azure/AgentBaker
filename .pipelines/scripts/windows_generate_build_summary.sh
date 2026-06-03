#!/bin/sh
# Generates a markdown build summary card for Windows VHD builds.
# Uploaded via ##vso[task.uploadsummary] to appear as an extension tab in ADO.
set -eu

SUMMARY_FILE="${SUMMARY_FILE:-build-summary.md}"
SETTINGS_FILE="${SETTINGS_FILE:-vhdbuilder/packer/settings.json}"
IMAGE_BOM_FILE="${IMAGE_BOM_FILE:-image-bom.json}"
RELEASE_NOTES_FILE="${RELEASE_NOTES_FILE:-release-notes.txt}"

# read_setting extracts a field from settings.json, returning "N/A" on failure.
read_setting() {
    _rs_key="$1"
    _rs_val=""
    if [ -f "$SETTINGS_FILE" ]; then
        _rs_val=$(jq -r ".[\"${_rs_key}\"] // empty" <"$SETTINGS_FILE" 2>/dev/null) || true
    fi
    if [ -z "${_rs_val:-}" ]; then
        echo "N/A"
    else
        echo "$_rs_val"
    fi
}

{
    echo "# Windows VHD Build Summary"
    echo ""

    # --- Source Image ---
    echo "## Source Image"
    echo ""

    direct_gallery_id=$(read_setting "windows_sigmode_direct_shared_gallery_image_id")
    sigmode_source_id=$(read_setting "windows_sigmode_source_id")
    publisher=$(read_setting "windows_image_publisher")
    offer=$(read_setting "windows_image_offer")
    sku=$(read_setting "windows_image_sku")
    version=$(read_setting "windows_image_version")
    image_url=$(read_setting "windows_image_url")

    if [ "$direct_gallery_id" != "N/A" ] && [ -n "$direct_gallery_id" ]; then
        echo "**Source type**: 1P Shared Gallery (direct)"
        echo ""
        echo "| Property | Value |"
        echo "|----------|-------|"
        echo "| Gallery Image ID | \`${direct_gallery_id}\` |"
    elif [ "$sigmode_source_id" != "N/A" ] && [ -n "$sigmode_source_id" ]; then
        echo "**Source type**: Shared Image Gallery (ARM)"
        echo ""
        echo "| Property | Value |"
        echo "|----------|-------|"
        echo "| SIG Resource ID | \`${sigmode_source_id}\` |"
    elif [ "$image_url" != "N/A" ] && [ -n "$image_url" ]; then
        echo "**Source type**: VHD URL"
        echo ""
        echo "| Property | Value |"
        echo "|----------|-------|"
        echo "| Image URL | \`${image_url}\` |"
    else
        echo "**Source type**: Azure Marketplace"
        echo ""
        echo "| Property | Value |"
        echo "|----------|-------|"
        echo "| Publisher | ${publisher} |"
        echo "| Offer | ${offer} |"
        echo "| SKU | ${sku} |"
        echo "| Version | ${version} |"
    fi
    echo ""

    # --- Build Configuration ---
    echo "## Build Configuration"
    echo ""
    echo "| Property | Value |"
    echo "|----------|-------|"
    echo "| Windows SKU | ${WINDOWS_SKU:-N/A} |"
    echo "| HyperV Generation | ${HYPERV_GENERATION:-N/A} |"
    echo "| Architecture | ${ARCHITECTURE:-N/A} |"
    echo "| VM Size | $(read_setting 'vm_size') |"
    echo "| OS Disk Size (GB) | $(read_setting 'os_disk_size_gb') |"
    echo "| Build Date | $(read_setting 'build_date') |"
    echo "| Build Location | $(read_setting 'location') |"
    echo ""

    # --- Output VHD ---
    echo "## Output VHD"
    echo ""
    echo "| Property | Value |"
    echo "|----------|-------|"
    echo "| Image Version | ${AKS_WINDOWS_IMAGE_VERSION:-N/A} |"
    echo "| SIG Gallery | $(read_setting 'sig_gallery_name') |"
    echo "| SIG Image Name | $(read_setting 'sig_image_name') |"
    echo "| Captured SIG Version | $(read_setting 'captured_sig_version') |"
    echo ""

    # --- Cached Container Images ---
    echo "## Cached Container Images"
    echo ""
    if [ -f "$IMAGE_BOM_FILE" ]; then
        image_count=$(jq '.imageBom | length' <"$IMAGE_BOM_FILE" 2>/dev/null) || image_count=0
        echo "**Total images**: ${image_count}"
        echo ""
        if [ "$image_count" -gt 0 ] 2>/dev/null; then
            echo "| # | Image Tags |"
            echo "|---|------------|"
            jq -r '.imageBom | to_entries[] | "\(.key + 1) | \(.value.repoTags | join(", "))"' <"$IMAGE_BOM_FILE" 2>/dev/null | while IFS= read -r line; do
                echo "| ${line} |"
            done
        fi
    else
        echo "_image-bom.json not found_"
    fi
    echo ""

    # --- Cached Files ---
    echo "## Cached Files"
    echo ""
    if [ -f "$RELEASE_NOTES_FILE" ]; then
        # Extract cached files section — it's a Format-Table output after "Cached Files:"
        # Columns: File, Sha256, SizeBytes (whitespace-separated, variable width)
        file_count=0
        total_bytes=0
        echo "| File | Size |"
        echo "|------|------|"
        in_cached=false
        while IFS= read -r line; do
            if echo "$line" | grep -q "^Cached Files:"; then
                in_cached=true
                continue
            fi
            if [ "$in_cached" = true ]; then
                # skip header/separator lines (contain dashes or are blank)
                case "$line" in
                    "") continue ;;
                    *---*) continue ;;
                    File*) continue ;;
                esac
                # extract filename (first field) and size (last field)
                fname=$(echo "$line" | awk '{print $1}')
                size_bytes=$(echo "$line" | awk '{print $NF}')
                if [ -z "$fname" ] || [ -z "$size_bytes" ]; then
                    continue
                fi
                # validate size_bytes is numeric
                case "$size_bytes" in
                    ''|*[!0-9]*) continue ;;
                esac
                file_count=$((file_count + 1))
                total_bytes=$((total_bytes + size_bytes))
                # human-readable size
                if [ "$size_bytes" -ge 1073741824 ]; then
                    human_size="$(awk "BEGIN {printf \"%.1f GB\", ${size_bytes}/1073741824}")"
                elif [ "$size_bytes" -ge 1048576 ]; then
                    human_size="$(awk "BEGIN {printf \"%.1f MB\", ${size_bytes}/1048576}")"
                elif [ "$size_bytes" -ge 1024 ]; then
                    human_size="$(awk "BEGIN {printf \"%.1f KB\", ${size_bytes}/1024}")"
                else
                    human_size="${size_bytes} B"
                fi
                # shorten path for display
                short_name=$(echo "$fname" | sed 's|c:\\akse-cache\\||' | sed 's|c:/akse-cache/||')
                echo "| \`${short_name}\` | ${human_size} |"
            fi
        done <"$RELEASE_NOTES_FILE"

        echo ""
        if [ "$total_bytes" -ge 1073741824 ]; then
            total_human="$(awk "BEGIN {printf \"%.2f GB\", ${total_bytes}/1073741824}")"
        elif [ "$total_bytes" -ge 1048576 ]; then
            total_human="$(awk "BEGIN {printf \"%.1f MB\", ${total_bytes}/1048576}")"
        else
            total_human="${total_bytes} B"
        fi
        echo "**Total cached files**: ${file_count} (${total_human})"
    else
        echo "_release-notes.txt not found_"
    fi
    echo ""
} >"$SUMMARY_FILE"

echo "Build summary written to ${SUMMARY_FILE}"
echo "##vso[task.uploadsummary]${SUMMARY_FILE}"
