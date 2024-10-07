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
  if [[ ${array_size} -gt 0 ]]; then
    last_index=$(( ${#array_name[@]} - 1 ))
  else
    return 1
  fi
}

installJq() {
  # jq is not available until downloaded in install-dependencies.sh with the installDeps function
  # but it is needed earlier to call the capture_benchmarks function in pre-install-dependencies.sh
  output=$(jq --version)
  if [ -n "$output" ]; then
    echo "$output"
  else
    if isMarinerOrAzureLinux "$OS"; then
      sudo tdnf install -y jq && echo "jq was installed: $(jq --version)"
    else
      apt_get_install 5 1 60 jq && echo "jq was installed: $(jq --version)"
    fi
  fi
}

capture_benchmark() {
  set +x
  local title="$1"
  title="${title//[[:space:]]/_}"
  title="${title//-/_}"
  local is_final_section=${2:-false}

  local current_time
  current_time=$(date +%s)
  if [[ "$is_final_section" == true ]]; then
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
  set +x
  check_array_size benchmarks || { echo "Benchmarks array is empty"; return; }
  # create script object, then append each section object to it in the for loop
  script_object=$(jq -n --arg script_name "${SCRIPT_NAME}" '{($script_name): {}}')

  for ((i=0; i<${#benchmarks_order[@]}; i+=1)); do
    section_name=${benchmarks_order[i]}
    section_object=$(jq -n --arg section_name "${section_name}" --arg total_time_elapsed "${benchmarks[${section_name}]}" \
    '{($section_name): $total_time_elapsed'})
    script_object=$(jq -n --argjson script_object "$script_object" --argjson section_object "$section_object" --arg script_name "${SCRIPT_NAME}" \
    '$script_object | .[$script_name] += $section_object')
  done
 
  jq ". += $script_object" ${PERFORMANCE_DATA_FILE} > temp-build-perf-file.json && mv temp-build-perf-file.json ${PERFORMANCE_DATA_FILE}
  chmod 755 ${PERFORMANCE_DATA_FILE}
}


#capture_benchmark() {
  #set +x
  #local title="$1"
  #title="${title//[[:space:]]/_}"
  #title="${title//-/_}"
  #local is_final_section=${2:-false}

  #local current_time
  #current_time=$(date +%s.%N) # use %N to get nanoseconds
  #if [[ "$is_final_section" == true ]]; then
    #local start_time=$script_start_stopwatch
  #else
    #local start_time=$section_start_stopwatch
  #fi
  
  #local total_time_elapsed
  #total_time_elapsed=$(bc <<< "scale=3; ($current_time - $start_time) / 1") # scale=3 to get milliseconds
  #total_time_elapsed=$(awk -v t=$total_time_elapsed 'BEGIN {printf "%02d:%02d:%06.3f\n", t/3600, t%3600/60, t%60}')
  #benchmarks[$title]=${total_time_elapsed}
  #benchmarks_order+=($title)

  #section_start_stopwatch=$(date +%s.%N)
}