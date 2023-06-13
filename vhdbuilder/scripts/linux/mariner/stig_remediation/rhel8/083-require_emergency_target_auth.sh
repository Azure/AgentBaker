#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 83/364: 'require_emergency_target_auth'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

service_file="/usr/lib/systemd/system/emergency.service"

sulogin="/usr/lib/systemd/systemd-sulogin-shell emergency"

if grep "^ExecStart=.*" "$service_file" ; then
    sed -i "s%^ExecStart=.*%ExecStart=-$sulogin%" "$service_file"
else
    echo "ExecStart=-$sulogin" >> "$service_file"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
