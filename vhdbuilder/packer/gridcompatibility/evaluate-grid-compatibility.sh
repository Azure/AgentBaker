#!/bin/bash

log_and_exit () {
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

echo "=== DEBUG INFO ==="
echo "Current working directory: $(pwd)"
echo "Environment variables:"
echo "ENVIRONMENT: ${ENVIRONMENT}"
echo "USER: ${USER}"
echo "HOME: ${HOME}"
echo "=== END DEBUG INFO ==="

echo "Contents of current directory:"
ls -la

echo "Checking for vhdbuilder/packer structure:"
ls -la vhdbuilder/ || echo "vhdbuilder directory not found"
ls -la vhdbuilder/packer/ || echo "vhdbuilder/packer directory not found"

echo -e "\nENVIRONMENT is: ${ENVIRONMENT}"
if [ "${ENVIRONMENT,,}" != "tme" ]; then
  echo "Checking if gridcompatibility directory exists..."
  ls -la vhdbuilder/packer/gridcompatibility/ || echo "Directory not found"
  
  # mv ${SIG_IMAGE_NAME}-grid-compatibility.json vhdbuilder/packer/gridcompatibility
  pushd vhdbuilder/packer/gridcompatibility || exit 0
    echo "Current directory: $(pwd)"
    echo "Contents of gridcompatibility directory:"
    ls -la
    
    echo -e "\nRunning grid compatibility evaluation program...\n"
    if [ -f "gridCompatibilityProgram" ]; then
      echo "gridCompatibilityProgram found, making executable..."
      chmod +x gridCompatibilityProgram
      ls -la gridCompatibilityProgram
      
      # Set environment variables for the grid compatibility program
      export KUSTO_PROD_ENDPOINT="https://sparkle.eastus.kusto.windows.net"
      export KUSTO_PROD_DATABASE="defaultdb"
      
      echo "Environment variables set:"
      echo "KUSTO_PROD_ENDPOINT=${KUSTO_PROD_ENDPOINT}"
      echo "KUSTO_PROD_DATABASE=${KUSTO_PROD_DATABASE}"
      
      echo "Executing: ./gridCompatibilityProgram gpu-driver-production"
      
      # Set expected major version
      EXPECTED_MAJOR_VERSION=17
      echo "Expected major version: ${EXPECTED_MAJOR_VERSION}"
      
      # Capture program output for version analysis
      PROGRAM_OUTPUT=$(./gridCompatibilityProgram gpu-driver-production 2>&1)
      PROGRAM_EXIT_CODE=$?
      
      # Display the program output
      echo "${PROGRAM_OUTPUT}"
      echo "gridCompatibilityProgram exit code: ${PROGRAM_EXIT_CODE}"
      
      # Analyze version compatibility if program succeeded
      if [ ${PROGRAM_EXIT_CODE} -eq 0 ]; then
        echo ""
        echo "=== GRID VERSION COMPATIBILITY ANALYSIS ==="
        
        # Extract version numbers from the output using multiple patterns
        # First try to find explicit version patterns (v16, v17, etc.)
        VERSIONS=$(echo "${PROGRAM_OUTPUT}" | grep -oE "v[0-9]+" | sed 's/v//g' | sort -u)
        
        # If no v-prefixed versions found, look for standalone numbers in typical GRID version range
        if [ -z "${VERSIONS}" ]; then
          echo "DEBUG: No v-prefixed versions found, trying to extract standalone version numbers..."
          echo "DEBUG: Full program output:"
          echo "${PROGRAM_OUTPUT}" | head -20
          echo "DEBUG: Looking for version numbers in GRID range (10-30)..."
          
          # Look for lines containing only numbers in the GRID version range (10-30)
          VERSIONS=$(echo "${PROGRAM_OUTPUT}" | while IFS= read -r line; do
            # Check if line contains only a number (with optional whitespace)
            if [[ "$line" =~ ^[[:space:]]*([0-9]+)[[:space:]]*$ ]]; then
              num="${BASH_REMATCH[1]}"
              # Check if it's in GRID version range (10-30)
              if [ "$num" -ge 10 ] && [ "$num" -le 30 ]; then
                echo "$num"
              fi
            fi
          done | sort -u)
        fi
        
        if [ -z "${VERSIONS}" ]; then
          echo "WARNING: No GRID driver versions found in program output"
          echo "##vso[task.logissue type=warning;]No GRID driver versions detected in output"
          echo "DEBUG: Program output for analysis:"
          echo "${PROGRAM_OUTPUT}"
        else
          echo "Detected major versions: $(echo ${VERSIONS} | tr '\n' ' ')"
          
          COMPATIBILITY_ISSUES=false
          
          for VERSION in ${VERSIONS}; do
            # Validate that VERSION is numeric
            if ! [[ "$VERSION" =~ ^[0-9]+$ ]]; then
              echo "WARNING: Skipping invalid version format: $VERSION"
              continue
            fi
            
            VERSION_DIFF=$((VERSION > EXPECTED_MAJOR_VERSION ? VERSION - EXPECTED_MAJOR_VERSION : EXPECTED_MAJOR_VERSION - VERSION))
            
            if [ ${VERSION_DIFF} -gt 1 ]; then
              echo "❌ COMPATIBILITY ISSUE: Version v${VERSION} differs by ${VERSION_DIFF} from expected v${EXPECTED_MAJOR_VERSION}"
              echo "##vso[task.logissue type=error;]GRID driver version v${VERSION} is incompatible (differs by ${VERSION_DIFF} from expected v${EXPECTED_MAJOR_VERSION})"
              COMPATIBILITY_ISSUES=true
            else
              echo "✅ Version v${VERSION} is compatible (difference: ${VERSION_DIFF})"
            fi
          done
          
          if [ "${COMPATIBILITY_ISSUES}" = "true" ]; then
            echo ""
            echo "❌ GRID COMPATIBILITY CHECK FAILED: Incompatible driver versions detected"
            echo "##vso[task.logissue type=error;]Grid compatibility check failed due to incompatible driver versions"
            # Don't exit with error, just log the issue
          else
            echo ""
            echo "✅ GRID COMPATIBILITY CHECK PASSED: All driver versions are compatible"
            # No Azure DevOps logging for success case since 'info' type is not supported
          fi
        fi
        
        echo "=== END COMPATIBILITY ANALYSIS ==="
      else
        echo "Skipping version analysis due to program failure (exit code: ${PROGRAM_EXIT_CODE})"
      fi
      
      rm gridCompatibilityProgram
    else
      echo "ERROR: gridCompatibilityProgram not found in $(pwd)"
      echo "Available files:"
      ls -la
    fi
  popd || exit 0
else
  echo -e "Skipping grid compatibility evaluation for tme environment"
fi

rm -f vhdbuilder/packer/gridcompatibility/${SIG_IMAGE_NAME}-grid-compatibility.json

echo -e "\nGrid compatibility evaluation script completed."