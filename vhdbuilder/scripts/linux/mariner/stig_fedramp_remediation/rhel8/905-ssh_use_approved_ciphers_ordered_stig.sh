#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 251/274: 'ssh_use_approved_ciphers_ordered_stig'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if grep -q -P '^\s*[Cc]iphers\s+' /etc/ssh/ssh_config; then
  sed -i 's/^\s*[Cc]iphers.*/Ciphers aes256-ctr,aes192-ctr,aes128-ctr/' /etc/ssh/ssh_config
else
  echo "Ciphers aes256-ctr,aes192-ctr,aes128-ctr" >> /etc/ssh/ssh_config
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
