#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 89/364: 'no_tmux_in_shells'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if grep -q 'tmux$' /etc/shells ; then
	sed -i '/tmux$/d' /etc/shells
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
