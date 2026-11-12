#!/bin/bash

# Isolate scan functions: sourcing vhd-scanning.sh would create an Azure VM.
Describe 'AKS committed-plan kompli scan'
  BeforeAll 'eval "$(sed -n "/^kompli_plan_definitions() {/,/^}/p;/^kompli_plan_rule_ids() {/,/^}/p;/^run_kompli_scan() {/,/^}/p" vhdbuilder/packer/vhd-scanning.sh)"'

  setup_scan() {
    # Fixtures use the real mktemp; only the production mktemp -d call for
    # the run-private output dir is intercepted below (scoped mock), so this
    # setup never falls victim to its own mock (review finding).
    TEST_ROOT=$(command mktemp -d "${SHELLSPEC_TMPDIR:-$PWD}/kompli-spec.XXXXXX")
    CDIR="$TEST_ROOT"
    KOMPLI_PLAN_DIR="$TEST_ROOT/plans"
    KOMPLI_DEF_DIR="$TEST_ROOT/definitions"
    KOMPLI_BIN_LOCAL="$TEST_ROOT/kompli"
    PLAN_SUBDIR="$KOMPLI_PLAN_DIR/ubuntu-22.04"
    RENDER_TRACE="$TEST_ROOT/render.trace"
    UPLOAD_TRACE_DIR="$TEST_ROOT/uploads"
    mkdir -p "$PLAN_SUBDIR" "$KOMPLI_DEF_DIR" "$UPLOAD_TRACE_DIR"
    : > "$RENDER_TRACE"
    : > "$TEST_ROOT/invoke.trace"
    printf '{"benchmarks":{"ubuntu-22.04-l1":"pin","ubuntu-22.04-l2":"pin"}}\n' > "$TEST_ROOT/kompli-scan.json"
    printf '{"benchmarks":[{"file":"/etc/kompli/definitions/ubuntu-22.04-l1.benchmark.json"}]}\n' > "$PLAN_SUBDIR/plan-a.plan.json"
    printf '{}\n' > "$KOMPLI_DEF_DIR/ubuntu-22.04-l1.benchmark.json"
    printf '{}\n' > "$KOMPLI_DEF_DIR/ubuntu-22.04-l2.benchmark.json"
    printf '{}\n' > "$KOMPLI_DEF_DIR/unrelated.benchmark.json"
    cat > "$KOMPLI_BIN_LOCAL" <<FAKE
#!/bin/bash
echo "\$@" >> "$RENDER_TRACE"
if [ "\${RENDER_FAIL:-false}" = true ]; then exit 1; fi
# Strict stub: accepts only what vhd-scanning.sh's renderer call actually
# uses (render --format junit --suite-name <s> [--section=<id> ...] <path>),
# so an unsupported flag added later is caught here instead of silently
# passing through to a loose stub.
if [ "\$1" != render ]; then
  echo "fake kompli: unsupported subcommand: \$1" >&2
  exit 1
fi
shift
seen_format=false
seen_suite=false
while [ "\$#" -gt 1 ]; do
  case "\$1" in
    --format)
      if [ "\$2" != junit ]; then
        echo "fake kompli: unsupported --format: \$2" >&2
        exit 1
      fi
      seen_format=true
      shift 2
      ;;
    --suite-name)
      seen_suite=true
      shift 2
      ;;
    --section=*)
      shift
      ;;
    *)
      echo "fake kompli: unsupported option: \$1" >&2
      exit 1
      ;;
  esac
done
if [ "\$#" -ne 1 ]; then
  echo "fake kompli: expected exactly one input path, got \$#" >&2
  exit 1
fi
if ! "\$seen_format" || ! "\$seen_suite"; then
  echo "fake kompli: missing required --format/--suite-name" >&2
  exit 1
fi
printf '<testsuite/>\n'
FAKE
    chmod +x "$KOMPLI_BIN_LOCAL"
    OS_VERSION=22.04 BUILD_ID=42 SCAN_VM_NAME=scan RESOURCE_GROUP_NAME=rg
    SIG_CONTAINER_NAME=blobs STORAGE_ACCOUNT_NAME=storage AZURE_MSI_RESOURCE_STRING=msi
    ENABLE_TRUSTED_LAUNCH=false SCAN_VM_ADMIN_USERNAME=azureuser OS_SKU=Ubuntu
    MOCK_STATUS_PLAN_A=0 MOCK_STATUS_PLAN_B=0
    MOCK_STATUS_DL_RC_PLAN_A=0 MOCK_STATUS_DL_RC_PLAN_B=0
    MOCK_RESULT_PLAN_A='{"status":"NonCompliant"}' MOCK_RESULT_PLAN_B='{"status":"NonCompliant"}'
    INVOKE_RC=0 RENDER_FAIL=false SKIP_KOMPLI=false
    export RENDER_FAIL
    cd "$TEST_ROOT" || return 1
  }
  cleanup_scan() {
    cd - >/dev/null || true
    rm -rf -- "$TEST_ROOT"
  }
  BeforeEach 'setup_scan'
  AfterEach 'cleanup_scan'

  # Scoped so it never intercepts fixture setup's own `command mktemp` call
  # (review finding): only the production `mktemp -d ".../kompli-scan.XXXXXXXX"`
  # call is redirected, to a fixed name so output paths are assertable.
  mktemp() {
    case "${*: -1}" in
      */kompli-scan.XXXXXXXX)
        command mkdir -m 0700 "$TEST_ROOT/kompli-scan.fixed"
        printf '%s\n' "$TEST_ROOT/kompli-scan.fixed"
        ;;
      *) command mktemp "$@" ;;
    esac
  }

  az() {
    local action="$1 $2 $3" name= file= previous=
    shift 3
    while [ "$#" -gt 0 ]; do
      case "$previous" in
        --name) name="$1" ;;
        --file) file="$1" ;;
      esac
      previous="$1"
      shift
    done
    case "$action" in
      "vm run-command invoke")
        printf 'invoked\n' >> "$TEST_ROOT/invoke.trace"
        return "$INVOKE_RC"
        ;;
      "storage blob upload")
        case "$name" in
          kompli-defs-*) cp -- "$file" "$UPLOAD_TRACE_DIR/defs.tar" ;;
          kompli-plans-*) cp -- "$file" "$UPLOAD_TRACE_DIR/plans.tar" ;;
        esac
        return 0
        ;;
      "storage blob delete") return 0 ;;
      "storage blob download")
        case "$name" in
          *-status-*plan-a-*)
            [ "$MOCK_STATUS_DL_RC_PLAN_A" -eq 0 ] || return 1
            printf '%s\n' "$MOCK_STATUS_PLAN_A" > "$file" ;;
          *-status-*plan-b-*)
            [ "$MOCK_STATUS_DL_RC_PLAN_B" -eq 0 ] || return 1
            printf '%s\n' "$MOCK_STATUS_PLAN_B" > "$file" ;;
          *-result-*plan-a-*) printf '%s\n' "$MOCK_RESULT_PLAN_A" > "$file" ;;
          *-result-*plan-b-*) printf '%s\n' "$MOCK_RESULT_PLAN_B" > "$file" ;;
          *-log-*plan-a-*) printf 'log-a\n' > "$file" ;;
          *-log-*plan-b-*) printf 'log-b\n' > "$file" ;;
          *) return 1 ;;
        esac
        ;;
      *) return 1 ;;
    esac
  }

  invoke_count() { wc -l < "$TEST_ROOT/invoke.trace"; }
  defs_tar_entry_count() { tar -tf "$UPLOAD_TRACE_DIR/defs.tar" | wc -l; }
  plan_tar_entries_sorted() { tar -tf "$UPLOAD_TRACE_DIR/plans.tar" | LC_ALL=C sort | tr '\n' ','; }

  Describe 'kompli_plan_definitions (plan-to-definition mapping)'
    It 'maps only plan references, not extra acquired definitions'
      When call kompli_plan_definitions "$PLAN_SUBDIR/plan-a.plan.json" "$CDIR/kompli-scan.json" "$KOMPLI_DEF_DIR"
      The status should be success
      The output should eq 'ubuntu-22.04-l1.benchmark.json'
    End
    It 'rejects missing acquired definitions'
      rm "$KOMPLI_DEF_DIR/ubuntu-22.04-l1.benchmark.json"
      When call kompli_plan_definitions "$PLAN_SUBDIR/plan-a.plan.json" "$CDIR/kompli-scan.json" "$KOMPLI_DEF_DIR"
      The status should equal 1
    End
    It 'rejects duplicate plan paths'
      printf '{"benchmarks":[{"file":"/etc/kompli/definitions/ubuntu-22.04-l1.benchmark.json"},{"file":"/etc/kompli/definitions/ubuntu-22.04-l1.benchmark.json"}]}\n' > "$PLAN_SUBDIR/plan-a.plan.json"
      When call kompli_plan_definitions "$PLAN_SUBDIR/plan-a.plan.json" "$CDIR/kompli-scan.json" "$KOMPLI_DEF_DIR"
      The status should equal 1
    End
    It 'rejects unsafe parent paths'
      printf '{"benchmarks":[{"file":"/etc/kompli/definitions/../ubuntu-22.04-l1.benchmark.json"}]}\n' > "$PLAN_SUBDIR/plan-a.plan.json"
      When call kompli_plan_definitions "$PLAN_SUBDIR/plan-a.plan.json" "$CDIR/kompli-scan.json" "$KOMPLI_DEF_DIR"
      The status should equal 1
    End
    It 'rejects unpinned definition names'
      printf '{"benchmarks":[{"file":"/etc/kompli/definitions/unrelated.benchmark.json"}]}\n' > "$PLAN_SUBDIR/plan-a.plan.json"
      When call kompli_plan_definitions "$PLAN_SUBDIR/plan-a.plan.json" "$CDIR/kompli-scan.json" "$KOMPLI_DEF_DIR"
      The status should equal 1
    End
    It 'rejects symlinked acquired definitions'
      rm "$KOMPLI_DEF_DIR/ubuntu-22.04-l1.benchmark.json"
      ln -s unrelated.benchmark.json "$KOMPLI_DEF_DIR/ubuntu-22.04-l1.benchmark.json"
      When call kompli_plan_definitions "$PLAN_SUBDIR/plan-a.plan.json" "$CDIR/kompli-scan.json" "$KOMPLI_DEF_DIR"
      The status should equal 1
    End
    It 'rejects multiple plan JSON documents'
      printf '{}\n' >> "$PLAN_SUBDIR/plan-a.plan.json"
      When call kompli_plan_definitions "$PLAN_SUBDIR/plan-a.plan.json" "$CDIR/kompli-scan.json" "$KOMPLI_DEF_DIR"
      The status should equal 1
    End
    It 'rejects a benchmarks array with a non-object entry'
      printf '{"benchmarks":["not-an-object"]}\n' > "$PLAN_SUBDIR/plan-a.plan.json"
      When call kompli_plan_definitions "$PLAN_SUBDIR/plan-a.plan.json" "$CDIR/kompli-scan.json" "$KOMPLI_DEF_DIR"
      The status should equal 1
    End
  End

  Describe 'kompli_plan_rule_ids (per-plan --section filter source)'
    It 'extracts the unique rule ids a plan selects'
      printf '{"benchmarks":[{"file":"/etc/kompli/definitions/ubuntu-22.04-l1.benchmark.json","rules":{"1.1.1":{"mode":"audit"},"2.2.2":{"mode":"audit"}}}]}\n' > "$PLAN_SUBDIR/plan-a.plan.json"
      When call kompli_plan_rule_ids "$PLAN_SUBDIR/plan-a.plan.json"
      The status should be success
      The line 1 of output should eq '1.1.1'
      The line 2 of output should eq '2.2.2'
    End
    It 'rejects a plan that selects no rules'
      printf '{"benchmarks":[{"file":"/etc/kompli/definitions/ubuntu-22.04-l1.benchmark.json","rules":{}}]}\n' > "$PLAN_SUBDIR/plan-a.plan.json"
      When call kompli_plan_rule_ids "$PLAN_SUBDIR/plan-a.plan.json"
      The status should equal 1
      The stderr should include 'selects no rules'
    End
    It 'rejects an unsafe rule id'
      printf '{"benchmarks":[{"file":"/etc/kompli/definitions/ubuntu-22.04-l1.benchmark.json","rules":{"../etc/passwd":{"mode":"audit"}}}]}\n' > "$PLAN_SUBDIR/plan-a.plan.json"
      When call kompli_plan_rule_ids "$PLAN_SUBDIR/plan-a.plan.json"
      The status should equal 1
      The stderr should include 'unsafe rule id'
    End
    It 'rejects a malformed rules field'
      printf '{"benchmarks":[{"file":"/etc/kompli/definitions/ubuntu-22.04-l1.benchmark.json","rules":["1.1.1"]}]}\n' > "$PLAN_SUBDIR/plan-a.plan.json"
      When call kompli_plan_rule_ids "$PLAN_SUBDIR/plan-a.plan.json"
      The status should equal 1
      The stderr should include 'invalid committed plan (rules)'
    End
  End

  Describe 'run_kompli_scan (multi-plan execution)'
    add_plan_b() {
      printf '{"benchmarks":[{"file":"/etc/kompli/definitions/ubuntu-22.04-l1.benchmark.json","rules":{"2.2.2":{"mode":"audit"}}}]}\n' > "$PLAN_SUBDIR/plan-b.plan.json"
    }
    add_rules_to_plan_a() {
      printf '{"benchmarks":[{"file":"/etc/kompli/definitions/ubuntu-22.04-l1.benchmark.json","rules":{"1.1.1":{"mode":"audit"}}}]}\n' > "$PLAN_SUBDIR/plan-a.plan.json"
    }

    It 'skips (non-blocking) when the plan directory is missing'
      rm -rf -- "$PLAN_SUBDIR"
      When run run_kompli_scan
      The status should be success
      The output should include 'skipping'
    End
    It 'skips (non-blocking) when the plan directory is empty'
      rm -f -- "$PLAN_SUBDIR"/*.plan.json
      When run run_kompli_scan
      The status should be success
      The output should include 'skipping'
    End
    It 'rejects a plan file name with unsafe characters'
      printf '{"benchmarks":[]}\n' > "$PLAN_SUBDIR/Not_Safe.plan.json"
      add_rules_to_plan_a
      When run run_kompli_scan
      The status should equal 1
      The stderr should include 'unsafe plan name'
    End

    Describe 'with two plans sharing one definition'
      BeforeEach 'add_rules_to_plan_a' 'add_plan_b'

      It 'invokes the VM exactly once for both plans'
        When run run_kompli_scan
        The status should be success
        The result of function invoke_count should eq 1
      End
      It 'stages the union of referenced definitions once'
        When run run_kompli_scan
        The status should be success
        The result of function defs_tar_entry_count should eq 1
        The result of function plan_tar_entries_sorted should eq 'plan-a.plan.json,plan-b.plan.json,'
      End
      It 'filters each plan JUnit render to that plan rule id via --section'
        When run run_kompli_scan
        The status should be success
        The contents of file "$RENDER_TRACE" should include '--suite-name kompli ubuntu-22.04 plan-a --section=1.1.1'
        The contents of file "$RENDER_TRACE" should include '--suite-name kompli ubuntu-22.04 plan-b --section=2.2.2'
      End
      It 'renders on success even when findings are NonCompliant'
        When run run_kompli_scan
        The status should be success
        The path "$TEST_ROOT/kompli-scan.fixed/kompli-ubuntu-22.04-plan-a.junit.xml" should be file
        The path "$TEST_ROOT/kompli-scan.fixed/kompli-ubuntu-22.04-plan-b.junit.xml" should be file
        The output should include 'plan outcomes: plan-a: ok plan-b: ok'
      End
      It 'continues after one plan operational failure and still renders the other JUnit'
        MOCK_STATUS_PLAN_A=1
        When run run_kompli_scan
        The status should equal 1
        The stderr should include 'kompli ubuntu-22.04 plan-a: operational failure (engine exit 1)'
        The path "$TEST_ROOT/kompli-scan.fixed/kompli-ubuntu-22.04-plan-a.junit.xml" should not be exist
        The path "$TEST_ROOT/kompli-scan.fixed/kompli-ubuntu-22.04-plan-b.junit.xml" should be file
        The output should include 'plan outcomes: plan-a: failed (engine exit 1) plan-b: ok'
      End
      It 'fails closed for a plan when its status transfer is missing, without blocking the other'
        MOCK_STATUS_DL_RC_PLAN_B=1
        When run run_kompli_scan
        The status should equal 1
        The stderr should include 'kompli ubuntu-22.04 plan-b: operational failure (missing or invalid engine status)'
        The path "$TEST_ROOT/kompli-scan.fixed/kompli-ubuntu-22.04-plan-a.junit.xml" should be file
        The path "$TEST_ROOT/kompli-scan.fixed/kompli-ubuntu-22.04-plan-b.junit.xml" should not be exist
      End
      It 'captures a renderer failure for one plan without blocking the other'
        cat > "$KOMPLI_BIN_LOCAL" <<FAKE
#!/bin/bash
echo "\$@" >> "$RENDER_TRACE"
case "\$*" in
  *"plan-a"*) exit 1 ;;
esac
printf '<testsuite/>\n'
FAKE
        chmod +x "$KOMPLI_BIN_LOCAL"
        When run run_kompli_scan
        The status should equal 1
        The stderr should include 'kompli ubuntu-22.04 plan-a: operational failure (JUnit render)'
        The path "$TEST_ROOT/kompli-scan.fixed/kompli-ubuntu-22.04-plan-a.junit.xml" should not be exist
        The path "$TEST_ROOT/kompli-scan.fixed/kompli-ubuntu-22.04-plan-b.junit.xml" should be file
      End
      It 'fails closed when the VM invocation itself fails'
        INVOKE_RC=1
        When run run_kompli_scan
        The status should equal 1
        The stderr should include 'run-command or result/log transfer'
      End
      It 'stays non-blocking and skips cleanly when the scan is skipped'
        SKIP_KOMPLI=true
        When run run_kompli_scan
        The status should be success
        The output should include 'Skipping kompli scan'
      End
    End
  End
End
