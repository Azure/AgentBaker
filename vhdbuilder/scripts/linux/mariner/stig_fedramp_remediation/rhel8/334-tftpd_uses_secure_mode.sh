#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 334/364: 'tftpd_uses_secure_mode'")
#!/bin/bash


var_tftpd_secure_directory="/var/lib/tftpboot"



if grep -q 'server_args' /etc/xinetd.d/tftp; then
    sed -i -E "s;^([[:blank:]]*server_args[[:blank:]]+=[[:blank:]]+.*?)(-s[[:blank:]]+[[:graph:]]+)*(.*)$;\1 -s $var_tftpd_secure_directory \3;" /etc/xinetd.d/tftp
else
    echo "server_args = -s $var_tftpd_secure_directory" >> /etc/xinetd.d/tftp
fi
