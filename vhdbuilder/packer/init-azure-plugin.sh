#!/usr/bin/env bash

init_packer_azure_plugin() (
    set -euo pipefail

    local plugin_config=./vhdbuilder/packer/packer-plugin.pkr.hcl
    local version=2.5.0
    local init_log work_dir="" archive="" plugin_binary="" arch sha256
    init_log=$(mktemp)

    cleanup() {
        rm -f -- "$init_log"
        if [[ -n "$work_dir" ]]; then
            rm -f -- "$work_dir/$archive" "$work_dir/$plugin_binary" "$work_dir/LICENSE.txt"
            rmdir -- "$work_dir"
        fi
    }
    trap cleanup EXIT

    if packer init "$plugin_config" >"$init_log" 2>&1; then
        cat "$init_log"
        return 0
    fi
    cat "$init_log" >&2

    if ! grep -Fq "packer-plugin-azure_v${version}_SHA256SUMS: 404" "$init_log"; then
        return 1
    fi

    if [[ $(uname -s) != Linux ]]; then
        echo "Azure Packer plugin CDN fallback only supports Linux" >&2
        return 1
    fi
    case "$(uname -m)" in
        x86_64)
            arch=amd64
            sha256=0817b59e20eab4acac1a18b4d1186923abfd6f927ce842b0b754145a8a5dcc42
            ;;
        aarch64)
            arch=arm64
            sha256=326637b4472e35f1d34110da17402daeff83dbcac7f3ce4adb13a1595cf0e0d3
            ;;
        *)
            echo "Unsupported Azure Packer plugin host architecture: $(uname -m)" >&2
            return 1
            ;;
    esac

    archive="packer-plugin-azure_${version}_linux_${arch}.zip"
    plugin_binary="packer-plugin-azure_v${version}_x5.0_linux_${arch}"
    work_dir=$(mktemp -d)
    echo "Installing verified Azure Packer plugin ${version} from releases.hashicorp.com"
    curl --fail --location --silent --show-error --output "$work_dir/$archive" \
        "https://releases.hashicorp.com/packer-plugin-azure/${version}/${archive}"
    if ! printf '%s  %s\n' "$sha256" "$work_dir/$archive" | sha256sum --check --status; then
        echo "Azure Packer plugin archive checksum mismatch" >&2
        return 1
    fi
    python3 -m zipfile -e "$work_dir/$archive" "$work_dir"
    chmod +x "$work_dir/$plugin_binary"
    packer plugins install --path "$work_dir/$plugin_binary" github.com/hashicorp/azure
    packer init "$plugin_config"
)

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    init_packer_azure_plugin "$@"
fi
