#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 249/364: 'dir_perms_world_writable_sticky_bits'")

df --local -P | awk '{if (NR!=1) print $6}' \
| xargs -I '{}' find '{}' -xdev -type d \
\( -perm -0002 -a ! -perm -1000 \) 2>/dev/null \
| xargs --no-run-if-empty chmod a+t
