#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 8/274: 'grub2_enable_fips_mode_boot_dir'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

# include remediation functions library


# prelink not installed
if test -e /etc/sysconfig/prelink -o -e /usr/sbin/prelink; then
    if grep -q ^PRELINKING /etc/sysconfig/prelink
    then
        sed -i 's/^PRELINKING[:blank:]*=[:blank:]*[:alpha:]*/PRELINKING=no/' /etc/sysconfig/prelink
    else
        printf '\n' >> /etc/sysconfig/prelink
        printf '%s\n' '# Set PRELINKING=no per security requirements' 'PRELINKING=no' >> /etc/sysconfig/prelink
    fi

    # Undo previous prelink changes to binaries if prelink is available.
    if test -x /usr/sbin/prelink; then
        /usr/sbin/prelink -ua
    fi
fi

if ! rpm -q --quiet "dracut-fips" ; then
    dnf install -y "dracut-fips"
fi


mkinitrd -f


if [ ! -e /.buildenv ]; then
    # Correct the form of default kernel command line in  grub
    if grep -q '^\s*linux.*fips=.*'  /boot/grub2/grub.cfg; then
        # modify the GRUB command-line if a fips= arg already exists
        sed -i 's/\(^\s*linux.*\)fips=[^[:space:]]*\(.*\)/\1 fips=1 \2/'  /boot/grub2/grub.cfg
    else
        # no existing fips=arg is present, append it
        sed -i 's/\(^\s*linux.*\)/\1 fips=1/'  /boot/grub2/grub.cfg
    fi

    # Add the boot= cmd line parameter if the boot dir is not the same as the main dir
    boot_dev="$(df /boot/ | tail -1 | cut -d' ' -f1)"
    root_dev="$(df / | tail -1 | cut -d' ' -f1)"
    if [ ! "$root_dev" == "$boot_dev" ]; then
        boot_uuid="UUID=$(blkid $boot_dev -s UUID -o value)"

        if grep -q '^\s*linux.*boot=.*'  /boot/grub2/grub.cfg; then
            # modify the GRUB command-line if a fips= arg already exists
            sed -i 's/\(^\s*linux.*\)boot=[^[:space:]]*\(.*\)/\1 boot='"${boot_uuid}"' \2/'  /boot/grub2/grub.cfg
        else
            # no existing fips=arg is present, append it
            sed -i 's/\(^\s*linux.*\)/\1 boot='"${boot_uuid}"'/'  /boot/grub2/grub.cfg
        fi
    fi
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
