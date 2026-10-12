#!/bin/bash
set -euo pipefail

# Reuse the package's root/kernel discovery, adding Azure's nested-root flag.
/etc/grub.d/50_mariner_mshv_menuentry |
    sed -e 's/menuentry "Dom0"/menuentry "OpenVMM SNP"/' \
        -e '/chainloader/s/MSHV_FORCE_NESTED=[^[:space:]]*/MSHV_FORCE_NESTED=TRUE/g' \
        -e '/chainloader/ { /MSHV_FORCE_NESTED=TRUE/! s/MSHV_ENABLE=TRUE/MSHV_ENABLE=TRUE MSHV_FORCE_NESTED=TRUE/; }'
