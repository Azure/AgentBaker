#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 264/364: 'file_ownership_binary_dirs'")

DIRS="/bin/ /usr/bin/ /usr/local/bin/ /sbin/ /usr/sbin/ /usr/libexec"


find $DIRS \! -user root -execdir chown root {} \;
