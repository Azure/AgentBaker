#!/bin/bash
# Selects the correct managed identity for AzCopy when downloading private Kubernetes packages
# during the Linux VHD build (see cacheKubePackageFromPrivateUrl in install-dependencies.sh).
#
# install-dependencies.sh sets AZCOPY_AUTO_LOGIN_TYPE=AZCLI for azcopy, which delegates
# authentication to whatever identity the Azure CLI is already logged in as (see PR #7487/#7496,
# which moved away from azcopy's own --login-type=MSI). But nothing was logging the Azure CLI in
# on this build VM first. This installs azure-cli (only when private packages are actually
# configured, so ordinary builds are unaffected) and logs in as the exact user-assigned managed
# identity attached to the build VM via `az login --identity --resource-id`, matching the pattern
# already used by the post-build scan VM scripts (cis-report.sh, trivy-scan.sh).

# azure_cli_is_present is a thin wrapper around `command -v az` so tests can mock it directly
# instead of depending on whether az happens to be on the PATH of whatever machine runs the tests.
azure_cli_is_present() {
  command -v az > /dev/null 2>&1
}

# write_apt_azure_cli_repo/write_yum_azure_cli_repo are split out so tests can stub the actual
# filesystem writes (which require root on a real build VM) without needing to fake root access.
write_apt_azure_cli_repo() {
  echo "deb [arch=$(dpkg --print-architecture)] https://packages.microsoft.com/repos/azure-cli/ $(lsb_release -cs) main" > /etc/apt/sources.list.d/azure-cli.list
}

write_yum_azure_cli_repo() {
  cat > /etc/yum.repos.d/azure-cli.repo <<-'EOF'
[azure-cli]
name=Azure CLI
baseurl=https://packages.microsoft.com/yumrepos/azure-cli
enabled=1
gpgcheck=1
gpgkey=https://packages.microsoft.com/keys/microsoft.asc
EOF
}

# install_azure_cli_for_private_packages installs the az CLI for the OS families exercised by AKS
# Linux VHD builds that use PRIVATE_PACKAGES_URL. Reuses the apt_get_install/dnf_install retry
# helpers install-dependencies.sh already sources, so this behaves consistently with every other
# package that script installs.
install_azure_cli_for_private_packages() {
  if azure_cli_is_present; then
    echo "azure-cli is already installed, skipping install"
    return 0
  fi

  if isACL "$OS" "$OS_VARIANT"; then
    # ACL (Azure Container Linux) is Flatcar-derived and reports as either OS=AZURECONTAINERLINUX
    # (matched by the `*)` fallback below) or the newer OS=AZURELINUX + VARIANT=AZURECONTAINERLINUX
    # combination - which, left unchecked, would otherwise match the Azure Linux/Mariner branch
    # below and wrongly attempt an rpm/dnf install against a system that doesn't support it. Treat
    # ACL as unsupported explicitly, the same as the `*)` fallback, rather than risk that.
    echo "install_azure_cli_for_private_packages: no azure-cli install recipe for Azure Container Linux (ACL) - private package download will fall back to whatever identity (if any) the Azure CLI is already logged in as"
    return 1
  fi

  case "$OS" in
    "$UBUNTU_OS_NAME")
      if [ "$OS_VERSION" = "26.04" ]; then
        # azure-cli isn't yet published in the Ubuntu 26.04 (Resolute) PMC apt repo - same gap
        # trivy-scan.sh already works around for the post-build scan VM (see its install_azure_cli
        # "TODO(2604)" branch) - so fall back to pip here too until PMC catches up.
        apt_get_update || return 1
        apt_get_install 5 1 60 python3-pip || return 1
        python3 -m pip install azure-cli --break-system-packages || return 1
        export PATH="$HOME/.local/bin:/usr/local/bin:$PATH"
        hash -r
      else
        apt_get_install 5 1 60 ca-certificates curl apt-transport-https lsb-release gnupg || return 1
        write_apt_azure_cli_repo || return 1
        apt_get_update || return 1
        apt_get_install 5 1 60 azure-cli || return 1
      fi
      ;;
    "$MARINER_OS_NAME" | "$MARINER_KATA_OS_NAME" | "$AZURELINUX_OS_NAME" | "$AZURELINUX_KATA_OS_NAME")
      rpm --import https://packages.microsoft.com/keys/microsoft.asc || return 1
      write_yum_azure_cli_repo || return 1
      dnf_install 5 1 60 azure-cli || return 1
      ;;
    *)
      echo "install_azure_cli_for_private_packages: no azure-cli install recipe for OS '$OS' - private package download will fall back to whatever identity (if any) the Azure CLI is already logged in as"
      return 1
      ;;
  esac

  # Belt-and-suspenders: confirm the install actually put a usable az on PATH (e.g. the pip
  # fallback above depends on PATH updates taking effect) rather than silently reporting success.
  if ! azure_cli_is_present; then
    echo "install_azure_cli_for_private_packages: azure-cli install completed but az is still not on PATH"
    return 1
  fi
}

# login_with_user_assigned_managed_identity logs the Azure CLI in as the exact UAMI attached to
# this build VM (by resource ID), rather than letting `az`/`azcopy` guess at which identity to use.
login_with_user_assigned_managed_identity() {
  local resource_id="$1"
  echo "logging into azure with user-assigned managed identity: $resource_id"
  az login --identity --resource-id "$resource_id"
}

# clear_azure_cli_login_state removes the token cache `az login` created (normally under
# $HOME/.azure, i.e. /root/.azure since install-dependencies.sh runs as root) so a live
# managed-identity access token isn't captured into the released VHD image. install-dependencies.sh
# calls this once every private package download has finished, mirroring its existing `rm -f
# ./azcopy` cleanup of the azcopy binary itself right after the same loop.
clear_azure_cli_login_state() {
  az account clear > /dev/null 2>&1 || true
  rm -rf "${HOME:-/root}/.azure"
}

# ensure_azure_login_for_private_packages is the single entry point install-dependencies.sh calls
# before any private package download. It's a no-op (existing AZCLI-delegated azcopy behavior is
# unchanged) when AZURE_MSI_RESOURCE_STRING isn't set, or when azure-cli can't be installed for
# this OS - matching how the Windows build VM's AzCopy login degrades gracefully when no MSI
# resource string is configured.
ensure_azure_login_for_private_packages() {
  if [ -z "${AZURE_MSI_RESOURCE_STRING:-}" ]; then
    echo "AZURE_MSI_RESOURCE_STRING is not set - azcopy will use whatever identity (if any) the Azure CLI is already logged in as"
    return 0
  fi

  if ! install_azure_cli_for_private_packages; then
    echo "Could not install azure-cli - azcopy will use whatever identity (if any) the Azure CLI is already logged in as"
    return 0
  fi

  login_with_user_assigned_managed_identity "$AZURE_MSI_RESOURCE_STRING"
}
