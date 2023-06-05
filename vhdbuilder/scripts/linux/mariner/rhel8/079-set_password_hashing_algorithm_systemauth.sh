#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 79/364: 'set_password_hashing_algorithm_systemauth'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q pam; then

AUTH_FILES[0]="/etc/pam.d/system-password"
control="required"

# fix control if it's wrong
if grep -q -P "^\\s*password\\s+(?"'!'"${control})[[:alnum:]]+\\s+pam_unix\.so" < "${AUTH_FILES[0]}" ; then
	sed --follow-symlinks -i -E -e "s/^(\\s*password\\s+)[[:alnum:]]+(\\s+pam_unix\.so)/\\1${control}\\2/" "${AUTH_FILES[0]}"
fi


for pamFile in "${AUTH_FILES[@]}"
do
	if ! grep -q "^password.*${control}.*pam_unix.so.*sha512" $pamFile; then
		sed -i --follow-symlinks "/^password.*${control}.*pam_unix.so/ s/$/ sha512/" $pamFile
	fi
done

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
