#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 122/364: 'grub2_audit_argument'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ] && { rpm --quiet -q grub2-efi-binary; }; then

# Correct the form of default kernel command line in GRUB
if grep -q '.*bootprefix.*mariner_linux.*audit=.*'  '/boot/grub2/grub.cfg' ; then
	# modify the GRUB command-line if an audit= arg already exists
	sed -i 's/\(^.*bootprefix.*mariner_linux.*\)audit=[^[:space:]]*\(.*\)/\1 audit=1 \2/'  '/boot/grub2/grub.cfg'
else
	# no audit=arg is present, append it
	sed -i 's/\(^.*bootprefix.*mariner_linux.*\)/\1 audit=1/'  '/boot/grub2/grub.cfg'
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
