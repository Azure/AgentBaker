#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 103/364: 'accounts_password_pam_unix_rounds_password_auth'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q pam; then


var_password_pam_unix_rounds="5000"



pamFile="/etc/pam.d/password-auth"

if grep -q "rounds=" $pamFile; then
    sed -iP --follow-symlinks "/password[[:space:]]\+sufficient[[:space:]]\+pam_unix\.so/ \
                                    s/rounds=[[:digit:]]\+/rounds=$var_password_pam_unix_rounds/" $pamFile
else
    sed -iP --follow-symlinks "/password[[:space:]]\+sufficient[[:space:]]\+pam_unix\.so/ s/$/ rounds=$var_password_pam_unix_rounds/" $pamFile
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
