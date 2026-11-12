#!/usr/bin/env python3
"""Install WALinuxAgent from the Azure wireserver manifest.

Queries the wireserver to discover the manifest URL for WALinuxAgent,
then downloads the zip for the *specified* version and installs it under
/var/lib/waagent/WALinuxAgent-<version>/.

The target version is passed explicitly (from components.json) rather than
being discovered from the GAFamily block in the extensions config.

Usage:
    python3 install_walinuxagent.py <download_dir> <wireserver_url> <version>

Arguments:
    download_dir   Directory to store the downloaded zip for provenance tracking.
    wireserver_url Base URL of the Azure wireserver (e.g. http://168.63.129.16:80).
    version        Target WALinuxAgent version (e.g. 2.15.0.1) from components.json.

Exit codes:
    0  Success
    1  Fatal error (logged to stderr)

Note: SAS tokens in manifest/blob URLs are never logged. Only the base URL
(before the '?' query string) is printed on error.
"""

import os
import re
import shutil
import sys
import tempfile
import time
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
import zipfile
from html import unescape as html_unescape
from typing import Optional

# Retry configuration for wireserver requests
MAX_RETRIES = 10
RETRY_WAIT_SECONDS = 5
REQUEST_TIMEOUT_SECONDS = 60

# Wireserver request headers (mimic WALinuxAgent)
WIRESERVER_HEADERS = {
    "x-ms-agent-name": "WALinuxAgent",
    "x-ms-version": "2012-11-30",
}


def strip_sas_token(url: str) -> str:
    """Return the URL without query parameters to avoid leaking SAS tokens."""
    return url.split("?")[0]


def sanitize_error(err: Exception) -> str:
    """Return a string representation of an exception with any embedded URLs stripped of query strings.
    """
    msg = str(err)
    # Strip query strings from any https:// or http:// URLs embedded in the message.
    return re.sub(r'(https?://[^\s"\'<>]*?)\?[^\s"\'>]*', r'\1?<redacted>', msg)


def fetch_url(url: str, headers: Optional[dict] = None, silent: bool = False) -> str:
    """Fetch a URL with retry logic. Returns the response body as a string.

    Args:
        url: The URL to fetch.
        headers: Optional HTTP headers to include.
        silent: If True, suppress URL from error messages (for SAS token safety).

    Raises:
        RuntimeError: If all retries are exhausted.
    """
    safe_url = strip_sas_token(url) if silent else url
    req = urllib.request.Request(url, headers=headers or {})

    last_error = None
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            with urllib.request.urlopen(req, timeout=REQUEST_TIMEOUT_SECONDS) as resp:
                return resp.read().decode("utf-8")
        except Exception as exc:
            last_error = exc
            if attempt < MAX_RETRIES:
                time.sleep(RETRY_WAIT_SECONDS)

    raise RuntimeError(
        f"Failed to fetch {safe_url} after {MAX_RETRIES} attempts: {sanitize_error(last_error)}"
    )


def download_file(url: str, dest_path: str, silent: bool = False) -> None:
    """Download a URL to a local file with retry logic.

    Args:
        url: The URL to download.
        dest_path: Local file path to write to.
        silent: If True, suppress URL from error messages (for SAS token safety).

    Raises:
        RuntimeError: If all retries are exhausted.
    """
    safe_url = strip_sas_token(url) if silent else url
    req = urllib.request.Request(url)

    last_error = None
    for attempt in range(1, MAX_RETRIES + 1):
        try:
            with urllib.request.urlopen(req, timeout=REQUEST_TIMEOUT_SECONDS) as resp:
                with open(dest_path, "wb") as f:
                    shutil.copyfileobj(resp, f)
            return
        except Exception as exc:
            last_error = exc
            if attempt < MAX_RETRIES:
                time.sleep(RETRY_WAIT_SECONDS)

    raise RuntimeError(
        f"Failed to download {safe_url} after {MAX_RETRIES} attempts: {sanitize_error(last_error)}"
    )


def extract_extensions_config_url(goalstate_xml: str) -> str:
    """Extract and decode the ExtensionsConfig URL from the goalstate XML.

    The URL may be URL-encoded and contain XML-escaped ampersands (&amp;).
    Both are handled here.
    """
    match = re.search(r"<ExtensionsConfig>([^<]+)</ExtensionsConfig>", goalstate_xml)
    if not match:
        raise RuntimeError("No <ExtensionsConfig> element found in goalstate")

    url = match.group(1).strip()
    url = urllib.parse.unquote(url)
    url = html_unescape(url)
    return url


def extract_ga_family_manifest_uri(extensions_config_xml: str) -> str:
    """Extract the GAFamily manifest URI from extensions config.

    Returns:
        The manifest URI string.
    """
    uri_match = re.search(
        r"<GAFamily>.*?<Uri>([^<]+)</Uri>",
        extensions_config_xml,
        re.DOTALL,
    )
    if not uri_match:
        raise RuntimeError("No GAFamily manifest URI found in extensions config")

    return html_unescape(uri_match.group(1).strip())


def find_zip_url_in_manifest(manifest_xml: str, target_version: str) -> str:
    """Parse the manifest XML and find the download URI for the target version."""
    root = ET.fromstring(manifest_xml)

    for plugin in root.findall(".//Plugin"):
        ver_elem = plugin.find("Version")
        if ver_elem is not None and ver_elem.text == target_version:
            uri_elem = plugin.find(".//Uri")
            if uri_elem is not None and uri_elem.text:
                return uri_elem.text

    raise RuntimeError(f"Version {target_version} not found in WALinuxAgent manifest")


def install_walinuxagent(download_dir: str, wireserver_url: str, version: str) -> None:
    """Main installation logic.

    1. Fetch goalstate from wireserver
    2. Extract ExtensionsConfig URL from goalstate
    3. Fetch extensions config
    4. Extract GAFamily manifest URI (version comes from components.json)
    5. Fetch the manifest
    6. Find the zip URL for the target version
    7. Download the zip
    8. Extract to /var/lib/waagent/WALinuxAgent-<version>/
    9. Copy zip to download_dir for provenance tracking
    """
    # Validate version is a safe string (e.g. "2.15.0.1") before using in paths.
    if not re.match(r"^[0-9]+(\.[0-9]+)*$", version):
        raise RuntimeError(f"Version contains unexpected characters: {version!r}")

    print(f"Installing WALinuxAgent {version} from wireserver manifest...")

    # Step 1: Fetch goalstate
    goalstate_url = f"{wireserver_url}/machine/?comp=goalstate"
    print(f"Fetching goalstate from {goalstate_url}")
    goalstate = fetch_url(goalstate_url, headers=WIRESERVER_HEADERS)

    # Step 2: Extract ExtensionsConfig URL
    extensions_config_url = extract_extensions_config_url(goalstate)

    # Step 3: Fetch extensions config (silent to avoid logging query params)
    print("Fetching extensions config...")
    extensions_config = fetch_url(extensions_config_url, headers=WIRESERVER_HEADERS, silent=True)

    # Step 4: Extract manifest URI from GAFamily block
    manifest_url = extract_ga_family_manifest_uri(extensions_config)

    print(f"Target version (from components.json): {version}")

    # Step 5: Fetch the manifest (silent to avoid logging SAS token)
    print(f"Fetching manifest from {strip_sas_token(manifest_url)}")
    manifest = fetch_url(manifest_url, silent=True)

    # Step 6: Find the zip URL
    zip_url = find_zip_url_in_manifest(manifest, version)
    print(f"Found WALinuxAgent {version} zip at: {strip_sas_token(zip_url)}")

    # Step 7: Download the zip to a temp directory
    tmp_dir = tempfile.mkdtemp()
    try:
        zip_path = os.path.join(tmp_dir, f"WALinuxAgent-{version}.zip")
        print(f"Downloading WALinuxAgent {version}...")
        download_file(zip_url, zip_path, silent=True)

        # Step 8: Extract to /var/lib/waagent/WALinuxAgent-<version>/
        install_dir = f"/var/lib/waagent/WALinuxAgent-{version}"
        os.makedirs(install_dir, exist_ok=True)

        with zipfile.ZipFile(zip_path, "r") as zf:
            # Guard against path traversal by verifying every entry resolves inside install_dir.
            resolved_base = os.path.realpath(install_dir)
            for member in zf.namelist():
                resolved = os.path.realpath(os.path.join(install_dir, member))
                if not resolved.startswith(resolved_base + os.sep) and resolved != resolved_base:
                    raise RuntimeError(
                        f"Zip entry {member!r} would extract outside {install_dir} (zip-slip)"
                    )
            zf.extractall(install_dir)

        print(f"WALinuxAgent {version} installed successfully to {install_dir}")

        # Step 9: Store zip for provenance tracking
        os.makedirs(download_dir, exist_ok=True)
        dest_zip = os.path.join(download_dir, f"WALinuxAgent-{version}.zip")
        shutil.copy2(zip_path, dest_zip)

    finally:
        shutil.rmtree(tmp_dir, ignore_errors=True)


def main() -> int:
    if len(sys.argv) != 4:
        print(f"Usage: {sys.argv[0]} <download_dir> <wireserver_url> <version>", file=sys.stderr)
        return 1

    download_dir = sys.argv[1]
    wireserver_url = sys.argv[2]
    version = sys.argv[3]

    try:
        install_walinuxagent(download_dir, wireserver_url, version)
    except Exception as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
