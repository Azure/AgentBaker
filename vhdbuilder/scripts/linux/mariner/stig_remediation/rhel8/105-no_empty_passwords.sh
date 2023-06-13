#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 105/364: 'no_empty_passwords'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

sed --follow-symlinks -i 's/\<nullok\>//g' /etc/pam.d/system-auth

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
