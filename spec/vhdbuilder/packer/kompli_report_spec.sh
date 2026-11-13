#!/bin/bash

# Extract target-only helpers: never run the root/Azure VM entrypoint in specs
# unless explicitly isolated below with mocks for az, stat and the fixed
# ancestor/work-root paths (KOMPLI_WORK_ROOT/KOMPLI_ETC_DIR/
# KOMPLI_ETC_KOMPLI_DIR/KOMPLI_DEFS_DIR are overridable for exactly this).
Describe 'kompli target definitions'
  BeforeAll 'eval "$(sed -n "/^cleanup() {/,/^}/p;/^kompli_check_definition_dir() {/,/^}/p" vhdbuilder/packer/kompli-report.sh)"'
  setup_target() {
    TEST_ROOT=$(mktemp -d "${SHELLSPEC_TMPDIR:-$PWD}/kompli-target-spec.XXXXXX")
    KOMPLI_DEFS_DIR="$TEST_ROOT/definitions"
    WORK_DIR="$TEST_ROOT/work"
    mkdir -p "$KOMPLI_DEFS_DIR" "$WORK_DIR"
    printf 'untouched\n' > "$KOMPLI_DEFS_DIR/unrelated.benchmark.json"
    printf 'installed\n' > "$KOMPLI_DEFS_DIR/selected.benchmark.json"
    installed_defs=(selected.benchmark.json)
    # Record the real current inode, exactly as the install loop does right
    # after exclusive creation: cleanup's safety property is that it only
    # ever removes a file whose on-disk identity still matches this.
    installed_inodes=("$(stat -c %d:%i "$KOMPLI_DEFS_DIR/selected.benchmark.json")")
  }
  cleanup_target() { rm -rf -- "$TEST_ROOT"; }
  BeforeEach 'setup_target'
  AfterEach 'cleanup_target'

  It 'cleans only definitions installed by this run'
    When call cleanup
    The status should be success
    The path "$KOMPLI_DEFS_DIR/unrelated.benchmark.json" should be file
    The path "$KOMPLI_DEFS_DIR/selected.benchmark.json" should not be exist
    The path "$KOMPLI_DEFS_DIR" should be directory
  End
  It 'leaves a replaced definition untouched (inode no longer matches)'
    # Write the replacement to a separate file while the original still
    # exists, then move it into place: this guarantees a distinct inode,
    # unlike rm-then-recreate which can reuse the just-freed one.
    printf 'replaced-by-someone-else\n' > "$KOMPLI_DEFS_DIR/selected.benchmark.json.new"
    mv -f -- "$KOMPLI_DEFS_DIR/selected.benchmark.json.new" "$KOMPLI_DEFS_DIR/selected.benchmark.json"
    When call cleanup
    The status should be success
    The path "$KOMPLI_DEFS_DIR/selected.benchmark.json" should be file
    The contents of file "$KOMPLI_DEFS_DIR/selected.benchmark.json" should eq 'replaced-by-someone-else'
    The stderr should include 'leaving replaced definition'
  End
  It 'rejects a symlink target without modifying its source'
    ln -s "$KOMPLI_DEFS_DIR" "$TEST_ROOT/linked"
    When call kompli_check_definition_dir "$TEST_ROOT/linked"
    The status should equal 1
    The stderr should include "kompli: definition path is a symlink: $TEST_ROOT/linked"
    The path "$KOMPLI_DEFS_DIR/unrelated.benchmark.json" should be file
  End
  It 'rejects a writable existing directory without chmod or chown'
    chmod 0777 "$KOMPLI_DEFS_DIR"
    stat() {
      if [ "$1" = -c ] && [ "$2" = %u ]; then printf '0\n'; else command stat "$@"; fi
    }
    When call kompli_check_definition_dir "$KOMPLI_DEFS_DIR"
    The status should equal 1
    The stderr should include 'kompli: definition path is not root-owned and non-writable'
  End
End

# The exclusive-install loop, isolated from the rest of the entrypoint so its
# noclobber/inode-recording behavior can be asserted directly with `call`
# (top-level `exit` is rewritten to `return` for this one extracted block
# only; the loop's own exit points are the only `exit N` tokens in range).
Describe 'kompli_install_definitions (exclusive root-owned install)'
  BeforeAll 'eval "$(printf "kompli_install_definitions() {\n%s\n}\n" "$(sed -n "/^umask 077$/,/^done$/p" vhdbuilder/packer/kompli-report.sh | sed -E "s/\bexit ([0-9]+)/return \1/g")")"'

  setup_install() {
    TEST_ROOT=$(mktemp -d "${SHELLSPEC_TMPDIR:-$PWD}/kompli-install-spec.XXXXXX")
    KOMPLI_DEFS_DIR="$TEST_ROOT/definitions"
    STAGE_DIR="$TEST_ROOT/staged"
    mkdir -p "$KOMPLI_DEFS_DIR" "$STAGE_DIR"
    printf 'definition-a-content\n' > "$STAGE_DIR/def-a.benchmark.json"
    def_names=(def-a.benchmark.json)
    installed_defs=()
    installed_inodes=()
  }
  cleanup_install() { rm -rf -- "$TEST_ROOT"; }
  BeforeEach 'setup_install'
  AfterEach 'cleanup_install'

  installed_defs_joined() { local IFS=,; echo "${installed_defs[*]}"; }
  installed_inodes_count() { echo "${#installed_inodes[@]}"; }

  It 'installs a definition exclusively when the destination is absent'
    When call kompli_install_definitions
    The status should be success
    The contents of file "$KOMPLI_DEFS_DIR/def-a.benchmark.json" should eq 'definition-a-content'
    The result of function installed_defs_joined should eq 'def-a.benchmark.json'
    The result of function installed_inodes_count should eq 1
  End
  It 'fails when the destination already exists, without modifying it'
    printf 'pre-existing-content\n' > "$KOMPLI_DEFS_DIR/def-a.benchmark.json"
    When call kompli_install_definitions
    The status should equal 1
    The stderr should include 'definition already exists: def-a.benchmark.json'
    The contents of file "$KOMPLI_DEFS_DIR/def-a.benchmark.json" should eq 'pre-existing-content'
    The result of function installed_defs_joined should eq ''
  End
End

# Whole-script entrypoint, run via the ShellSpec subshell evaluator so its own
# `set -eux`, `exit N` calls and `trap cleanup EXIT` behave exactly as they do
# on the scan VM, without touching the real /root or /etc. KOMPLI_WORK_ROOT,
# KOMPLI_ETC_DIR, KOMPLI_ETC_KOMPLI_DIR and KOMPLI_DEFS_DIR are the test seams
# kompli-report.sh exposes for this.
Describe 'kompli-report.sh entrypoint (multi-plan, two plans sharing one definition)'
  BeforeAll 'eval "$(printf "kompli_report_main() {\n%s\n}\n" "$(sed 1d vhdbuilder/packer/kompli-report.sh)")"'

  setup_report_entrypoint() {
    TEST_ROOT=$(mktemp -d "${SHELLSPEC_TMPDIR:-$PWD}/kompli-report-entrypoint.XXXXXX")
    WORK_ROOT="$TEST_ROOT/root"
    ETC_DIR="$TEST_ROOT/etc"
    ETC_KOMPLI_DIR="$ETC_DIR/kompli"
    DEFS_DIR="$ETC_KOMPLI_DIR/definitions"
    UPLOAD_TRACE_DIR="$TEST_ROOT/uploads"
    FIXTURE_DIR="$TEST_ROOT/fixtures"
    mkdir -p "$WORK_ROOT" "$ETC_DIR" "$ETC_KOMPLI_DIR" "$DEFS_DIR" "$UPLOAD_TRACE_DIR" \
      "$FIXTURE_DIR/plans" "$FIXTURE_DIR/defs"
    chmod 0755 "$ETC_DIR" "$ETC_KOMPLI_DIR" "$DEFS_DIR"

    printf '{"benchmarks":[{"file":"/etc/kompli/definitions/def-a.benchmark.json","rules":{"1.1":{"mode":"audit"}}}]}\n' > "$FIXTURE_DIR/plans/plan-a.plan.json"
    printf '{"benchmarks":[{"file":"/etc/kompli/definitions/def-a.benchmark.json","rules":{"2.2":{"mode":"audit"}}}]}\n' > "$FIXTURE_DIR/plans/plan-b.plan.json"
    printf 'definition content\n' > "$FIXTURE_DIR/defs/def-a.benchmark.json"
    FIXTURE_PLAN_TAR="$TEST_ROOT/plans.tar"
    FIXTURE_DEFS_TAR="$TEST_ROOT/defs.tar"
    tar -cf "$FIXTURE_PLAN_TAR" -C "$FIXTURE_DIR/plans" plan-a.plan.json plan-b.plan.json
    tar -cf "$FIXTURE_DEFS_TAR" -C "$FIXTURE_DIR/defs" def-a.benchmark.json

    # Fake engine: per-plan exit code / JSON content, chosen by fixed,
    # non-indirect variable names (only two plans ever exist here).
    FIXTURE_KOMPLI_BIN="$TEST_ROOT/kompli-fixture"
    cat > "$FIXTURE_KOMPLI_BIN" <<'FAKE'
#!/bin/bash
for plan_path in "$@"; do :; done
plan_name="$(basename "$plan_path" .plan.json)"
case "$plan_name" in
  plan-a) rc="${KOMPLI_FIXTURE_EXIT_PLAN_A:-0}"; content="${KOMPLI_FIXTURE_JSON_PLAN_A:-}" ;;
  plan-b) rc="${KOMPLI_FIXTURE_EXIT_PLAN_B:-0}"; content="${KOMPLI_FIXTURE_JSON_PLAN_B:-}" ;;
  *) rc=0; content="" ;;
esac
if [ -z "$content" ]; then
  content='{"status":"NonCompliant","plan":"'"$plan_name"'"}'
fi
printf '%s' "$content"
printf 'kompli fixture log for %s (exit %s)\n' "$plan_name" "$rc" >&2
exit "$rc"
FAKE
    chmod +x "$FIXTURE_KOMPLI_BIN"

    KOMPLI_BLOB_NAME=kompli-bin-blob
    PLAN_TAR_BLOB_NAME=plans-blob
    PLAN_NAMES=plan-a,plan-b
    DEFS_TAR_BLOB_NAME=defs-blob
    RESULT_BLOB_NAMES=result-a,result-b
    LOG_BLOB_NAMES=log-a,log-b
    STATUS_BLOB_NAMES=status-a,status-b
    STORAGE_ACCOUNT_NAME=storage
    SIG_CONTAINER_NAME=blobs
    AZURE_MSI_RESOURCE_STRING=msi
    ENABLE_TRUSTED_LAUNCH=false
    TEST_VM_ADMIN_USERNAME=azureuser
    OS_SKU=Ubuntu
    KOMPLI_WORK_ROOT="$WORK_ROOT"
    KOMPLI_ETC_DIR="$ETC_DIR"
    KOMPLI_ETC_KOMPLI_DIR="$ETC_KOMPLI_DIR"
    KOMPLI_DEFS_DIR="$DEFS_DIR"
    UPLOAD_FAIL_NAMES=""
  }
  cleanup_report_entrypoint() {
    unset KOMPLI_FIXTURE_EXIT_PLAN_A KOMPLI_FIXTURE_EXIT_PLAN_B \
      KOMPLI_FIXTURE_JSON_PLAN_A KOMPLI_FIXTURE_JSON_PLAN_B
    rm -rf -- "$TEST_ROOT"
  }
  BeforeEach 'setup_report_entrypoint'
  AfterEach 'cleanup_report_entrypoint'

  # Ownership checks only; falls through to the real `stat` for the
  # inode-identity calls in the install loop and in cleanup().
  stat() {
    if [ "$1" = -c ] && [ "$2" = %u ]; then printf '0\n'; else command stat "$@"; fi
  }
  az() {
    case "$1" in
      login) return "${MOCK_LOGIN_RC:-0}" ;;
      storage)
        local verb="$2 $3" name= file= prev=
        shift 3
        while [ "$#" -gt 0 ]; do
          case "$prev" in
            --name) name="$1" ;;
            --file) file="$1" ;;
          esac
          prev="$1"
          shift
        done
        case "$verb" in
          "blob download")
            case "$name" in
              "$KOMPLI_BLOB_NAME") cp -- "$FIXTURE_KOMPLI_BIN" "$file" ;;
              "$PLAN_TAR_BLOB_NAME") cp -- "$FIXTURE_PLAN_TAR" "$file" ;;
              "$DEFS_TAR_BLOB_NAME") cp -- "$FIXTURE_DEFS_TAR" "$file" ;;
              *) return 1 ;;
            esac
            ;;
          "blob upload")
            local fail_name
            mkdir -p "$UPLOAD_TRACE_DIR"
            cp -- "$file" "$UPLOAD_TRACE_DIR/$name"
            for fail_name in $UPLOAD_FAIL_NAMES; do
              [ "$name" = "$fail_name" ] && return 1
            done
            return 0
            ;;
          "blob delete")
            # Mirrors a real delete: removes whatever upload trace exists for
            # this blob name, so a withheld/removed status blob is
            # observable afterward as a missing trace file.
            rm -f -- "$UPLOAD_TRACE_DIR/$name"
            return 0
            ;;
          *) return 1 ;;
        esac
        ;;
      *) return 1 ;;
    esac
  }

  It 'continues past one plan engine failure with partial JSON, leaving the other plan intact'
    export KOMPLI_FIXTURE_EXIT_PLAN_A=1
    export KOMPLI_FIXTURE_JSON_PLAN_A='{"status":"Non'
    When run kompli_report_main
    The status should be success
    The contents of file "$UPLOAD_TRACE_DIR/status-a" should eq '1'
    The contents of file "$UPLOAD_TRACE_DIR/status-b" should eq '0'
    The contents of file "$UPLOAD_TRACE_DIR/result-a" should eq '{"status":"Non'
    The contents of file "$UPLOAD_TRACE_DIR/result-b" should include '"plan":"plan-b"'
    The output should include 'kompli: 2 plan(s) run, 1 with failures'
    The stderr should be present
  End
  It 'withholds status (without attempting it) when the result upload fails, leaving the other plan intact'
    UPLOAD_FAIL_NAMES="result-a"
    When run kompli_report_main
    The status should be success
    The stderr should include 'kompli (plan-a): upload failure for result/log; withholding status'
    The path "$UPLOAD_TRACE_DIR/status-a" should not be exist
    The contents of file "$UPLOAD_TRACE_DIR/result-b" should include '"plan":"plan-b"'
    The contents of file "$UPLOAD_TRACE_DIR/status-b" should eq '0'
    The output should include 'kompli: 2 plan(s) run, 1 with failures'
  End
  It 'removes a partial status blob when the status upload itself fails, leaving the other plan intact'
    UPLOAD_FAIL_NAMES="status-a"
    When run kompli_report_main
    The status should be success
    The stderr should include 'kompli (plan-a): upload failure for status; removing any partial status blob'
    The path "$UPLOAD_TRACE_DIR/status-a" should not be exist
    The contents of file "$UPLOAD_TRACE_DIR/result-a" should include '"plan":"plan-a"'
    The contents of file "$UPLOAD_TRACE_DIR/result-b" should include '"plan":"plan-b"'
    The contents of file "$UPLOAD_TRACE_DIR/status-b" should eq '0'
    The output should include 'kompli: 2 plan(s) run, 1 with failures'
  End
End
