#!/bin/bash
# shellcheck disable=SC2034,SC2329

Describe 'Minimal AMDGPU host diagnostics bake'
  setup_amd_diagnostics() {
    TEST_DIR=$(mktemp -d)
    TRACE="${TEST_DIR}/trace"
    : > "${TRACE}"
    OS=UBUNTU OS_VERSION=24.04 CPU_ARCH=amd64 HYPERV_GENERATION=v2 ENABLE_FIPS=false
    COMPONENTS_FILEPATH="${TEST_DIR}/components.json"
    VHD_LOGS_FILEPATH="${TEST_DIR}/vhd.log"
    FAIL_STAGE=""
    PACKAGE_ARCH=amd64
    mkdir -p "${TEST_DIR}/work" "${TEST_DIR}/bin" "${TEST_DIR}/rocm/core-10.0/bin" "${TEST_DIR}/rocm/core-10.0/share/amd_smi"
    jq --arg cli_path "${TEST_DIR}/rocm/core-10.0/bin/amd-smi" '.AMDGPUDiagnostics.cliPath = $cli_path' \
      parts/common/components.json > "${COMPONENTS_FILEPATH}"
    AMDSMI_PACKAGE=$(jq -r '.AMDGPUDiagnostics.amdsmiPackage' "${COMPONENTS_FILEPATH}")
    AMDSMI_VERSION=$(jq -r '.AMDGPUDiagnostics.amdsmiVersion' "${COMPONENTS_FILEPATH}")
    SYSDEPS_PACKAGE=$(jq -r '.AMDGPUDiagnostics.sysdepsPackage' "${COMPONENTS_FILEPATH}")
    SYSDEPS_VERSION=$(jq -r '.AMDGPUDiagnostics.sysdepsVersion' "${COMPONENTS_FILEPATH}")
    # Run the production function, remapping only owned filesystem locations.
    # Network, package metadata and package installation are mocked below.
    eval "$(sed -n '/^installAMDGPUDiagnostics()/,/^}$/p' vhdbuilder/scripts/linux/ubuntu/tool_installs_ubuntu.sh |
      sed -e "s|/opt/rocm|${TEST_DIR}/rocm|g" -e "s|/usr/local/bin|${TEST_DIR}/bin|g" \
          -e "s|/tmp/amd-gpu-diagnostics|${TEST_DIR}/work/amd-gpu-diagnostics|g")"
  }
  cleanup_amd_diagnostics() { rm -rf "${TEST_DIR}"; }
  BeforeEach setup_amd_diagnostics
  AfterEach cleanup_amd_diagnostics

  retrycmd_if_failure() { shift 3; "$@"; }
  apt_get_update() { echo ubuntu-update >> "${TRACE}"; }
  apt_get_install() {
    shift 3
    echo "ubuntu-install $*" >> "${TRACE}"
    case " $* " in
      *'.deb '*)
        echo vendor-install >> "${TRACE}"
        [ "${FAIL_STAGE}" != install ] || return 1
        [ "${FAIL_STAGE}" != missing_cli ] || return 0
        # A CPU builder cannot run CLI commands that initialize GPU hardware.
        printf '#!/bin/bash\necho "amd-smi $*" >> "%s"\nexit 99\n' \
          "${TRACE}" > "${TEST_DIR}/rocm/core-10.0/bin/amd-smi"
        [ "${FAIL_STAGE}" = nonexecutable_cli ] || chmod +x "${TEST_DIR}/rocm/core-10.0/bin/amd-smi"
        if [ "${FAIL_STAGE}" = python_import ]; then
          echo 'raise RuntimeError("diagnostics import failed")' > "${TEST_DIR}/rocm/core-10.0/share/amd_smi/amdsmi.py"
        elif [ "${FAIL_STAGE}" = library_version ]; then
          printf 'def amdsmi_get_lib_version():\n    return {"major": 0}\n' > "${TEST_DIR}/rocm/core-10.0/share/amd_smi/amdsmi.py"
        else
          printf '__version__ = "26.0.0"\ndef amdsmi_get_lib_version():\n    with open("%s", "a") as trace:\n        trace.write("amdsmi-library-version\\n")\n    return {"major": 26, "minor": 0, "release": 0}\n' \
            "${TRACE}" > "${TEST_DIR}/rocm/core-10.0/share/amd_smi/amdsmi.py"
        fi
        ;;
      *) [ "${FAIL_STAGE}" != prerequisites ] ;;
    esac
  }
  apt-mark() {
    echo "apt-mark $*" >> "${TRACE}"
    printf 'retained-package %s\n' "$@" >> "${TRACE}"
    [ "${FAIL_STAGE}" != retain ]
  }
  apt-get() {
    echo "apt-get $*" >> "${TRACE}"
    local option
    for option in "$@"; do
      case "${option}" in
        Dir::Etc::sourcelist=*) printf 'repository %s\n' "$(cat "${option#*=}")" >> "${TRACE}" ;;
      esac
    done
    case " $* " in
      *' update '*) [ "${FAIL_STAGE}" != metadata ] ;;
      *' download '*)
        [ "${FAIL_STAGE}" != download ] || return 1
        touch amdsmi.deb sysdeps.deb
        [ "${FAIL_STAGE}" != extra ] || touch runtime.deb
        ;;
      *) return 99 ;;
    esac
  }
  curl() {
    echo key-download >> "${TRACE}"
    while [ "$#" -gt 0 ]; do
      if [ "$1" = -o ]; then printf 'test-key\n' > "$2"; return; fi
      shift
    done
    return 99
  }
  gpg() {
    case " $* " in
      *' --show-keys '*)
        echo 'pub:::::::::'
        if [ "${FAIL_STAGE}" = key ]; then
          echo 'fpr:::::::::WRONG:'
        else
          echo 'fpr:::::::::D0F004A0025A1145C7807FCD0701EAC4D5E02107:'
        fi
        echo 'sub:::::::::'
        echo 'fpr:::::::::UNRELATED_SUBKEY:'
        ;;
      *)
        while [ "$#" -gt 0 ]; do
          if [ "$1" = --output ]; then touch "$2"; return; fi
          shift
        done
        return 99
        ;;
    esac
  }
  dpkg-deb() {
    case "$2:$3" in
      */amdsmi.deb:Package) echo "${AMDSMI_PACKAGE}" ;;
      */sysdeps.deb:Package) echo "${SYSDEPS_PACKAGE}" ;;
      */runtime.deb:Package) echo amdrocm-runtime10.0 ;;
      *:Architecture) echo "${PACKAGE_ARCH}" ;;
      */amdsmi.deb:Version)
        if [ "${FAIL_STAGE}" = amdsmi_version ]; then echo wrong-version; else echo "${AMDSMI_VERSION}"; fi
        ;;
      */sysdeps.deb:Version)
        if [ "${FAIL_STAGE}" = sysdeps_version ]; then echo wrong-version; else echo "${SYSDEPS_VERSION}"; fi
        ;;
      *) return 99 ;;
    esac
  }
  dpkg-query() {
    case "${*: -1}" in
      "${AMDSMI_PACKAGE}")
        if [ "${FAIL_STAGE}" = installed_version ]; then echo 'install ok installed wrong-version';
        else echo "install ok installed ${AMDSMI_VERSION}"; fi
        ;;
      "${SYSDEPS_PACKAGE}") echo "install ok installed ${SYSDEPS_VERSION}" ;;
      *) return 99 ;;
    esac
  }
  run_diagnostics_installer() {
    local status=0
    # Conditional callers disable errexit: installation must still fail closed.
    installAMDGPUDiagnostics || status=$?
    echo "remaining-work-files=$(find "${TEST_DIR}/work" -mindepth 1 | wc -l | tr -d ' ')"
    return "${status}"
  }
  repeat_diagnostics_installer() {
    installAMDGPUDiagnostics && installAMDGPUDiagnostics &&
      [ "$(readlink "${TEST_DIR}/bin/amd-smi")" = "${TEST_DIR}/rocm/core-10.0/bin/amd-smi" ]
  }

  It 'installs only the pinned AMD SMI subset from authenticated isolated metadata'
    When run run_diagnostics_installer
    The status should be success
    The output should include '"python_module": "26.0.0"'
    The output should include 'remaining-work-files=0'
    The stderr should eq ''
    The contents of file "${TRACE}" should include "download ${AMDSMI_PACKAGE}=${AMDSMI_VERSION} ${SYSDEPS_PACKAGE}=${SYSDEPS_VERSION}"
    The contents of file "${TRACE}" should include 'https://stable.repo.amd.com/rocm/core/packages/ubuntu2404/ stable main'
    The contents of file "${TRACE}" should include 'Dir::Etc::sourceparts=-'
    The contents of file "${TRACE}" should include 'AllowUnauthenticated=false'
    The contents of file "${TRACE}" should include 'AllowInsecureRepositories=false'
    The contents of file "${TRACE}" should include 'apt-mark manual'
    The contents of file "${TRACE}" should include 'retained-package pciutils'
    The contents of file "${TRACE}" should include 'retained-package numactl'
    The contents of file "${TRACE}" should include 'retained-package python3'
    The contents of file "${TRACE}" should include 'retained-package libstdc++6'
    The contents of file "${TRACE}" should include 'retained-package libgcc-s1'
    The contents of file "${TRACE}" should include "${AMDSMI_PACKAGE} ${SYSDEPS_PACKAGE}"
    The contents of file "${TRACE}" should include 'amdsmi-library-version'
    The contents of file "${TRACE}" should not include 'amd-smi --help'
    The contents of file "${TRACE}" should not include 'amdrocm-runtime'
    The contents of file "${TRACE}" should not include 'amdrocm10.0-gfx942'
    The contents of file "${TRACE}" should not include 'amdrocm-llvm'
    The path "${TEST_DIR}/bin/amd-smi" should be symlink
  End

  It 'rejects a signing key mismatch before querying the AMD repository'
    FAIL_STAGE=key
    When run run_diagnostics_installer
    The status should be failure
    The output should eq 'remaining-work-files=0'
    The stderr should include 'signing key fingerprint mismatch'
    The contents of file "${TRACE}" should not include 'apt-get '
    The path "${TEST_DIR}/bin/amd-smi" should not be exist
  End

  Describe 'failed repository and package transactions'
    Parameters
      prerequisites
      metadata
      download
      amdsmi_version
      sysdeps_version
      install
      retain
      installed_version
    End
    It 'fails and removes temporary repository state'
      FAIL_STAGE="$1"
      When run run_diagnostics_installer
      The status should be failure
      The output should eq 'remaining-work-files=0'
      The stderr should eq ''
    End
  End

  It 'rejects an unexpected extra vendor archive before installing it'
    FAIL_STAGE=extra
    When run run_diagnostics_installer
    The status should be failure
    The output should eq 'remaining-work-files=0'
    The stderr should include 'Unexpected'
    The contents of file "${TRACE}" should not include 'vendor-install'
  End

  It 'rejects a package for the wrong architecture before installing it'
    PACKAGE_ARCH=arm64
    When run run_diagnostics_installer
    The status should be failure
    The output should eq 'remaining-work-files=0'
    The stderr should include 'architecture'
    The contents of file "${TRACE}" should not include 'vendor-install'
  End

  Describe 'a broken installed diagnostic CLI'
    Parameters
      missing_cli
      nonexecutable_cli
    End
    It 'fails its CPU-only executable smoke check'
      FAIL_STAGE="$1"
      When run run_diagnostics_installer
      The status should be failure
      The output should eq 'remaining-work-files=0'
      The stderr should eq ''
    End
  End

  It 'fails when the installed Python binding cannot import'
    FAIL_STAGE=python_import
    When run run_diagnostics_installer
    The status should be failure
    The output should eq 'remaining-work-files=0'
    The stderr should include 'diagnostics import failed'
  End

  It 'fails when the diagnostic library returns an invalid version'
    FAIL_STAGE=library_version
    When run run_diagnostics_installer
    The status should be failure
    The output should eq 'remaining-work-files=0'
    The stderr should include 'AssertionError'
  End

  It 'replaces a stale command symlink and repeats installation idempotently'
    ln -s "${TEST_DIR}/missing" "${TEST_DIR}/bin/amd-smi"
    When run repeat_diagnostics_installer
    The status should be success
    The output should include '"python_module": "26.0.0"'
    The stderr should eq ''
    The path "${TEST_DIR}/bin/amd-smi" should be symlink
  End

  It 'does not overwrite an existing regular file at the command location'
    printf 'owned-by-another-package\n' > "${TEST_DIR}/bin/amd-smi"
    When run run_diagnostics_installer
    The status should be failure
    The output should include 'remaining-work-files=0'
    The stderr should include 'amd-smi'
    The contents of file "${TEST_DIR}/bin/amd-smi" should eq 'owned-by-another-package'
  End

  It 'does not replace or write into an existing directory at the command location'
    mkdir "${TEST_DIR}/bin/amd-smi"
    printf 'preserve\n' > "${TEST_DIR}/bin/amd-smi/owned-file"
    When run run_diagnostics_installer
    The status should be failure
    The output should include 'remaining-work-files=0'
    The stderr should include 'Refusing to replace a non-symlink'
    The contents of file "${TEST_DIR}/bin/amd-smi/owned-file" should eq 'preserve'
    The path "${TEST_DIR}/bin/amd-smi/amd-smi" should not be exist
  End
End
