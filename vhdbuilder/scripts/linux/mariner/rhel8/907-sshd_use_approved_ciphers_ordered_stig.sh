#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 267/274: 'sshd_use_approved_ciphers_ordered_stig'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if grep -q -P '^\s*[Cc]iphers\s+' /etc/ssh/sshd_config; then
  sed -i 's/^\s*[Cc]iphers.*/Ciphers aes256-ctr,aes192-ctr,aes128-ctr/' /etc/ssh/sshd_config
else
  echo "Ciphers aes256-ctr,aes192-ctr,aes128-ctr" >> /etc/ssh/sshd_config
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
