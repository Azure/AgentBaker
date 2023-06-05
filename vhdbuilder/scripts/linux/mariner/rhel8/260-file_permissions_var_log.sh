#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 260/364: 'file_permissions_var_log'")


chmod 0755 /var/log/
