#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 115/364: 'file_permission_user_init_files'")

find /home -type f -name '\.[^.]*' \( -perm /0037 \) | xargs chmod -0037
