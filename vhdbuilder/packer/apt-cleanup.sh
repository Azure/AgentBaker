#!/bin/bash
# shellcheck shell=bash

# cleanup_apt_artifacts removes cached apt artifacts from the build VM.
# The cleanup runs only for Ubuntu images and preserves the top-level
# directories so future provisioning can recreate apt metadata as needed.
cleanup_apt_artifacts() {
  local os_name=${1:-}
  if [ -z "${UBUNTU_OS_NAME:-}" ] || [ "${os_name}" != "${UBUNTU_OS_NAME}" ]; then
    return 0
  fi

  local cache_dir="${APT_CACHE_DIR:-/var/cache/apt}"
  local lists_dir="${APT_LISTS_DIR:-/var/lib/apt/lists}"

  echo "Trimming apt caches under ${cache_dir} and ${lists_dir}"

  if [ -d "${cache_dir}" ]; then
    find "${cache_dir}" -mindepth 1 -delete
  fi

  if [ -d "${lists_dir}" ]; then
    find "${lists_dir}" -mindepth 1 -delete
  fi
}
