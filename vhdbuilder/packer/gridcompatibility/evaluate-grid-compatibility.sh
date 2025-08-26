#!/bin/bash
set -euo pipefail

readonly EXPECTED_MAJOR_VERSION=17
readonly VERSION_TOLERANCE=1
readonly GRID_VERSION_MIN=10
readonly GRID_VERSION_MAX=30

log_and_exit() {
  local FILE=${1}
  local ERR=${2}
  local SHOW_FILE=${3:-false}
  echo "##vso[task.logissue type=warning;sourcepath=$(basename $0);]${FILE} ${ERR}. Skipping grid compatibility evaluation."
  echo "##vso[task.complete result=SucceededWithIssues;]"
  if [ "${SHOW_FILE}" = "true" ]; then
    cat ${FILE}
  fi
  exit 0
}

parse_versions_from_output() {
  local program_output="$1"
  
  # First try to find explicit version patterns (v16, v17, etc.)
  local versions=$(echo "${program_output}" | grep -oE "v[0-9]+" | sed 's/v//g' | sort -u)
  
  # If no v-prefixed versions found, look for standalone numbers
  if [ -z "${versions}" ]; then
    versions=$(echo "${program_output}" | while IFS= read -r line; do
      if [[ "$line" =~ ^[[:space:]]*([0-9]+)[[:space:]]*$ ]]; then
        num="${BASH_REMATCH[1]}"
        if [ "$num" -ge "$GRID_VERSION_MIN" ] && [ "$num" -le "$GRID_VERSION_MAX" ]; then
          echo "$num"
        fi
      fi
    done | sort -u)
  fi
  
  echo "${versions}"
}

check_version_compatibility() {
  local versions="$1"
  local compatibility_issues=false
  
  for version in ${versions}; do
    # Validate that version is numeric
    if ! [[ "$version" =~ ^[0-9]+$ ]]; then
      echo "WARNING: Skipping invalid version format: $version"
      continue
    fi
    
    local version_diff=$((version > EXPECTED_MAJOR_VERSION ? version - EXPECTED_MAJOR_VERSION : EXPECTED_MAJOR_VERSION - version))
    
    if [ ${version_diff} -gt ${VERSION_TOLERANCE} ]; then
      echo "❌ COMPATIBILITY ISSUE: Version v${version} differs by ${version_diff} from expected v${EXPECTED_MAJOR_VERSION}"
      echo "##vso[task.logissue type=error;]GRID driver version v${version} is incompatible (differs by ${version_diff} from expected v${EXPECTED_MAJOR_VERSION})"
      compatibility_issues=true
    else
      echo "✅ Version v${version} is compatible (difference: ${version_diff})"
    fi
  done
  
  if [ "${compatibility_issues}" = "true" ]; then
    echo ""
    echo "❌ GRID COMPATIBILITY CHECK FAILED: Incompatible driver versions detected"
    echo "##vso[task.logissue type=error;]Grid compatibility check failed due to incompatible driver versions"
  else
    echo ""
    echo "✅ GRID COMPATIBILITY CHECK PASSED: All driver versions are compatible"
  fi
}

analyze_grid_compatibility() {
  local program_output="$1"
  
  echo ""
  echo "=== GRID VERSION COMPATIBILITY ANALYSIS ==="
  
  local versions=$(parse_versions_from_output "${program_output}")
  
  # Check if we need to try fallback parsing and inform user
  if [ -z "${versions}" ]; then
    echo "No v-prefixed versions found, trying standalone numbers..."
    # Re-parse with standalone number logic
    versions=$(echo "${program_output}" | while IFS= read -r line; do
      if [[ "$line" =~ ^[[:space:]]*([0-9]+)[[:space:]]*$ ]]; then
        num="${BASH_REMATCH[1]}"
        if [ "$num" -ge "$GRID_VERSION_MIN" ] && [ "$num" -le "$GRID_VERSION_MAX" ]; then
          echo "$num"
        fi
      fi
    done | sort -u)
  fi
  
  if [ -z "${versions}" ]; then
    echo "WARNING: No GRID driver versions found in program output"
    echo "##vso[task.logissue type=warning;]No GRID driver versions detected in output"
    return
  fi
  
  echo "Detected major versions: $(echo ${versions} | tr '\n' ' ')"
  check_version_compatibility "${versions}"
  echo "=== END COMPATIBILITY ANALYSIS ==="
}

run_grid_compatibility_program() {
  if [ ! -f "gridCompatibilityProgram" ]; then
    echo "ERROR: gridCompatibilityProgram not found in $(pwd)"
    return 1
  fi
  
  echo "Found gridCompatibilityProgram, making executable..."
  chmod +x gridCompatibilityProgram
  
  # Set environment variables for the grid compatibility program
  export KUSTO_PROD_ENDPOINT="https://sparkle.eastus.kusto.windows.net"
  export KUSTO_PROD_DATABASE="defaultdb"
  
  echo "Executing: ./gridCompatibilityProgram gpu-driver-production"
  echo "Expected major version: ${EXPECTED_MAJOR_VERSION}"
  
  # Capture program output for version analysis
  local program_output=$(./gridCompatibilityProgram gpu-driver-production 2>&1)
  local exit_code=$?
  
  # Display the program output
  echo "${program_output}"
  echo "gridCompatibilityProgram exit code: ${exit_code}"
  
  if [ ${exit_code} -eq 0 ]; then
    analyze_grid_compatibility "${program_output}"
  else
    echo "Skipping version analysis due to program failure (exit code: ${exit_code})"
  fi
  
  rm gridCompatibilityProgram
}

main() {
  echo "Starting grid compatibility evaluation..."
  echo "ENVIRONMENT: ${ENVIRONMENT}"
  
  # Early return for TME environment
  if [ "${ENVIRONMENT,,}" = "tme" ]; then
    echo "Skipping grid compatibility evaluation for tme environment"
    return 0
  fi
  
  # Change to grid compatibility directory
  pushd vhdbuilder/packer/gridcompatibility || {
    echo "ERROR: Cannot access gridcompatibility directory"
    exit 1
  }
  
  echo "Running grid compatibility evaluation program..."
  run_grid_compatibility_program
  
  popd || exit 0
  
  # Cleanup
  rm -f vhdbuilder/packer/gridcompatibility/${SIG_IMAGE_NAME}-grid-compatibility.json
  
  echo -e "\nGrid compatibility evaluation script completed."
}

# Run main function
main