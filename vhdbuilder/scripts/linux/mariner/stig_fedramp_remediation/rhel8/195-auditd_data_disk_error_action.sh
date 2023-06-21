#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 195/364: 'auditd_data_disk_error_action'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q audit; then


var_auditd_disk_error_action="halt"



#
# If disk_error_action present in /etc/audit/auditd.conf, change value
# to var_auditd_disk_error_action, else
# add "disk_error_action = $var_auditd_disk_error_action" to /etc/audit/auditd.conf
#

if grep --silent ^disk_error_action /etc/audit/auditd.conf ; then
        sed -i 's/^disk_error_action.*/disk_error_action = '"$var_auditd_disk_error_action"'/g' /etc/audit/auditd.conf
else
        echo -e "\n# Set disk_error_action to $var_auditd_disk_error_action per security requirements" >> /etc/audit/auditd.conf
        echo "disk_error_action = $var_auditd_disk_error_action" >> /etc/audit/auditd.conf
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
