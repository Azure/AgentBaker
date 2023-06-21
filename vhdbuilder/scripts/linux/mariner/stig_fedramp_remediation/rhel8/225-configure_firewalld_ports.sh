#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 225/364: 'configure_firewalld_ports'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "firewalld" ; then
    dnf install -y "firewalld"
fi


firewalld_sshd_zone="public"



# This assumes that firewalld_sshd_zone is one of the pre-defined zones
if [ ! -f /etc/firewalld/zones/${firewalld_sshd_zone}.xml ]; then
    cp /usr/lib/firewalld/zones/${firewalld_sshd_zone}.xml /etc/firewalld/zones/${firewalld_sshd_zone}.xml
fi
if ! grep -q 'service name="ssh"' /etc/firewalld/zones/${firewalld_sshd_zone}.xml; then
    sed -i '/<\/description>/a \
  <service name="ssh"/>' /etc/firewalld/zones/${firewalld_sshd_zone}.xml
fi

# Check if any eth interface is bounded to the zone with SSH service enabled
nic_bound=false
eth_interface_list=$(ip link show up | cut -d ' ' -f2 | cut -d ':' -s -f1 | grep -E '^(en|eth)')
for interface in $eth_interface_list; do
    if grep -q "ZONE=$firewalld_sshd_zone" /etc/sysconfig/network-scripts/ifcfg-$interface; then
        nic_bound=true
        break;
    fi
done

if [ $nic_bound = false ];then
    # Add first NIC to SSH enabled zone

    if ! firewall-cmd --state -q; then
        # Test if the config_file is a symbolic link. If so, use --follow-symlinks with sed.
        # Otherwise, regular sed command will do.
        sed_command=('sed' '-i')
        if test -L "/etc/sysconfig/network-scripts/ifcfg-${eth_interface_list[0]}"; then
            sed_command+=('--follow-symlinks')
        fi

        # If the cce arg is empty, CCE is not assigned.
        if [ -z "" ]; then
            cce="CCE"
        else
            cce=""
        fi

        # Strip any search characters in the key arg so that the key can be replaced without
        # adding any search characters to the config file.
        stripped_key=$(sed 's/[\^=\$,;+]*//g' <<< "^ZONE=")

        # shellcheck disable=SC2059
        printf -v formatted_output "%s=%s" "$stripped_key" "$firewalld_sshd_zone"

        # If the key exists, change it. Otherwise, add it to the config_file.
        # We search for the key string followed by a word boundary (matched by \>),
        # so if we search for 'setting', 'setting2' won't match.
        if LC_ALL=C grep -q -m 1 -i -e "^ZONE=\\>" "/etc/sysconfig/network-scripts/ifcfg-${eth_interface_list[0]}"; then
            "${sed_command[@]}" "s/^ZONE=\\>.*/$formatted_output/gi" "/etc/sysconfig/network-scripts/ifcfg-${eth_interface_list[0]}"
        else
            # \n is precaution for case where file ends without trailing newline
            printf '\n# Per %s: Set %s in %s\n' "$cce" "$formatted_output" "/etc/sysconfig/network-scripts/ifcfg-${eth_interface_list[0]}" >> "/etc/sysconfig/network-scripts/ifcfg-${eth_interface_list[0]}"
            printf '%s\n' "$formatted_output" >> "/etc/sysconfig/network-scripts/ifcfg-${eth_interface_list[0]}"
        fi
    else
        # If firewalld service is running, we need to do this step with firewall-cmd
        # Otherwise firewalld will comunicate with NetworkManage and will revert assigned zone
        # of NetworkManager managed interfaces upon reload
        firewall-cmd --permanent --zone=$firewalld_sshd_zone --add-interface=${eth_interface_list[0]}
        firewall-cmd --reload
    fi
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
