#!/bin/bash

Describe 'aks-node-controller-wrapper.sh'
    Include "./parts/linux/cloud-init/artifacts/aks-node-controller-wrapper.sh"

    Describe 'error handling'
        setup() {
            TEST_DIR="$(mktemp -d)"
            BIN_PATH="${TEST_DIR}/aks-node-controller"
            CONFIG_PATH="${TEST_DIR}/aks-node-controller-config.json"
            EVENTS_LOGGING_DIR="${TEST_DIR}/events/"
            export BIN_PATH CONFIG_PATH EVENTS_LOGGING_DIR

            cat > "${BIN_PATH}" <<'EOF'
#!/bin/bash
exit 42
EOF
            chmod +x "${BIN_PATH}"
            touch "${CONFIG_PATH}"
        }

        cleanup() {
            rm -rf "${TEST_DIR}"
        }

        logger() {
            echo "logger $*"
        }

        date() {
            echo "111"
        }

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'writes a Guest Agent event on non-zero exit'
            When run ./parts/linux/cloud-init/artifacts/aks-node-controller-wrapper.sh
            The status should be failure
            The contents of file "${EVENTS_LOGGING_DIR}111.json" should include '"TaskName":"AKS.AKSNodeController.UnexpectedError"'
            The contents of file "${EVENTS_LOGGING_DIR}111.json" should include '"EventLevel":"Error"'
            The contents of file "${EVENTS_LOGGING_DIR}111.json" should include '"Message":"aks-node-controller exited with code 42"'
        End
    End
End
