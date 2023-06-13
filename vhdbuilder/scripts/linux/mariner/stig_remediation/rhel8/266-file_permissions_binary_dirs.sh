#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 266/364: 'file_permissions_binary_dirs'")

DIRS="/bin /usr/bin /usr/local/bin /sbin /usr/sbin /usr/libexec"

for dirPath in $DIRS; do
	find "$dirPath" -perm /022 -exec chmod go-w '{}' \;
done
