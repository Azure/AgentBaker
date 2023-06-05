#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 114/364: 'file_groupownership_home_directories'")

awk -F: '$7 == "/bin/bash" && $1 != "nobody" {print $1}' /etc/passwd | while read -r USER ; do
        HOME_DIR=$(grep "^${USER}:" /etc/passwd | awk -F: '{print $6}')
        chgrp ${USER} ${HOME_DIR}
done
