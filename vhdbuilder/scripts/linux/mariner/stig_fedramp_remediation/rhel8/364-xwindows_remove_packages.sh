#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 364/364: 'xwindows_remove_packages'")


# remove packages
if rpm -q --quiet "xorg-x11-server-Xorg" ; then

    dnf remove -y "xorg-x11-server-Xorg"

fi
if rpm -q --quiet "xorg-x11-server-utils" ; then

    dnf remove -y "xorg-x11-server-utils"

fi
if rpm -q --quiet "xorg-x11-server-common" ; then

    dnf remove -y "xorg-x11-server-common"

fi

if rpm -q --quiet "xorg-x11-server-Xwayland" ; then

    dnf remove -y "xorg-x11-server-Xwayland"

fi


# configure run level
systemctl set-default multi-user.target
