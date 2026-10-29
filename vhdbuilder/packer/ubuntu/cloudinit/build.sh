#!/bin/bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
for release in 24.04 26.04; do
    docker build --build-arg "UBUNTU_RELEASE=${release}" \
        --output "type=local,dest=${script_dir}/${release}" "${script_dir}"
done
