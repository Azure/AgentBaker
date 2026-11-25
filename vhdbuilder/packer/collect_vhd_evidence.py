#!/usr/bin/env python3
"""Collect and validate machine-readable evidence for a Linux VHD build."""

import argparse
import datetime
import json
import os
import platform
import re
import shlex
import subprocess
import tempfile
import urllib.parse
from pathlib import Path


SCHEMA_VERSION = "1.0.0"
COLLECTOR_VERSION = "1.0.0"
PACKAGE_MANAGERS = {
    "ubuntu": "dpkg",
    "debian": "dpkg",
    "azurelinux": "rpm",
    "mariner": "rpm",
}


def read_os_release(path):
    values = {}
    with open(path, encoding="utf-8") as release_file:
        for line in release_file:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, value = line.split("=", 1)
            values[key] = shlex.split(value)[0] if value else ""
    return values


def normalize_architecture(machine):
    return {
        "x86_64": "amd64",
        "aarch64": "arm64",
    }.get(machine, machine)


def run_package_query(package_manager, runner=subprocess.run):
    if package_manager == "dpkg":
        query_format = "${db:Status-Status}\t${Package}\t${Version}\t${Architecture}\n"
        command = ["dpkg-query", "-W", "-f=" + query_format]
    elif package_manager == "rpm":
        query_format = "%{NAME}\t%{EPOCHNUM}\t%{VERSION}\t%{RELEASE}\t%{ARCH}\n"
        command = ["rpm", "-qa", "--qf", query_format]
    else:
        raise ValueError("unsupported package manager: " + package_manager)

    result = runner(command, check=True, capture_output=True, text=True, timeout=120)
    packages = []
    for line in result.stdout.splitlines():
        fields = line.split("\t")
        if package_manager == "dpkg":
            if len(fields) != 4 or fields[0] != "installed":
                continue
            _, name, version, architecture = fields
        else:
            if len(fields) != 5:
                raise ValueError("invalid rpm package query row")
            name, epoch, version, release, architecture = fields
            if epoch not in ("", "(none)", "0"):
                version = epoch + ":" + version
            version += "-" + release
        if not name or not version or not architecture:
            raise ValueError("package query returned an incomplete package record")
        packages.append(
            {
                "name": name,
                "version": version,
                "architecture": architecture,
                "package_manager": package_manager,
            }
        )
    return sorted(
        packages,
        key=lambda package: (
            package["name"],
            package["architecture"],
            package["version"],
        ),
    )


def collect_evidence(environment=os.environ, os_release_path="/etc/os-release", runner=subprocess.run):
    os_release = read_os_release(os_release_path)
    distribution = os_release.get("ID", "").lower()
    package_manager = PACKAGE_MANAGERS.get(distribution)
    if not package_manager:
        raise ValueError("unsupported Linux distribution: " + distribution)

    packages = run_package_query(package_manager, runner)
    image_architecture = environment.get("ARCHITECTURE") or normalize_architecture(platform.machine())
    return {
        "schema_version": SCHEMA_VERSION,
        "producer": {
            "name": "AgentBaker",
            "agentbaker_commit": environment.get("AGENTBAKER_COMMIT", ""),
            "ado_build_id": environment.get("ADO_BUILD_ID", ""),
            "ado_build_number": environment.get("ADO_BUILD_NUMBER", ""),
            "collector_version": COLLECTOR_VERSION,
        },
        "image": {
            "version": environment.get("IMAGE_VERSION", ""),
            "sku": environment.get("SKU_NAME", ""),
            "distribution": distribution,
            "distribution_version": os_release.get("VERSION_ID", ""),
            "architecture": image_architecture,
            "hyperv_generation": environment.get("HYPERV_GENERATION", ""),
            "feature_flags": environment.get("FEATURE_FLAGS", ""),
        },
        "collection": {
            "captured_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "stage": "after_deprovision",
            "status": "partial",
        },
        "os_packages": packages,
        "source_artifacts": [],
        "components": [],
        "container_images": [],
        "coverage": {
            "os_packages": "complete",
            "source_artifacts": "not_collected",
            "components": "not_collected",
            "container_images": "not_collected",
        },
    }


def validate_evidence(evidence):
    required = {
        "schema_version",
        "producer",
        "image",
        "collection",
        "os_packages",
        "source_artifacts",
        "components",
        "container_images",
        "coverage",
    }
    if not isinstance(evidence, dict) or set(evidence) != required:
        raise ValueError("evidence must contain exactly the required top-level fields")
    if evidence["schema_version"] != SCHEMA_VERSION:
        raise ValueError("unsupported evidence schema version")
    for section in ("producer", "image", "collection", "coverage"):
        if not isinstance(evidence[section], dict):
            raise ValueError(section + " must be an object")
    for section in ("os_packages", "source_artifacts", "components", "container_images"):
        if not isinstance(evidence[section], list):
            raise ValueError(section + " must be an array")

    producer_fields = {
        "name",
        "agentbaker_commit",
        "ado_build_id",
        "ado_build_number",
        "collector_version",
    }
    image_fields = {
        "version",
        "sku",
        "distribution",
        "distribution_version",
        "architecture",
        "hyperv_generation",
        "feature_flags",
    }
    collection_fields = {"captured_at", "stage", "status"}
    for section, fields in (
        ("producer", producer_fields),
        ("image", image_fields),
        ("collection", collection_fields),
    ):
        value = evidence[section]
        if set(value) != fields or any(not isinstance(value[field], str) for field in fields):
            raise ValueError(section + " is missing required string fields")
    if evidence["producer"]["name"] != "AgentBaker":
        raise ValueError("unexpected evidence producer")
    if evidence["collection"]["stage"] != "after_deprovision":
        raise ValueError("evidence was not collected after deprovision")
    if evidence["collection"]["status"] not in {"complete", "partial", "failed"}:
        raise ValueError("invalid collection status")
    try:
        captured_at = datetime.datetime.fromisoformat(
            evidence["collection"]["captured_at"].replace("Z", "+00:00")
        )
    except ValueError as error:
        raise ValueError("invalid collection timestamp") from error
    if captured_at.tzinfo is None:
        raise ValueError("collection timestamp must include a timezone")

    package_fields = {"name", "version", "architecture", "package_manager"}
    package_keys = set()
    for package in evidence["os_packages"]:
        if not isinstance(package, dict) or set(package) != package_fields:
            raise ValueError("invalid OS package record")
        if not all(isinstance(package[field], str) and package[field] for field in package_fields):
            raise ValueError("OS package fields must be non-empty strings")
        if package["package_manager"] not in {"dpkg", "rpm"}:
            raise ValueError("invalid OS package manager")
        package_key = (
            package["package_manager"],
            package["name"],
            package["version"],
            package["architecture"],
        )
        if package_key in package_keys:
            raise ValueError("duplicate OS package identity")
        package_keys.add(package_key)

    artifact_ids = set()
    for artifact in evidence["source_artifacts"]:
        artifact_id = artifact.get("id") if isinstance(artifact, dict) else None
        if not artifact_id or artifact_id in artifact_ids:
            raise ValueError("source artifact IDs must be present and unique")
        if not set(artifact).issubset(
            {"id", "kind", "format", "source_uri", "digest", "package"}
        ):
            raise ValueError("source artifact has unknown fields")
        if not all(
            isinstance(artifact.get(field), str) and artifact[field]
            for field in ("kind", "format")
        ):
            raise ValueError("source artifact kind and format are required")
        if artifact["kind"] not in {
            "os_package",
            "archive",
            "binary",
            "container_image",
        }:
            raise ValueError("invalid source artifact kind")
        digest = artifact.get("digest")
        if (
            not isinstance(digest, dict)
            or digest.get("algorithm") != "sha256"
            or not isinstance(digest.get("value"), str)
            or not re.fullmatch(r"[a-f0-9]{64}", digest["value"])
        ):
            raise ValueError("source artifact requires a valid SHA-256 digest")
        source_uri = artifact.get("source_uri")
        if source_uri is not None:
            if not isinstance(source_uri, str):
                raise ValueError("source URI must be a string")
            parsed_uri = urllib.parse.urlsplit(source_uri)
            if parsed_uri.username or parsed_uri.password or parsed_uri.query:
                raise ValueError("source URI must not contain credentials or query parameters")
        package_identity = artifact.get("package")
        if package_identity is not None and (
            not isinstance(package_identity, dict)
            or set(package_identity) != {"name", "version", "architecture"}
            or not all(
                isinstance(package_identity[field], str) and package_identity[field]
                for field in ("name", "version", "architecture")
            )
        ):
            raise ValueError("invalid source artifact package identity")
        artifact_ids.add(artifact_id)

    component_ids = set()
    for component in evidence["components"]:
        component_id = component.get("id") if isinstance(component, dict) else None
        if not component_id or component_id in component_ids:
            raise ValueError("component IDs must be present and unique")
        if set(component) != {
            "id",
            "name",
            "requested_version",
            "artifact_ids",
            "locations",
        }:
            raise ValueError("component has unknown or missing fields")
        if not all(
            isinstance(component.get(field), str) and component[field]
            for field in ("name", "requested_version")
        ):
            raise ValueError("component name and requested version are required")
        component_ids.add(component_id)
        if not isinstance(component.get("artifact_ids"), list):
            raise ValueError("component artifact_ids must be an array")
        if len(set(component["artifact_ids"])) != len(component["artifact_ids"]):
            raise ValueError("component artifact_ids must be unique")
        if not set(component["artifact_ids"]).issubset(artifact_ids):
            raise ValueError("component references an unknown source artifact")
        if not isinstance(component.get("locations"), list):
            raise ValueError("component locations must be an array")
        for location in component["locations"]:
            if not isinstance(location, dict) or not all(
                isinstance(location.get(field), str) and location[field]
                for field in ("path", "role", "presence")
            ):
                raise ValueError("invalid component location")
            if not set(location).issubset(
                {"path", "role", "presence", "source_artifact_id"}
            ):
                raise ValueError("component location has unknown fields")
            if location["role"] not in {
                "installed_file",
                "cached_package",
                "extracted_payload",
            } or location["presence"] not in {"present", "removed", "unknown"}:
                raise ValueError("invalid component location role or presence")
            source_artifact_id = location.get("source_artifact_id")
            if source_artifact_id and source_artifact_id not in artifact_ids:
                raise ValueError("component location references an unknown source artifact")

    image_ids = set()
    for image in evidence["container_images"]:
        image_id = image.get("id") if isinstance(image, dict) else None
        if not image_id or image_id in image_ids:
            raise ValueError("container image IDs must be present and unique")
        if not set(image).issubset(
            {"id", "reference", "digest", "source_artifact_id"}
        ):
            raise ValueError("container image has unknown fields")
        if not isinstance(image.get("reference"), str) or not image["reference"]:
            raise ValueError("container image reference is required")
        digest = image.get("digest")
        if (
            not isinstance(digest, dict)
            or digest.get("algorithm") != "sha256"
            or not isinstance(digest.get("value"), str)
            or not re.fullmatch(r"[a-f0-9]{64}", digest["value"])
        ):
            raise ValueError("container image requires a valid SHA-256 digest")
        source_artifact_id = image.get("source_artifact_id")
        if source_artifact_id and source_artifact_id not in artifact_ids:
            raise ValueError("container image references an unknown source artifact")
        image_ids.add(image_id)

    coverage_sections = {
        "os_packages",
        "source_artifacts",
        "components",
        "container_images",
    }
    statuses = {"complete", "partial", "not_collected", "unsupported", "failed"}
    if set(evidence["coverage"]) != coverage_sections:
        raise ValueError("coverage must describe each evidence section")
    if any(evidence["coverage"][section] not in statuses for section in coverage_sections):
        raise ValueError("invalid coverage status")
    if evidence["coverage"]["os_packages"] == "complete" and not evidence["os_packages"]:
        raise ValueError("complete OS package inventory cannot be empty")
    if (
        evidence["collection"]["status"] == "complete"
        and any(evidence["coverage"][section] != "complete" for section in coverage_sections)
    ):
        raise ValueError("complete collection status conflicts with incomplete coverage")


def write_evidence(evidence, output_path):
    validate_evidence(evidence)
    output = Path(output_path)
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary_path = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w", encoding="utf-8", dir=str(output.parent), delete=False
        ) as temporary:
            temporary_path = temporary.name
            json.dump(evidence, temporary, indent=2, sort_keys=True)
            temporary.write("\n")
        os.replace(temporary_path, output)
        output.chmod(0o644)
    finally:
        if temporary_path and os.path.exists(temporary_path):
            os.unlink(temporary_path)


def validate_file(path):
    with open(path, encoding="utf-8") as evidence_file:
        validate_evidence(json.load(evidence_file))


def main():
    parser = argparse.ArgumentParser()
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--output", help="collect evidence and write it to this path")
    mode.add_argument("--validate", help="validate an evidence JSON file")
    args = parser.parse_args()

    if args.validate:
        validate_file(args.validate)
        return
    write_evidence(collect_evidence(), args.output)


if __name__ == "__main__":
    main()
