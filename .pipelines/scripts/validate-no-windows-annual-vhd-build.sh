#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

python3 - "$repo_root" <<'PY'
import json
import pathlib
import sys

repo_root = pathlib.Path(sys.argv[1])
settings_path = repo_root / "vhdbuilder/packer/windows/windows_settings.json"
components_path = repo_root / "parts/common/components.json"

settings = json.loads(settings_path.read_text())
components = json.loads(components_path.read_text())

errors = []

base_versions = settings.get("WindowsBaseVersions", {})
for sku, version in base_versions.items():
    fields = [sku, version.get("base_image_sku", ""), version.get("windows_image_name", "")]
    if any("23h2" in field.lower() for field in fields):
        errors.append(f"{settings_path}: WindowsBaseVersions still contains WS23H2 entry {sku!r}")

for key in settings.get("WindowsRegistryKeys", []):
    sku_match = key.get("WindowsSkuMatch", "")
    if "23h2" in sku_match.lower():
        errors.append(f"{settings_path}: WindowsRegistryKeys still contains WS23H2 match {sku_match!r}")

def walk(obj, path):
    if isinstance(obj, dict):
        for key, value in obj.items():
            current = f"{path}.{key}" if path else str(key)
            if key == "windowsSkuMatch" and "23h2" in str(value).lower():
                errors.append(f"{components_path}: {current} still contains WS23H2 match {value!r}")
            walk(value, current)
    elif isinstance(obj, list):
        for index, value in enumerate(obj):
            walk(value, f"{path}[{index}]")

walk(components, "")

pipeline_paths = [
    repo_root / ".pipelines/.vsts-vhd-builder-pr-windows.yaml",
    repo_root / ".pipelines/.vsts-vhd-builder-release-windows.yaml",
    repo_root / ".pipelines/templates/.build-and-test-windows-vhds-template.yaml",
]
for pipeline_path in pipeline_paths:
    text = pipeline_path.read_text()
    for token in ("build23H2", "23H2", "WINDOWS_23H2", "windows-23H2"):
        if token.lower() in text.lower():
            errors.append(f"{pipeline_path}: still contains WS23H2 pipeline token {token!r}")
            break

if errors:
    print("\n".join(errors), file=sys.stderr)
    sys.exit(1)
PY
