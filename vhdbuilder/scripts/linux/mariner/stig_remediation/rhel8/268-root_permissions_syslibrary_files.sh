#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 268/364: 'root_permissions_syslibrary_files'")

find /lib \
/lib64 \
/usr/lib \
/usr/lib64 \
\! -group root -type f -exec chgrp root '{}' \;
