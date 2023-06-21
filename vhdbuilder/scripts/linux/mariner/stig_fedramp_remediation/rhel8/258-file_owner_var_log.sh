#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 258/364: 'file_owner_var_log'")


chown 0 /var/log/
