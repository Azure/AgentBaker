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
    The contents of file "${COMMAND_LOG}" should include 'go <run> <./cmd/e2e> <run> <Ubuntu 2204> <--flag=value>'
    The contents of file "${COMMAND_LOG}" should include 'runner-env <90m> <100>'
  End

  It 'preserves local timeout and parallel overrides'
    # shellcheck disable=SC2016
    When run bash -c 'cd "${TEST_REPO}/e2e" && TIMEOUT=45m PARALLEL=7 "${ROOT_DIR}/e2e/e2e-local.sh"'
    The status should be success
    The stderr should be present
    The contents of file "${COMMAND_LOG}" should include 'runner-env <45m> <7>'
  End

  It 'runs uncached unit tests before building and forwards pipeline options'
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
    The contents of file "${COMMAND_LOG}" should include 'go <build> <-o> <bin/e2e> <./cmd/e2e>'
    The contents of file "${COMMAND_LOG}" should include 'e2e <run> <--parallel> <60> <--suite-timeout> <75m> <--retries> <2> <--log-dir> <scenario-logs-12345> <--junit-file>'
    The contents of file "${COMMAND_LOG}" should include '<--output> <grouped>'
  End

  It 'stops the pipeline before build and execution when unit tests fail'
    When run env \
      SUBSCRIPTION_ID='test-subscription' \
      DefaultWorkingDirectory="${TEST_REPO}" \
      BUILD_SRC_DIR="${TEST_REPO}" \
      FAIL_GO_TEST='1' \
      bash "${ROOT_DIR}/.pipelines/scripts/e2e_run.sh"
    The status should be failure
    The output should be present
    The contents of file "${COMMAND_LOG}" should include 'go <test> <-count=1> <./...>'
    The contents of file "${COMMAND_LOG}" should not include 'go <build>'
    The contents of file "${COMMAND_LOG}" should not include 'e2e <run>'
  End
End
