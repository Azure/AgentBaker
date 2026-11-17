"""Render collected AgentBaker bundles, without contacting Azure or Kubernetes."""

import argparse
import json
from pathlib import Path
import sys

from milestones import import_cse
from report import write_html
from timeline import analyze, number, utc_seconds


def select_trials(root):
    trials = []
    identities = set()
    configurations = []
    labels = set()
    ca_modes = {}
    for marker in sorted(root.glob("**/boot-trace/result.json")):
        if json.loads(marker.read_text()).get("status") != "collected":
            raise ValueError(f"Incomplete trial: {marker.parent}")
        bundle = marker.parent
        metadata = json.loads((bundle / "metadata.json").read_text())
        readiness = json.loads((bundle / "readiness.json").read_text())
        backend = metadata["backend"]
        scenario = bundle.parent.name
        prefix = "Ubuntu2604Minimal_BootTrace_"
        if backend != "ftrace" or not scenario.startswith(prefix):
            raise ValueError("Unexpected boot-trace backend or scenario")
        label = scenario.removeprefix(prefix)
        if label not in ("ftrace", "ftrace_2", "ftrace_3") or label in labels:
            raise ValueError("Expected exactly three ftrace trials")
        labels.add(label)
        if metadata["boot_id"] != readiness["boot_id"] or metadata["status"] != "stopped" or not metadata.get("finalized_at"):
            raise ValueError("Capture identity or finalization does not match readiness")
        for field in ("boot_id", "node_uid", "provider_id"):
            identity = (field, readiness[field].lower())
            if identity in identities:
                raise ValueError(f"Boot trace trials must use distinct new nodes: duplicate {field}")
            identities.add(identity)
        provenance = metadata["image_provenance"]
        if provenance["backend"] != backend or provenance["kernel"] != metadata["kernel"]:
            raise ValueError("Baked provenance does not match the captured kernel/backend")
        configuration = readiness["configuration"]
        ca_mode = configuration.get("ca_update_mode", "enabled")
        if ca_mode not in ("enabled", "skip-update-ca-certificates"):
            raise ValueError("Unknown CA update experiment mode")
        ca_modes[label] = ca_mode
        required = ("vm_size", "os_sku", "kubernetes_version", "network_plugin",
                    "canonical_base_version", "cluster_resource_id", "provisioning")
        if any(not configuration.get(key) for key in required):
            raise ValueError("Missing required comparison configuration")
        if provenance["base_version"] != configuration["canonical_base_version"]:
            raise ValueError("Canonical base provenance does not match readiness configuration")
        configurations.append((
            metadata["kernel"], metadata["architecture"],
            *(configuration[key] for key in required),
            configuration.get("network_policy"), configuration.get("network_dataplane"),
        ))
        trials.append((label, bundle))
    if labels != {"ftrace", "ftrace_2", "ftrace_3"}:
        raise ValueError("Exactly three successful ftrace trials are required")
    if any(configuration != configurations[0] for configuration in configurations[1:]):
        raise ValueError("Trials differ in base image, kernel, architecture, VM, cluster or provisioning configuration")
    if "skip-update-ca-certificates" in ca_modes.values():
        if ca_modes != {"ftrace": "enabled", "ftrace_2": "enabled",
                        "ftrace_3": "skip-update-ca-certificates"}:
            raise ValueError("CA experiment requires exactly two controls and only ftrace_3 skipping CA update")
    order = {"ftrace": 0, "ftrace_2": 1, "ftrace_3": 2}
    return sorted(trials, key=lambda trial: order[trial[0]])


def render(root, output):
    trials = select_trials(root)
    ca_experiment = any(
        json.loads((bundle / "readiness.json").read_text())["configuration"].get("ca_update_mode")
        == "skip-update-ca-certificates" for _, bundle in trials
    )
    output.mkdir(parents=True, mode=0o700, exist_ok=False)
    results = []
    labels = []
    for label, bundle in trials:
        result = analyze(bundle)
        ca_mode = result.get("readiness", {}).get("configuration", {}).get("ca_update_mode", "enabled")
        cloud_tasks = result.get("targeted_diagnostics", {}).get("cloud_timings", [])
        skipped = [task for task in cloud_tasks
                   if task["task"] == "AKS.CSE.bootTrace.skippedUpdateCaCertificates"]
        if ca_mode == "skip-update-ca-certificates":
            if len(skipped) != 1 or skipped[0]["exit_status"] != 0 or any(
                task["task"] == "AKS.CSE.bootTrace.updateCaCertificates" for task in cloud_tasks
            ):
                raise ValueError("CA skip experiment has no unambiguous guest-side skip evidence")
            result["warnings"].append(
                "EXPERIMENT: update-ca-certificates was skipped; downloaded CAs were not activated. "
                "Not a production-safe optimization or a repeat of the controls."
            )
        elif skipped:
            raise ValueError("Control unexpectedly skipped update-ca-certificates")
        elif ca_experiment:
            updates = [task for task in cloud_tasks
                       if task["task"] == "AKS.CSE.bootTrace.updateCaCertificates"]
            if len(updates) != 1 or updates[0]["exit_status"] != 0:
                raise ValueError("CA experiment control has no successful guest-side CA update evidence")
        datapoints = bundle / "cse-boot-datapoints.json"
        if datapoints.is_file():
            points = json.loads(datapoints.read_text())
            if points.get("BootDatapoints"):
                # Reuse the same importer as the standalone CLI, without saving CSE internals.
                milestones = import_cse(points)
                clock = result["metadata"]["clock_start"]
                origin = number(clock["realtime"]) - number(clock["monotonic"])
                for point in milestones:
                    t = utc_seconds(point["timestamp"]) - origin
                    if t < 0:
                        result["warnings"].append(
                            f'{point["name"]} maps {t:.6f}s before boot and was excluded, '
                            "not clamped. Check CSE timestamp resolution and clock mapping."
                        )
                        continue
                    result["milestones"].append({
                        "name": point["name"], "t": t, "source": point["source"],
                    })
            else:
                result["warnings"].append("CSE boot datapoints were unavailable after provisioning.")
        else:
            result["warnings"].append("No sanitized CSE boot datapoints were collected.")
        capture_output = output / label.replace("_", "-")
        capture_output.mkdir(mode=0o700)
        (capture_output / "report.json").write_text(json.dumps(result, indent=2, allow_nan=False) + "\n")
        display_label = f"ftrace {label.removeprefix('ftrace').lstrip('_') or '1'}"
        if "ca_update_mode" in result.get("readiness", {}).get("configuration", {}):
            display_label += " / " + ("SKIP update-ca (experiment)" if ca_mode != "enabled" else "update-ca enabled")
        write_html(capture_output / "report.html", [result], [display_label])
        results.append(result)
        labels.append(display_label)
    write_html(output / "comparison.html", results, labels)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("artifacts", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        render(args.artifacts, args.output)
    except (OSError, ValueError, KeyError, RuntimeError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
