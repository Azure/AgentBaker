#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 12/364: 'configure_kerberos_crypto_policy'")

rm -f /etc/krb5.conf.d/crypto-policies
ln -s /etc/crypto-policies/back-ends/krb5.config /etc/krb5.conf.d/crypto-policies
