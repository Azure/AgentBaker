#!/bin/bash

VULNERABLE_KERNEL_MODULE_DENY_PATTERN='^(install[[:space:]]+(algif_aead|esp4|esp6|rxrpc)[[:space:]]+[/]bin[/]false|blacklist[[:space:]]+(algif_aead|esp4|esp6|rxrpc))([[:space:]]+.*)?$'

kernelVersionGe() {
  local version_a="$1"
  local version_b="$2"
  local sorted
  local highest_version

  sorted=$(printf "%s\n%s\n" "$version_a" "$version_b" | sort -V)
  highest_version=$(printf "%s\n" "$sorted" | tail -n 1)
  [ "$version_a" = "$highest_version" ]
}

ubuntuKernelIncludesVulnerableModuleFixes() {
  local kernel_release
  local fixed_kernel
  local os_version

  kernel_release="$(uname -r 2>/dev/null || true)"
  if [ -z "$kernel_release" ]; then
    return 1
  fi

  # Prefer the OS_VERSION env var, but fall back to /etc/os-release when it is empty.
  # VHD build scripts run in environments where OS_VERSION is not always exported, so
  # relying on it alone made out-of-scope releases (e.g. Ubuntu 26.04) fall through to the
  # default bake path and ship the deny rules. Reading /etc/os-release keeps this gate
  # consistent with cleanup-vhd.sh and cse_helpers.sh, which detect the release the same way.
  os_version="${OS_VERSION}"
  if [ -z "$os_version" ]; then
    os_version="$(awk -F= '$1 == "VERSION_ID" { gsub(/"/, "", $2); print $2; exit }' /etc/os-release 2>/dev/null || true)"
  fi
  if [ -z "$os_version" ]; then
    return 1
  fi

  case "$os_version" in
    20.04)
      printf '%s\n' "$kernel_release" | grep -Eq '^5\.4\.0-[0-9]+-azure-fips$' || return 1
      fixed_kernel="5.4.0-1164-azure-fips"
      ;;
    22.04)
      case "$kernel_release" in
        # azure-fde (CVM) and azure-fips share the azure kernel ABI and fix threshold.
        *-azure|*-azure-fde|*-azure-fips) fixed_kernel="5.15.0-1116-azure" ;;
        *-generic) fixed_kernel="5.15.0-181-generic" ;;
        *) return 1 ;;
      esac
      ;;
    24.04)
      case "$kernel_release" in
        # azure-fde (CVM) and azure-fips share the azure kernel ABI and fix threshold.
        *-azure|*-azure-fde|*-azure-fips) fixed_kernel="6.8.0-1058-azure" ;;
        *-generic) fixed_kernel="6.8.0-124-generic" ;;
        *) return 1 ;;
      esac
      ;;
    # Future Ubuntu releases are not in this mitigation scope by default; if a
    # future release needs the deny rules, add an explicit case above.
    *) return 0 ;;
  esac

  kernelVersionGe "$kernel_release" "$fixed_kernel"
}

removeVulnerableKernelModuleDenyRulesFromModprobeDirectory() {
  local modprobe_conf
  local tmp_modprobe_conf

  for modprobe_conf in "$@"; do
    [ -f "$modprobe_conf" ] || continue
    tmp_modprobe_conf="${modprobe_conf}.tmp.$$"
    sed -E "/$VULNERABLE_KERNEL_MODULE_DENY_PATTERN/d" "$modprobe_conf" > "$tmp_modprobe_conf" || {
      rm -f "$tmp_modprobe_conf"
      return "$ERR_PACKER_COPY_FILE"
    }
    cat "$tmp_modprobe_conf" > "$modprobe_conf" || {
      rm -f "$tmp_modprobe_conf"
      return "$ERR_PACKER_COPY_FILE"
    }
    rm -f "$tmp_modprobe_conf" || return "$ERR_PACKER_COPY_FILE"
  done
}

# CVE-2026-31431 (Copy Fail), DirtyFrag, Fragnesia mitigation: copyPackerFiles bakes the full
# modprobe-CIS.conf, including the algif_aead / esp4 / esp6 / rxrpc deny rules. Strip only
# those rules (and their comments) on OS streams that no longer need them; the rest of the
# CIS module deny list (dccp/sctp/rds/tipc/cramfs/etc.) stays intact on every OS stream.
#
# The vulnerable-module block is removed on fixed Ubuntu 22.04 / 24.04 kernels,
# by default on future Ubuntu releases that are not explicitly in mitigation scope,
# and on AzureLinux 3.0 (non-OSGuard) because:
#   1. Ubuntu 22.04 linux-azure 5.15.0-1116-azure and Ubuntu 24.04 linux-azure
#      6.8.0-1058-azure include the fixes. The CSE still applies the deny rules
#      at runtime if it detects an older vulnerable Ubuntu kernel.
#   2. The upstream kernel fix in 6.6.139.1-1.azl3+ supersedes the modprobe blacklist.
#   3. Customer workloads require those kernel modules; the bake-in actively blocks
#      legitimate use cases on fixed kernels.
# See https://github.com/Azure/AKS/issues/5753.
# Other Ubuntu 20.04 kernels, Mariner / AzureLinux 2.0, and AzureLinux OSGuard keep the full bake-in.
configureVulnerableKernelModuleDenyRules() {
  local modprobe_cis=/etc/modprobe.d/CIS.conf
  local context_message
  local modprobe_files=("$@")
  if [ "${#modprobe_files[@]}" -eq 0 ]; then
    modprobe_files=(/etc/modprobe.d/*.conf)
  fi

  if isUbuntu "$OS" && ubuntuKernelIncludesVulnerableModuleFixes; then
    context_message="on Ubuntu ${OS_VERSION} (fixed or future Ubuntu kernels are not in mitigation scope)"
  elif isAzureLinux "$OS" "$OS_VARIANT" && [ "${OS_VERSION}" = "3.0" ] && ! isAzureLinuxOSGuard "$OS" "$OS_VARIANT"; then
    context_message="on AzureLinux 3.0 (kernel 6.6.139.1-1.azl3+ has upstream fix; OSGuard intentionally retains the full bake-in)"
  else
    return 0
  fi

  echo "Removing algif_aead / esp4 / esp6 / rxrpc deny rules from modprobe.d ${context_message}"
  sed -i -E \
    -e '/^# CVE-2026-31431 \(Copy Fail\):/d' \
    -e '/^# until kernel fix is available\. See https:\/\/ubuntu\.com\/blog\/copy-fail-vulnerability-fixes-available$/d' \
    -e '/^# DirtyFrag \/ CopyFail2:/d' \
    -e '/^# write LPE vulnerabilities\. rxrpc path bypasses AppArmor userns restrictions\.$/d' \
    -e '/^# AKS platform components do not require kernel IPsec ESP\/XFRM or AFS\/RxRPC\.$/d' \
    -e '/^# Disabling these modules prevents workloads that rely on those protocols from working$/d' \
    -e '/^# on the node\. See https:\/\/github\.com\/V4bel\/dirtyfrag$/d' \
    "$modprobe_cis" || return 1
  removeVulnerableKernelModuleDenyRulesFromModprobeDirectory "${modprobe_files[@]}" || return 1
  if grep -qsE "$VULNERABLE_KERNEL_MODULE_DENY_PATTERN" "${modprobe_files[@]}" 2>/dev/null; then
    echo "Failed to remove vulnerable module deny rules from /etc/modprobe.d/*.conf"
    grep -nE "$VULNERABLE_KERNEL_MODULE_DENY_PATTERN" "${modprobe_files[@]}" || true
    return 1
  fi
}

configureServiceExecutionTelemetryForSystemdVersion() {
    local systemd_version="$1"
    local unit_dir="${2:-/etc/systemd/system}"
    local collector_unit
    local service

    case "$systemd_version" in
        ''|*[!0-9]*)
            echo "invalid systemd version for service execution telemetry: ${systemd_version}" >&2
            return 1
            ;;
    esac

    for service in \
        cgroup-memory-telemetry.service \
        cgroup-pressure-telemetry.service \
        aks-log-collector.service; do
        case "$service" in
            cgroup-memory-telemetry.service)
                collector_unit='service-execution-telemetry@cgroup\x2dmemory\x2dtelemetry.service.service'
                ;;
            cgroup-pressure-telemetry.service)
                collector_unit='service-execution-telemetry@cgroup\x2dpressure\x2dtelemetry.service.service'
                ;;
            aks-log-collector.service)
                collector_unit='service-execution-telemetry@aks\x2dlog\x2dcollector.service.service'
                ;;
        esac

        # Systemd retains the memory accounting of an exited unit from version 256, so the
        # ExecStopPost snapshot is only needed below that. Dropping it elsewhere also keeps
        # the snapshot process out of the very cgroup whose peak it would otherwise inflate.
        if [ "$systemd_version" -ge 256 ]; then
            sed -i \
                -e '/^ExecStopPost=-\/opt\/scripts\/service-execution-telemetry\.sh --snapshot-memory-peak %n$/d' \
                "${unit_dir}/${service}" || return 1
        fi

        if [ "$systemd_version" -ge 258 ]; then
            sed -i \
                -e '/^CPUAccounting=true$/d' \
                -e '/^MemoryAccounting=true$/d' \
                "${unit_dir}/${service}" || return 1
            continue
        fi

        if [ "$systemd_version" -ge 249 ]; then
            continue
        fi

        sed -i \
            -e '/^OnSuccess=service-execution-telemetry@/d' \
            -e '/^OnFailure=service-execution-telemetry@/d' \
            "${unit_dir}/${service}" || return 1
        # ExecStopPost consumes one escape layer before systemctl receives the unit name.
        printf 'ExecStopPost=-/bin/systemctl --no-block start %s\n' \
            "${collector_unit//\\/\\\\}" >> "${unit_dir}/${service}" || return 1
    done
}
