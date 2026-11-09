"""Synthetic, offline tests. No Azure, SSH, tracing, installs or image builds."""

import argparse
import io
import json
from pathlib import Path
import shutil
import signal
import struct
import subprocess
import sys
import tarfile
import tempfile
import types
import unittest
from unittest.mock import Mock, patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from events import ftrace_events, lttng_events
from guest import capture, diagnostics
from milestones import import_cse
from orchestrate import provider, scale_and_collect, unpack
from report import write_html
from timeline import analyze
from trace_boot import main


BOOT = "11111111-1111-1111-1111-111111111111"
SUB = "22222222-2222-2222-2222-222222222222"
PROVIDER_ID = (
    f"azure:///subscriptions/{SUB}/resourceGroups/nodes/providers/"
    "Microsoft.Compute/virtualMachineScaleSets/aks-pool-vmss/virtualMachines/0"
)


def event(t, prev, nxt, state="S", cpu=0):
    return (
        f"    worker-{prev} [{cpu:03d}] d..2 {t:.6f}: sched_switch: "
        f"prev_comm=worker prev_pid={prev} prev_prio=120 prev_state={state} ==> "
        f"next_comm=worker next_pid={nxt} next_prio=120\n"
    )


class AnalysisTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.bundle = self.root / "capture"
        self.bundle.mkdir()
        self.metadata = {
            "schema": 1, "backend": "ftrace", "boot_id": BOOT, "kernel": "test",
            "architecture": "x86_64", "status": "stopped", "finalized_at": "test",
            "clock_start": {"realtime": 1010, "monotonic": 10, "sample_span_seconds": 0.001},
            "clock_end": {"realtime": 1012, "monotonic": 12, "sample_span_seconds": 0.001},
        }
        self.write("metadata.json", self.metadata)
        self.write("readiness.json", {
            "boot_id": BOOT, "transition_time": "1970-01-01T00:16:42Z",
        })
        (self.bundle / "ftrace.txt").write_text(
            event(1, 0, 42) + event(1.2, 42, 0, "R")
            + event(1.3, 0, 42) + event(1.5, 42, 0) + event(2.2, 0, 0)
        )

    def write(self, name, value):
        (self.bundle / name).write_text(json.dumps(value))

    def test_cpu_and_runnable_accounting(self):
        result = analyze(self.bundle)
        thread = next(row for row in result["threads"] if row["tid"] == 42)
        self.assertAlmostEqual(thread["cpu_seconds"], 0.4)
        self.assertAlmostEqual(thread["runnable_seconds"], 0.1)
        self.assertEqual(result["ready_seconds"], 2)

    def test_running_interval_is_clipped_to_ready_not_last_pre_ready_event(self):
        (self.bundle / "ftrace.txt").write_text(event(1, 0, 42) + event(2.2, 42, 0))
        thread = next(row for row in analyze(self.bundle)["threads"] if row["tid"] == 42)
        self.assertAlmostEqual(thread["cpu_seconds"], 1)

    def test_wrong_boot_rejected(self):
        self.write("readiness.json", {"boot_id": "other", "transition_time": "1970-01-01T00:16:42Z"})
        with self.assertRaisesRegex(ValueError, "BootID"):
            analyze(self.bundle)

    def test_missing_ready_is_unknown_not_zero(self):
        (self.bundle / "readiness.json").unlink()
        result = analyze(self.bundle)
        self.assertIsNone(result["ready_seconds"])
        self.assertTrue(any("readiness evidence" in item for item in result["warnings"]))

    def test_clock_jump_and_ring_loss_are_visible(self):
        self.metadata["clock_end"]["realtime"] += 5
        self.write("metadata.json", self.metadata)
        self.write("ftrace-stats.json", {"per_cpu/cpu0/stats": "overrun: 17\n"})
        warnings = "\n".join(analyze(self.bundle)["warnings"])
        self.assertIn("mapping changed", warnings)
        self.assertIn("Ring buffer loss", warnings)

    def test_targeted_timings_waits_and_unavailable_csr_are_retained(self):
        self.write("containerd-waits.json", {"waits": [{"tid": 42, "kernel_stack": "io_schedule"}]})
        self.write("bootstrap-csr.json", {"status": "unavailable", "error": "forbidden"})
        (self.bundle / "cloud-timings.jsonl").write_text(json.dumps({
            "task": "AKS.CSE.rcv1p.retrieveCerts", "start": 1.1, "end": 1.7, "exit_status": 1,
        }) + "\n")
        result = analyze(self.bundle)
        self.assertEqual(result["targeted_diagnostics"]["containerd_waits"]["waits"][0]["tid"], 42)
        service = next(item for item in result["services"] if item["name"].endswith("retrieveCerts"))
        self.assertAlmostEqual(service["end"] - service["start"], 0.6)
        self.assertTrue(any("CSR diagnostics are unavailable" in item for item in result["warnings"]))
        self.assertTrue(any("exited with status 1" in item for item in result["warnings"]))
        output = self.root / "diagnostics.html"
        write_html(output, [result], ["targeted"])
        self.assertIn("io_schedule", output.read_text())

    def test_invalid_cloud_timing_is_rejected(self):
        (self.bundle / "cloud-timings.jsonl").write_text(
            '{"task":"certs","start":2,"end":1,"exit_status":0}\n'
        )
        with self.assertRaisesRegex(ValueError, "Invalid certificate timing"):
            analyze(self.bundle)

    def test_out_of_order_stream_is_not_silently_accounted(self):
        (self.bundle / "ftrace.txt").write_text(event(1.2, 0, 42) + event(1.1, 42, 0))
        with self.assertRaisesRegex(ValueError, "chronologically merged"):
            analyze(self.bundle)

    def test_cross_cpu_reordering_is_merged_without_losing_cpu_intervals(self):
        (self.bundle / "ftrace.txt").write_text(
            event(1, 0, 42) + event(1.1, 0, 43, cpu=1)
            + event(1.8, 43, 0, cpu=1) + event(1.5, 42, 0)
            + event(2.2, 0, 0)
        )
        result = analyze(self.bundle)
        threads = {row["tid"]: row for row in result["threads"]}
        self.assertAlmostEqual(threads[42]["cpu_seconds"], 0.5)
        self.assertAlmostEqual(threads[43]["cpu_seconds"], 0.7)
        self.assertTrue(any("no events discarded" in warning for warning in result["warnings"]))

    def test_text_merge_preserves_input_order_at_equal_timestamps(self):
        path = self.bundle / "ftrace.txt"
        path.write_text(
            event(1, 0, 42) + event(1, 0, 43, cpu=1)
            + event(1, 42, 0) + event(0.9, 0, 44, cpu=2)
        )
        events = list(ftrace_events(path, []))
        self.assertEqual([row["t"] for row in events], [0.9, 1, 1, 1])
        self.assertEqual([row["cpu"] for row in events], [2, 0, 1, 0])

    def test_cross_cpu_reordering_with_gaps_is_rejected(self):
        for marker in ("CPU:0 [LOST 3 EVENTS]\n", "unexpected record\n"):
            with self.subTest(marker=marker):
                (self.bundle / "ftrace.txt").write_text(
                    event(1.2, 0, 42) + marker
                    + event(1.1, 0, 43, cpu=1) + event(2.2, 42, 0)
                )
                with self.assertRaisesRegex(ValueError, "lost or undecodable"):
                    analyze(self.bundle)

    def test_unknown_text_is_reported(self):
        with (self.bundle / "ftrace.txt").open("a") as source:
            source.write("unexpected record\n")
        self.assertTrue(any("could not be decoded" in item for item in analyze(self.bundle)["warnings"]))

    def test_semantic_services_do_not_become_cpu_time(self):
        self.write("services.json", [
            {
                "name": "active.service", "active_state": "active", "sub_state": "running",
                "ExecMainStartTimestampMonotonic": "100000",
                "ActiveEnterTimestampMonotonic": "200000",
                "InactiveEnterTimestampMonotonic": "0",
                "ExecMainExitTimestampMonotonic": "0",
            },
            {
                "name": "stopped.service", "active_state": "inactive", "sub_state": "dead",
                "ExecMainStartTimestampMonotonic": "300000",
                "ActiveEnterTimestampMonotonic": "400000",
                "InactiveEnterTimestampMonotonic": "800000",
                "ExecMainExitTimestampMonotonic": "750000",
            },
            {
                "name": "stopped-after-ready.service", "active_state": "inactive",
                "sub_state": "dead",
                "ExecMainStartTimestampMonotonic": "350000",
                "ActiveEnterTimestampMonotonic": "500000",
                "InactiveEnterTimestampMonotonic": "3000000",
                "ExecMainExitTimestampMonotonic": "3000000",
            },
            {
                "name": "unknown-stop.service", "active_state": "inactive", "sub_state": "dead",
                "ExecMainStartTimestampMonotonic": "600000",
                "ActiveEnterTimestampMonotonic": "700000",
                "InactiveEnterTimestampMonotonic": "0",
                "ExecMainExitTimestampMonotonic": "900000",
            },
            {
                "name": "oneshot.service", "active_state": "inactive", "sub_state": "dead",
                "type": "oneshot",
                "ExecMainStartTimestampMonotonic": "900000",
                "ActiveEnterTimestampMonotonic": "0",
                "InactiveEnterTimestampMonotonic": "0",
                "ExecMainExitTimestampMonotonic": "1000000",
            },
            {
                "name": "after-ready.service", "active_state": "active", "sub_state": "running",
                "ExecMainStartTimestampMonotonic": "2100000",
                "ActiveEnterTimestampMonotonic": "2200000",
                "InactiveEnterTimestampMonotonic": "0",
                "ExecMainExitTimestampMonotonic": "0",
            },
        ])
        result = analyze(self.bundle)
        services = {row["name"]: row for row in result["services"]}
        self.assertEqual(list(services), [
            "active.service", "stopped.service", "stopped-after-ready.service",
            "unknown-stop.service", "oneshot.service",
        ])
        self.assertAlmostEqual(services["active.service"]["active_start"], 0.2)
        self.assertAlmostEqual(services["active.service"]["active_end"], 2)
        self.assertTrue(services["active.service"]["active_at_cutoff"])
        self.assertAlmostEqual(services["stopped.service"]["active_end"], 0.8)
        self.assertFalse(services["stopped.service"]["active_at_cutoff"])
        self.assertAlmostEqual(services["stopped-after-ready.service"]["active_end"], 2)
        self.assertTrue(services["stopped-after-ready.service"]["active_at_cutoff"])
        self.assertIsNone(services["unknown-stop.service"]["active_end"])
        self.assertIsNone(services["oneshot.service"]["active_start"])
        self.assertIsNone(services["oneshot.service"]["active_end"])
        self.assertAlmostEqual(services["oneshot.service"]["exec_exit"], 1)
        self.assertAlmostEqual(sum(row["cpu_seconds"] for row in result["threads"]), 0.4)

    def test_parallel_cpu_work_is_not_serial_wall_time(self):
        (self.bundle / "ftrace.txt").write_text(
            event(1, 0, 42, cpu=0) + event(1.1, 0, 43, cpu=1)
            + event(2.2, 42, 0, cpu=0) + event(2.3, 43, 0, cpu=1)
        )
        result = analyze(self.bundle)
        self.assertAlmostEqual(sum(row["cpu_seconds"] for row in result["threads"]), 1.9)
        self.assertEqual(result["ready_seconds"], 2)

    def test_reused_tid_gets_separate_generations(self):
        (self.bundle / "ftrace.txt").write_text(
            event(1, 0, 42) + event(1.2, 42, 0)
            + " worker-1 [000] d..2 1.400000: sched_process_fork: "
              "comm=parent pid=1 child_comm=child child_pid=42\n"
            + event(1.5, 0, 42) + event(1.8, 42, 0) + event(2.2, 0, 0)
        )
        threads = sorted(
            (row for row in analyze(self.bundle)["threads"] if row["tid"] == 42),
            key=lambda row: row["generation"],
        )
        self.assertEqual([row["generation"] for row in threads], [0, 1])
        self.assertAlmostEqual(threads[0]["cpu_seconds"], 0.2)
        self.assertAlmostEqual(threads[1]["cpu_seconds"], 0.3)

    def test_scheduler_gap_is_not_charged_as_cpu(self):
        (self.bundle / "ftrace.txt").write_text(
            event(1, 0, 42) + event(1.2, 99, 0) + event(2.2, 0, 0)
        )
        result = analyze(self.bundle)
        self.assertEqual(sum(row["cpu_seconds"] for row in result["threads"]), 0)
        self.assertTrue(any("discontinuities" in warning for warning in result["warnings"]))

    def test_missing_scheduler_is_unknown_not_zero(self):
        (self.bundle / "ftrace.txt").write_text(
            " worker-1 [000] d..2 1.400000: sched_process_fork: "
            "comm=parent pid=1 child_comm=child child_pid=42\n"
        )
        self.assertIsNone(analyze(self.bundle)["threads"][0]["cpu_seconds"])

    def test_loss_marker_invalidates_open_scheduler_intervals(self):
        (self.bundle / "ftrace.txt").write_text(
            event(1, 0, 42) + "CPU:0 [LOST 3 EVENTS]\n"
            + event(1.8, 42, 0) + event(2.2, 0, 0)
        )
        result = analyze(self.bundle)
        self.assertEqual(sum(row["cpu_seconds"] for row in result["threads"]), 0)
        self.assertTrue(any("lost events" in warning for warning in result["warnings"]))

    def test_io_activity_has_time_bins_not_thread_attribution(self):
        with (self.bundle / "ftrace.txt").open("a") as source:
            source.write(" worker-1 [000] d..2 2.400000: block_rq_issue: 8,0 WS 0 () 1 + 8 [worker]\n")
        # Events after Ready are excluded from the activity view.
        self.assertEqual(analyze(self.bundle)["block_event_count"], 0)
        (self.bundle / "readiness.json").unlink()
        result = analyze(self.bundle)
        self.assertEqual(result["block_event_count"], 1)
        self.assertEqual(result["io_bins"][0][1], {"block_rq_issue": 1})

    def test_network_activity_separates_tx_rx_and_excludes_events_after_ready(self):
        (self.bundle / "ftrace.txt").write_text(
            event(1, 0, 42)
            + " worker-42 [000] d..2 1.110000: net_dev_xmit: dev=eth0 len=100\n"
            + " worker-42 [000] d..2 1.120000: netif_receive_skb_entry: skbaddr=0x1\n"
            + " worker-42 [000] d..2 1.130000: netif_receive_skb_list_entry: skbaddr=0x2\n"
            + " worker-42 [000] d..2 1.140000: netif_rx_entry: skbaddr=0x3\n"
            + event(1.5, 42, 0)
            + " worker-0 [000] d..2 2.100000: net_dev_xmit: dev=eth0 len=200\n"
        )
        result = analyze(self.bundle)
        self.assertEqual(result["network_bins"], [[1.1, {
            "net_dev_xmit": 1, "netif_receive_skb_entry": 1,
            "netif_receive_skb_list_entry": 1, "netif_rx_entry": 1,
        }]])
        self.assertEqual(result["network_coverage"], {"tx": True, "rx": True})
        self.assertEqual(result["block_event_count"], 0)
        self.assertAlmostEqual(result["threads"][0]["cpu_seconds"], 0.5)
        self.assertTrue(any("not packets" in warning for warning in result["warnings"]))

    def test_network_coverage_distinguishes_legacy_partial_and_enabled_without_traffic(self):
        for enabled, expected in (
            ([], {"tx": False, "rx": False}),
            (["net:net_dev_xmit"], {"tx": True, "rx": False}),
            (["net_dev_xmit", "net_if_receive_skb_list_entry"], {"tx": True, "rx": True}),
        ):
            with self.subTest(enabled=enabled):
                self.metadata["enabled_events"] = enabled
                self.write("metadata.json", self.metadata)
                result = analyze(self.bundle)
                self.assertEqual(result["network_coverage"], expected)
                self.assertEqual(result["network_bins"], [])
                self.assertEqual(
                    any("tracing was not recorded" in item for item in result["warnings"]),
                    not all(expected.values()),
                )

    def test_no_script_injection_in_html_payload(self):
        result = analyze(self.bundle)
        result["warnings"].append('</script><img src=x onerror="alert(1)">')
        output = self.root / "report.html"
        write_html(output, [result], ["Capture"])
        text = output.read_text()
        self.assertNotIn("</script><img", text)
        self.assertIn("\\u003c/script\\u003e", text)
        self.assertIn("connect-src 'none'", text)

    def test_report_supports_service_filtering_and_selectable_timeline_row_labels(self):
        output = self.root / "report.html"
        write_html(output, [analyze(self.bundle)], ["Capture"])
        text = output.read_text()
        self.assertIn('id="service-filter-list"', text)
        self.assertIn('id="show-services"', text)
        self.assertIn('id="hide-services"', text)
        self.assertIn('id="timeline-service-labels"', text)
        self.assertIn('id="timeline-scroll"', text)
        self.assertIn('id="timeline-spacer"', text)
        self.assertIn('id="timeline-resize"', text)
        self.assertIn('role="separator"', text)
        self.assertIn("setTimelineHeight", text)
        self.assertIn('event.key==="ArrowDown"', text)
        self.assertIn('id="timeline-selection"', text)
        self.assertIn("beginRangeSelection", text)
        self.assertIn("finishRangeSelection", text)
        self.assertIn('$("from").value=from.toFixed(3);$("to").value=to.toFixed(3)', text)
        self.assertIn("Drag horizontally anywhere in the timeline graph", text)
        self.assertIn("serviceHasDataInRange", text)
        self.assertIn("threadHasDataInRange", text)
        self.assertIn("with no recorded activity in the selected time range are hidden automatically", text)
        self.assertIn('className="timeline-row-label"', text)
        self.assertIn('addRowLabel(s.name,y)', text)
        self.assertIn('addRowLabel("Block I/O events (not latency)",y)', text)
        self.assertIn('addRowLabel(label+(covered?" events":" (not captured)"),y)', text)
        self.assertIn('addRowLabel(`${t.comm} [${t.tid}/${t.generation}]`,y)', text)
        self.assertIn('addEventListener("scroll",draw,{passive:true})', text)
        self.assertIn("all matching scheduler threads", text)
        self.assertNotIn("slice(0,40)", text)
        self.assertNotIn("Top CPU threads only", text)
        self.assertIn("user-select:text", text)
        self.assertIn("active interval", text)

    def test_cli_writes_html_and_json(self):
        output = self.root / "report"
        with patch("sys.stdout", new_callable=io.StringIO):
            self.assertEqual(main(["report", str(self.bundle), "--output", str(output)]), 0)
        self.assertTrue((output / "report.html").is_file())
        self.assertEqual(json.loads((output / "report.json").read_text())["ready_seconds"], 2)

    @unittest.skipUnless(shutil.which("trace-cmd"), "binary import needs trace-cmd")
    def test_real_binary_trace_preserves_early_events_and_merges_cpus(self):
        # A synthetic ring-buffer page, not a capture of the test host's kernel.
        tracefs = self.root / "tracefs"
        formats = tracefs / "events" / "sched" / "sched_switch"
        formats.mkdir(parents=True)
        function = tracefs / "events" / "ftrace" / "function"
        function.mkdir(parents=True)
        (function / "format").write_text(
            "name: function\nID: 2\nformat:\n"
            "\tfield:unsigned short common_type;\toffset:0;\tsize:2;\tsigned:0;\n"
            "\tfield:unsigned char common_flags;\toffset:2;\tsize:1;\tsigned:0;\n"
            "\tfield:unsigned char common_preempt_count;\toffset:3;\tsize:1;\tsigned:0;\n"
            "\tfield:int common_pid;\toffset:4;\tsize:4;\tsigned:1;\n\n"
            "\tfield:unsigned long ip;\toffset:8;\tsize:8;\tsigned:0;\n\n"
            'print fmt: "ip=%lx", REC->ip\n'
        )
        (tracefs / "events" / "header_page").write_text(
            "field: u64 timestamp;\toffset:0;\tsize:8;\tsigned:0;\n"
            "field: local_t commit;\toffset:8;\tsize:8;\tsigned:1;\n"
            "field: int overwrite;\toffset:8;\tsize:1;\tsigned:1;\n"
            "field: char data;\toffset:16;\tsize:4080;\tsigned:1;\n"
        )
        (tracefs / "events" / "header_event").write_text(
            "type_len : 5 bits\ntime_delta : 27 bits\narray : 32 bits\n"
        )
        (tracefs / "saved_cmdlines").write_text("0 <idle>\n42 worker\n43 parallel\n")
        (tracefs / "printk_formats").write_text("")
        kallsyms = self.root / "kallsyms"
        kallsyms.write_text("")
        (formats / "format").write_text(
            "name: sched_switch\nID: 1\nformat:\n"
            "\tfield:unsigned short common_type;\toffset:0;\tsize:2;\tsigned:0;\n"
            "\tfield:unsigned char common_flags;\toffset:2;\tsize:1;\tsigned:0;\n"
            "\tfield:unsigned char common_preempt_count;\toffset:3;\tsize:1;\tsigned:0;\n"
            "\tfield:int common_pid;\toffset:4;\tsize:4;\tsigned:1;\n\n"
            "\tfield:char prev_comm[16];\toffset:8;\tsize:16;\tsigned:1;\n"
            "\tfield:pid_t prev_pid;\toffset:24;\tsize:4;\tsigned:1;\n"
            "\tfield:int prev_prio;\toffset:28;\tsize:4;\tsigned:1;\n"
            "\tfield:long prev_state;\toffset:32;\tsize:8;\tsigned:1;\n"
            "\tfield:char next_comm[16];\toffset:40;\tsize:16;\tsigned:1;\n"
            "\tfield:pid_t next_pid;\toffset:56;\tsize:4;\tsigned:1;\n"
            "\tfield:int next_prio;\toffset:60;\tsize:4;\tsigned:1;\n\n"
            'print fmt: "prev_comm=%s prev_pid=%d prev_prio=%d prev_state=%s%s ==> '
            'next_comm=%s next_pid=%d next_prio=%d", REC->prev_comm, REC->prev_pid, '
            'REC->prev_prio, (REC->prev_state & 255) ? "S" : "R", '
            'REC->prev_state & 256 ? "+" : "", REC->next_comm, REC->next_pid, REC->next_prio\n'
        )

        def page(rows):
            payload = b""
            previous = rows[0][0]
            for timestamp, prev, nxt, state in rows:
                delta = timestamp - previous
                payload += struct.pack("<I", (delta << 5) | 16)
                payload += struct.pack(
                    "<HBBi16siiq16sii", 1, 0, 0, prev, b"worker", prev, 120,
                    state, b"worker", nxt, 120,
                )
                previous = timestamp
            return (struct.pack("<QQ", rows[0][0], len(payload)) + payload).ljust(4096, b"\0")

        cpu0 = self.root / "cpu0"
        cpu0.write_bytes(page([
            (1_000_000_001, 0, 42, 1),
            (1_100_000_001, 42, 0, 256),
            (1_200_000_001, 0, 42, 1),
            (1_300_000_001, 42, 0, 1),
        ]))
        cpu1 = self.root / "cpu1"
        cpu1.write_bytes(page([
            (1_050_000_001, 0, 43, 1), (1_150_000_001, 43, 0, 1),
        ]))
        partial = self.root / "partial.dat"
        subprocess.run(
            ["trace-cmd", "restore", "-c", "-t", str(tracefs), "-k", str(kallsyms),
             "-o", str(partial)], check=True, capture_output=True,
        )
        subprocess.run(
            ["trace-cmd", "restore", "-i", str(partial), "-o",
             str(self.bundle / "trace.dat"), str(cpu0), str(cpu1)],
            check=True, capture_output=True,
        )
        self.metadata.update({"ftrace_format": "trace-cmd", "recorder_active_monotonic": 1.1})
        self.write("metadata.json", self.metadata)
        (self.bundle / "ftrace.txt").unlink()
        result = analyze(self.bundle)
        worker = next(row for row in result["threads"] if row["tid"] == 42)
        self.assertEqual(result["trace_first"], 1.000000001)
        self.assertAlmostEqual(worker["cpu_seconds"], 0.2)
        self.assertAlmostEqual(worker["runnable_seconds"], 0.1)
        self.assertEqual(result["event_counts"]["sched_switch"], 6)
        self.assertFalse(any("could not be decoded" in item for item in result["warnings"]))

    def test_binary_decoder_handles_raw_nanoseconds_and_decimal_seconds(self):
        (self.bundle / "trace.dat").write_bytes(b"mocked")
        for timestamps in (
            ("1000000001", "1200000001", "2200000001"),
            ("1.000000001", "1.200000001", "2.200000001"),
        ):
            with self.subTest(timestamps=timestamps):
                source = "".join(
                    event(t, prev, nxt).replace(f"{t:.6f}:", timestamp + ":")
                    for t, prev, nxt, timestamp in (
                        (1, 0, 42, timestamps[0]),
                        (1.2, 42, 0, timestamps[1]),
                        (2.2, 0, 0, timestamps[2]),
                    )
                )
                process = Mock(stdout=io.StringIO("cpus=2\n" + source))
                process.wait.return_value = 0
                process.poll.return_value = 0
                with patch("events.subprocess.Popen", return_value=process):
                    result = analyze(self.bundle)
                worker = next(row for row in result["threads"] if row["tid"] == 42)
                self.assertEqual(result["trace_first"], 1.000000001)
                self.assertAlmostEqual(worker["cpu_seconds"], 0.2)
                self.assertEqual(result["event_counts"]["sched_switch"], 2)
                self.assertFalse(any("could not be decoded" in item for item in result["warnings"]))

    def test_failed_binary_decoder_never_falls_back_to_existing_text(self):
        (self.bundle / "trace.dat").write_bytes(b"invalid")
        process = Mock(stdout=io.StringIO("cpus=2\n" + event(1, 0, 42)))
        process.wait.return_value = 1
        process.poll.return_value = 1
        with patch("events.subprocess.Popen", return_value=process):
            with self.assertRaisesRegex(RuntimeError, "trace-cmd report failed"):
                analyze(self.bundle)


class ArchiveTests(unittest.TestCase):
    def test_rejects_traversal_and_links(self):
        for name, kind in [(BOOT + "/../escape", tarfile.REGTYPE), (BOOT + "/link", tarfile.SYMTYPE)]:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                archive = root / "capture.tar"
                with tarfile.open(archive, "w") as output:
                    info = tarfile.TarInfo(name)
                    info.type = kind
                    info.linkname = "/etc/passwd" if kind == tarfile.SYMTYPE else ""
                    output.addfile(info)
                with self.assertRaises(ValueError):
                    unpack(archive, root / "output", BOOT, 1024)

    def test_rejects_expansion_over_limit(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            archive = root / "capture.tar"
            with tarfile.open(archive, "w") as output:
                info = tarfile.TarInfo(BOOT + "/large")
                info.size = 10
                output.addfile(info, io.BytesIO(b"0123456789"))
            with self.assertRaisesRegex(ValueError, "byte limit"):
                unpack(archive, root / "output", BOOT, 5)

class RecorderTests(unittest.TestCase):
    def test_inventory_collects_all_loaded_units_with_boot_timestamps(self):
        properties = {
            "worker.service": (
                "Id=worker.service\nLoadState=loaded\nActiveState=active\n"
                "SubState=running\nType=simple\nRemainAfterExit=no\n"
                "ExecMainStartTimestampMonotonic=1000000\n"
                "ActiveEnterTimestampMonotonic=2000000\n"
                "InactiveEnterTimestampMonotonic=0\n"
                "ExecMainExitTimestampMonotonic=0\n"
            ),
            "oneshot.service": (
                "Id=oneshot.service\nLoadState=loaded\nActiveState=inactive\n"
                "SubState=dead\nType=oneshot\nRemainAfterExit=no\n"
                "ExecMainStartTimestampMonotonic=3000000\n"
                "ActiveEnterTimestampMonotonic=0\n"
                "InactiveEnterTimestampMonotonic=0\n"
                "ExecMainExitTimestampMonotonic=4000000\n"
            ),
            "never-started.service": (
                "Id=never-started.service\nLoadState=loaded\nActiveState=inactive\n"
                "SubState=dead\nType=oneshot\nRemainAfterExit=no\n"
                "ExecMainStartTimestampMonotonic=0\n"
                "ActiveEnterTimestampMonotonic=0\n"
                "InactiveEnterTimestampMonotonic=0\n"
                "ExecMainExitTimestampMonotonic=0\n"
            ),
        }

        def show(args, **kwargs):
            if args[1] == "list-units":
                return subprocess.CompletedProcess(
                    args, 0,
                    stdout="worker.service loaded active running Worker\n"
                           "oneshot.service loaded inactive dead One shot\n"
                           "never-started.service loaded inactive dead Never started\n",
                    stderr="",
                )
            unit = args[2]
            return subprocess.CompletedProcess(args, 0, stdout=properties[unit], stderr="")

        diagnostics = []
        with patch.object(capture.subprocess, "run", side_effect=show):
            rows = capture.systemd_services(diagnostics)
        self.assertEqual(diagnostics, [])
        self.assertEqual([row["name"] for row in rows], ["oneshot.service", "worker.service"])
        self.assertEqual(rows[1]["ExecMainStartTimestampMonotonic"], "1000000")
        self.assertEqual(rows[0]["type"], "oneshot")
        self.assertEqual(rows[0]["ExecMainExitTimestampMonotonic"], "4000000")

    def test_per_unit_query_failure_is_reported_without_dropping_other_units(self):
        def show(args, **kwargs):
            if args[1] == "list-units":
                return subprocess.CompletedProcess(
                    args, 0,
                    stdout="good.service loaded active running Good\n"
                           "broken.service loaded active running Broken\n"
                           "partial.service loaded inactive dead Partial\n",
                    stderr="",
                )
            if args[2] == "broken.service":
                return subprocess.CompletedProcess(args, 1, stdout="", stderr="unit query failed")
            if args[2] == "partial.service":
                return subprocess.CompletedProcess(
                    args, 0, stdout="Id=partial.service\nLoadState=loaded\n", stderr="",
                )
            return subprocess.CompletedProcess(
                args, 0,
                stdout="Id=good.service\nLoadState=loaded\nActiveState=active\n"
                       "SubState=running\nType=simple\nRemainAfterExit=no\n"
                       "ExecMainStartTimestampMonotonic=1000000\n"
                       "ActiveEnterTimestampMonotonic=2000000\n"
                       "InactiveEnterTimestampMonotonic=0\n"
                       "ExecMainExitTimestampMonotonic=0\n",
                stderr="",
            )

        diagnostics = []
        with patch.object(capture.subprocess, "run", side_effect=show):
            rows = capture.systemd_services(diagnostics)
        self.assertEqual([row["name"] for row in rows], ["good.service"])
        self.assertEqual(diagnostics, [
            "Could not read systemd service metadata for broken.service: unit query failed",
            "Systemd metadata for partial.service omitted properties: ActiveState, SubState, Type, "
            "RemainAfterExit, ExecMainStartTimestampMonotonic, ActiveEnterTimestampMonotonic, "
            "InactiveEnterTimestampMonotonic, ExecMainExitTimestampMonotonic.",
        ])

    def test_service_query_failure_is_not_hidden(self):
        def fail(*args, **kwargs):
            raise subprocess.CalledProcessError(
                1, args, stderr="Failed to connect to bus",
            )

        with patch.object(capture.subprocess, "run", side_effect=fail):
            with self.assertRaisesRegex(RuntimeError, "systemctl failed"):
                capture.systemd_services([])

    def test_watchdog_and_native_failure_both_stop_and_finalize_owned_capture(self):
        for exited in (False, True):
            with self.subTest(exited=exited), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                tracefs = root / "tracefs"
                tracefs.mkdir()
                for name, value in {
                    "tracing_on": "1", "trace_clock": "[mono]",
                    "set_event": "sched:sched_switch", "current_tracer": "nop",
                }.items():
                    (tracefs / name).write_text(value)
                process = Mock()
                process.poll.return_value = 1 if exited else None

                def stop(_process):
                    self.assertEqual((tracefs / "tracing_on").read_text(), "0")
                    destination = next((root / "captures").iterdir())
                    (destination / "trace.dat").write_bytes(b"finalized binary")

                args = argparse.Namespace(
                    backend="ftrace", max_seconds=600, max_mib=16,
                    image_label="test", include_journal=False,
                )
                with (
                    patch.object(capture, "ROOT", root / "captures"),
                    patch.object(capture, "TRACEFS", tracefs),
                    patch.object(capture.os, "geteuid", return_value=0),
                    patch.object(capture.os, "umask"),
                    patch.object(capture.signal, "signal"),
                    patch.dict(capture.os.environ, {}, clear=True),
                    patch.object(capture, "start_ftrace", return_value=process),
                    patch.object(capture, "wait_ftrace_ready"),
                    patch.object(capture, "disk_size", return_value=16 * 1024 * 1024),
                    patch.object(capture, "stop_ftrace", side_effect=stop) as stopped,
                    patch.object(capture, "systemd_services", return_value=[]),
                    patch.object(capture.time, "sleep") as sleep,
                ):
                    if exited:
                        with self.assertRaisesRegex(RuntimeError, "exited before stop"):
                            capture.record(args)
                    else:
                        capture.record(args)
                    stopped.assert_called_once_with(process)
                    sleep.assert_not_called()
                destination = next((root / "captures").iterdir())
                metadata = json.loads((destination / "metadata.json").read_text())
                self.assertEqual(metadata["status"], "failed" if exited else "stopped")
                self.assertIn("finalized_at", metadata)
                self.assertFalse((destination / "ftrace.txt").exists())

    def test_native_recorder_preserves_boot_configuration_and_identifies_itself(self):
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory)
            metadata = {}
            with patch.object(capture.subprocess, "Popen") as start:
                start.return_value.pid = 123
                capture.start_ftrace(destination, metadata)
            command = start.call_args.args[0]
            self.assertIn("-K", command)
            self.assertIn("--no-filter", command)
            self.assertIn("--poll", command)
            self.assertNotIn("-m", command)  # Rotation would discard early events.
            self.assertNotIn("-C", command)  # Changing clocks resets buffers.
            self.assertNotIn("-e", command)
            self.assertEqual(metadata["recorder_pid"], 123)
            self.assertEqual(metadata["ftrace_format"], "trace-cmd")

    def test_ready_waits_for_each_cpu_reader(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            tracefs = root / "tracefs"
            (tracefs / "per_cpu" / "cpu0").mkdir(parents=True)
            (tracefs / "per_cpu" / "cpu1").mkdir()
            (root / "trace-cmd.log").write_text("Hit Ctrl^C to stop recording\n")
            (root / "trace.dat.cpu0").touch()
            process = Mock()
            process.poll.return_value = None
            with patch.object(capture, "TRACEFS", tracefs), patch.object(capture.time, "sleep") as sleep:
                sleep.side_effect = lambda _seconds: (root / "trace.dat.cpu1").touch()
                capture.wait_ftrace_ready(process, root)
                sleep.assert_called_once_with(0.05)

    def test_early_native_exit_is_not_ready(self):
        process = Mock()
        process.poll.return_value = 1
        with tempfile.TemporaryDirectory() as directory:
            tracefs = Path(directory)
            (tracefs / "per_cpu" / "cpu0").mkdir(parents=True)
            with patch.object(capture, "TRACEFS", tracefs):
                with self.assertRaisesRegex(RuntimeError, "exited during setup"):
                    capture.wait_ftrace_ready(process, tracefs)

    def test_stop_drains_native_recorder_and_rejects_failure(self):
        process = Mock(pid=123)
        process.poll.return_value = None
        process.wait.return_value = 0
        capture.stop_ftrace(process)
        process.send_signal.assert_called_once_with(signal.SIGINT)
        process.wait.assert_called_once_with(timeout=60)
        process.wait.return_value = 1
        for already_gone in (False, True):
            with self.subTest(already_gone=already_gone):
                with patch.object(capture.os, "killpg") as kill:
                    if already_gone:
                        kill.side_effect = ProcessLookupError
                    with self.assertRaisesRegex(RuntimeError, "status 1"):
                        capture.stop_ftrace(process)
                    kill.assert_called_once_with(123, signal.SIGKILL)

    def test_stalled_finalization_kills_only_owned_process_group(self):
        process = Mock(pid=123)
        process.poll.return_value = None
        process.wait.side_effect = [subprocess.TimeoutExpired("trace-cmd", 60), -9]
        with patch.object(capture.os, "killpg") as kill:
            with self.assertRaisesRegex(RuntimeError, "did not finish draining"):
                capture.stop_ftrace(process)
            kill.assert_called_once_with(123, signal.SIGKILL)

    def test_lttng_buffers_are_bounded_and_loss_is_not_silently_accepted(self):
        for discarded in (0, 940, None):
            with self.subTest(discarded=discarded), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                commands = []
                boot_id = Path("/proc/sys/kernel/random/boot_id").read_text().strip()
                destination = root / "captures" / boot_id
                destination.mkdir(parents=True)
                early_timing = '{"task":"AKS.CSE.rcv1p.retrieveCerts","start":1,"end":2,"exit_status":0}\n'
                (destination / "cloud-timings.jsonl").write_text(early_timing)

                def command(*args, **kwargs):
                    commands.append(args)
                    return f"Discarded events: {discarded}\n" if discarded is not None else "unknown output"

                args = argparse.Namespace(
                    backend="lttng", max_seconds=600, max_mib=16,
                    image_label="test", include_journal=False,
                )
                with (
                    patch.object(capture, "ROOT", root / "captures"),
                    patch.object(capture, "TRACEFS", root / "tracefs"),
                    patch.object(capture.os, "geteuid", return_value=0),
                    patch.object(capture.os, "umask"),
                    patch.object(capture.signal, "signal"),
                    patch.dict(capture.os.environ, {}, clear=True),
                    patch.object(capture, "run", side_effect=command),
                    patch.object(capture, "disk_size", return_value=16 * 1024 * 1024),
                    patch.object(capture, "systemd_services", return_value=[]),
                ):
                    if discarded is None:
                        with self.assertRaisesRegex(RuntimeError, "discarded-event counters"):
                            capture.record(args)
                    else:
                        capture.record(args)
                    with self.assertRaises(FileExistsError):
                        capture.record(args)
                enable = next(command for command in commands if command[1] == "enable-channel")
                self.assertIn("--subbuf-size=4M", enable)
                self.assertIn("--num-subbuf=16", enable)
                self.assertIn("--read-timer=20000", enable)
                self.assertIn("--switch-timer=0", enable)
                self.assertIn("--tracefile-size=0", enable)
                self.assertNotIn("--overwrite", enable)
                self.assertEqual(commands[-1][1], "destroy")
                self.assertEqual(len({command[-1] for command in commands if command[1] in ("stop", "destroy")}), 1)
                destination = next((root / "captures").iterdir())
                metadata = json.loads((destination / "metadata.json").read_text())
                self.assertEqual((destination / "cloud-timings.jsonl").read_text(), early_timing)
                if discarded is not None:
                    self.assertEqual(metadata["lttng_discarded_events"], discarded)
                    self.assertEqual(any("scheduler accounting is partial" in warning for warning in metadata["warnings"]), bool(discarded))
                else:
                    self.assertEqual(metadata["status"], "failed")


class TargetedDiagnosticsTests(unittest.TestCase):
    def test_cloud_probe_preserves_failure_and_excludes_arguments_and_certificates(self):
        original = Path(__file__).resolve().parents[3] / "parts/linux/cloud-init/artifacts/init-aks-cloud.sh"
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            script = root / "cloud.sh"
            script.write_text(original.read_text())
            diagnostics.instrument_cloud(script)
            with self.assertRaisesRegex(ValueError, "refusing to instrument"):
                diagnostics.instrument_cloud(script)
            (root / "boot-id").write_text("test-boot\n")
            (root / "uptime").write_text("12.34 0.00\n")
            text = script.read_text().replace("/var/log/aks-node-boot-trace", str(root))
            text = text.replace("/proc/sys/kernel/random/boot_id", str(root / "boot-id"))
            script.write_text(text.replace("/proc/uptime", str(root / "uptime")))
            wrapper = '''
__SOURCED__=1 . "$1"
EVENTS_LOGGING_DIR="$2/"
date() { printf '%s\\n' 0; }
jq() { printf '%s\\n' '{}'; }
logs_to_events AKS.CSE.rcv1p.retrieveCerts sh -c 'printf "%s" PRIVATE_ARGUMENT_SENTINEL; exit 7'
'''
            result = subprocess.run(
                ["bash", "-c", wrapper, "test", str(script), str(root)],
                text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False,
            )
            self.assertEqual(result.returncode, 7, result.stderr)
            self.assertEqual(result.stdout, "PRIVATE_ARGUMENT_SENTINEL")
            timing = (root / "test-boot/cloud-timings.jsonl").read_text()
            self.assertNotIn("PRIVATE_ARGUMENT_SENTINEL", timing)
            self.assertEqual(json.loads(timing), {
                "task": "AKS.CSE.rcv1p.retrieveCerts", "start": 12.34, "end": 12.34, "exit_status": 7,
            })
            (root / "test-boot/cloud-timings.jsonl").unlink()
            (root / "test-boot/cloud-timings.jsonl").mkdir()
            result = subprocess.run(
                ["bash", "-c", wrapper, "test", str(script), str(root)],
                text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False,
            )
            self.assertEqual(result.returncode, 7)
            self.assertIn("WARNING: boot-trace could not save certificate timing", result.stderr)

    def test_cloud_probe_refuses_unknown_source_without_modifying_it(self):
        with tempfile.TemporaryDirectory() as directory:
            script = Path(directory) / "cloud.sh"
            script.write_text("changed source\n")
            with self.assertRaisesRegex(ValueError, "refusing to instrument"):
                diagnostics.instrument_cloud(script)
            self.assertEqual(script.read_text(), "changed source\n")

    def test_containerd_wait_sampling_captures_stack_and_fd_without_reading_payloads(self):
        def stat(tid, state, birth="10"):
            return f"{tid} (containerd) {state} " + " ".join(["0"] * 18 + [birth]) + "\n"

        with tempfile.TemporaryDirectory() as directory:
            proc = Path(directory)
            process = proc / "42"
            task = process / "task/43"
            task.mkdir(parents=True)
            (process / "fd").mkdir()
            (process / "comm").write_text("containerd\n")
            (process / "stat").write_text(stat(42, "S"))
            (task / "stat").write_text(stat(43, "D"))
            (task / "stack").write_text("io_schedule\nfilemap_read\n")
            (task / "wchan").write_text("io_schedule")
            (task / "syscall").write_text("17 0x7 PRIVATE_ARGUMENT_SENTINEL\n")
            payload = proc / "metadata.db"
            payload.write_text("PRIVATE_FILE_SENTINEL")
            (process / "fd/7").symlink_to(payload)
            warnings = []
            sampler = diagnostics.ContainerdWaitSampler(warnings, proc)
            sampler.sample(1.0)
            sampler.sample(1.2)
            self.assertEqual(len(sampler.waits), 1)
            self.assertEqual(sampler.waits[0]["fd"], 7)
            self.assertEqual(sampler.waits[0]["last_observed_monotonic"], 1.2)
            self.assertIn("filemap_read", sampler.waits[0]["kernel_stack"])
            text = json.dumps(sampler.result())
            self.assertIn("metadata.db", text)
            self.assertNotIn("PRIVATE_FILE_SENTINEL", text)
            self.assertNotIn("PRIVATE_ARGUMENT_SENTINEL", text)
            (task / "stat").write_text(stat(43, "S"))
            sampler.sample(1.4)
            (task / "stat").write_text(stat(43, "D"))
            sampler.sample(1.6)
            self.assertEqual(len(sampler.waits), 2)
            (process / "stat").write_text(stat(42, "S", birth="11"))
            sampler.sample(1.8)
            self.assertFalse(sampler.processes)
            self.assertEqual(warnings, [])


class OrchestrationTests(unittest.TestCase):
    def args(self, output):
        return argparse.Namespace(
            subscription=SUB, resource_group="rg", cluster="test", pool="pool",
            kubeconfig=Path("/not-read-by-mock"), context="explicit", ssh_user="azureuser",
            ssh_config=None, backend="ftrace", output=output, timeout=600, max_mib=256,
            execute=False, instrumentation_confirmed=False,
        )

    def responses(self, autoscale=False):
        return [
            json.dumps({"id": "test-cluster", "provisioningState": "Succeeded",
                        "nodeResourceGroup": "nodes", "fqdn": "test.example"}),
            json.dumps({"type": "VirtualMachineScaleSets", "osType": "Linux",
                        "provisioningState": "Succeeded", "count": 1,
                        "enableAutoScaling": autoscale}),
            json.dumps({"clusters": [{"cluster": {"server": "https://test.example:443"}}]}),
            json.dumps({"metadata": {"resourceVersion": "10"}, "items": [
                {"metadata": {"name": "old", "uid": "old"}, "spec": {"providerID": PROVIDER_ID}},
            ]}),
            '["0"]',
        ]

    def test_dry_run_never_scales_watches_or_creates_output(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "absent"
            with patch("orchestrate.command", side_effect=self.responses()) as call, \
                    patch("orchestrate.NodeWatch") as watch, patch("sys.stdout", new_callable=io.StringIO):
                scale_and_collect(self.args(output))
            self.assertFalse(output.exists())
            watch.assert_not_called()
            self.assertTrue(all("scale" not in invocation.args[0] for invocation in call.call_args_list))

    def test_autoscaler_rejected_before_mutation(self):
        with patch("orchestrate.command", side_effect=self.responses(True)):
            with self.assertRaisesRegex(ValueError, "without autoscaler"):
                scale_and_collect(self.args(Path("/unused")))

    def test_provider_identity_is_not_inferred_from_node_name(self):
        with self.assertRaisesRegex(ValueError, "providerID"):
            provider({"metadata": {"name": "aks-pool-vmss000000"}, "spec": {}})

    def test_watch_is_started_before_scale_and_survives_no_ready_as_partial(self):
        with tempfile.TemporaryDirectory() as directory:
            args = self.args(Path(directory) / "capture")
            args.execute = args.instrumentation_confirmed = True
            sequence = []
            responses = iter(self.responses())

            def invoke(command, timeout=90):
                if "scale" in command:
                    sequence.append("scale")
                    return ""
                return next(responses)

            watch = unittest.mock.Mock()
            watch.next.side_effect = RuntimeError("watch disconnected")

            def start_watch(*_args):
                sequence.append("watch")
                return watch

            with patch("orchestrate.command", side_effect=invoke), \
                    patch("orchestrate.NodeWatch", side_effect=start_watch):
                with self.assertRaisesRegex(RuntimeError, "disconnected"):
                    scale_and_collect(args)
            self.assertEqual(sequence, ["watch", "scale"])
            watch.close.assert_called_once()
            self.assertEqual(json.loads((args.output / "result.json").read_text())["status"], "incomplete")


class LttngTests(unittest.TestCase):
    def test_receive_event_aliases_are_normalized(self):
        class EventMessage:
            default_clock_snapshot = types.SimpleNamespace(
                clock_class=types.SimpleNamespace(origin_is_unix_epoch=True),
                ns_from_origin=1_000_000_000,
            )

            def __init__(self, name):
                self.event = types.SimpleNamespace(
                    name=name, payload_field={"skbaddr": "0x1"},
                    common_context_field={}, packet=None,
                )

        fake = types.SimpleNamespace(
            _EventMessageConst=EventMessage,
            _DiscardedEventsMessageConst=type("DiscardedEvents", (), {}),
            _DiscardedPacketsMessageConst=type("DiscardedPackets", (), {}),
            TraceCollectionMessageIterator=lambda _path: [
                EventMessage(name) for name in (
                    "net_dev_xmit", "net_if_receive_skb_entry",
                    "net_if_receive_skb_list_entry", "net_if_rx_entry",
                )
            ],
        )
        with patch.dict(sys.modules, {"bt2": fake}):
            events = list(lttng_events(Path("/unused"), []))
        self.assertEqual([item["name"] for item in events], [
            "net_dev_xmit", "netif_receive_skb_entry",
            "netif_receive_skb_list_entry", "netif_rx_entry",
        ])
        self.assertEqual([item["t"] for item in events], [1.0] * 4)

    def test_non_epoch_ctf_requires_explicit_clock_mapping(self):
        class EventMessage:
            default_clock_snapshot = types.SimpleNamespace(
                clock_class=types.SimpleNamespace(origin_is_unix_epoch=False),
            )

        fake = types.SimpleNamespace(
            _EventMessageConst=EventMessage,
            _DiscardedEventsMessageConst=type("DiscardedEvents", (), {}),
            _DiscardedPacketsMessageConst=type("DiscardedPackets", (), {}),
            TraceCollectionMessageIterator=lambda _path: [EventMessage()],
        )
        with patch.dict(sys.modules, {"bt2": fake}):
            with self.assertRaisesRegex(ValueError, "no Unix origin"):
                list(lttng_events(Path("/unused"), []))

    def test_ftrace_headers_are_not_unknown_records(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "trace.txt"
            path.write_text("# tracer: nop\n" + event(1, 0, 42))
            warnings = []
            self.assertEqual(len(list(ftrace_events(path, warnings))), 1)
            self.assertEqual(warnings, [])


class CseTests(unittest.TestCase):
    def test_existing_systemd_and_unixdate_formats(self):
        result = import_cse({"BootDatapoints": {
            "KernelStartTime": "Fri 2026-11-06 17:00:00 UTC",
            "CSEStartTime": "Fri Nov  6 17:00:12 UTC 2026",
        }})
        self.assertEqual(result[0]["timestamp"], "2026-11-06T17:00:00+00:00")
        self.assertEqual(result[1]["timestamp"], "2026-11-06T17:00:12+00:00")

    def test_ambiguous_timezone_needs_explicit_offset(self):
        data = {"KubeletStartTime": "Fri 2026-11-06 09:00:00 PST"}
        with self.assertRaisesRegex(ValueError, "ambiguous"):
            import_cse(data)
        self.assertEqual(
            import_cse(data, "-08:00")[0]["timestamp"], "2026-11-06T09:00:00-08:00",
        )

    def test_only_boot_fields_are_exported(self):
        result = import_cse({
            "BootDatapoints": {"KernelStartTime": "2026-11-06T17:00:00Z"},
            "Output": "sensitive output must not enter the report",
        })
        self.assertNotIn("sensitive", json.dumps(result))


if __name__ == "__main__":
    unittest.main()
