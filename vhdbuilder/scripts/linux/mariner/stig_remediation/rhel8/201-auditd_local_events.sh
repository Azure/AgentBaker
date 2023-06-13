#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 201/364: 'auditd_local_events'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q audit; then

if [ -e "/etc/audit/auditd.conf" ] ; then
    
    LC_ALL=C sed -i "/^\s*local_events\s*=\s*/Id" "/etc/audit/auditd.conf"
else
    touch "/etc/audit/auditd.conf"
fi
cp "/etc/audit/auditd.conf" "/etc/audit/auditd.conf.bak"
# Insert at the end of the file
printf '%s\n' "local_events = yes" >> "/etc/audit/auditd.conf"
# Clean up after ourselves.
rm "/etc/audit/auditd.conf.bak"

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
