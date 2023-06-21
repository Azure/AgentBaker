#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 91/364: 'install_smartcard_packages'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "openssl-pkcs11" ; then
    dnf install -y "openssl-pkcs11"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
