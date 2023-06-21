#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 70/290: 'accounts_password_pam_unix_rounds_system_auth'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q pam; then


var_password_pam_unix_rounds="5000"




control="required"
pamFile="/etc/pam.d/system-password"

# fix control if it's wrong
if grep -q -P "^\\s*password\\s+(?"'!'"${control})[[:alnum:]]+\\s+pam_unix\.so" < "$pamFile" ; then
    sed --follow-symlinks -i -E -e "s/^(\\s*password\\s+)[[:alnum:]]+(\\s+pam_unix\.so)/\\1${control}\\2/" "$pamFile"
fi


if grep -q "rounds=" $pamFile; then
    sed -iP --follow-symlinks "/password[[:space:]]\+${control}[[:space:]]\+pam_unix\.so/ \
                                    s/rounds=[[:digit:]]\+/rounds=$var_password_pam_unix_rounds/" $pamFile
else
    sed -iP --follow-symlinks "/password[[:space:]]\+${control}[[:space:]]\+pam_unix\.so/ s/$/ rounds=$var_password_pam_unix_rounds/" $pamFile
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
