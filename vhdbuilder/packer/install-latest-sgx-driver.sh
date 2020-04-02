#!/bin/bash

set -e

SCRIPT_NAME="$(basename ${0})"
SCRIPT_DIR="$(dirname ${0})"

if [[ $EUID > 0 ]]; then
    echo "Please run ${SCRIPT_NAME} as sudo"
    exit 1
fi

readonly LOG_FILE="${SCRIPT_DIR}/${SCRIPT_NAME%.*}.log"
touch "${LOG_FILE}"
# exec 1>"${LOG_FILE}"
# exec 2>&1

set -x

version="$(grep DISTRIB_RELEASE /etc/*-release | cut -f 2 -d "=")"
release=$(lsb_release -cs)

function error_exit() {
    echo "Error: ${1}"
    echo "${SCRIPT_NAME} failed! See ${LOG_FILE} for details." >/dev/tty
    exit 1
}

function retrycmd_if_failure() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        timeout $timeout ${@}
        [ $? -eq 0 ] && break || \
        if [ $i -eq $retries ]; then
            echo "Error: Failed to execute \"$@\" after $i attempts"
            return 1
        else
            sleep $wait_sleep
        fi
    done
    echo Executed \"$@\" $i times;
}

function install_dkms() {
    apt update || error_exit "failed to run apt update"
    apt-get -y install dkms || error_exit "failed to install dkms"
}

function install_ubuntu() {
    retrycmd_if_failure 10 10 120 curl -fsSL -O "https://download.01.org/intel-sgx/latest/version.xml" || error_exit "failed to download version.xml"
    dcap_version="$(grep dcap version.xml | grep -o -E "[.0-9]+")"
    sgx_driver_folder_url="https://download.01.org/intel-sgx/sgx-dcap/${dcap_version}/linux"
    retrycmd_if_failure 10 10 120 curl -fsSL -O "${sgx_driver_folder_url}/SHA256SUM_dcap_${dcap_version}" || error_exit "failed to download SHA256SUM_dcap*"
    matched_line="$(grep "distro/ubuntuServer${version}/sgx_linux_x64_driver_.*bin" SHA256SUM_dcap_${dcap_version})"
    read -ra tmp_array <<< "${matched_line}"
    sgx_driver_sha256sum_expected="${tmp_array[0]}"
    sgx_driver_remote_path="${tmp_array[1]}"
    sgx_driver_url="${sgx_driver_folder_url}/${sgx_driver_remote_path}"
    sgx_driver="$(basename "${sgx_driver_url}")"
    retrycmd_if_failure 10 10 120 curl -fsSL -O ${sgx_driver_url} || error_exit "failed to download SGX driver"
    read -ra tmp_array <<< "$(sha256sum "${sgx_driver}")"
    sgx_driver_sha256sum_real="${tmp_array[0]}"
    [[ "${sgx_driver_sha256sum_real}" == "${sgx_driver_sha256sum_expected}" ]] || error_exit "failed SGX driver sha256sum check, downloaded=${sgx_driver_sha256sum_real}, expected=${sgx_driver_sha256sum_expected}"
    chmod a+x ./"${sgx_driver}"
    ./"${sgx_driver}" || error_exit "failed to install SGX driver"
}

function cleanup() {
    rm -f version.xml SHA256SUM_dcap* "${sgx_driver}" "${LOG_FILE}"
}

install_dkms
install_ubuntu && cleanup

set +x
# exec &>/dev/tty
echo "${SCRIPT_NAME} succeeded!"
exit 0
