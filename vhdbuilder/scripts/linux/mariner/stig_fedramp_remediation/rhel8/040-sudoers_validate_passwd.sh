#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 40/364: 'sudoers_validate_passwd'")

if [ -e "/etc/sudoers" ] ; then
    
    LC_ALL=C sed -i "/Defaults !targetpw/d" "/etc/sudoers"
else
    touch "/etc/sudoers"
fi
cp "/etc/sudoers" "/etc/sudoers.bak"
# Insert at the end of the file
printf '%s\n' "Defaults !targetpw" >> "/etc/sudoers"
# Clean up after ourselves.
rm "/etc/sudoers.bak"
if [ -e "/etc/sudoers" ] ; then
    
    LC_ALL=C sed -i "/Defaults !rootpw/d" "/etc/sudoers"
else
    touch "/etc/sudoers"
fi
cp "/etc/sudoers" "/etc/sudoers.bak"
# Insert at the end of the file
printf '%s\n' "Defaults !rootpw" >> "/etc/sudoers"
# Clean up after ourselves.
rm "/etc/sudoers.bak"
if [ -e "/etc/sudoers" ] ; then
    
    LC_ALL=C sed -i "/Defaults !runaspw/d" "/etc/sudoers"
else
    touch "/etc/sudoers"
fi
cp "/etc/sudoers" "/etc/sudoers.bak"
# Insert at the end of the file
printf '%s\n' "Defaults !runaspw" >> "/etc/sudoers"
# Clean up after ourselves.
rm "/etc/sudoers.bak"
