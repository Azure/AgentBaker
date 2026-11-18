#!/bin/bash
# shellcheck disable=SC3010

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

kernel_package_architecture() {
  local directory="$1" file architecture
  for file in "$directory/"*.deb; do
    architecture=$(dpkg-deb -f "$file" Architecture) || return 1
    if [ "$architecture" != all ]; then
      printf '%s\n' "$architecture"
      return
    fi
  done
  custom_kernel_error "No architecture-specific kernel package."
  return 1
}

inspect_kernel_packages() {
  local directory="$1" file package release="" image="" version="" architecture abi
  local flavor="" image_architecture="" image_count=0 modules_count=0
  local -A seen=()
  local files=("$directory/"*.deb)
  local rpms=("$directory/"*.rpm)
  [ ! -f "${rpms[0]}" ] ||
    { custom_kernel_error "Only Ubuntu .deb packages are supported."; return 1; }
  [ -f "${files[0]}" ] || { custom_kernel_error "No .deb files in $directory"; return 1; }

  for file in "${files[@]}"; do
    [[ "$(basename "$file")" =~ ^[a-zA-Z0-9._+~-]+\.deb$ ]] ||
      { custom_kernel_error "Unsupported package filename: $file"; return 1; }
    package=$(dpkg-deb -f "$file" Package) || return 1
    if [[ "$package" =~ ^linux-image-(unsigned-)?([0-9]+\.[0-9]+\.[0-9]+-[0-9]+)-([a-z][a-z0-9-]*)$ ]]; then
      abi="${BASH_REMATCH[2]}"
      flavor="${BASH_REMATCH[3]}"
      release="$abi-$flavor"
      image="$package"
      version=$(dpkg-deb -f "$file" Version) || return 1
      image_architecture=$(dpkg-deb -f "$file" Architecture) || return 1
      image_count=$((image_count + 1))
    fi
  done
  [ "$image_count" -eq 1 ] ||
    { custom_kernel_error "Supply exactly one versioned Ubuntu kernel image package."; return 1; }
  case "$image_architecture" in
    amd64|arm64) ;;
    *) custom_kernel_error "Only amd64 or arm64 kernels are supported."; return 1 ;;
  esac
  case "$flavor" in
    *fips*|*cvm*|*mshv*|azure-fde) custom_kernel_error "Protected kernel flavors are not supported."; return 1 ;;
  esac

  for file in "${files[@]}"; do
    package=$(dpkg-deb -f "$file" Package) || return 1
    architecture=$(dpkg-deb -f "$file" Architecture) || return 1
    [ "$(dpkg-deb -f "$file" Version)" = "$version" ] ||
      { custom_kernel_error "Package versions must match: $package"; return 1; }
    [ -z "${seen[$package]:-}" ] ||
      { custom_kernel_error "Duplicate package: $package"; return 1; }
    seen[$package]=1
    case "$package" in
      "$image"|"linux-modules-extra-$release"|"linux-tools-$release"|"linux-cloud-tools-$release") ;;
      "linux-$flavor-tools-$abi"|"linux-$flavor-cloud-tools-$abi"|linux-"$flavor"-*-tools-"$abi"|linux-"$flavor"-*-cloud-tools-"$abi") ;;
      "linux-modules-$release") modules_count=$((modules_count + 1)) ;;
      "linux-headers-$release"|"linux-headers-$abi"|"linux-$flavor-headers-$abi"|linux-"$flavor"-*-headers-"$abi")
        if [ "$architecture" = all ]; then continue; fi
        ;;
      *) custom_kernel_error "Unexpected package: $package"; return 1 ;;
    esac
    [ "$architecture" = "$image_architecture" ] ||
      { custom_kernel_error "Package architecture must match $image_architecture: $package"; return 1; }
  done
  [ "$modules_count" -eq 1 ] ||
    { custom_kernel_error "Supply linux-modules-$release alongside the image."; return 1; }
  printf '%s\n' "$release"
}

kernel_target() {
  local directory="$1" target architecture
  architecture=$(kernel_package_architecture "$directory") || return 1
  if [ -f "$directory/kernel-target" ]; then
    target=$(< "$directory/kernel-target")
  else
    # Keep already-uploaded 26.04 AMD64 bundles usable, without guessing other targets.
    target=ubuntu:26.04:amd64
  fi
  case "$target" in
    ubuntu:22.04:amd64|ubuntu:22.04:arm64|ubuntu:24.04:amd64|ubuntu:24.04:arm64|ubuntu:26.04:amd64|ubuntu:26.04:arm64) ;;
    *) custom_kernel_error "Unsupported kernel-target: $target"; return 1 ;;
  esac
  [ "${target##*:}" = "$architecture" ] ||
    { custom_kernel_error "kernel-target architecture does not match the packages."; return 1; }
  printf '%s\n' "$target"
}

validate_kernel_manifest() {
  local directory="$1" release expected actual
  release=$(inspect_kernel_packages "$directory") || return 1
  kernel_target "$directory" >/dev/null || return 1
  if [ ! -f "$directory/kernel-release" ] || [ ! -f "$directory/SHA256SUMS" ]; then
    custom_kernel_error "Missing kernel-release or SHA256SUMS."
    return 1
  fi
  [ "$(< "$directory/kernel-release")" = "$release" ] ||
    { custom_kernel_error "kernel-release does not match the image package."; return 1; }
  expected=$(cd "$directory" && LC_ALL=C sha256sum -- *.deb kernel-release) || return 1
  if [ -f "$directory/kernel-target" ]; then
    expected=$(cd "$directory" && LC_ALL=C sha256sum -- *.deb kernel-release kernel-target) || return 1
  fi
  actual=$(< "$directory/SHA256SUMS")
  [ "$actual" = "$expected" ] ||
    { custom_kernel_error "SHA256SUMS does not match the complete package set."; return 1; }
  printf '%s\n' "$release"
}

require_test_kernel_build() {
  local fips="${ENABLE_FIPS:-}" trusted_launch="${ENABLE_TRUSTED_LAUNCH:-}"
  local publishing="${GENERATE_PUBLISHING_INFO:-}"
  if [ "${ENVIRONMENT:-}" != test ] ||
    [ "${fips,,}" != false ] ||
    [ "${trusted_launch,,}" != false ] ||
    [ "${publishing,,}" != false ] ||
    [ "${BUILDER:-}" = imagecustomizer ]; then
    custom_kernel_error "Only non-publishing test builds without FIPS, Trusted Launch or Image Customizer are supported."
    return 1
  fi
  case "${FEATURE_FLAGS:-}" in
    None|minimal|NVIDIA_GB) ;;
    *) custom_kernel_error "Unsupported custom-kernel feature flags (CVM is excluded)."; return 1 ;;
  esac
  case "${HYPERV_GENERATION:-}:${ARCHITECTURE:-}" in
    V1:X86_64|V2:X86_64|V2:ARM64) ;;
    *) custom_kernel_error "Unsupported kernel build architecture or Hyper-V generation."; return 1 ;;
  esac
  build_kernel_target >/dev/null
}

build_kernel_target() {
  local architecture distro version="${OS_VERSION:-}"
  case "${ARCHITECTURE:-}" in
    X86_64) architecture=amd64 ;;
    ARM64) architecture=arm64 ;;
    *) custom_kernel_error "Unsupported build architecture."; return 1 ;;
  esac
  case "${OS_SKU:-}:$version" in
    Ubuntu:22.04|Ubuntu:24.04|Ubuntu:26.04) distro=ubuntu ;;
    *) custom_kernel_error "Unsupported custom-kernel distribution/version."; return 1 ;;
  esac
  echo "$distro:$version:$architecture"
}

upload_kernel_packages() (
  local subscription="$1" account="$2" container="$3" prefix="$4" directory="$5"
  local distro="${6:-ubuntu}" os_version="${7:-26.04}"
  local release staging file public_access architecture
  validate_kernel_prefix "$prefix" || return 1
  release=$(inspect_kernel_packages "$directory") || return 1
  architecture=$(kernel_package_architecture "$directory") || return 1
  staging=$(mktemp -d)
  trap 'rm -rf -- "$staging"' EXIT
  cp -- "$directory/"*.deb "$staging/"
  printf '%s\n' "$release" > "$staging/kernel-release"
  printf '%s:%s:%s\n' "$distro" "$os_version" "$architecture" > "$staging/kernel-target"
  kernel_target "$staging" >/dev/null || return 1
  (cd "$staging" && LC_ALL=C sha256sum -- *.deb kernel-release kernel-target > SHA256SUMS)

  az storage container create --subscription "$subscription" --account-name "$account" \
    --name "$container" --public-access off --auth-mode login --only-show-errors --output none || return 1
  public_access=$(az storage container show --subscription "$subscription" --account-name "$account" \
    --name "$container" --auth-mode login --query properties.publicAccess \
    --only-show-errors --output tsv) || return 1
  [ -z "$public_access" ] ||
    { custom_kernel_error "Refusing to upload to a public container."; return 1; }
  for file in "$staging/"*.deb "$staging/kernel-release" "$staging/kernel-target" "$staging/SHA256SUMS"; do
    az storage blob upload --subscription "$subscription" --account-name "$account" \
      --container-name "$container" --name "$prefix/$(basename "$file")" \
      --file "$file" --overwrite false --auth-mode login --only-show-errors --output none || return 1
  done
  echo "Uploaded $release to $account/$container/$prefix (no existing blobs overwritten)."
  cat "$staging/SHA256SUMS"
)

download_kernel_packages() (
  local subscription="$1" account="$2" container="$3" prefix="$4" destination="$5"
  local staging release target
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
  target=$(kernel_target "$staging/$prefix") || return 1
  [ "$target" = "$(build_kernel_target)" ] ||
    { custom_kernel_error "Bundle target $target does not match this build."; return 1; }
  mkdir -p "$destination"
  cp -- "$staging/$prefix/"*.deb "$staging/$prefix/kernel-release" \
    "$staging/$prefix/SHA256SUMS" "$destination/"
  if [ -f "$staging/$prefix/kernel-target" ]; then
    cp -- "$staging/$prefix/kernel-target" "$destination/"
  fi
  echo "Downloaded and verified test kernel $release"
)

kernel_host_target() {
  local ID VERSION_ID
  # shellcheck disable=SC1091
  source /etc/os-release
  local architecture
  case "$(uname -m)" in
    x86_64) architecture=amd64 ;;
    aarch64) architecture=arm64 ;;
    *) custom_kernel_error "Unsupported host architecture."; return 1 ;;
  esac
  echo "$ID:$VERSION_ID:$architecture"
}

require_test_kernel_host() {
  local target secure_boot
  target=$(kernel_target "$1") || return 1
  [ "$(kernel_host_target)" = "$target" ] ||
    { custom_kernel_error "Installation requires a host matching $target."; return 1; }
  if [ -r /proc/sys/crypto/fips_enabled ] && [ "$(< /proc/sys/crypto/fips_enabled)" = 1 ]; then
    custom_kernel_error "Test kernels must not be installed on a FIPS host."
    return 1
  fi
  for secure_boot in /sys/firmware/efi/efivars/SecureBoot-*; do
    [ -f "$secure_boot" ] || continue
    if [ "$(od -An -j4 -N1 -t u1 "$secure_boot" | tr -d ' ')" = 1 ]; then
      custom_kernel_error "Test kernels must not be installed with Secure Boot enabled."
      return 1
    fi
  done
  if command -v mokutil >/dev/null && mokutil --sb-state | grep -qi 'SecureBoot enabled'; then
    custom_kernel_error "Test kernels must not be installed with Secure Boot enabled."
    return 1
  fi
}

install_deb_kernel_packages() {
  local directory="$1" release file package
  local packages=() required_packages=()
  release=$(inspect_kernel_packages "$directory") || return 1
  for file in "$directory/"*.deb; do
    package=$(dpkg-deb -f "$file" Package) || return 1
    packages+=("$package")
  done
  required_packages=("linux-headers-$release")
  if [[ "$release" = *-azure ]]; then
    required_packages+=("linux-tools-$release" "linux-cloud-tools-$release")
  fi
  for package in "${required_packages[@]}"; do
    if [[ " ${packages[*]} " != *" $package "* ]] &&
      ! apt-cache show "$package" >/dev/null 2>&1; then
      custom_kernel_error "Matching $package is unavailable. Request its .deb and matching dependencies from the kernel provider."
      return 1
    fi
  done
  DEBIAN_FRONTEND=noninteractive apt-get -o DPkg::Lock::Timeout=300 install \
    -y --reinstall --allow-downgrades --no-install-recommends "$directory/"*.deb \
    "${required_packages[@]}" || return 1
  apt-mark manual "${packages[@]}" || return 1
  apt-mark hold "${packages[@]}" || return 1
}

check_kernel_boot_files() {
  local release="$1" initrd="/boot/initrd.img-$1"
  if [ ! -s "/boot/vmlinuz-$release" ] || [ ! -s "$initrd" ]; then
    custom_kernel_error "Missing kernel or initramfs boot files for $release."
    return 1
  fi
}

generate_kernel_grub_config() {
  update-grub
}

pin_kernel_boot() {
  local release="$1" root="${2:-}" config entry
  config="$root/boot/grub/grub.cfg"
  mkdir -p "$root/etc/default/grub.d" "$root/etc/grub.d"
  printf 'GRUB_DISABLE_SUBMENU=true\nGRUB_SAVEDEFAULT=false\n' > "$root/etc/default/grub.d/99-aks-test-kernel.cfg"
  generate_kernel_grub_config "$config" || return 1
  entry=$(awk -F "'" -v prefix="gnulinux-$release-advanced-" '
    /^[[:space:]]*menuentry / { for (i=1; i<=NF; i++) if (index($i, prefix)==1) print $i }
  ' "$config") || return 1
  [[ "$entry" =~ ^[a-zA-Z0-9._+-]+$ ]] ||
    { custom_kernel_error "Expected unique kernel entry is absent from grub.cfg."; return 1; }
  printf 'GRUB_DEFAULT="%s"\n' "$entry" >> "$root/etc/default/grub.d/99-aks-test-kernel.cfg"
  # Run after hardware-specific selectors, which otherwise override GRUB_DEFAULT on ARM64.
  printf '#!/bin/sh\necho '\''set default="%s"'\''\n' "$entry" > "$root/etc/grub.d/99_aks_test_kernel"
  chmod 755 "$root/etc/grub.d/99_aks_test_kernel"
  generate_kernel_grub_config "$config" || return 1
  grep -Fxq "set default=\"$entry\"" "$config" ||
    { custom_kernel_error "Custom kernel boot selector is absent from grub.cfg."; return 1; }
}

install_kernel_packages() {
  local directory="$1" release
  release=$(validate_kernel_manifest "$directory") || return 1
  require_test_kernel_host "$directory" || return 1
  install_deb_kernel_packages "$directory" || return 1
  check_kernel_boot_files "$release" || return 1
  pin_kernel_boot "$release" || return 1
  echo "Installed test kernel $release; verification follows the existing Packer reboot."
}

verify_kernel_boot() {
  local directory="$1" release running
  release=$(validate_kernel_manifest "$directory") || return 1
  running=$(uname -r)
  [ "$running" = "$release" ] ||
    { custom_kernel_error "Expected running kernel $release, got $running"; return 1; }
  systemctl is-active --quiet hv-kvp-daemon.service ||
    { custom_kernel_error "hv-kvp-daemon.service is not active for kernel $release"; return 1; }
  printf 'CUSTOM_KERNEL_VERIFIED=%s\n' "$running" | tee -a /opt/azure/vhd-install.complete
  tee -a /opt/azure/vhd-install.complete < "$directory/SHA256SUMS"
}

custom_kernel_main() {
  local action="${1:-}" enabled directory
  case "$action" in
    validate-build)
      enabled="${2:-false}"
      [ "${enabled,,}" = true ] || return 0
      require_test_kernel_build
      ;;
    upload|download)
      if { [ "$action" = upload ] && [ "$#" -ne 6 ] && [ "$#" -ne 8 ]; } ||
        { [ "$action" = download ] && [ "$#" -ne 6 ]; }; then
        echo "Usage: bash $0 $action SUBSCRIPTION ACCOUNT CONTAINER PREFIX DIRECTORY [ubuntu OS_VERSION]" >&2
        return 1
      fi
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
      echo "Usage: bash $0 {upload|download|validate-build|install|verify} ..." >&2
      return 1
      ;;
  esac
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  set -euo pipefail
  export LC_ALL=C
  custom_kernel_main "$@"
fi
