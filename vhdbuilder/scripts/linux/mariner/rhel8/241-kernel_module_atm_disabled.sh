#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 241/364: 'kernel_module_atm_disabled'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if LC_ALL=C grep -q -m 1 "^install atm" /etc/modprobe.d/atm.conf ; then
	
	sed -i 's#^install atm.*#install atm /bin/true#g' /etc/modprobe.d/atm.conf
else
	echo -e "\n# Disable per security requirements" >> /etc/modprobe.d/atm.conf
	echo "install atm /bin/true" >> /etc/modprobe.d/atm.conf
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
