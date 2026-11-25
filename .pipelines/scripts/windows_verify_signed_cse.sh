#!/bin/bash
set -euo pipefail

# Verifies the Windows CSE scripts signed by aks-rp's windows-cse-sign pipeline against the unsigned scripts
# packaged from this commit. The signed tree must:
#   - come from this commit (provenance.json sourceVersion),
#   - contain exactly the same files,
#   - keep every non-PowerShell file byte-identical,
#   - keep every PowerShell file identical except for an appended Authenticode signature block that contains
#     only signature comment lines.
#
# Usage: windows_verify_signed_cse.sh <unsigned dir> <signed dir> <provenance.json> <expected commit>

if [ "$#" -ne 4 ]; then
  echo "Usage: $0 <unsigned dir> <signed dir> <provenance.json> <expected commit>"
  exit 1
fi

UNSIGNED_DIR="$1"
SIGNED_DIR="$2"
PROVENANCE_FILE="$3"
EXPECTED_COMMIT="$4"
BEGIN_MARKER="# SIG # Begin signature block"
END_MARKER="# SIG # End signature block"
PROBLEMS=0

problem() {
  echo "ERROR: $*"
  PROBLEMS=$((PROBLEMS + 1))
}

# Prints the size of a file, excluding trailing CR and LF bytes.
trimmed_size() {
  local file="$1" size last
  size=$(stat -c %s "$file")
  while [ "$size" -gt 0 ]; do
    last=$(tail -c +"$size" "$file" | head -c 1 | od -An -tx1 | tr -d ' \n')
    if [ "$last" != "0a" ] && [ "$last" != "0d" ]; then
      break
    fi
    size=$((size - 1))
  done
  echo "$size"
}

# Checks that a signed PowerShell file is the unsigned file plus a trailing signature block. The block may only
# hold its markers and "# <base64>" comment lines, so nothing in it can run.
verify_signed_script() {
  local unsigned="$1" signed="$2" name="$3" offset block bad_lines content unsigned_size content_size
  # Use the last marker, in case the script itself mentions the marker text.
  offset=$(grep -a -b -o -F "$BEGIN_MARKER" "$signed" | tail -n 1 | cut -d: -f1 || true)
  if [ -z "$offset" ]; then
    problem "$name has no signature block"
    return
  fi

  block=$(mktemp)
  tail -c +"$((offset + 1))" "$signed" | tr -d '\r' >"$block"
  bad_lines=$(LC_ALL=C grep -a -c -v -x -E -e "$BEGIN_MARKER|$END_MARKER|# [A-Za-z0-9+/=]+" -e '' "$block" || true)
  if [ "$bad_lines" -ne 0 ] || [ "$(grep -a -v -x -e '' "$block" | tail -n 1)" != "$END_MARKER" ]; then
    problem "$name has a malformed signature block (only signature comment lines are allowed)"
  fi
  rm -f "$block"

  content=$(mktemp)
  head -c "$offset" "$signed" >"$content"
  unsigned_size=$(trimmed_size "$unsigned")
  content_size=$(trimmed_size "$content")
  if [ "$unsigned_size" -ne "$content_size" ] || ! cmp -s <(head -c "$unsigned_size" "$unsigned") <(head -c "$content_size" "$content"); then
    problem "$name differs from this commit's script outside its signature block"
  fi
  rm -f "$content"
}

commit=$(jq -r '.sourceVersion // empty' "$PROVENANCE_FILE")
if [ "$commit" != "$EXPECTED_COMMIT" ]; then
  problem "signed scripts are from commit '${commit}', but this build is for commit '${EXPECTED_COMMIT}'"
fi

if ! diff <(cd "$UNSIGNED_DIR" && find . -type f | LC_ALL=C sort) <(cd "$SIGNED_DIR" && find . -type f | LC_ALL=C sort); then
  problem "the signed scripts contain different files than this commit's package (diff above)"
fi

while IFS= read -r -d '' file; do
  name="${file#./}"
  signed="$SIGNED_DIR/$name"
  if [ ! -f "$signed" ]; then
    continue # Reported by the file list comparison.
  fi
  case "${name,,}" in
    *.ps1 | *.psm1 | *.psd1) verify_signed_script "$UNSIGNED_DIR/$name" "$signed" "$name" ;;
    *) cmp -s "$UNSIGNED_DIR/$name" "$signed" || problem "$name differs from this commit's file" ;;
  esac
done < <(cd "$UNSIGNED_DIR" && find . -type f -print0)

if [ "$PROBLEMS" -ne 0 ]; then
  echo "Signed CSE scripts failed verification with $PROBLEMS problem(s)"
  exit 1
fi
echo "Signed CSE scripts match commit $EXPECTED_COMMIT"
