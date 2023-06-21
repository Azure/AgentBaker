#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 262/364: 'dir_group_ownership_library_dirs'")

find /lib \
/lib64 \
/usr/lib \
/usr/lib64 \
\! -group root -type d -exec chgrp root '{}' \;
