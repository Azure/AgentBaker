#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 37/292: 'accounts_passwords_pam_faillock_audit'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q pam; then



if [ -e "/etc/security/faillock.conf" ] ; then
    
    LC_ALL=C sed -i "#^\s*audit#Id" "/etc/security/faillock.conf"
else
    touch "/etc/security/faillock.conf"
fi
cp "/etc/security/faillock.conf" "/etc/security/faillock.conf.bak"
# Insert at the end of the file
printf '%s\n' "audit" >> "/etc/security/faillock.conf"
# Clean up after ourselves.
rm "/etc/security/faillock.conf.bak"

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
