#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 116/364: 'file_permissions_home_directories'")

awk -F: '$7 == "/bin/bash" && $1 != "nobody" {print $6}' /etc/passwd | while read -r HOME_DIR ; do
        chmod 0750 ${HOME_DIR}
done
