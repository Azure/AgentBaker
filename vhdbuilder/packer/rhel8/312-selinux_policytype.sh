#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 312/364: 'selinux_policytype'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then




if ! rpm -q --quiet "selinux-policy" ; then
    dnf install -y "selinux-policy"
fi
var_selinux_policy_name="targeted"



if [ -e "/etc/selinux/config" ] ; then
    
    LC_ALL=C sed -i "/^SELINUXTYPE=/Id" "/etc/selinux/config"
else
    touch "/etc/selinux/config"
fi
cp "/etc/selinux/config" "/etc/selinux/config.bak"
# Insert at the end of the file
printf '%s\n' "SELINUXTYPE=$var_selinux_policy_name" >> "/etc/selinux/config"
# Clean up after ourselves.
rm "/etc/selinux/config.bak"

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
