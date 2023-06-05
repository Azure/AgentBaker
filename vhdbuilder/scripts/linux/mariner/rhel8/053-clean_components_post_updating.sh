#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 53/364: 'clean_components_post_updating'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q dnf; then

if grep --silent ^clean_requirements_on_remove /etc/dnf/dnf.conf ; then
        sed -i "s/^clean_requirements_on_remove.*/clean_requirements_on_remove=1/g" /etc/dnf/dnf.conf
else
        echo -e "\n# Set clean_requirements_on_remove to 1 per security requirements" >> /etc/dnf/dnf.conf
        echo "clean_requirements_on_remove=1" >> /etc/dnf/dnf.conf
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
