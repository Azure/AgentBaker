#!/bin/bash

check() {
    return 0
}

depends() {
    echo rootfs-block
    return 0
}

install() {
    module_dir=/usr/lib/dracut/modules.d/99aksrootjournal
    inst_multiple growpart lsblk sfdisk partx blockdev flock mktemp udevadm resize2fs tune2fs e2fsck debugfs blkid mke2fs dd rm grep mount umount sync readlink awk sed cat mkdir cut tr sleep
    inst_simple "$module_dir/resize-root-journal" /sbin/aks-root-journal-resize
    inst_simple /etc/mke2fs.conf /etc/mke2fs.conf
    inst_hook pre-mount 90 "$module_dir/dracut-hook.sh"
}
