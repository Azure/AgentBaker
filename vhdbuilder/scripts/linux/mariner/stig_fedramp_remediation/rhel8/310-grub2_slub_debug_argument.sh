#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 310/364: 'grub2_slub_debug_argument'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ] && { rpm --quiet -q grub2-efi-binary; }; then

# Correct the form of default kernel command line in GRUB
if grep -q '.*bootprefix.*mariner_linux.*slub_debug=.*'  '/boot/grub2/grub.cfg' ; then
	# modify the GRUB command-line if an slub_debug= arg already exists
	sed -i 's/\(^.*bootprefix.*mariner_linux.*\)slub_debug=[^[:space:]]*\(.*\)/\1 slub_debug=P \2/'  '/boot/grub2/grub.cfg'
else
	# no slub_debug=arg is present, append it
	sed -i 's/\(^.*bootprefix.*mariner_linux.*\)/\1 slub_debug=P/'  '/boot/grub2/grub.cfg'
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
