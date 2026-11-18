"""Render the complete, private 21-capture matrix as a standalone HTML report."""

import argparse
import collections
import hashlib
import html
import json
from pathlib import Path
import re
import statistics
import sys
import xml.etree.ElementTree as ET
import zipfile

from milestones import import_cse
from prepare_matrix import IMAGES, VARIANTS, scenario_names
from report import write_html
from timeline import analyze, number, utc_seconds


SCENARIO = re.compile(r"(Ubuntu2604Minimal|AzureLinuxV3)_BootTrace_CA_(\w+)_([123])")
METRICS = (
    ("retrieve_seconds", "Retrieve CA (s)"), ("install_seconds", "Install trust store (s)"),
    ("ca_path_seconds", "CA path elapsed (s)"), ("ready_seconds", "Boot to Ready (s)"),
)
COMMON = (
    "vm_size", "kubernetes_version", "network_plugin", "network_policy",
    "network_dataplane", "cluster_resource_id", "provisioning",
    "cse_command_instrumentation",
)


def load(path):
    return json.loads(path.read_text())


def save(path, value):
    path.write_text(json.dumps(value, indent=2, allow_nan=False) + "\n")


def identity(name):
    match = SCENARIO.fullmatch(name)
    if not match or match[2] not in VARIANTS[match[1]]:
        raise ValueError(f"Unknown matrix scenario: {name}")
    return match[1], match[2], int(match[3])


def stats(values):
    values = [number(value) for value in values]
    return {"n": len(values), "min": min(values), "median": statistics.median(values),
            "max": max(values), "samples": values}


def validate(bundle, name, manifest, fixtures):
    os_name, variant, repeat = identity(name)
    metadata, ready = load(bundle / "metadata.json"), load(bundle / "readiness.json")
    experiment = load(bundle / "experiment.json")
    cfg = ready["configuration"]
    if load(bundle / "result.json").get("status") != "collected":
        raise ValueError(f"Incomplete capture: {name}")
    if metadata["status"] != "stopped" or not metadata.get("finalized_at"):
        raise ValueError(f"Capture was not finalized: {name}")
    if metadata["boot_id"] != ready["boot_id"] or metadata["backend"] != "ftrace":
        raise ValueError(f"Mismatched boot identity or backend: {name}")
    provenance = metadata["image_provenance"]
    if provenance["kernel"] != metadata["kernel"] or provenance["backend"] != "ftrace":
        raise ValueError(f"Mismatched baked provenance: {name}")
    if not all(cfg.get(key) for key in ("vm_size", "os_sku", "cluster_resource_id",
                                       "image_resource_id", "base_image_version")):
        raise ValueError(f"Missing comparison configuration: {name}")
    if provenance["base_version"] != cfg["base_image_version"]:
        raise ValueError(f"Mismatched base image version: {name}")
    retrieve = "parallel" if variant.startswith("parallel") else "original"
    mode = "rust" if "rust" in variant else (
        "skip-update-ca-certificates" if variant == "skip_ca" else "enabled")
    expected = {"variant": variant, "retrieve_mode": retrieve, "ca_update_mode": mode,
                "rust_sha256": manifest["rust_sha256"],
                "init_sha256": hashlib.sha256((fixtures / f"init-{retrieve}.sh").read_bytes()).hexdigest()}
    if any(experiment.get(key) != value for key, value in expected.items()):
        raise ValueError(f"Experiment inputs do not match requested variant: {name}")
    for key in ("retrieve_mode", "ca_update_mode", "init_sha256", "rust_sha256"):
        if cfg.get(key) != expected[key]:
            raise ValueError(f"Readiness input mismatch ({key}): {name}")
    if cfg.get("experiment_variant") != variant:
        raise ValueError(f"Readiness variant mismatch: {name}")
    if cfg["os_sku"] != ("Ubuntu" if os_name == "Ubuntu2604Minimal" else "AzureLinux"):
        raise ValueError(f"OS mismatch: {name}")
    if cfg["image_resource_id"].lower() != manifest["images"][IMAGES[os_name]]["resourceId"].lower():
        raise ValueError(f"Unexpected image: {name}")
    if not isinstance(experiment["certificate_count"], int) or experiment["certificate_count"] <= 0:
        raise ValueError(f"Missing downloaded certificates: {name}")
    for key in ("certificate_set_sha256", "bundle_sha256"):
        if not re.fullmatch(r"[a-f0-9]{64}", experiment[key]):
            raise ValueError(f"Invalid certificate evidence ({key}): {name}")
    by_name = collections.defaultdict(list)
    for event in (json.loads(line) for line in (bundle / "cloud-timings.jsonl").read_text().splitlines()):
        if event["exit_status"] != 0:
            raise ValueError(f"Failed CA task: {name}/{event['task']}")
        if number(event["start"]) < 0 or number(event["end"]) < number(event["start"]):
            raise ValueError(f"Invalid CA task interval: {name}")
        by_name[event["task"]].append(event)
    selected = []
    for task in ("retrieveCerts", "installCertsToTrustStore"):
        values = by_name["AKS.CSE.rcv1p." + task]
        if len(values) != 1:
            raise ValueError(f"Missing or duplicated {task}: {name}")
        selected.append(values[0])
    update = by_name["AKS.CSE.bootTrace.updateCaCertificates"]
    skipped = by_name["AKS.CSE.bootTrace.skippedUpdateCaCertificates"]
    wanted = (0, 0) if os_name == "AzureLinuxV3" else ((0, 1) if variant == "skip_ca" else (1, 0))
    if (len(update), len(skipped)) != wanted:
        raise ValueError(f"Unexpected updater evidence: {name}")
    retrieve_event, install = selected
    if install["start"] < retrieve_event["end"]:
        raise ValueError(f"CA stages unexpectedly overlap: {name}")
    return {"name": name, "os": os_name, "variant": variant, "repeat": repeat,
            "configuration": cfg, "experiment": experiment,
            "identities": {key: ready[key].lower() for key in ("boot_id", "node_uid", "provider_id")},
            "kernel": metadata["kernel"], "architecture": metadata["architecture"],
            "retrieve_seconds": retrieve_event["end"] - retrieve_event["start"],
            "install_seconds": install["end"] - install["start"],
            "ca_path_seconds": install["end"] - retrieve_event["start"]}


def analyze_trial(bundle, summary, destination):
    result = analyze(bundle)
    datapoints = bundle / "cse-boot-datapoints.json"
    if datapoints.is_file() and (points := load(datapoints)).get("BootDatapoints"):
        clock = result["metadata"]["clock_start"]
        origin = number(clock["realtime"]) - number(clock["monotonic"])
        for point in import_cse(points):
            timestamp = utc_seconds(point["timestamp"]) - origin
            if timestamp >= 0:
                result["milestones"].append(
                    {"name": point["name"], "t": timestamp, "source": point["source"]})
            else:
                result["warnings"].append(
                    f"CSE milestone {point['name']} predates mapped boot; excluded, not clamped.")
    else:
        result["warnings"].append("No sanitized CSE boot datapoints were collected.")
    if summary["variant"] == "skip_ca":
        result["warnings"].append("UNSAFE CONTROL: CA update skipped. Downloaded certificates were not activated.")
    result["warnings"].append(
        "Common pinned Rust fixture preparation precedes CA spans on all variants, "
        "but is included in boot-to-Ready. This is not a pristine image benchmark.")
    summary["ready_seconds"] = number(result["ready_seconds"])
    summary["warnings"] = result["warnings"]
    summary["event_counts"] = result["event_counts"]
    cpu = collections.defaultdict(float)
    for thread in result["threads"]:
        if thread["cpu_seconds"] is not None:
            cpu[thread["comm"]] += number(thread["cpu_seconds"])
    summary["thread_cpu_seconds"] = dict(cpu)
    destination.mkdir(mode=0o700, parents=True)
    save(destination / "report.json", result)
    save(destination / "summary.json", summary)
    write_html(destination / "report.html", [result], [summary["name"]])


def aggregate(trials):
    identities, common, per_os = set(), set(), {}
    grouped = collections.defaultdict(list)
    for trial in trials:
        for key, value in trial["identities"].items():
            if (key, value) in identities:
                raise ValueError(f"Repeated node/boot identity: {key}")
            identities.add((key, value))
        cfg = trial["configuration"]
        common.add(tuple(cfg.get(key) for key in COMMON))
        signature = tuple(cfg.get(key) for key in ("image_resource_id", "base_image_version")) + (
            trial["kernel"], trial["architecture"])
        if per_os.setdefault(trial["os"], signature) != signature:
            raise ValueError("Image, architecture or kernel differs within one OS")
        grouped[trial["os"], trial["variant"]].append(trial)
    if len(common) > 1:
        raise ValueError("Trials differ in VM, Kubernetes, networking, cluster or provisioning")
    groups = []
    for (os_name, variant), samples in sorted(grouped.items()):
        samples.sort(key=lambda trial: trial["repeat"])
        metrics = {key: stats([trial[key] for trial in samples]) for key, _ in METRICS}
        cpu_names = collections.Counter(name for trial in samples for name in trial["thread_cpu_seconds"])
        patterns = [{"name": name, **stats([trial["thread_cpu_seconds"][name] for trial in samples
                                            if name in trial["thread_cpu_seconds"]])}
                    for name, count in cpu_names.items() if count >= 2]
        patterns.sort(key=lambda value: value["median"], reverse=True)
        groups.append({"os": os_name, "variant": variant, "n": len(samples),
                       "metrics": metrics, "trials": [trial["name"] for trial in samples],
                       "certificate_sets": sorted({x["experiment"]["certificate_set_sha256"] for x in samples}),
                       "bundle_hashes": sorted({x["experiment"]["bundle_sha256"] for x in samples}),
                       "recurring_cpu": patterns[:20]})
    baselines = {group["os"]: group for group in groups if group["variant"] == "original"}
    for group in groups:
        baseline = baselines.get(group["os"])
        group["delta_from_original"] = {
            key: group["metrics"][key]["median"] - baseline["metrics"][key]["median"]
            for key, _ in METRICS
        } if baseline and baseline["n"] == 3 and group["n"] == 3 else None
    return groups


def render_index(output, run_id, trials, missing, outcomes):
    groups = aggregate(trials)
    ordered = sorted(trials, key=lambda trial: (
        trial["repeat"], trial["os"] != "Ubuntu2604Minimal",
        ("original", "parallel", "rust", "parallel_rust", "skip_ca").index(trial["variant"])))
    complete = not missing and len(outcomes) == 21 and all(case["status"] == "passed" for case in outcomes)
    report = {"run_id": run_id, "campaign_state": "completed" if complete else "failed",
              "concurrency": 7, "scheduling": "three-balanced-waves", "final": True,
              "valid_captures": len(trials), "expected_captures": 21,
              "missing_or_incomplete": missing, "groups": groups, "trials": ordered,
              "junit_outcomes": outcomes}
    fingerprints = {os_name: len({trial["experiment"]["certificate_set_sha256"]
                                 for trial in trials if trial["os"] == os_name})
                    for os_name in sorted({trial["os"] for trial in trials})}
    report["distinct_downloaded_certificate_sets_per_os"] = fingerprints
    save(output / "summary.json", report)
    esc = lambda value: html.escape(str(value), quote=True)
    fmt = lambda value: f"{value:.3f}" if value is not None else "unknown"
    rows = []
    for group in groups:
        cells = [esc(group["os"]), esc(group["variant"]), f'{group["n"]}/3']
        for key, _ in METRICS:
            value = group["metrics"][key]
            cells.append(f'{fmt(value["median"])} [{fmt(value["min"])} .. {fmt(value["max"])}]')
        deltas = group["delta_from_original"]
        cells.extend(fmt(deltas[key]) if deltas else "requires 3+3"
                     for key in ("ca_path_seconds", "ready_seconds"))
        rows.append("<tr>" + "".join(f"<td>{value}</td>" for value in cells) + "</tr>")
    trial_rows = []
    for index, trial in enumerate(ordered):
        values = [f'<a href="#timelines" data-capture="{index}">{esc(trial["name"])}</a>',
                  fmt(trial["retrieve_seconds"]), fmt(trial["install_seconds"]), fmt(trial["ready_seconds"]),
                  str(trial["experiment"]["certificate_count"]),
                  esc(trial["experiment"]["certificate_set_sha256"][:16]),
                  esc(trial["experiment"]["bundle_sha256"][:16])]
        trial_rows.append("<tr>" + "".join(f"<td>{value}</td>" for value in values) + "</tr>")
    patterns = []
    for group in groups:
        lines = "".join(
            f'<tr><td>{esc(value["name"])}</td><td>{value["n"]}/{group["n"]}</td>'
            f'<td>{fmt(value["median"])}</td><td>{fmt(value["min"])} .. {fmt(value["max"])}</td></tr>'
            for value in group["recurring_cpu"])
        patterns.append(f'<details><summary>{esc(group["os"])} / {esc(group["variant"])}</summary>'
                        '<table><tr><th>Thread name</th><th>Captures</th><th>Median CPU (s)</th>'
                        f'<th>Range (s)</th></tr>{lines}</table></details>')
    timeline_frame = "<p>No collected timelines.</p>"
    if ordered:
        write_html(output / "timelines.html",
                   [load(output / "trials" / trial["name"] / "report.json") for trial in ordered],
                   [trial["name"] for trial in ordered], show_comparison=False)
        timeline_frame = ('<iframe id="capture-timelines" title="All boot timelines" srcdoc="'
                          + esc((output / "timelines.html").read_text()) + '"></iframe>')
    vm_sizes = sorted({trial["configuration"]["vm_size"] for trial in trials})
    body = f"""<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; frame-src 'self'; base-uri 'none'">
<title>CA boot comparison - {esc(run_id)}</title>
<style>body{{font:15px system-ui;background:#0b1220;color:#e7edf7;margin:30px}}
a{{color:#59d9dc}}section,details{{background:#152036;padding:18px;margin:18px 0;border-radius:10px}}
table{{border-collapse:collapse;white-space:nowrap;width:100%}}td,th{{padding:10px;border-bottom:1px solid #293a55;text-align:left}}
.scroll{{overflow:auto}}.warning{{border-left:4px solid #efb668;padding-left:16px}}li{{margin:10px 0}}summary{{cursor:pointer}}
iframe{{display:block;box-sizing:border-box;width:100%;height:1000px;border:1px solid #293a55;border-radius:10px}}</style>
</head><body><h1>CA boot comparison</h1>
<p><b>{"COMPLETE" if complete else "INCOMPLETE / FAILED"}</b> - {len(trials)}/21 validated captures</p>
<p>E2E outcomes: {esc(json.dumps(dict(collections.Counter(case['status'] for case in outcomes))))}</p>
<p>Build {esc(run_id)} | VM sizes: {esc(", ".join(vm_sizes))} | concurrency 7 | three-balanced-waves</p>
<section class="warning"><h2>Interpretation limits</h2><ul>
<li>Three fresh VM boots per variant, not reboots. Comparisons are within one OS/image/kernel.
Values are descriptive medians [minimum .. maximum], not established causal or statistical gains.</li>
<li>The same managed-identity download and verification of the pinned Rust fixture runs on every
variant before CA spans. It IS included in boot-to-Ready. These are not pristine-image timings.</li>
<li>ftrace, disk-wait sampling, Bastion/SSH and the test ANC download add overhead.
Shared infrastructure and concurrency add variability. Ready uses kubelet LastTransitionTime;
cross-host clock error is unmeasured. Instrumented timestamps have 10 ms resolution.</li>
<li><b>skip_ca is an unsafe lower-bound control</b>: downloaded CAs are not activated.
Rust is selected only on Ubuntu; Azure Linux keeps its native update-ca-trust.</li>
<li>Delta = variant median minus same-OS original median. Negative means shorter.
Only complete 3/3 groups support the planned comparison.</li>
<li>Downloaded certificate set fingerprints per OS: {esc(json.dumps(fingerprints))}.
Different inputs confound comparisons; bundle byte hashes do not prove semantic trust-store equivalence.</li>
<li>Individual reports retain trace-loss, clock and coverage warnings. Recurring CPU values sum observed
scheduler time by thread name, not CA-specific attribution, process counts or critical-path savings.</li>
</ul></section>
<section><h2>Variant summary</h2><div class="scroll"><table><tr><th>OS</th><th>Variant</th><th>Captures</th>
{''.join('<th>'+esc(title)+'</th>' for _, title in METRICS)}
<th>CA delta (s)</th><th>Ready delta (s)</th></tr>{''.join(rows)}</table></div></section>
<section id="timelines"><h2>All capture timelines</h2>
<p>Use Capture to switch between all {len(trials)} boots, grouped by repetition 1, then 2, then 3.
Statistics stay grouped by identical OS and variant, never pooled across 21 boots.</p>
<p>All timeline data is embedded in this file; no separate page is required.</p>
<p>All threads, services and collected AKS.CSE tasks remain available.
CSE command start (cse_cmd.sh) is distinct from the later cse_start.sh entry.
Imported CSE BootDatapoints have whole-second resolution; systemd and instrumented spans retain their provenance.</p>
{timeline_frame}</section>
<section><h2>Individual ftrace reports</h2><div class="scroll"><table><tr><th>Capture</th><th>Retrieve (s)</th>
<th>Install (s)</th><th>Ready (s)</th><th>Downloaded CAs</th><th>CA set SHA prefix</th><th>Bundle SHA prefix</th>
</tr>{''.join(trial_rows)}</table></div></section>
<section><h2>Recurring CPU signals, grouped by identical variant</h2>{''.join(patterns)}</section>
<section><h2>Missing or incomplete captures</h2><pre>{esc(json.dumps(missing,indent=2))}</pre>
<h2>E2E outcomes</h2><pre>{esc(json.dumps(outcomes,indent=2))}</pre>
<p>Raw private provisioning logs and SSH keys are NOT included in this report.</p></section>
<script>
document.querySelectorAll("a[data-capture]").forEach(link=>{{
 link.addEventListener("click",()=>{{
  const frame=document.getElementById("capture-timelines");
  const selector=frame.contentDocument.getElementById("capture");
  selector.value=link.dataset.capture;
  selector.dispatchEvent(new frame.contentWindow.Event("change"));
 }});
}});
window.addEventListener("message",event=>{{
 const frame=document.getElementById("capture-timelines"),message=event.data;
 if(!frame||event.source!==frame.contentWindow||message?.type!=="aks-boot-report-height"
    ||!Number.isFinite(message.height)||message.height<=0)return;
 frame.style.height=(Math.ceil(message.height)+frame.offsetHeight-frame.clientHeight)+"px";
}});
</script></body></html>"""
    (output / "index.html").write_text(body)
    return report


def render(artifacts, output, fixtures, run_id):
    manifest = load(fixtures / "fixture.json")
    expected = scenario_names()
    if sorted(manifest["scenario_names"]) != sorted(expected):
        raise ValueError("Expected exactly the 21 CA matrix scenarios")
    bundles = {}
    for marker in sorted(artifacts.glob("**/boot-trace/result.json")):
        name = marker.parent.parent.name
        identity(name)
        if name in bundles:
            raise ValueError(f"Duplicate capture artifact: {name}")
        bundles[name] = marker.parent
    output.mkdir(parents=True, mode=0o700, exist_ok=False)
    trials, missing = [], []
    for name in expected:
        bundle = bundles.get(name)
        if bundle is None or load(bundle / "result.json").get("status") != "collected":
            missing.append(name)
            continue
        summary = validate(bundle, name, manifest, fixtures)
        print(f"Analyzing {name}", flush=True)
        analyze_trial(bundle, summary, output / "trials" / name)
        trials.append(summary)
    outcomes = []
    failed_children = set()
    junit = fixtures / "junit.xml"
    if junit.is_file():
        for case in ET.parse(junit).getroot().iter("testcase"):
            name = case.attrib.get("name")
            parent = (name or "").split("/", 1)[0]
            if parent in expected and name != parent:
                if case.find("failure") is not None or case.find("error") is not None:
                    failed_children.add(parent)
                continue
            if name not in expected and (skipped := case.find("skipped")) is not None and skipped.attrib.get("message", "").startswith("filtered:"):
                continue
            if name not in expected or any(row["name"] == name for row in outcomes):
                raise ValueError(f"Unexpected or duplicate JUnit scenario: {name}")
            status = ("failed" if case.find("failure") is not None or case.find("error") is not None
                      else "skipped" if case.find("skipped") is not None else "passed")
            outcomes.append({"name": name, "status": status})
    for outcome in outcomes:
        if outcome["name"] in failed_children:
            outcome["status"] = "failed"
    report = render_index(output, run_id, trials, missing, outcomes)
    with zipfile.ZipFile(output / "ca-comparison-report.zip", "w", zipfile.ZIP_DEFLATED) as archive:
        for path in sorted(output.rglob("*")):
            if path.is_file() and path.suffix in (".html", ".json"):
                archive.write(path, path.relative_to(output))
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("artifacts", type=Path)
    parser.add_argument("--fixtures", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--run-id", required=True)
    args = parser.parse_args()
    try:
        result = render(args.artifacts, args.output, args.fixtures, args.run_id)
        if result["campaign_state"] != "completed":
            raise ValueError(f"Incomplete matrix: {result['valid_captures']}/21 captures; inspect index.html and JUnit outcomes")
    except (OSError, ValueError, KeyError, RuntimeError, ET.ParseError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
