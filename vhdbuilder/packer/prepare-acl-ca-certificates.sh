#!/bin/bash
set -euo pipefail

prepareACLTrustSources() {
    local arch=$1 destination=$2
    local certificates_sha shared_sha
    case "$arch" in
        X86_64)
            arch=x86_64
            certificates_sha=230c46aef2514de0ba49670a8f3eecdee16debcb9304877b387ca5189b890914
            shared_sha=555afcd8e1ffc61117ccd5644398889f8d35a56e9201ff3cd84b0211f9c0f942
            ;;
        ARM64)
            arch=aarch64
            certificates_sha=41037fcfc3b366295304ee2f886b0a1d15271f55a878920706a1b3f7e1ccc7ee
            shared_sha=8399e850467e0bbff3d454ff87878a4fdd3f22ff735be2baac4b43b831a7f6c6
            ;;
        *)
            echo "Unsupported ACL certificate architecture: $arch" >&2
            return 1
            ;;
    esac

    local package checksum rpm_file attempt
    local version=3.0.0-19.azl3
    mkdir -p "$destination"
    for package in ca-certificates ca-certificates-shared; do
        checksum=$certificates_sha
        [ "$package" != ca-certificates-shared ] || checksum=$shared_sha
        rpm_file="$destination/$package.rpm"
        for attempt in 1 2 3 4 5; do
            if retrycmd_curl_file 1 5 120 "$rpm_file" \
                "https://packages.microsoft.com/azurelinux/3.0/prod/base/$arch/Packages/c/$package-$version.noarch.rpm" &&
                printf '%s  %s\n' "$checksum" "$rpm_file" | sha256sum -c -; then
                break
            fi
            # The download helper accepts existing files, including partial RPMs.
            rm -f "$rpm_file" || return 1
            if [ "$attempt" -eq 5 ]; then
                echo "Failed to download and verify ACL package: $package" >&2
                return 1
            fi
            sleep 5
        done
        # Only extract trust inputs; never install RPMs or run their scriptlets.
        rpm2cpio "$rpm_file" | (cd "$destination" && cpio -idm --quiet --no-absolute-filenames \
            './usr/share/pki/ca-trust-source/*.p11-kit') || return 1
        rm -f "$rpm_file"
    done

    local source_file
    for source_file in ca-bundle.trust.microsoft.p11-kit ca-bundle.trust.distrusted.p11-kit; do
        test -s "$destination/usr/share/pki/ca-trust-source/$source_file" || {
            echo "Missing refreshed ACL trust source: $source_file" >&2
            return 1
        }
        mv "$destination/usr/share/pki/ca-trust-source/$source_file" "$destination/"
    done
    printf '%s\n' "$version" > "$destination/version"
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    source ./parts/linux/cloud-init/artifacts/cse_helpers.sh
    prepareACLTrustSources "${ARCHITECTURE:?}" vhdbuilder/packer/acl-ca-certificates
fi
