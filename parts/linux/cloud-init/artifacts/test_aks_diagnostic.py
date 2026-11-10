"""Compare legacy invocations with pinned main; test the new token mode separately.

Both scripts run their full __main__, with file I/O, HTTP uploads and sleeps mocked.
Requests, file reads, exits, retries and console output must match. Only the exact
new positional-token warning is allowed to differ. This is not a live node test.

Run from the repo root:
    git fetch --no-tags --depth=1 origin 6994f3ccd1991affbc7bf381397d0b604031b61b
    python3 -m pip install -r .github/requirements/aks-diagnostic-tests.txt
    python3 -m pytest parts/linux/cloud-init/artifacts/test_aks_diagnostic.py -v
"""
import io
import os
import runpy
import string
import subprocess
import sys
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path
from unittest.mock import Mock, mock_open, patch
from urllib.parse import quote

import pytest
import requests
from hypothesis import given, settings, strategies as st

SCRIPT = Path(__file__).with_name("aks-diagnostic.py")
REPO_ROOT = Path(__file__).resolve().parents[4]
BASELINE_COMMIT = "6994f3ccd1991affbc7bf381397d0b604031b61b"  # main, 2026-11-09
SCRIPT_PATH = "parts/linux/cloud-init/artifacts/aks-diagnostic.py"
ARGS = ["mycontainer", "https://mystorageaccount.blob.core.windows.net"]
TOKEN = "sv=2020-08-04&sp=w&sig=fake_signature"
TOKEN_ENV = "AKS_DIAGNOSTIC_SAS_TOKEN"
WARNING = (
    "Warning: SAS token was passed as a CLI argument; it is visible to other local "
    "processes via /proc/<pid>/cmdline. Prefer the AKS_DIAGNOSTIC_SAS_TOKEN environment "
    "variable instead.\n"
)
RESPONSES = [(201,), (403,), (503, 201), (503, 503, 503), ("timeout",) * 3]


@pytest.fixture(scope="session")
def baseline_script(tmp_path_factory):
    try:
        source = subprocess.check_output(
            ["git", "show", f"{BASELINE_COMMIT}:{SCRIPT_PATH}"], cwd=REPO_ROOT,
        )
    except subprocess.CalledProcessError:
        pytest.fail(f"Fetch the baseline first: git fetch --no-tags origin {BASELINE_COMMIT}")
    path = tmp_path_factory.mktemp("baseline") / "aks-diagnostic.py"
    path.write_bytes(source)
    return path


def run_script(script, args, payload=b"archive", responses=(201,), env=None):
    """Capture observable behavior without contacting Azure or touching host logs."""
    uploads = []
    outcomes = iter(responses)
    stdout, stderr = io.StringIO(), io.StringIO()

    def upload(url, *, headers, data, timeout):
        uploads.append((url, headers, data.read(), timeout))
        status = next(outcomes)
        if status == "timeout":
            raise requests.exceptions.Timeout()
        return Mock(status_code=status, reason="test response", text="")

    with (
        patch.object(sys, "argv", ["aks-diagnostic.py", *args]),
        patch.dict(os.environ, env or {}, clear=True),
        patch("os.path.isfile", return_value=True),
        patch("os.path.getsize", return_value=len(payload)),
        patch("builtins.open", mock_open(read_data=payload)) as files,
        patch("requests.put", side_effect=upload),
        patch("time.sleep") as sleep,
        redirect_stdout(stdout), redirect_stderr(stderr),
        pytest.raises(SystemExit) as exit_info,
    ):
        runpy.run_path(str(script), run_name="__main__")
    return {
        "exit": exit_info.value.code,
        "stdout": stdout.getvalue(),
        "stderr": stderr.getvalue(),
        "uploads": uploads,
        "files": files.call_args_list,
        "sleeps": sleep.call_args_list,
    }


def assert_legacy_matches_main(baseline_script, args, payload=b"archive", responses=(201,)):
    before = run_script(baseline_script, args, payload, responses)
    after = run_script(SCRIPT, args, payload, responses)
    assert before.pop("stderr") == ""
    assert after.pop("stderr") == WARNING
    assert after == before


@pytest.mark.parametrize("file_path", [None, "/custom/packet capture.zip"])
@pytest.mark.parametrize("responses", RESPONSES)
def test_rp_invocations_match_main(baseline_script, file_path, responses):
    args = [*ARGS, TOKEN] + ([file_path] if file_path else [])
    assert_legacy_matches_main(baseline_script, args, responses=responses)


@settings(max_examples=200, deadline=None)
@given(
    container=st.text(string.ascii_lowercase + string.digits, min_size=3, max_size=30),
    trailing_slash=st.booleans(),
    signature=st.text(string.ascii_letters + string.digits + "+/=", min_size=8, max_size=80),
    file_path=st.one_of(
        st.none(),
        st.text(string.ascii_letters + string.digits + " _-", min_size=1, max_size=40)
        .map(lambda name: f"/custom/{name}.zip"),
    ),
    payload=st.binary(min_size=1, max_size=512),
    responses=st.sampled_from(RESPONSES),
)
def test_generated_legacy_invocations_match_main(
    baseline_script, container, trailing_slash, signature, file_path, payload, responses,
):
    url = ARGS[1] + ("/" if trailing_slash else "")
    token = f"sv=2020-08-04&sp=w&sig={quote(signature, safe='')}"
    args = [container, url, token] + ([file_path] if file_path else [])
    assert_legacy_matches_main(baseline_script, args, payload, responses)


@pytest.mark.parametrize("file_path", [None, "/custom/packet capture.zip"])
@pytest.mark.parametrize("positional_token", [None, "sig=unused_positional_token"])
def test_env_token_matches_legacy_upload(baseline_script, file_path, positional_token):
    legacy_args = [*ARGS, TOKEN] + ([file_path] if file_path else [])
    env_args = [*ARGS] + ([positional_token] if positional_token else [])
    if file_path:
        env_args += ["--file-path", file_path]
    before = run_script(baseline_script, legacy_args)
    after = run_script(SCRIPT, env_args, env={TOKEN_ENV: TOKEN})
    assert before["exit"] == 0 and len(before["uploads"]) == 1
    assert after == before


def test_missing_token_fails_without_upload():
    result = run_script(SCRIPT, ARGS)
    assert result["exit"] == 1
    assert result["uploads"] == []
