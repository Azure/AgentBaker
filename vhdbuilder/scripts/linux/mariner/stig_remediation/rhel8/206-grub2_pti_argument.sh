#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 206/364: 'grub2_pti_argument'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q grub2-efi-binary && { [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; }; then

# Correct the form of default kernel command line in GRUB
if grep -q '.*bootprefix.*mariner_linux.*pti=.*'  '/boot/grub2/grub.cfg' ; then
	# modify the GRUB command-line if an pti= arg already exists
	sed -i 's/\(^.*bootprefix.*mariner_linux.*\)pti=[^[:space:]]*\(.*\)/\1 pti=on \2/'  '/boot/grub2/grub.cfg'
else
	# no pti=arg is present, append it
	sed -i 's/\(^.*bootprefix.*mariner_linux.*\)/\1 pti=on/'  '/boot/grub2/grub.cfg'
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
