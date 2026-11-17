#!/bin/bash

Describe 'image BOM generation'
  setup() {
    TEST_ROOT="$(mktemp -d)"
    TEST_SCRIPT="${TEST_ROOT}/list-images.sh"
    export CALLS_FILE="${TEST_ROOT}/calls"
    export SKU_NAME="test-sku" IMAGE_VERSION="202611.09.0"
    export FETCHER_STATUS=0
    mkdir -p "${TEST_ROOT}/containers"
    sed "s|/opt/azure|${TEST_ROOT}|g" ./parts/root/common/opt/azure/image-build/list-images.sh > "$TEST_SCRIPT"
    cat > "${TEST_ROOT}/containers/image-fetcher" <<'EOF'
#!/bin/bash
printf '%s\n' "$@" > "$CALLS_FILE"
if [ "$FETCHER_STATUS" -ne 0 ]; then
  exit "$FETCHER_STATUS"
fi
printf '{}\n' > "${CALLS_FILE%/*}/containers/image-bom.json"
EOF
    chmod +x "${TEST_ROOT}/containers/image-fetcher"
  }

  cleanup() {
    rm -rf "$TEST_ROOT"
  }

  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'uses the image-fetcher list-images subcommand with VHD metadata'
    When run bash "$TEST_SCRIPT"
    The status should be success
    The output should include 'Generating image-bom'
    The error should be present
    The contents of file "$CALLS_FILE" should equal "$(printf '%s\n' \
      list-images --sku test-sku --node-image-version 202611.09.0 \
      --output-path "${TEST_ROOT}/containers/image-bom.json")"
    The path "${TEST_ROOT}/containers/image-bom.json" should be file
  End

  It 'propagates inventory generation failures'
    export FETCHER_STATUS=42
    When run bash "$TEST_SCRIPT"
    The status should eq 42
    The output should include 'Generating image-bom'
    The error should be present
    The path "${TEST_ROOT}/containers/image-bom.json" should not be exist
  End

  It 'rejects a non-executable image-fetcher'
    chmod -x "${TEST_ROOT}/containers/image-fetcher"
    When run bash "$TEST_SCRIPT"
    The status should be failure
    The output should include 'could not find executable image-fetcher'
    The error should be present
    The path "$CALLS_FILE" should not be exist
  End
End
