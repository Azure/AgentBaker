#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 78/364: 'set_password_hashing_algorithm_logindefs'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q shadow-utils; then


var_password_hashing_algorithm="SHA512"



if grep --silent ^ENCRYPT_METHOD /etc/login.defs ; then
	sed -i "s/^ENCRYPT_METHOD .*/ENCRYPT_METHOD $var_password_hashing_algorithm/g" /etc/login.defs
else
	echo "" >> /etc/login.defs
	echo "ENCRYPT_METHOD $var_password_hashing_algorithm" >> /etc/login.defs
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
