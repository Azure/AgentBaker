#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 267/364: 'file_permissions_library_dirs'")
DIRS="/lib /lib64 /usr/lib /usr/lib64"
for dirPath in $DIRS; do
	find "$dirPath" -perm /022 -type f -exec chmod go-w '{}' \;
done
