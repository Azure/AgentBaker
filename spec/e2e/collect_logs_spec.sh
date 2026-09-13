#!/usr/bin/env shellspec

Describe 'Linux diagnostic archive'
  ROOT_DIR="$(pwd)"

  setup_collection() {
    TEST_ROOT="$(mktemp -d)"
    mkdir "$TEST_ROOT/work" "$TEST_ROOT/result"
  }

  cleanup_collection() {
    rm -f "$TEST_ROOT/result/"* "$TEST_ROOT/one" "$TEST_ROOT/two"
    rmdir "$TEST_ROOT/work" "$TEST_ROOT/result" "$TEST_ROOT"
  }

  run_collection() {
    TMPDIR="$TEST_ROOT/work" COPYFILE_DISABLE=1 \
      bash "$ROOT_DIR/e2e/scenario/collect_logs.sh" "$@" >"$TEST_ROOT/result/archive.tar.gz" &&
      tar -xzf "$TEST_ROOT/result/archive.tar.gz" -C "$TEST_ROOT/result"
  }

  BeforeEach 'setup_collection'
  AfterEach 'cleanup_collection'

  It 'runs commands concurrently'
    first="touch '$TEST_ROOT/one'; while [ ! -f '$TEST_ROOT/two' ]; do sleep 0.01; done; printf one"
    second="touch '$TEST_ROOT/two'; while [ ! -f '$TEST_ROOT/one' ]; do sleep 0.01; done; printf two"
    When call run_collection 2 0.1 "$first" "$second"
    The status should be success
    The contents of file "$TEST_ROOT/result/0.exit" should eq '0'
    The contents of file "$TEST_ROOT/result/1.exit" should eq '0'
    The contents of file "$TEST_ROOT/result/0.stdout" should eq 'one'
    The contents of file "$TEST_ROOT/result/1.stdout" should eq 'two'
  End

  It 'keeps stdout, stderr and nonzero exit codes separate'
    When call run_collection 2 0.1 "printf output; printf error >&2; exit 7"
    The status should be success
    The contents of file "$TEST_ROOT/result/0.stdout" should eq 'output'
    The contents of file "$TEST_ROOT/result/0.stderr" should eq 'error'
    The contents of file "$TEST_ROOT/result/0.exit" should eq '7'
  End

  It 'archives partial output when a command times out'
    When call run_collection 0.2 0.1 "printf partial; sleep 10" "printf complete"
    The status should be success
    The contents of file "$TEST_ROOT/result/0.stdout" should eq 'partial'
    The contents of file "$TEST_ROOT/result/0.exit" should eq '124'
    The contents of file "$TEST_ROOT/result/1.stdout" should eq 'complete'
    The contents of file "$TEST_ROOT/result/1.exit" should eq '0'
  End

  It 'stops commands that ignore termination'
    When call run_collection 0.2 0.1 "trap '' TERM; printf partial; sleep 10"
    The status should be success
    The stderr should include 'Killed'
    The contents of file "$TEST_ROOT/result/0.stdout" should eq 'partial'
    The contents of file "$TEST_ROOT/result/0.exit" should eq '137'
  End
End
