#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 278/364: 'mount_option_nodev_nonroot_local_partitions'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

MOUNT_OPTION="nodev"
# Create array of local non-root partitions
readarray -t partitions_records < <(findmnt --mtab --raw --evaluate | grep "^/\w" | grep "\s/dev/\w")

for partition_record in "${partitions_records[@]}"; do
    # Get all important information for fstab
    mount_point="$(echo ${partition_record} | cut -d " " -f1)"
    device="$(echo ${partition_record} | cut -d " " -f2)"
    device_type="$(echo ${partition_record} | cut -d " " -f3)"
    # device and device_type will be used only in case when the device doesn't have fstab record
    mount_point_match_regexp="$(printf "[[:space:]]%s[[:space:]]" $mount_point)"

# If the mount point is not in /etc/fstab, get previous mount options from /etc/mtab
if [ "$(grep -c "$mount_point_match_regexp" /etc/fstab)" -eq 0 ]; then
    # runtime opts without some automatic kernel/userspace-added defaults
    previous_mount_opts=$(grep "$mount_point_match_regexp" /etc/mtab | head -1 |  awk '{print $4}' \
                | sed -E "s/(rw|defaults|seclabel|$MOUNT_OPTION)(,|$)//g;s/,$//")
    [ "$previous_mount_opts" ] && previous_mount_opts+=","
    echo "$device $mount_point $device_type defaults,${previous_mount_opts}$MOUNT_OPTION 0 0" >> /etc/fstab
# If the mount_opt option is not already in the mount point's /etc/fstab entry, add it
elif [ "$(grep "$mount_point_match_regexp" /etc/fstab | grep -c "$MOUNT_OPTION")" -eq 0 ]; then
    previous_mount_opts=$(grep "$mount_point_match_regexp" /etc/fstab | awk '{print $4}')
    sed -i "s|\(${mount_point_match_regexp}.*${previous_mount_opts}\)|\1,$MOUNT_OPTION|" /etc/fstab
fi
    if mkdir -p "$mount_point"; then
    if mountpoint -q "$mount_point"; then
        mount -o remount --target "$mount_point"
    else
        mount --target "$mount_point"
    fi
fi
done

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
