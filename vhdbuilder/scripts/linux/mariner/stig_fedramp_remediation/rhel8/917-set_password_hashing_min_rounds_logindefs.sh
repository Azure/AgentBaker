#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule: 'set_password_hashing_min_rounds_logindefs'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q shadow-utils; then



if [ -e "/etc/login.defs" ] ; then
    
    LC_ALL=C sed -i "/^\s*SHA_CRYPT_MIN_ROUNDS\s*/Id" "/etc/login.defs"
else
    printf '%s\n' "Path '/etc/login.defs' wasn't found on this system. Refusing to continue." >&2
    return 1
fi
cp "/etc/login.defs" "/etc/login.defs.bak"
# Insert at the end of the file
printf '%s\n' "SHA_CRYPT_MIN_ROUNDS 5000" >> "/etc/login.defs"
# Clean up after ourselves.
rm "/etc/login.defs.bak"

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi