#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 10/274: 'configure_gnutls_tls'")

CONF_FILE=/etc/gnutls/default-priorities
correct_value='SYSTEM=SECURE256:!VERS-DTLS0.9:!VERS-SSL3.0:!VERS-TLS1.0:!VERS-TLS1.1:!VERS-DTLS1.0:+VERS-TLS1.2:+AES-128-CBC:+RSA:+SHA1:+COMP-NULL'

if ! grep -q "${correct_value}" ${CONF_FILE}; then
    # We need to get the existing value, using PCRE to maintain same regex
    existing_value=$(grep -Po '^SYSTEM=[A-Z0-9]+.*$' ${CONF_FILE} || true)

    if [[ ! -z ${existing_value} ]]; then
        # replace existing_value with correct_value
        sed -i "s/${existing_value}/${correct_value}/g" ${CONF_FILE}
    else
        # ***NOTE*** #
        # This probably means this file is not here or it's been modified
        # unintentionally.
        # ********** #
        # echo correct_value to end
        echo ${correct_value} >> ${CONF_FILE}
    fi
fi
