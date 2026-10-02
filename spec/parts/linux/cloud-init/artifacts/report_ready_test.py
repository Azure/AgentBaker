#!/usr/bin/env python3

import json
import os
import sys
import unittest
from unittest.mock import patch

ARTIFACTS_DIR = os.path.abspath(
    os.path.join(
        os.path.dirname(__file__),
        "../../../../../parts/linux/cloud-init/artifacts",
    )
)
sys.path.insert(0, ARTIFACTS_DIR)

import report_ready


class ReportReadyTest(unittest.TestCase):
    def test_builds_in_progress_report(self):
        report = report_ready.build_provisioning_health_report(
            state="NotReady",
            substatus="Provisioning",
            description="CSE is still running",
        )

        self.assertEqual(
            json.loads(report),
            {
                "state": "NotReady",
                "details": {
                    "subStatus": "Provisioning",
                    "description": "CSE is still running",
                },
            },
        )

    @patch.object(report_ready, "http_post")
    def test_sends_in_progress_report_to_provisioning_health(self, http_post):
        report_ready._send_provisioning_health_report(
            endpoint="168.63.129.16",
            state="NotReady",
            substatus="Provisioning",
            description="CSE is still running",
            retries=1,
            retry_delay=0,
        )

        http_post.assert_called_once_with(
            "168.63.129.16",
            "/provisioning/health",
            (
                b'{"state":"NotReady","details":{"subStatus":"Provisioning",'
                b'"description":"CSE is still running"}}'
            ),
            headers=report_ready.PROVISIONING_HEALTH_HEADERS,
            content_type="application/json",
        )

    @patch.object(report_ready, "_send_provisioning_health_report")
    @patch.object(report_ready, "_get_vm_id", return_value="test-vm-id")
    @patch.object(report_ready.os.path, "exists", return_value=True)
    def test_reports_cse_in_progress(
        self,
        _marker_exists,
        _get_vm_id,
        send_report,
    ):
        report_ready.report_in_progress(
            endpoint="168.63.129.16",
            retries=2,
            retry_delay=0,
        )

        send_report.assert_called_once_with(
            endpoint="168.63.129.16",
            state="NotReady",
            substatus="Provisioning",
            description=(
                "AKS CSE provisioning is still in progress "
                "for vm_id=test-vm-id."
            ),
            retries=2,
            retry_delay=0,
        )

    def test_continues_reporting_when_provisioning_exceeds_five_minutes(self):
        class ProvisioningFinished(Exception):
            pass

        elapsed_seconds = 0

        def advance_virtual_time(seconds):
            nonlocal elapsed_seconds
            elapsed_seconds += seconds
            if elapsed_seconds > 300:
                raise ProvisioningFinished

        with patch.object(report_ready, "report_in_progress") as send_report:
            with patch.object(
                report_ready.time,
                "sleep",
                side_effect=advance_virtual_time,
            ):
                with self.assertRaises(ProvisioningFinished):
                    report_ready.report_in_progress_forever(interval=60)

        self.assertEqual(elapsed_seconds, 360)
        self.assertEqual(send_report.call_count, 6)

    @patch.object(report_ready, "_send_provisioning_health_report")
    @patch.object(report_ready.os.path, "exists", return_value=False)
    def test_skips_in_progress_report_without_marker(
        self,
        _marker_exists,
        send_report,
    ):
        report_ready.report_in_progress()

        send_report.assert_not_called()


if __name__ == "__main__":
    unittest.main()
