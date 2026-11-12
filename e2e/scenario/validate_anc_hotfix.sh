#!/bin/bash
set -euo pipefail

root=/opt/azure/containers
config="$root/aks-node-controller-hotfix.json"
original="$root/aks-node-controller"
hotfix="$root/aks-node-controller-hotfix"

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
skip() { printf 'SKIP: %s\n' "$*"; exit 0; }

[[ -e "$config" || -L "$config" ]] || skip "No hotfix config"
[[ -f "$config" ]] || fail "Hotfix config is not a regular file: $config"
json=$(cat "$config") || fail "Cannot read $config"
[[ -n "${json//[[:space:]]/}" ]] || skip "Empty hotfix config"

command -v jq >/dev/null || fail "jq is required to validate the hotfix config"
jq -e -s '
    length == 1 and (.[0] |
        type == "object" and
        ((.version == null) or (.version | type == "string")) and
        ((.hotfixes == null) or
         (.hotfixes | type == "object" and all(.[]; type == "string"))))
' <<<"$json" >/dev/null || fail "Invalid hotfix config"

count=$(jq '(.hotfixes // {}) | length' <<<"$json")
legacy=$(jq -r '.version // "" | gsub("^\\s+|\\s+$"; "")' <<<"$json")
[[ "$count" != 0 || -n "$legacy" ]] || skip "No binary hotfix requested"

current=$("$original" version) || fail "Cannot read baked ANC version"

# Preserve the literal base, including leading zeros in the day.
version_pattern='^([0-9]+)\.([0-9]+)\.([0-9]+)([-+].*)?$'
[[ "$current" =~ $version_pattern ]] ||
    fail "Unrecognized baked ANC version: $current"
major=${BASH_REMATCH[1]}
minor=${BASH_REMATCH[2]}
patch=${BASH_REMATCH[3]}
base="$major.$minor"

if (( count > 0 )); then
    # Exact base lookup matches the baked version's prefix at a dot boundary.
    # A populated map takes precedence, even when this base has no entry.
    target=$(jq -r --arg base "$base" '.hotfixes[$base] // "" | gsub("^\\s+|\\s+$"; "")' <<<"$json")
else
    target=$legacy
fi
[[ -n "$target" ]] || skip "No hotfix override for baked ANC $current"

[[ "$target" =~ $version_pattern ]] ||
    fail "Unrecognized hotfix target: $target"
target_major=${BASH_REMATCH[1]}
target_minor=${BASH_REMATCH[2]}
target_patch=${BASH_REMATCH[3]}

# ANC downloads only same-base, strictly-higher-patch hotfixes.
if (( 10#$major != 10#$target_major ||
      10#$minor != 10#$target_minor ||
      10#$target_patch <= 10#$patch )); then
    skip "Target $target is not a patch upgrade for baked ANC $current"
fi

[[ -x "$hotfix" ]] ||
    fail "Baked ANC $current requires $target, but $hotfix is missing or not executable"

actual=$("$hotfix" version) || fail "Cannot read hotfix ANC version (baked ANC=$current expected=$target)"
[[ "$actual" == "$target" ]] ||
    fail "Baked ANC=$current expected hotfix=$target actual hotfix=$actual"

printf 'PASS: baked ANC=%s hotfix ANC=%s\n' "$current" "$actual"
