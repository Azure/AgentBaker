#!/bin/bash

Describe 'cleanup_apt_cache'
  load_cleanup_function() {
    eval "$(sed -n '/^cleanup_apt_cache()/,/^}/p' './vhdbuilder/packer/install-dependencies.sh')"
  }

  BeforeAll 'load_cleanup_function'

  setup_environment() {
    TEST_TMP=$(mktemp -d)
    export APT_CACHE_PARENT_DIR="$TEST_TMP/cache"
    export APT_CACHE_DIR="$APT_CACHE_PARENT_DIR/archives"
    export APT_LISTS_DIR="$TEST_TMP/lists"
    mkdir -p "$APT_CACHE_DIR" "$APT_LISTS_DIR"
    echo "package" > "$APT_CACHE_DIR/dummy.deb"
    echo "list" > "$APT_LISTS_DIR/dummy.list"
    : > "$TEST_TMP/apt.log"

    UBUNTU_OS_NAME="UBUNTU"
    OS="$UBUNTU_OS_NAME"

    wait_for_apt_locks() { :; }
    apt-get() {
      echo "$@" >> "$TEST_TMP/apt.log"
      return 0
    }
  }

  teardown_environment() {
    rm -rf "$TEST_TMP"
    unset TEST_TMP
    unset APT_CACHE_PARENT_DIR APT_CACHE_DIR APT_LISTS_DIR OS
    unset -f apt-get 2>/dev/null || true
    unset -f wait_for_apt_locks 2>/dev/null || true
  }

  BeforeEach 'setup_environment'
  AfterEach 'teardown_environment'

  It 'cleans apt caches on Ubuntu systems'
    When call cleanup_apt_cache
    The stdout should include "Cleaning apt cache directories to reclaim disk space"
    The file "$APT_CACHE_DIR/dummy.deb" should not be exist
    The file "$APT_LISTS_DIR/dummy.list" should not be exist
    The path "$APT_LISTS_DIR" should be exist
    The contents of file "$TEST_TMP/apt.log" should include "clean"
    The contents of file "$TEST_TMP/apt.log" should include "autoclean"
  End

  It 'skips cleanup on non-Ubuntu systems'
    OS="MARINER"
    When call cleanup_apt_cache
    The file "$APT_CACHE_DIR/dummy.deb" should be exist
    The file "$APT_LISTS_DIR/dummy.list" should be exist
    The contents of file "$TEST_TMP/apt.log" should equal ""
  End
End
