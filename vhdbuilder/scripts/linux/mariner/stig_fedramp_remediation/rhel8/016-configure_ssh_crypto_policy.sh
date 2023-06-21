#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 16/364: 'configure_ssh_crypto_policy'")

SSH_CONF="/etc/sysconfig/sshd"

sed -i "/^\s*CRYPTO_POLICY.*$/d" $SSH_CONF
