#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 204/364: 'auditd_overflow_action'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q audit; then

if [ -e "/etc/audit/auditd.conf" ] ; then
    
    LC_ALL=C sed -i "/^\s*overflow_action\s*=\s*/Id" "/etc/audit/auditd.conf"
else
    printf '%s\n' "Path '/etc/audit/auditd.conf' wasn't found on this system. Refusing to continue." >&2
    return 1
fi
cp "/etc/audit/auditd.conf" "/etc/audit/auditd.conf.bak"
# Insert at the end of the file
printf '%s\n' "overflow_action = syslog" >> "/etc/audit/auditd.conf"
# Clean up after ourselves.
rm "/etc/audit/auditd.conf.bak"

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
