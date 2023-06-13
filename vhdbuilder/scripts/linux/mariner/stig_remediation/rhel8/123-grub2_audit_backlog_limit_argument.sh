#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 123/364: 'grub2_audit_backlog_limit_argument'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ] && { rpm --quiet -q grub2-efi-binary; }; then

# Correct the form of default kernel command line in GRUB
if grep -q '.*bootprefix.*mariner_linux.*audit_backlog_limit=.*'  '/boot/grub2/grub.cfg' ; then
	# modify the GRUB command-line if an audit_backlog_limit= arg already exists
	sed -i 's/\(^.*bootprefix.*mariner_linux.*\)audit_backlog_limit=[^[:space:]]*\(.*\)/\1 audit_backlog_limit=8192 \2/'  '/boot/grub2/grub.cfg'
else
	# no audit_backlog_limit=arg is present, append it
	sed -i 's/\(^.*bootprefix.*mariner_linux.*\)/\1 audit_backlog_limit=8192/'  '/boot/grub2/grub.cfg'
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
