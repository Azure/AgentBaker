#!/usr/bin/env shellspec

Describe 'E2E runner scripts'
  ROOT_DIR="$(pwd)"

  setup_runner_scripts() {
    TEST_ROOT="$(mktemp -d)"
    TEST_REPO="${TEST_ROOT}/repo"
    MOCK_BIN="${TEST_ROOT}/bin"
    COMMAND_LOG="${TEST_ROOT}/commands.log"
    mkdir -p "${TEST_REPO}/e2e" "${MOCK_BIN}"
    : > "${COMMAND_LOG}"

    cat > "${MOCK_BIN}/go" <<'EOF'
#!/bin/sh
printf 'go' >> "${COMMAND_LOG}"
for arg in "$@"; do
  printf ' <%s>' "${arg}" >> "${COMMAND_LOG}"
done
printf '\n' >> "${COMMAND_LOG}"

case "${1:-}" in
  env)
    printf '/tmp/mock-gopath\n'
    ;;
  run)
    printf 'runner-env <%s> <%s>\n' "${TIMEOUT:-}" "${PARALLEL:-}" >> "${COMMAND_LOG}"
    ;;
  test)
    if [ "${FAIL_GO_TEST:-0}" = "1" ]; then
      exit 1
    fi
    ;;
  build)
    output=""
    shift
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-o" ]; then
        output="$2"
        break
      fi
      shift
    done
    mkdir -p "$(dirname "${output}")"
    cat > "${output}" <<'RUNNER'
#!/bin/sh
printf 'e2e' >> "${COMMAND_LOG}"
for arg in "$@"; do
  printf ' <%s>' "${arg}" >> "${COMMAND_LOG}"
done
printf '\n' >> "${COMMAND_LOG}"
RUNNER
    chmod +x "${output}"
    ;;
esac
EOF

    cat > "${MOCK_BIN}/az" <<'EOF'
#!/bin/sh
printf 'az' >> "${COMMAND_LOG}"
for arg in "$@"; do
  printf ' <%s>' "${arg}" >> "${COMMAND_LOG}"
done
printf '\n' >> "${COMMAND_LOG}"
if [ "${1:-} ${2:-} ${3:-}" = 'vm image terms' ] && [ "${FAIL_TERMS_ACCEPT:-0}" = '1' ]; then
  exit 1
fi
EOF

    cat > "${MOCK_BIN}/date" <<'EOF'
#!/bin/sh
printf '12345\n'
EOF

    chmod +x "${MOCK_BIN}/go" "${MOCK_BIN}/az" "${MOCK_BIN}/date"
    export ROOT_DIR TEST_ROOT TEST_REPO MOCK_BIN COMMAND_LOG
    PATH="${MOCK_BIN}:${PATH}"
    export PATH
  }

  cleanup_runner_scripts() {
    rm -rf "${TEST_ROOT}"
  }

  BeforeEach 'setup_runner_scripts'
  AfterEach 'cleanup_runner_scripts'

  It 'runs the local CLI with defaults and forwards arguments'
    # shellcheck disable=SC2016
    When run bash -c 'cd "${TEST_REPO}/e2e" && unset TIMEOUT PARALLEL && "${ROOT_DIR}/e2e/e2e-local.sh" "Ubuntu 2204" "--flag=value"'
    The status should be success
    The stderr should be present
    The contents of file "${COMMAND_LOG}" should include 'go <run> <.> <run> <Ubuntu 2204> <--flag=value>'
    The contents of file "${COMMAND_LOG}" should include 'runner-env <90m> <100>'
  End

  It 'preserves local timeout and parallel overrides'
    # shellcheck disable=SC2016
    When run bash -c 'cd "${TEST_REPO}/e2e" && TIMEOUT=45m PARALLEL=7 "${ROOT_DIR}/e2e/e2e-local.sh"'
    The status should be success
    The stderr should be present
    The contents of file "${COMMAND_LOG}" should include 'runner-env <45m> <7>'
  End

  It 'runs uncached unit tests before the CLI and forwards pipeline options'
    When run env \
      SUBSCRIPTION_ID='test-subscription' \
      DefaultWorkingDirectory="${TEST_REPO}" \
      BUILD_SRC_DIR="${TEST_REPO}" \
      E2E_GO_TEST_TIMEOUT='75m' \
      E2E_FAILED_TESTS_RETRY_COUNT='2' \
      bash "${ROOT_DIR}/.pipelines/scripts/e2e_run.sh"
    The status should be success
    The output should be present
    The contents of file "${COMMAND_LOG}" should include 'go <test> <-count=1> <./...>'
    The contents of file "${COMMAND_LOG}" should include 'go <run> <.> <run> <--parallel> <60> <--suite-timeout> <75m> <--retries> <2> <--log-dir> <scenario-logs-12345> <--junit-file>'
    The contents of file "${COMMAND_LOG}" should include '<--output> <grouped>'
    The contents of file "${COMMAND_LOG}" should not include 'az <vm> <image> <terms>'
  End

  It 'accepts WS2028 preview terms in the resolved E2E subscription'
    When run env \
      SUBSCRIPTION_ID='build-subscription' \
      SUBSCRIPTION_ID_OVERRIDE='e2e-subscription' \
      TAGS_TO_RUN='imageName=windows-2028-gen2' \
      DefaultWorkingDirectory="${TEST_REPO}" \
      BUILD_SRC_DIR="${TEST_REPO}" \
      bash "${ROOT_DIR}/.pipelines/scripts/e2e_run.sh"
    The status should be success
    The output should be present
    The contents of file "${COMMAND_LOG}" should include 'az <vm> <image> <terms> <accept> <--subscription> <e2e-subscription> <--publisher> <microsoftwindowsserver> <--offer> <microsoftserveroperatingsystems-previews> <--plan> <windows-server-vnext-azure-edition-core> <--output> <none>'
    The contents of file "${COMMAND_LOG}" should include 'go <run> <.> <run>'
  End

  It 'stops before running scenarios if WS2028 terms acceptance fails'
    When run env \
      SUBSCRIPTION_ID='test-subscription' \
      TAGS_TO_RUN='imageName=windows-2028-gen2' \
      FAIL_TERMS_ACCEPT='1' \
      DefaultWorkingDirectory="${TEST_REPO}" \
      BUILD_SRC_DIR="${TEST_REPO}" \
      bash "${ROOT_DIR}/.pipelines/scripts/e2e_run.sh"
    The status should be failure
    The output should be present
    The contents of file "${COMMAND_LOG}" should include 'az <vm> <image> <terms> <accept>'
    The contents of file "${COMMAND_LOG}" should not include 'go <run>'
  End

  Describe 'other images'
    Parameters
      windows-2022-containerd-gen2
      windows-2025-gen2
      windows-2025-gen2-tl
      ubuntu-2204-gen2
    End

    It "does not accept preview terms for $1"
      When run env \
        SUBSCRIPTION_ID='test-subscription' \
        TAGS_TO_RUN="imageName=$1" \
        DefaultWorkingDirectory="${TEST_REPO}" \
        BUILD_SRC_DIR="${TEST_REPO}" \
        bash "${ROOT_DIR}/.pipelines/scripts/e2e_run.sh"
      The status should be success
      The output should be present
      The contents of file "${COMMAND_LOG}" should not include 'az <vm> <image> <terms>'
      The contents of file "${COMMAND_LOG}" should include 'go <run> <.> <run>'
    End
  End

  It 'stops the pipeline before CLI execution when unit tests fail'
    When run env \
      SUBSCRIPTION_ID='test-subscription' \
      DefaultWorkingDirectory="${TEST_REPO}" \
      BUILD_SRC_DIR="${TEST_REPO}" \
      FAIL_GO_TEST='1' \
      bash "${ROOT_DIR}/.pipelines/scripts/e2e_run.sh"
    The status should be failure
    The output should be present
    The contents of file "${COMMAND_LOG}" should include 'go <test> <-count=1> <./...>'
    The contents of file "${COMMAND_LOG}" should not include 'go <run>'
  End
End
