#!/bin/bash

echo "Sourcing tool_installs_mariner.sh"

installAscBaseline() {
   echo "Mariner TODO: installAscBaseline"
}

installBcc() {
    echo "Installing BCC tools..."
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
    apt_get_install 120 5 25 bcc-tools kernel-headers-$(uname -r) || exit $ERR_BCC_INSTALL_TIMEOUT
}

configGPUDrivers() {
    echo "Not installing GPU drivers on Mariner"
}

disableSystemdTimesyncdAndEnableNTP() {
    echo "Mariner TODO: disableSystemdTimesyncdAndEnableNTP"
}