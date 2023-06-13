#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 243/364: 'kernel_module_firewire-core_disabled'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if LC_ALL=C grep -q -m 1 "^install firewire-core" /etc/modprobe.d/firewire-core.conf ; then
	
	sed -i 's#^install firewire-core.*#install firewire-core /bin/true#g' /etc/modprobe.d/firewire-core.conf
else
	echo -e "\n# Disable per security requirements" >> /etc/modprobe.d/firewire-core.conf
	echo "install firewire-core /bin/true" >> /etc/modprobe.d/firewire-core.conf
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
