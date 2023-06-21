#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 261/364: 'file_permissions_var_log_messages'")

if [ ! -e /.buildenv ]; then
chmod 0640 /var/log/messages
fi
