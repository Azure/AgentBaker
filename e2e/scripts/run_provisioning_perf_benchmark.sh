#!/usr/bin/env bash
# Runs the CSE-vs-CustomData-only provisioning perf benchmark tests
# (Test_Ubuntu2204_ProvisioningPerf_CSE and Test_Ubuntu2204_ProvisioningPerf_CustomDataOnly)
# against the same pinned VHD build, N times each (default 5), running up to PARALLELISM
# iterations concurrently (across both modes combined), and prints the average of two
# durations for each mode:
#
#   vmss_creation_seconds     - wall-clock time from VMSS create call until ARM considers
#                                the VM provisioned (extension exit-code report in CSE mode,
#                                WALinuxAgent report_ready in CustomData-only mode).
#   total_cse_duration_seconds - the provisioning script's own internal exec duration
#                                (read from /var/log/azure/aks/provision.json via SSH).
#
# Usage:
#   ./e2e/scripts/run_provisioning_perf_benchmark.sh [iterations]
#
# Env vars:
#   SIG_VERSION_TAG_NAME / SIG_VERSION_TAG_VALUE - pin the VHD build under test
#                                                   (default: buildId / 175942544)
#   ITERATIONS                                    - number of times to run each test
#                                                    (default: 5, overridden by $1)
#   PARALLELISM                                    - max concurrent go test invocations,
#                                                   across both modes combined
#                                                   (default: 10)
#
# Each iteration creates its own uniquely-named VMSS, so concurrent runs don't collide;
# they do share the same underlying AKS cluster/subnet/bastion, same as any other e2e
# tests run in parallel. Keep PARALLELISM modest to avoid Azure API throttling / VM-SKU
# capacity errors - those show up as per-run failures below, not script bugs.
#
# Requires az login / AZURE_SUBSCRIPTION_ID etc. to already be configured for the
# AgentBaker e2e test infra, same as any other e2e test run.
set -uo pipefail

# e2e is its own Go module (e2e/go.mod), so `go test` must be run with cwd inside it
# and with a package path relative to that module (not "./e2e/...").
cd "$(dirname "${BASH_SOURCE[0]}")/.."

export SIG_VERSION_TAG_NAME="${SIG_VERSION_TAG_NAME:-buildId}"
export SIG_VERSION_TAG_VALUE="${SIG_VERSION_TAG_VALUE:-175942544}"
ITERATIONS="${1:-${ITERATIONS:-5}}"
PARALLELISM="${PARALLELISM:-10}"

OUT_DIR="$(mktemp -d)"

echo "Using SIG_VERSION_TAG_NAME=${SIG_VERSION_TAG_NAME} SIG_VERSION_TAG_VALUE=${SIG_VERSION_TAG_VALUE}"
echo "Running ${ITERATIONS} iteration(s) per mode (${PARALLELISM} concurrent max). Logs in ${OUT_DIR}"

# metric_file <mode> <metric_name>
metric_file() {
  echo "${OUT_DIR}/${1}_${2}.txt"
}

for mode in cse customdata_only; do
  : > "$(metric_file "${mode}" vmss_creation_seconds)"
  : > "$(metric_file "${mode}" total_cse_duration_seconds)"
done

# run_once <test_name_regex> <provisioning_mode> <run_index>
# Writes go test's full output only to its own per-run log file (not stdout) so that
# many concurrent runs don't interleave into an unreadable mess; only a short
# start/finish status line is printed live.
run_once() {
  local test_regex="$1" mode="$2" idx="$3"
  local log_file="${OUT_DIR}/${mode}_run${idx}.log"

  echo "[$(date +%H:%M:%S)] [${mode}] run ${idx}/${ITERATIONS} started"
  go test . -run "${test_regex}" -v -count=1 -timeout 60m > "${log_file}" 2>&1
  local test_exit=$?

  if [ "${test_exit}" -ne 0 ]; then
    if [ ! -s "${log_file}" ]; then
      echo "[$(date +%H:%M:%S)] [${mode}] run ${idx} FAILED with no output captured (exit ${test_exit})" \
        "- likely interrupted/killed before go test could run, see ${log_file}" >&2
    else
      echo "[$(date +%H:%M:%S)] [${mode}] run ${idx} FAILED (exit ${test_exit}), see ${log_file} - excluded from average" >&2
    fi
    return
  fi

  local benchmark_line
  benchmark_line="$(grep -E "BENCHMARK provisioning_mode=${mode} " "${log_file}" | tail -1 || true)"
  if [ -z "${benchmark_line}" ]; then
    echo "[$(date +%H:%M:%S)] [${mode}] run ${idx} FAILED: no BENCHMARK line found in ${log_file} - excluded from average" >&2
    return
  fi

  local vmss_creation total_cse
  vmss_creation="$(echo "${benchmark_line}" | grep -oE 'vmss_creation_seconds=[0-9.]+' | grep -oE '[0-9.]+$')"
  total_cse="$(echo "${benchmark_line}" | grep -oE 'total_cse_duration_seconds=[0-9.]+' | grep -oE '[0-9.]+$')"

  # Single short appends (< PIPE_BUF) are atomic on Linux even with concurrent writers,
  # so no external locking is needed here despite many runs writing to the same file.
  if [ -n "${vmss_creation}" ]; then
    echo "${vmss_creation}" >> "$(metric_file "${mode}" vmss_creation_seconds)"
  fi
  if [ -n "${total_cse}" ]; then
    echo "${total_cse}" >> "$(metric_file "${mode}" total_cse_duration_seconds)"
  fi
  echo "[$(date +%H:%M:%S)] [${mode}] run ${idx} done -> vmss_creation_seconds=${vmss_creation:-N/A} total_cse_duration_seconds=${total_cse:-N/A}"
}

# Build one combined job list across both modes so the parallelism budget is shared
# instead of finishing all CSE runs before starting any CustomData-only runs.
declare -a JOB_REGEX JOB_MODE JOB_IDX
for i in $(seq 1 "${ITERATIONS}"); do
  JOB_REGEX+=('^Test_Ubuntu2204_ProvisioningPerf_CSE$'); JOB_MODE+=("cse"); JOB_IDX+=("${i}")
done
for i in $(seq 1 "${ITERATIONS}"); do
  JOB_REGEX+=('^Test_Ubuntu2204_ProvisioningPerf_CustomDataOnly$'); JOB_MODE+=("customdata_only"); JOB_IDX+=("${i}")
done

running=0
for j in "${!JOB_REGEX[@]}"; do
  run_once "${JOB_REGEX[$j]}" "${JOB_MODE[$j]}" "${JOB_IDX[$j]}" &
  running=$((running + 1))
  if [ "${running}" -ge "${PARALLELISM}" ]; then
    wait -n
    running=$((running - 1))
  fi
done
wait

# average <times_file>
average() {
  local times_file="$1"
  awk 'NF { sum += $1; n++ } END { if (n > 0) printf "%.2f %d", sum/n, n; else print "NaN 0" }' "${times_file}"
}

# print_metric_summary <label> <metric_name>
print_metric_summary() {
  local label="$1" metric="$2"
  local cse_file customdata_file
  cse_file="$(metric_file cse "${metric}")"
  customdata_file="$(metric_file customdata_only "${metric}")"

  read -r cse_avg cse_n <<< "$(average "${cse_file}")"
  read -r customdata_avg customdata_n <<< "$(average "${customdata_file}")"

  echo
  echo "--- ${label} ---"
  echo "  CSE:             samples=[$(paste -sd, "${cse_file}" 2>/dev/null || echo none)]  avg=${cse_avg}s (n=${cse_n}/${ITERATIONS})"
  echo "  CustomData-only: samples=[$(paste -sd, "${customdata_file}" 2>/dev/null || echo none)]  avg=${customdata_avg}s (n=${customdata_n}/${ITERATIONS})"

  if [ "${cse_n}" -gt 0 ] && [ "${customdata_n}" -gt 0 ]; then
    awk -v cse="${cse_avg}" -v cd="${customdata_avg}" -v label="${label}" 'BEGIN {
      diff = cse - cd
      pct = (cse != 0) ? (diff / cse * 100) : 0
      printf "  CustomData-only is %.2fs faster than CSE on average for %s (%.1f%% reduction)\n", diff, label, pct
    }'
  fi
}

echo
echo "=== Benchmark summary (${ITERATIONS} iteration(s) requested per mode, up to ${PARALLELISM} concurrent) ==="
print_metric_summary "VMSS creation -> ARM ready (extension exit-code report / report_ready)" vmss_creation_seconds
print_metric_summary "Provisioning script exec duration (CSE start -> completion)" total_cse_duration_seconds

echo
echo "Full logs and raw samples are in: ${OUT_DIR}"
