#!/usr/local/lib/node_modules/bats/bin/bats

load ./test_helpers.bats.sh

setup() {
  load ../parts/linux/cloud-init/artifacts/cse_install.sh
  mock rm
}

teardown() {
  unmock rm
}

@test "test calling removeManDbAutoUpdateFlagFile" {
  removeManDbAutoUpdateFlagFile

  assert_called rm -f /var/lib/man-db/auto-update
}

@test "failing test" {
  assert_called rm -f /var/lib/man-db/auto-update
}

