#!/bin/bash

Describe 'setup-custom-search-domains.sh'
    setup() {
        TEST_DIR=$(mktemp -d)
        mkdir -p "${TEST_DIR}/bin"
        cat > "${TEST_DIR}/helpers.sh" <<'EOF'
wait_for_apt_locks() { :; }
systemctl_restart() { :; }
retrycmd_if_failure() { return 0; }
EOF
        : > "${TEST_DIR}/distro_helpers.sh"
        cat > "${TEST_DIR}/bin/realm" <<'EOF'
#!/bin/sh
cat >/dev/null
exit "${REALM_EXIT_CODE:-0}"
EOF
        cat > "${TEST_DIR}/bin/tee" <<'EOF'
#!/bin/sh
cat >/dev/null
EOF
        cat > "${TEST_DIR}/bin/run-script" <<EOF
#!/bin/bash
bash ./parts/linux/cloud-init/artifacts/setup-custom-search-domains.sh 2>"${TEST_DIR}/stderr"
EOF
        chmod +x "${TEST_DIR}/bin/realm" "${TEST_DIR}/bin/tee" "${TEST_DIR}/bin/run-script"
    }

    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'preserves realm join failures without tracing the password'
        When run env \
            PATH="${TEST_DIR}/bin:${PATH}" \
            CSE_HELPERS_FILEPATH="${TEST_DIR}/helpers.sh" \
            CSE_DISTRO_HELPERS_FILEPATH="${TEST_DIR}/distro_helpers.sh" \
            CUSTOM_SEARCH_REALM_PASSWORD='realm-password-sentinel' \
            CUSTOM_SEARCH_REALM_USER='test-user' \
            CUSTOM_SEARCH_DOMAIN_NAME='example.com' \
            REALM_EXIT_CODE=37 \
            "${TEST_DIR}/bin/run-script"

        The status should equal 37
        The value "$(grep -F 'realm-password-sentinel' "${TEST_DIR}/stderr")" should equal ''
    End
End
