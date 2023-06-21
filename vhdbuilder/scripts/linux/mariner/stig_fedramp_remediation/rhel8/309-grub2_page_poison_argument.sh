#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 309/364: 'grub2_page_poison_argument'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ] && { rpm --quiet -q grub2-efi-binary; }; then

# Correct the form of default kernel command line in GRUB
if grep -q '.*bootprefix.*mariner_linux.*page_poison=.*'  '/boot/grub2/grub.cfg' ; then
	# modify the GRUB command-line if an page_poison= arg already exists
	sed -i 's/\(^.*bootprefix.*mariner_linux.*\)page_poison=[^[:space:]]*\(.*\)/\1 page_poison=1 \2/'  '/boot/grub2/grub.cfg'
else
	# no page_poison=arg is present, append it
	sed -i 's/\(^.*bootprefix.*mariner_linux.*\)/\1 page_poison=1/'  '/boot/grub2/grub.cfg'
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
