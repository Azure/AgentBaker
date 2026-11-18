#!/bin/bash

Describe 'boot-trace serial certificate control'
  Include './e2e/boot-trace/fixtures/serial-certificate-requests.sh'

  It 'returns the response body without its HTTP status'
    curl() { printf 'certificate body\n200'; }
    When call make_request_with_retry 'http://wireserver/test'
    The status should be success
    The output should eq 'certificate body'
  End

  It 'preserves bounded retries and reports exhaustion'
    curl() { printf 'unavailable\n503'; }
    sleep() { :; }
    When call make_request_with_retry 'http://wireserver/test'
    The status should be failure
    The stderr should include 'attempt 10/10'
    The stderr should include 'exhausted all retries'
    The output should be blank
  End
End
