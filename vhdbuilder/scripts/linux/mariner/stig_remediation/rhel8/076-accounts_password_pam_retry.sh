#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 76/364: 'accounts_password_pam_retry'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q pam; then


var_password_pam_retry="3"




pam_file="/etc/pam.d/system-password"


if grep -q "retry=" "$pam_file" ; then
	sed -i --follow-symlinks "s/\(retry *= *\).*/\1$var_password_pam_retry/" "$pam_file"
else
	sed -i --follow-symlinks "/pam_pwquality.so/ s/$/ retry=$var_password_pam_retry/" "$pam_file"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
