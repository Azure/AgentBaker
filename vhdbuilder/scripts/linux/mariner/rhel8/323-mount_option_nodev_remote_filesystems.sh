#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 323/364: 'mount_option_nodev_remote_filesystems'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

vfstype_points=()
readarray -t vfstype_points < <(grep -E "[[:space:]]nfs[4]?[[:space:]]" /etc/fstab | awk '{print $2}')

for vfstype_point in "${vfstype_points[@]}"
do
    mount_point_match_regexp="$(printf "[[:space:]]%s[[:space:]]" $vfstype_point)"

    # If the mount point is not in /etc/fstab, get previous mount options from /etc/mtab
    if [ "$(grep -c "$mount_point_match_regexp" /etc/fstab)" -eq 0 ]; then
        # runtime opts without some automatic kernel/userspace-added defaults
        previous_mount_opts=$(grep "$mount_point_match_regexp" /etc/mtab | head -1 |  awk '{print $4}' \
                    | sed -E "s/(rw|defaults|seclabel|nodev)(,|$)//g;s/,$//")
        [ "$previous_mount_opts" ] && previous_mount_opts+=","
        echo " $vfstype_point nfs4 defaults,${previous_mount_opts}nodev 0 0" >> /etc/fstab
    # If the mount_opt option is not already in the mount point's /etc/fstab entry, add it
    elif [ "$(grep "$mount_point_match_regexp" /etc/fstab | grep -c "nodev")" -eq 0 ]; then
        previous_mount_opts=$(grep "$mount_point_match_regexp" /etc/fstab | awk '{print $4}')
        sed -i "s|\(${mount_point_match_regexp}.*${previous_mount_opts}\)|\1,nodev|" /etc/fstab
    fi
done

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
