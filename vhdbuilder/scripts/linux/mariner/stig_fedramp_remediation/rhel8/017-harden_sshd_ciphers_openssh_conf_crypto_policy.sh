#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 17/364: 'harden_sshd_ciphers_openssh_conf_crypto_policy'")

sshd_approved_ciphers="aes256-ctr,aes192-ctr,aes128-ctr"



if [ -e "/etc/crypto-policies/back-ends/openssh.config" ] ; then
    
    LC_ALL=C sed -i "/^.*Ciphers\s\+/d" "/etc/crypto-policies/back-ends/openssh.config"
else
    touch "/etc/crypto-policies/back-ends/openssh.config"
fi
cp "/etc/crypto-policies/back-ends/openssh.config" "/etc/crypto-policies/back-ends/openssh.config.bak"
# Insert at the end of the file
printf '%s\n' "Ciphers ${sshd_approved_ciphers}" >> "/etc/crypto-policies/back-ends/openssh.config"
# Clean up after ourselves.
rm "/etc/crypto-policies/back-ends/openssh.config.bak"
