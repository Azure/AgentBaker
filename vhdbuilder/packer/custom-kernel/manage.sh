#!/bin/bash

custom_kernel_error() {
  echo "Custom kernel: $*" >&2
  return 1
}

validate_kernel_prefix() {
  if [[ ! "$1" =~ ^[a-zA-Z0-9][a-zA-Z0-9._/-]*$ ]] ||
    [[ "$1" = */ ]] || [[ "$1" =~ (^|/)\.\.?(/|$) ]]; then
    custom_kernel_error "Use a nonempty relative blob prefix without wildcards or dot segments."
    return 1
  fi
}

inspect_kernel_packages() {
  local directory="$1" file package release="" image="" version="" architecture abi
  local image_count=0 modules_count=0
  local -A seen=()
  local files=("$directory/"*.deb)
  [ -f "${files[0]}" ] || { custom_kernel_error "No .deb files in $directory"; return 1; }

  for file in "${files[@]}"; do
    [[ "$(basename "$file")" =~ ^[a-zA-Z0-9._+~-]+\.deb$ ]] ||
      { custom_kernel_error "Unsupported package filename: $file"; return 1; }
    package=$(dpkg-deb -f "$file" Package) || return 1
    if [[ "$package" =~ ^linux-image-(unsigned-)?([0-9]+\.[0-9]+\.[0-9]+-[0-9]+-azure)$ ]]; then
      release="${BASH_REMATCH[2]}"
      image="$package"
      version=$(dpkg-deb -f "$file" Version) || return 1
      image_count=$((image_count + 1))
    fi
  done
  [ "$image_count" -eq 1 ] ||
    { custom_kernel_error "Supply exactly one versioned Azure kernel image package."; return 1; }
  abi="${release%-azure}"

  for file in "${files[@]}"; do
    package=$(dpkg-deb -f "$file" Package) || return 1
    architecture=$(dpkg-deb -f "$file" Architecture) || return 1
    [ "$(dpkg-deb -f "$file" Version)" = "$version" ] ||
      { custom_kernel_error "Package versions must match: $package"; return 1; }
    [ -z "${seen[$package]:-}" ] ||
      { custom_kernel_error "Duplicate package: $package"; return 1; }
    seen[$package]=1
    case "$package" in
      "$image"|"linux-modules-extra-$release") ;;
      "linux-modules-$release") modules_count=$((modules_count + 1)) ;;
      "linux-headers-$release"|"linux-headers-$abi"|"linux-azure-headers-$abi"|linux-azure-*-headers-"$abi")
        if [ "$architecture" = all ]; then continue; fi
        ;;
      *) custom_kernel_error "Unexpected package: $package"; return 1 ;;
    esac
    [ "$architecture" = amd64 ] ||
      { custom_kernel_error "Only amd64 packages (or architecture-independent headers) are supported: $package"; return 1; }
  done
  [ "$modules_count" -eq 1 ] ||
    { custom_kernel_error "Supply linux-modules-$release alongside the image."; return 1; }
  printf '%s\n' "$release"
}

validate_kernel_manifest() {
  local directory="$1" release expected actual
  release=$(inspect_kernel_packages "$directory") || return 1
  if [ ! -f "$directory/kernel-release" ] || [ ! -f "$directory/SHA256SUMS" ]; then
    custom_kernel_error "Missing kernel-release or SHA256SUMS."
    return 1
  fi
  [ "$(< "$directory/kernel-release")" = "$release" ] ||
    { custom_kernel_error "kernel-release does not match the image package."; return 1; }
  expected=$(cd "$directory" && LC_ALL=C sha256sum -- *.deb kernel-release) || return 1
  actual=$(< "$directory/SHA256SUMS")
  [ "$actual" = "$expected" ] ||
    { custom_kernel_error "SHA256SUMS does not match the complete package set."; return 1; }
  printf '%s\n' "$release"
}

require_test_kernel_build() {
  local fips="${ENABLE_FIPS:-}" trusted_launch="${ENABLE_TRUSTED_LAUNCH:-}"
  local publishing="${GENERATE_PUBLISHING_INFO:-}"
  if [ "${ENVIRONMENT:-}" != test ] ||
    [ "${OS_SKU:-}" != Ubuntu ] ||
    [ "${OS_VERSION:-}" != 26.04 ] ||
    [ "${ARCHITECTURE:-}" != X86_64 ] ||
    [ "${HYPERV_GENERATION:-}" != V2 ] ||
    [ "${FEATURE_FLAGS:-}" != minimal ] ||
    [ "${fips,,}" != false ] ||
    [ "${trusted_launch,,}" != false ] ||
    [ "${publishing,,}" != false ] ||
    [ "${BUILDER:-}" = imagecustomizer ]; then
    custom_kernel_error "Only non-publishing test builds of Ubuntu 26.04 Minimal AMD64 Gen2 are supported."
    return 1
  fi
}

upload_kernel_packages() (
  local subscription="$1" account="$2" container="$3" prefix="$4" directory="$5"
  local release staging file public_access
  validate_kernel_prefix "$prefix" || return 1
  release=$(inspect_kernel_packages "$directory") || return 1
  staging=$(mktemp -d)
  trap 'rm -rf -- "$staging"' EXIT
  cp -- "$directory/"*.deb "$staging/"
  printf '%s\n' "$release" > "$staging/kernel-release"
  (cd "$staging" && LC_ALL=C sha256sum -- *.deb kernel-release > SHA256SUMS)

  az storage container create --subscription "$subscription" --account-name "$account" \
    --name "$container" --public-access off --auth-mode login --only-show-errors --output none || return 1
  public_access=$(az storage container show --subscription "$subscription" --account-name "$account" \
    --name "$container" --auth-mode login --query properties.publicAccess \
    --only-show-errors --output tsv) || return 1
  [ -z "$public_access" ] ||
    { custom_kernel_error "Refusing to upload to a public container."; return 1; }
  for file in "$staging/"*.deb "$staging/kernel-release" "$staging/SHA256SUMS"; do
    az storage blob upload --subscription "$subscription" --account-name "$account" \
      --container-name "$container" --name "$prefix/$(basename "$file")" \
      --file "$file" --overwrite false --auth-mode login --only-show-errors --output none || return 1
  done
  echo "Uploaded $release to $account/$container/$prefix (no existing blobs overwritten)."
  cat "$staging/SHA256SUMS"
)

download_kernel_packages() (
  local subscription="$1" account="$2" container="$3" prefix="$4" destination="$5"
  local staging release
  require_test_kernel_build || return 1
  if [ -z "$subscription" ] || [ -z "$account" ] || [ -z "$container" ]; then
    custom_kernel_error "Subscription, storage account and container are required."
    return 1
  fi
  validate_kernel_prefix "$prefix" || return 1
  [ ! -e "$destination" ] ||
    { custom_kernel_error "Destination already exists; use a clean checkout: $destination"; return 1; }
  staging=$(mktemp -d)
  trap 'rm -rf -- "$staging"' EXIT
  az storage blob download-batch --subscription "$subscription" --account-name "$account" \
    --source "$container" --destination "$staging" --pattern "$prefix/*" \
    --auth-mode login --only-show-errors --output none || return 1
  release=$(validate_kernel_manifest "$staging/$prefix") || return 1
  mkdir -p "$destination"
  cp -- "$staging/$prefix/"*.deb "$staging/$prefix/kernel-release" \
    "$staging/$prefix/SHA256SUMS" "$destination/"
  echo "Downloaded and verified test kernel $release"
)

require_test_kernel_host() {
  local ID VERSION_ID
  # shellcheck disable=SC1091
  source /etc/os-release
  if [ "$ID" != ubuntu ] || [ "$VERSION_ID" != 26.04 ] ||
    [ "$(dpkg --print-architecture)" != amd64 ]; then
    custom_kernel_error "Installation requires Ubuntu 26.04 amd64."
    return 1
  fi
  if command -v mokutil >/dev/null && mokutil --sb-state | grep -qi 'SecureBoot enabled'; then
    custom_kernel_error "Test kernels must not be installed with Secure Boot enabled."
    return 1
  fi
}

install_kernel_packages() {
  local directory="$1" release file package
  local packages=()
  release=$(validate_kernel_manifest "$directory") || return 1
  require_test_kernel_host || return 1
  for file in "$directory/"*.deb; do
    package=$(dpkg-deb -f "$file" Package) || return 1
    packages+=("$package")
  done
  if [[ " ${packages[*]} " != *" linux-headers-$release "* ]] &&
    ! apt-cache show "linux-headers-$release" >/dev/null 2>&1; then
    custom_kernel_error "Matching linux-headers-$release is unavailable. Request its .deb and common header dependencies from the kernel provider."
    return 1
  fi
  DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 install \
    -y --reinstall --allow-downgrades --no-install-recommends "$directory/"*.deb \
    "linux-headers-$release" || return 1
  apt-mark manual "${packages[@]}" || return 1
  apt-mark hold "${packages[@]}" || return 1
  mkdir -p /etc/default/grub.d
  # Pin the boot entry even when the base image already has a newer kernel.
  printf 'GRUB_DEFAULT="Advanced options for Ubuntu>Ubuntu, with Linux %s"\nGRUB_DISABLE_SUBMENU=false\nGRUB_SAVEDEFAULT=false\n' "$release" \
    > /etc/default/grub.d/99-aks-test-kernel.cfg
  update-grub
  grep -Fq "menuentry 'Ubuntu, with Linux $release'" /boot/grub/grub.cfg ||
    { custom_kernel_error "Expected kernel entry is absent from grub.cfg."; return 1; }
  echo "Installed test kernel $release; verification follows the existing Packer reboot."
}

verify_kernel_boot() {
  local directory="$1" release running
  release=$(validate_kernel_manifest "$directory") || return 1
  running=$(uname -r)
  [ "$running" = "$release" ] ||
    { custom_kernel_error "Expected running kernel $release, got $running"; return 1; }
  printf 'CUSTOM_KERNEL_VERIFIED=%s\n' "$running" | tee -a /opt/azure/vhd-install.complete
  tee -a /opt/azure/vhd-install.complete < "$directory/SHA256SUMS"
}

custom_kernel_main() {
  local action="${1:-}" enabled directory
  case "$action" in
    upload|download)
      [ "$#" -eq 6 ] || {
        echo "Usage: bash $0 $action SUBSCRIPTION ACCOUNT CONTAINER PREFIX DIRECTORY" >&2
        return 1
      }
      "${action}_kernel_packages" "${@:2}"
      ;;
    install|verify)
      enabled="${2:-false}"
      [ "${enabled,,}" = true ] || return 0
      directory="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/packages"
      if [ "$action" = install ]; then
        install_kernel_packages "$directory"
      else
        verify_kernel_boot "$directory"
      fi
      ;;
    *)
      echo "Usage: bash $0 {upload|download|install|verify} ..." >&2
      return 1
      ;;
  esac
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  set -euo pipefail
  export LC_ALL=C
  custom_kernel_main "$@"
fi
