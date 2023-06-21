#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 339/364: 'file_permissions_sshd_pub_key'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

readarray -t files < <(find /etc/ssh/)
for file in "${files[@]}"; do
    if basename $file | grep -q '^.*.pub$'; then
        chmod 0644 $file
    fi    
done

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
