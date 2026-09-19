#!/bin/bash
# shellcheck disable=SC2034,SC2329

Describe 'AMD VHD content validation'
  setup_amd_content() {
    TEST_DIR=$(mktemp -d)
    FEATURE_FLAGS=AMD_GPU OS_SKU=Ubuntu OS_VERSION=24.04 ENABLE_FIPS=false
    COMPONENTS_FILEPATH="${PWD}/parts/common/components.json"
    KERNEL=6.8.0-test-azure
    FAIL_STAGE=""
    PACKAGE_VERSION=$(jq -r '.AMDGPUDriver.packageVersion' "${COMPONENTS_FILEPATH}")
    FIRMWARE_VERSION=$(jq -r '.AMDGPUDriver.firmwarePackageVersion' "${COMPONENTS_FILEPATH}")
    MODULE_VERSION=$(jq -r '.AMDGPUDriver.moduleVersion' "${COMPONENTS_FILEPATH}")
    DKMS_VERSION=$(jq -r '.AMDGPUDriver.dkmsVersion' "${COMPONENTS_FILEPATH}")
    AMDSMI_PACKAGE=$(jq -r '.AMDGPUDiagnostics.amdsmiPackage' "${COMPONENTS_FILEPATH}")
    SYSDEPS_PACKAGE=$(jq -r '.AMDGPUDiagnostics.sysdepsPackage' "${COMPONENTS_FILEPATH}")
    mkdir -p "${TEST_DIR}/marker" "${TEST_DIR}/modules/${KERNEL}/build" "${TEST_DIR}/sources/amdgpu-${DKMS_VERSION}"
    touch "${TEST_DIR}/modules/${KERNEL}/build/Makefile" "${TEST_DIR}/sources/amdgpu-${DKMS_VERSION}/dkms.conf"
    jq -n --arg driver "${PACKAGE_VERSION}" --arg firmware "${FIRMWARE_VERSION}" --arg module "${MODULE_VERSION}" \
      '{schema_version: 1, package_version: $driver, firmware_package_version: $firmware, module_version: $module,
        kernel_version: "earlier-bake-kernel"}' > "${TEST_DIR}/marker/driver.json"
    eval "$(sed -n '/^testAMDGPUDriver()/,/^}$/p' vhdbuilder/packer/test/linux-vhd-content-test.sh |
      sed -e "s|/opt/azure/amd-gpu|${TEST_DIR}/marker|g" -e "s|/lib/modules|${TEST_DIR}/modules|g" \
          -e "s|/usr/src|${TEST_DIR}/sources|g")"
  }
  cleanup_amd_content() { rm -rf "${TEST_DIR}"; }
  BeforeEach setup_amd_content
  AfterEach cleanup_amd_content
  err() { printf '%s: %s\n' "$1" "$2" >&2; }
  uname() { if [ "$1" = -m ]; then echo x86_64; else echo "${KERNEL}"; fi; }
  dpkg-query() {
    if [ "$#" -eq 2 ]; then
      printf 'ii  amdgpu-dkms\nii  amdgpu-dkms-firmware\n'
      printf 'ii  %s\nii  %s:amd64\n' "${AMDSMI_PACKAGE}" "${SYSDEPS_PACKAGE}"
      if [ "${FAIL_STAGE}" = sdk ]; then echo 'ii  amdrocm-hip-sdk'; fi
      if [ "${FAIL_STAGE}" = similar_package ]; then echo "ii  ${AMDSMI_PACKAGE}-dev"; fi
      if [ "${FAIL_STAGE}" = older_diagnostics ]; then echo 'ii  amdrocm-amdsmi9.0'; fi
      return
    fi
    case "${*: -1}" in
      amdgpu-dkms) echo "install ok installed ${PACKAGE_VERSION}" ;;
      amdgpu-dkms-firmware)
        if [ "${FAIL_STAGE}" = firmware ]; then echo 'install ok installed wrong'; else echo "install ok installed ${FIRMWARE_VERSION}"; fi
        ;;
      build-essential) if [ "${FAIL_STAGE}" = dependencies ]; then return 1; else echo 'install ok installed'; fi ;;
      *) echo 'install ok installed' ;;
    esac
  }
  dkms() {
    if [ "${FAIL_STAGE}" = registration ]; then
      echo "amdgpu/${DKMS_VERSION}, ${KERNEL}, x86_64: added"
    else
      echo "amdgpu/${DKMS_VERSION}, ${KERNEL}, x86_64: installed"
    fi
  }
  modinfo() {
    case "$4" in
      filename)
        if [ "${FAIL_STAGE}" = inbox ]; then echo /kernel/inbox/amdgpu.ko;
        else echo "${TEST_DIR}/modules/${KERNEL}/updates/dkms/amdgpu.ko.zst"; fi
        ;;
      version) if [ "${FAIL_STAGE}" = version ]; then echo wrong; else echo "${MODULE_VERSION}"; fi ;;
      vermagic) if [ "${FAIL_STAGE}" = kernel ]; then echo wrong; else echo "${KERNEL} SMP mod_unload modversions"; fi ;;
    esac
  }
  modprobe() {
    [ "$*" = -c ] || { echo 'unexpected hardware-dependent command' >&2; return 99; }
    if [ "${FAIL_STAGE}" = blacklist ]; then echo 'blacklist amdgpu'; fi
  }

  It 'validates retained DKMS state on a CPU machine, treating bake kernel as provenance'
    When run testAMDGPUDriver
    The status should be success
    The output should include 'rebuild dependencies verified'
    The stderr should eq ''
  End

  It 'skips ordinary images without inspecting AMD state'
    FEATURE_FLAGS=NVIDIA_CUDA_PREBAKE
    rm "${TEST_DIR}/marker/driver.json"
    When run testAMDGPUDriver
    The status should be success
    The output should eq ''
    The stderr should eq ''
  End

  Describe 'unqualified image contents'
    Parameters
      firmware 'differs from pinned versions'
      registration 'not installed for the running kernel'
      inbox 'inbox or a missing module'
      version 'version or vermagic mismatch'
      kernel 'version or vermagic mismatch'
      dependencies 'Missing retained build dependency'
      sdk 'Unexpected GPU userspace'
      similar_package 'Unexpected GPU userspace'
      older_diagnostics 'Unexpected GPU userspace'
      blacklist 'disabled in modprobe configuration'
    End
    It 'reports every failure on stderr as required by the VM content-test runner'
      FAIL_STAGE="$1"
      When run testAMDGPUDriver
      The status should be failure
      The output should eq ''
      The stderr should include "$2"
    End
  End

  It 'reports missing marker metadata on stderr'
    printf '{}\n' > "${TEST_DIR}/marker/driver.json"
    When run testAMDGPUDriver
    The status should be failure
    The output should eq ''
    The stderr should include 'Missing or inconsistent AMDGPU driver marker'
  End

  It 'reports removed DKMS source even if the compiled module still exists'
    rm "${TEST_DIR}/sources/amdgpu-${DKMS_VERSION}/dkms.conf"
    When run testAMDGPUDriver
    The status should be failure
    The output should eq ''
    The stderr should include 'Headers or AMDGPU sources'
  End
End

Describe 'AMD SMI VHD content validation'
  setup_amd_diagnostics_content() {
    TEST_DIR=$(mktemp -d)
    FEATURE_FLAGS=AMD_GPU
    FAIL_STAGE=""
    COMPONENTS_FILEPATH="${TEST_DIR}/components.json"
    CLI_PATH="${TEST_DIR}/rocm/core-10.0/bin/amd-smi"
    CLI_TARGET="${TEST_DIR}/rocm/core-10.0/libexec/amdsmi_cli/amdsmi_cli.py"
    MODULE_PATH="${TEST_DIR}/rocm/core-10.0/share/amd_smi/amdsmi/__init__.py"
    mkdir -p "$(dirname "${CLI_PATH}")" "$(dirname "${CLI_TARGET}")" "$(dirname "${MODULE_PATH}")" \
      "${TEST_DIR}/bin" "${TEST_DIR}/environment"
    printf '#!/bin/sh\nexit 99\n' > "${CLI_TARGET}"
    ln -s ../libexec/amdsmi_cli/amdsmi_cli.py "${CLI_PATH}"
    chmod +x "${CLI_PATH}"
    ln -s "${CLI_PATH}" "${TEST_DIR}/bin/amd-smi"
    cat > "${MODULE_PATH}" <<'PY'
from pathlib import Path
__version__ = "27.0.0-test"
def amdsmi_get_lib_version():
    Path(__file__).with_name("library-queried").touch()
    return {"major": 27, "minor": 0, "release": 0}
def amdsmi_init():
    raise RuntimeError("GPU initialization must not run on the CPU bake machine")
PY
    printf 'raise ImportError("unrelated PYTHONPATH must not affect the bake check")\n' > "${TEST_DIR}/environment/amdsmi.py"
    jq --arg cli "${CLI_PATH}" '.AMDGPUDiagnostics.cliPath = $cli' parts/common/components.json > "${COMPONENTS_FILEPATH}"
    AMDSMI_PACKAGE=$(jq -r '.AMDGPUDiagnostics.amdsmiPackage' "${COMPONENTS_FILEPATH}")
    AMDSMI_VERSION=$(jq -r '.AMDGPUDiagnostics.amdsmiVersion' "${COMPONENTS_FILEPATH}")
    SYSDEPS_PACKAGE=$(jq -r '.AMDGPUDiagnostics.sysdepsPackage' "${COMPONENTS_FILEPATH}")
    SYSDEPS_VERSION=$(jq -r '.AMDGPUDiagnostics.sysdepsVersion' "${COMPONENTS_FILEPATH}")
    eval "$(sed -n '/^testAMDGPUDiagnostics()/,/^}$/p' vhdbuilder/packer/test/linux-vhd-content-test.sh |
      sed -e "s|/usr/local/bin/amd-smi|${TEST_DIR}/bin/amd-smi|g" \
          -e "s|/opt/rocm/|${TEST_DIR}/rocm/|g")"
  }
  cleanup_amd_diagnostics_content() { rm -rf "${TEST_DIR}"; }
  BeforeEach setup_amd_diagnostics_content
  AfterEach cleanup_amd_diagnostics_content
  err() { printf '%s: %s\n' "$1" "$2" >&2; }
  dpkg-query() {
    case "${*: -1}" in
      "${AMDSMI_PACKAGE}")
        if [ "${FAIL_STAGE}" = amdsmi ]; then echo 'install ok installed wrong'; else echo "install ok installed ${AMDSMI_VERSION}"; fi
        ;;
      "${SYSDEPS_PACKAGE}")
        if [ "${FAIL_STAGE}" = sysdeps ]; then echo 'deinstall ok config-files'; else echo "install ok installed ${SYSDEPS_VERSION}"; fi
        ;;
      pciutils|numactl)
        if [ "${FAIL_STAGE}" = "${*: -1}_package" ]; then echo 'deinstall ok config-files'; else echo 'install ok installed'; fi
        ;;
    esac
  }
  lspci() {
    [ "$*" = --version ] || return 99
    [ "${FAIL_STAGE}" != lspci_cli ] || return 1
    echo 'lspci version 3.10.0'
  }
  numactl() {
    [ "$*" = --version ] || return 99
    [ "${FAIL_STAGE}" != numactl_cli ] || return 1
    echo 'numactl version 2.0.18'
  }

  It 'imports the library beside the vendor bin-to-libexec symlink with isolated Python'
    PYTHONPATH="${TEST_DIR}/environment"
    export PYTHONPATH
    When run testAMDGPUDiagnostics
    The status should be success
    The output should include 'hardware-independent library loading verified'
    The stderr should eq ''
    The path "${TEST_DIR}/rocm/core-10.0/share/amd_smi/amdsmi/library-queried" should be exist
  End

  It 'skips ordinary images'
    FEATURE_FLAGS=NVIDIA_CUDA_PREBAKE
    rm "${TEST_DIR}/bin/amd-smi"
    When run testAMDGPUDiagnostics
    The status should be success
    The output should eq ''
    The stderr should eq ''
  End

  Describe 'incomplete diagnostics packages'
    Parameters
      amdsmi 'differ from pinned packages'
      sysdeps 'differ from pinned packages'
      library 'library failed to load'
      pciutils_package 'Missing host diagnostics package pciutils'
      numactl_package 'Missing host diagnostics package numactl'
      lspci_cli 'Host diagnostics command lspci failed'
      numactl_cli 'Host diagnostics command numactl failed'
    End
    It 'fails the content check'
      FAIL_STAGE="$1"
      if [ "${FAIL_STAGE}" = library ]; then printf 'raise ImportError("missing AMD SMI library")\n' > "${MODULE_PATH}"; fi
      When run testAMDGPUDiagnostics
      The status should be failure
      The output should eq ''
      The stderr should include "$2"
    End
  End

  It 'rejects a command symlink pointing to a different executable'
    ln -sfn /bin/false "${TEST_DIR}/bin/amd-smi"
    When run testAMDGPUDiagnostics
    The status should be failure
    The output should eq ''
    The stderr should include 'executable or command symlink'
  End

  It 'rejects a non-executable CLI'
    chmod -x "${CLI_PATH}"
    When run testAMDGPUDiagnostics
    The status should be failure
    The output should eq ''
    The stderr should include 'executable or command symlink'
  End

  It 'reports missing diagnostics metadata'
    printf '{}\n' > "${COMPONENTS_FILEPATH}"
    When run testAMDGPUDiagnostics
    The status should be failure
    The output should eq ''
    The stderr should include 'Missing AMDGPU diagnostics component metadata'
  End
End
