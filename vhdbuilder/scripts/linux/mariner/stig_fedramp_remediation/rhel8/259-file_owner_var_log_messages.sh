#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 259/364: 'file_owner_var_log_messages'")


chown 0 /var/log/messages
