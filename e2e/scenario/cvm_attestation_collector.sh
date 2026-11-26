#!/usr/bin/env bash
set -euo pipefail
if ! command -v timeout >/dev/null; then
  echo '[cvm-collector] phase=prerequisites status=failed: GNU timeout is required' >&2
  exit 1
fi

sudo timeout --verbose --signal=TERM --kill-after=10s 480s \
  bash -s -- "$(id -u):$(id -g)" <<'CVM_COLLECTOR'
set -euo pipefail
umask 077
phase=prerequisites
trap 'printf "[cvm-collector] phase=%s status=failed exit_code=%s elapsed_seconds=%s\n" "$phase" "$?" "$SECONDS" >&2' ERR
trap 'printf "[cvm-collector] phase=%s status=timed_out elapsed_seconds=%s\n" "$phase" "$SECONDS" >&2; exit 124' TERM
mark_phase() {
  phase="$1"
  printf '[cvm-collector] phase=%s status=started elapsed_seconds=%s\n' "$phase" "$SECONDS" >&2
}

work_dir="@@REMOTE_DIR@@"
repo_dir="${work_dir}/repo"
deps_dir="${work_dir}/deps"
token_path="${work_dir}/token.jwt"
token_owner="$1"
mkdir -p "${work_dir}"
chown -- "${token_owner}" "${work_dir}"

mark_phase prerequisites
if ! command -v git >/dev/null || ! command -v python3 >/dev/null ||
   ! python3 -m pip --version >/dev/null 2>&1 || ! command -v tpm2_pcrread >/dev/null; then
  if command -v apt-get >/dev/null; then
    env DEBIAN_FRONTEND=noninteractive apt-get update -qq
    env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq git python3 python3-pip tpm2-tools ca-certificates coreutils
  elif command -v dnf >/dev/null; then
    dnf install -y -q git python3 python3-pip tpm2-tools ca-certificates coreutils
  elif command -v tdnf >/dev/null; then
    tdnf install -y -q git python3 python3-pip tpm2-tools ca-certificates coreutils
  else
    echo "no supported package manager found" >&2
    exit 1
  fi
fi

mark_phase tools-download
git init --quiet "${repo_dir}"
git -C "${repo_dir}" remote add origin https://github.com/Azure/cvm-attestation-tools.git
git -C "${repo_dir}" fetch --quiet --depth 1 origin "@@COMMIT@@"
git -C "${repo_dir}" checkout --quiet --detach FETCH_HEAD
mark_phase python-dependencies
python3 -m pip install --quiet --disable-pip-version-check --target "${deps_dir}" \
  'requests==2.31.0; python_version < "3.10"' \
  'requests==2.34.2; python_version >= "3.10"' \
  'PyJWT==2.8.0; python_version < "3.9"' \
  'PyJWT==2.15.1; python_version >= "3.9"' \
  'cryptography==42.0.8; python_version < "3.9"' \
  'cryptography==50.0.2; python_version >= "3.9"' \
  construct==2.10.70

mark_phase attestation
# Keep Python in the outer timeout's process group so its deadline kills the entire collector.
timeout --foreground --verbose --signal=TERM --kill-after=10s 180s env \
  PYTHONPATH="${deps_dir}:${repo_dir}/cvm-attestation" \
  python3 - "@@PLATFORM@@" "@@NONCE@@" "${token_path}" "@@ENDPOINT@@" <<'PY'
@@PYTHON@@
PY

mark_phase token-ownership
chown -- "${token_owner}" "${token_path}"
printf 'attestation collector completed for %s\n' "@@PLATFORM@@"
printf '[cvm-collector] phase=complete status=passed elapsed_seconds=%s\n' "$SECONDS" >&2
CVM_COLLECTOR
