#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 303/364: 'coredump_disable_backtraces'")
if [ -e "/etc/systemd/coredump.conf" ] ; then
    
    LC_ALL=C sed -i "/^\s*ProcessSizeMax\s*=\s*/Id" "/etc/systemd/coredump.conf"
else
    touch "/etc/systemd/coredump.conf"
fi
cp "/etc/systemd/coredump.conf" "/etc/systemd/coredump.conf.bak"
# Insert at the end of the file
printf '%s\n' "ProcessSizeMax=0" >> "/etc/systemd/coredump.conf"
# Clean up after ourselves.
rm "/etc/systemd/coredump.conf.bak"
