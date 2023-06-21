#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 19/364: 'harden_sshd_macs_openssh_conf_crypto_policy'")

sshd_approved_macs="hmac-sha2-512,hmac-sha2-256"



if [ -e "/etc/crypto-policies/back-ends/openssh.config" ] ; then
    
    LC_ALL=C sed -i "/^.*MACs\s\+/d" "/etc/crypto-policies/back-ends/openssh.config"
else
    touch "/etc/crypto-policies/back-ends/openssh.config"
fi
cp "/etc/crypto-policies/back-ends/openssh.config" "/etc/crypto-policies/back-ends/openssh.config.bak"
# Insert at the end of the file
printf '%s\n' "MACs ${sshd_approved_macs}" >> "/etc/crypto-policies/back-ends/openssh.config"
# Clean up after ourselves.
rm "/etc/crypto-policies/back-ends/openssh.config.bak"
