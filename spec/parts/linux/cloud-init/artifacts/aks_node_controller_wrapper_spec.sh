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

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'writes a Guest Agent event on non-zero exit'
            When run bash ./parts/linux/cloud-init/artifacts/aks-node-controller-wrapper.sh
            The status should be failure
            The output should include "Launching aks-node-controller with config ${CONFIG_PATH}"
            The output should include "Spawned aks-node-controller (pid"
            The output should include "aks-node-controller exited with code 42"
            eventsFilePath=$(ls -t ${EVENTS_LOGGING_DIR}*.json | head -n 1)
            taskName=$(jq -r '.TaskName' ${eventsFilePath})
            eventLevel=$(jq -r '.EventLevel' ${eventsFilePath})
            message=$(jq -r '.Message' ${eventsFilePath})
            The variable taskName should equal "AKS.AKSNodeController.UnexpectedError"
            The variable eventLevel should equal "Error"
            The variable message should equal "aks-node-controller exited with code 42"
        End
    End

    Describe 'success handling'
        setup() {
            TEST_DIR="$(mktemp -d)"
            BIN_PATH="${TEST_DIR}/aks-node-controller"
            CONFIG_PATH="${TEST_DIR}/aks-node-controller-config.json"
            EVENTS_LOGGING_DIR="${TEST_DIR}/events/"
            export BIN_PATH CONFIG_PATH EVENTS_LOGGING_DIR

            cat > "${BIN_PATH}" <<'EOF'
#!/bin/bash
exit 0
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

        BeforeEach 'setup'
        AfterEach 'cleanup'

        It 'does not write a Guest Agent event on success'
            When run bash ./parts/linux/cloud-init/artifacts/aks-node-controller-wrapper.sh
            The status should be success
            The output should include "Launching aks-node-controller with config ${CONFIG_PATH}"
            The output should include "Spawned aks-node-controller (pid"
            The output should include "aks-node-controller completed successfully"
            if compgen -G "${EVENTS_LOGGING_DIR}*.json" > /dev/null; then
                eventsFileCount=1
            else
                eventsFileCount=0
            fi
            The variable eventsFileCount should equal 0
        End
    End
End
