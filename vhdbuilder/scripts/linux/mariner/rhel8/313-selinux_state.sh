#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 313/364: 'selinux_state'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then




if ! rpm -q --quiet "selinux-policy" ; then
    dnf install -y "selinux-policy"
fi
if [ ! -e /.buildenv ]; then
	# Correct the form of default kernel command line in  grub
	if grep -q '^\s*linux.*security=.*'  /boot/grub2/grub.cfg; then
		# modify the GRUB command-line if a security= arg already exists
		sed -i 's/\(^\s*linux.*\)security=[^[:space:]]*\(.*\)/\1 security=selinux \2/'  /boot/grub2/grub.cfg
	else
		# no existing security=arg is present, append it
		sed -i 's/\(^\s*linux.*\)/\1 security=selinux/'  /boot/grub2/grub.cfg
	fi
fi
var_selinux_state="enforcing"



if [ -e "/etc/selinux/config" ] ; then
    
    LC_ALL=C sed -i "/^SELINUX=/Id" "/etc/selinux/config"
else
    touch "/etc/selinux/config"
fi
cp "/etc/selinux/config" "/etc/selinux/config.bak"
# Insert at the end of the file
printf '%s\n' "SELINUX=$var_selinux_state" >> "/etc/selinux/config"
# Clean up after ourselves.
rm "/etc/selinux/config.bak"

fixfiles onboot
fixfiles -f relabel

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
