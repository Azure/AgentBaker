#!/bin/bash

script_start_stopwatch=$(date +%s)
section_start_stopwatch=$(date +%s)
SCRIPT_NAME=$(basename $0 .sh)
SCRIPT_NAME="${SCRIPT_NAME//-/_}"
declare -A benchmarks=()
declare -a benchmarks_order=()

check_array_size() {
  declare -n array_name=$1
  local array_size=${#array_name[@]}
  if [ "${array_size}" -gt 0 ]; then
    last_index=$(( ${#array_name[@]} - 1 ))
  else
    return 1
  fi
}

capture_benchmark() {
  local title="$1"
  title="${title//[[:space:]]/_}"
  title="${title//-/_}"
  local is_final_section=${2:-false}

  local current_time
  current_time=$(date +%s)
  if [ "$is_final_section" = "true" ]; then
    local start_time=$script_start_stopwatch
  else
    local start_time=$section_start_stopwatch
  fi
  
  local total_time_elapsed
  total_time_elapsed=$(date -d@$((current_time - start_time)) -u +%H:%M:%S)
  benchmarks[$title]=${total_time_elapsed}
  benchmarks_order+=($title) # use this array to maintain order of benchmarks

  # reset timers for next section
  section_start_stopwatch=$(date +%s)
}

process_benchmarks() {
  if [ -z "${PERFORMANCE_DATA_FILE}" ] ; then
    return
  fi

  if [ ! -f "${PERFORMANCE_DATA_FILE}" ]; then
    echo '{}' > "${PERFORMANCE_DATA_FILE}"
  fi

  check_array_size benchmarks || { echo "Benchmarks array is empty"; return; }

  for ((i=0; i<${#benchmarks_order[@]}; i+=1)); do
    section_name=${benchmarks_order[i]}
    section_object=$(jq -n --arg section_name "${section_name}" --arg total_time_elapsed "${benchmarks[${section_name}]}" \
    '{($section_name): $total_time_elapsed'})
    jq ". += $section_object" "${PERFORMANCE_DATA_FILE}" > temp-perf-file.json && mv temp-perf-file.json "${PERFORMANCE_DATA_FILE}"
  done
 
  chmod 755 "${PERFORMANCE_DATA_FILE}"
}