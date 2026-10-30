#!/bin/bash

Describe 'Build telemetry event cleanup'
  setup_events() {
    TEST_ROOT="$(mktemp -d)"
    EVENTS_ROOT="${TEST_ROOT}/events"
    export EVENTS_ROOT
    mkdir "${EVENTS_ROOT}"
  }
  cleanup_events_fixture() { rm -rf "${TEST_ROOT}"; }
  run_cleanup() {
    {
      sed -n '/^dumpBuildTelemetryEvents()/,/^}/p' vhdbuilder/packer/install-dependencies.sh
      printf '%s\n' 'dumpBuildTelemetryEvents "$1"'
    } | bash -s -- "${EVENTS_ROOT}"
  }
  BeforeEach 'setup_events'
  AfterEach 'cleanup_events_fixture'

  It 'prints and removes published files without deleting the queue directory'
    printf 'first\n' > "${EVENTS_ROOT}/1.json"
    When run run_cleanup
    The status should be success
    The output should equal first
    The path "${EVENTS_ROOT}/1.json" should not be exist
    The path "${EVENTS_ROOT}" should be directory
  End

  It 'leaves newly published events and in-flight temporary files intact'
    printf 'first\n' > "${EVENTS_ROOT}/1.json"
    printf 'in-flight\n' > "${TEST_ROOT}/temporary-event"
    Mock cat
      /bin/cat "$@"
      printf 'new event\n' > "${EVENTS_ROOT}/2.json"
    End
    When run run_cleanup
    The status should be success
    The output should equal first
    The path "${EVENTS_ROOT}/1.json" should not be exist
    The contents of file "${EVENTS_ROOT}/2.json" should equal 'new event'
    The contents of file "${TEST_ROOT}/temporary-event" should equal in-flight
  End

  It 'accepts an empty queue'
    When run run_cleanup
    The status should be success
    The output should be blank
    The path "${EVENTS_ROOT}" should be directory
  End

  It 'does not delete an event that could not be read'
    printf 'first\n' > "${EVENTS_ROOT}/1.json"
    Mock cat
      echo 'cannot read event' >&2
      exit 19
    End
    When run run_cleanup
    The status should equal 19
    The stderr should equal 'cannot read event'
    The path "${EVENTS_ROOT}/1.json" should be exist
  End

  It 'propagates a failed file removal'
    printf 'first\n' > "${EVENTS_ROOT}/1.json"
    Mock rm
      echo 'cannot remove event' >&2
      exit 17
    End
    When run run_cleanup
    The status should equal 17
    The output should equal first
    The stderr should equal 'cannot remove event'
    The path "${EVENTS_ROOT}/1.json" should be exist
  End

  It 'uses the race-safe cleanup after configuring build telemetry'
    When call grep -Fx 'dumpBuildTelemetryEvents || exit 1' vhdbuilder/packer/install-dependencies.sh
    The status should be success
    The output should equal 'dumpBuildTelemetryEvents || exit 1'
  End
End
