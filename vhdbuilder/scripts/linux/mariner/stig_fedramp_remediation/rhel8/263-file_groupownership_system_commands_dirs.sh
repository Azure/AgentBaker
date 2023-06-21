#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 263/364: 'file_groupownership_system_commands_dirs'")


DIRS="/bin /sbin /usr/bin /usr/sbin /usr/local/bin"


for SYSCMDFILES in $DIRS
do
   find -L $SYSCMDFILES \! -group root -type f -exec chgrp root '{}' \;
done
