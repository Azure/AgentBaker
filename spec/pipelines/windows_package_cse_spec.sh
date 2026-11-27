#!/usr/bin/env shellspec

Describe 'Windows CSE signed script packaging'
  ROOT_DIR="$(pwd)"

  setup_package() {
    unset MOCK_DOWNLOAD_FAIL
    TEST_ROOT="$(mktemp -d)"
    TEST_REPO="$TEST_ROOT/repo"
    MOCK_BIN="$TEST_ROOT/bin"
    MOCK_SIGNED_ARTIFACT_DIR="$TEST_ROOT/signed"
    CSE_RELEASE_DIR="$TEST_ROOT/release"
    CSE_PUBLISH_DIR="$TEST_ROOT/packages"
    CSE_FILE_NAME="windows-cse.zip"
    CSE_SIGNING_BUILD_ID="12345"
    SYSTEM_COLLECTIONURI="https://dev.azure.com/test/"
    SYSTEM_TEAMPROJECTID="test-project"
    AZURE_DEVOPS_EXT_PAT="test-token"
    source_dir="$TEST_REPO/staging/cse/windows"
    signed_dir="$MOCK_SIGNED_ARTIFACT_DIR/staging/cse/windows"
    mkdir -p "$source_dir/provisioningscripts" "$source_dir/debug" \
      "$source_dir/helpers.tests.suites" "$signed_dir/provisioningscripts" "$MOCK_BIN"
    printf 'unsigned helper\n' > "$source_dir/helpers.ps1"
    printf 'unsigned start\n' > "$source_dir/provisioningscripts/start.ps1"
    printf 'signed helper\n' > "$signed_dir/helpers.ps1"
    printf 'signed start\n' > "$signed_dir/provisioningscripts/start.ps1"
    printf 'test script\n' > "$source_dir/helpers.tests.ps1"
    printf 'test suite\n' > "$source_dir/helpers.tests.suites/test.ps1"
    printf 'debug script\n' > "$source_dir/debug/update-scripts.ps1"
    printf 'readme\n' > "$source_dir/README"
    printf 'configuration\n' > "$source_dir/config.json"
    cat > "$MOCK_BIN/az" <<'EOF'
#!/bin/bash
set -euo pipefail
if [ "${MOCK_DOWNLOAD_FAIL:-0}" = "1" ]; then
  echo 'artifact download failed' >&2
  exit 1
fi
[ "$1 $2 $3 $4" = 'pipelines runs artifact download' ]
shift 4
while [ "$#" -gt 0 ]; do
  case "$1" in
    --organization) [ "$2" = "$SYSTEM_COLLECTIONURI" ]; shift 2 ;;
    --project) [ "$2" = "$SYSTEM_TEAMPROJECTID" ]; shift 2 ;;
    --run-id) [ "$2" = "$CSE_SIGNING_BUILD_ID" ]; shift 2 ;;
    --artifact-name) [ "$2" = 'signed-powershell-scripts' ]; shift 2 ;;
    --path) download_path="$2"; shift 2 ;;
    --only-show-errors) shift ;;
    *) echo "unexpected argument: $1" >&2; exit 1 ;;
  esac
done
cp -r "$MOCK_SIGNED_ARTIFACT_DIR/." "$download_path"
EOF
    chmod +x "$MOCK_BIN/az"
    export ROOT_DIR TEST_ROOT TEST_REPO MOCK_BIN MOCK_SIGNED_ARTIFACT_DIR \
      CSE_RELEASE_DIR CSE_PUBLISH_DIR CSE_FILE_NAME CSE_SIGNING_BUILD_ID \
      SYSTEM_COLLECTIONURI SYSTEM_TEAMPROJECTID AZURE_DEVOPS_EXT_PAT
    PATH="$MOCK_BIN:$PATH"
    export PATH
  }

  cleanup_package() {
    rm -rf -- "$TEST_ROOT"
  }

  run_package() {
    cd "$TEST_REPO" || return 1
    bash "$ROOT_DIR/.pipelines/scripts/windows_package_cse.sh"
  }

  package_and_read_zip() {
    run_package || return
    unzip -p "$CSE_PUBLISH_DIR/$CSE_FILE_NAME" helpers.ps1 provisioningscripts/start.ps1 config.json
  }

  BeforeEach 'setup_package'
  AfterEach 'cleanup_package'

  It 'packages the signed files from the exact run and preserves non-script assets'
    When call package_and_read_zip
    The status should be success
    The output should include 'signed helper'
    The output should include 'signed start'
    The output should include 'configuration'
    The contents of file "$CSE_RELEASE_DIR/helpers.ps1" should eq 'signed helper'
    The contents of file "$CSE_RELEASE_DIR/provisioningscripts/start.ps1" should eq 'signed start'
    The contents of file "$CSE_RELEASE_DIR/config.json" should eq 'configuration'
    The path "$CSE_RELEASE_DIR/helpers.tests.ps1" should not be exist
    The path "$CSE_RELEASE_DIR/helpers.tests.suites" should not be exist
    The path "$CSE_RELEASE_DIR/debug/update-scripts.ps1" should not be exist
    The path "$CSE_PUBLISH_DIR/$CSE_FILE_NAME" should be exist
  End

  It 'fails without creating a ZIP when artifact download fails'
    export MOCK_DOWNLOAD_FAIL=1
    When call run_package
    The status should be failure
    The output should include 'Downloading signed PowerShell scripts from run 12345'
    The stderr should include 'artifact download failed'
    The path "$CSE_PUBLISH_DIR/$CSE_FILE_NAME" should not be exist
  End

  It 'rejects an incomplete artifact rather than packaging an unsigned script'
    rm "$MOCK_SIGNED_ARTIFACT_DIR/staging/cse/windows/provisioningscripts/start.ps1"
    When call run_package
    The status should be failure
    The output should be present
    The stderr should include 'Missing signed script in run 12345: provisioningscripts/start.ps1'
    The path "$CSE_PUBLISH_DIR/$CSE_FILE_NAME" should not be exist
  End

  It 'requires the signing run ID instead of downloading the latest run'
    unset CSE_SIGNING_BUILD_ID
    When call run_package
    The status should be failure
    The stderr should include 'CSE_SIGNING_BUILD_ID must be set'
    The path "$CSE_PUBLISH_DIR/$CSE_FILE_NAME" should not be exist
  End
End
