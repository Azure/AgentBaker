#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 281/364: 'mount_option_nosuid_removable_partitions'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then


var_removable_partition="/dev/cdrom"



device_regex="^\s*$var_removable_partition\s\+"
mount_option="nosuid"

if grep -q $device_regex /etc/fstab ; then
    previous_opts=$(grep $device_regex /etc/fstab | awk '{print $4}')
    sed -i "s|\($device_regex.*$previous_opts\)|\1,$mount_option|" /etc/fstab
else
    echo "Not remediating, because there is no record of $var_removable_partition in /etc/fstab" >&2
    return 1
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
