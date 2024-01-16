#!/bin/bash
CNI_PREFETCH_SCRIPT_PATH="/opt/azure/containers/cni-prefetch.sh"

function runCNIPrefetch {
    chmod +x $CNI_PREFETCH_SCRIPT_PATH
    echo "running CNI prefetch driver script at $CNI_PREFETCH_SCRIPT_PATH..."
    sudo /bin/bash $CNI_PREFETCH_SCRIPT_PATH
    echo "CNI prefetch driver script completed successfully"

    echo "deleting CNI prefetch driver script at $CNI_PREFETCH_SCRIPT_PATH..."
    rm -- "$0"
    echo "CNI prefetch driver script deleted"
}

function testCNIPrefetchScriptExists() {
  local test="testPrefetchScriptExists"

  echo "$test: checking existence of CNI prefetch script at $CNI_PREFETCH_SCRIPT_PATH"

  if [ ! -f "$CNI_PREFETCH_SCRIPT_PATH" ]; then
    err "$test: CNI prefetch script does not exist at $CNI_PREFETCH_SCRIPT_PATH"
    return 1
  fi

  echo "$test: CNI prefetch script exists"
  return 0
}

function testCNIPrefetchScriptPermissions() {
    local test="testPrefetchScriptPermissions"

    echo "$test: checking permissions of CNI prefetch script at $CNI_PREFETCH_SCRIPT_PATH"
    if [ ! -x "$CNI_PREFETCH_SCRIPT_PATH" ]; then
        echo "$test: CNI prefetch script is not executable by the current user"
        return 1
    fi

    echo "$test: CNI prefetch script is executable"
}

function runTests(){
    testCNIPrefetchScriptExists || return 1
    testCNIPrefetchScriptPermissions || return 1
    return 0
}

if runTests; then
    runCNIPrefetch
fi
