#!/bin/bash

Describe 'aks-log-collector.sh'
    SCRIPT="./parts/linux/cloud-init/artifacts/aks-log-collector.sh"

    # The collector queries IMDS and builds a zip at load time, so it cannot be sourced.
    # What these tests protect is the declarative part: the GLOBS list that decides which
    # files reach the support bundle. Extracting the literal GLOBS+=(...) lines and matching
    # paths against them exercises the same patterns the script runs, without its side
    # effects -- and a refactor that drops or narrows an entry fails here.
    setup_globs() {
        GLOB_FILE="$(mktemp)"
        # Only unconditional entries: the ones inside `if [ "$COLLECT_WAAGENT_FULL" ... ]`
        # are opt-in, and the paths under test must be collected on every node.
        sed -n 's/^GLOBS+=(\(.*\))$/\1/p' "$SCRIPT" >"$GLOB_FILE"
    }
    cleanup_globs() { rm -f "$GLOB_FILE"; }

    BeforeEach setup_globs
    AfterEach cleanup_globs

    # Reports whether any collected glob matches $1, using the same shell options the
    # collector sets (extglob for its @(...) patterns, nocaseglob for case-insensitivity).
    glob_covers() {
        local target="$1" pattern
        shopt -s extglob nocaseglob
        while read -r pattern; do
            [ -z "$pattern" ] && continue
            # shellcheck disable=SC2053 # pattern is a glob on purpose
            [[ $target == $pattern ]] && return 0
        done <"$GLOB_FILE"
        return 1
    }

    It 'collects the ANC hotfix timing artifact'
        # /var/log/azure/*/* requires a subdirectory, so this file -- written directly into
        # /var/log/azure by aks-node-controller (defaultHotfixTimingPath in hotfix.go) --
        # needs its own entry. Without it the hotfix timing breakdown is missing from every
        # support bundle, which is only noticeable once someone needs it.
        When call glob_covers /var/log/azure/aks-node-controller-hotfix-timing.json
        The status should be success
    End

    It 'does not collect unnamed files directly under /var/log/azure'
        # The reason the entry above is required: /var/log/azure/*/* needs a subdirectory,
        # so a file sitting directly in /var/log/azure is only collected if named. Were a
        # refactor to make this pass, the explicit timing entry would be redundant -- and
        # the test above would stop proving anything.
        When call glob_covers /var/log/azure/cluster-provision.log
        The status should be failure
    End

    It 'collects logs nested one level under /var/log/azure'
        When call glob_covers /var/log/azure/custom-script/handler.log
        The status should be success
    End

    It 'does not collect unrelated paths'
        # Confirms the matcher can fail, so the assertions above are meaningful.
        When call glob_covers /home/azureuser/.ssh/id_rsa
        The status should be failure
    End
End
