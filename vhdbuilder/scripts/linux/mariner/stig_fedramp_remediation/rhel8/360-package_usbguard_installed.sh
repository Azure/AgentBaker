#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 360/364: 'package_usbguard_installed'")

if ! rpm -q --quiet "usbguard" ; then
    dnf install -y "usbguard"
fi
