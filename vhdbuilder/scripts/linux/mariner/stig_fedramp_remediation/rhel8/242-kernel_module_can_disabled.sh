#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 242/364: 'kernel_module_can_disabled'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if LC_ALL=C grep -q -m 1 "^install can" /etc/modprobe.d/can.conf ; then
	
	sed -i 's#^install can.*#install can /bin/true#g' /etc/modprobe.d/can.conf
else
	echo -e "\n# Disable per security requirements" >> /etc/modprobe.d/can.conf
	echo "install can /bin/true" >> /etc/modprobe.d/can.conf
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
