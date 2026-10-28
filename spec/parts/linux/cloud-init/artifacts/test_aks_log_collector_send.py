import contextlib
import hashlib
import importlib.util
import io
from pathlib import Path
import runpy
import tempfile
import unittest
from unittest import mock


SCRIPT = Path(__file__).resolve().parents[5] / "parts/linux/cloud-init/artifacts/aks-log-collector-send.py"
GOAL_STATE = b"""<GoalState><Container><ContainerId>test-container</ContainerId>
<RoleInstanceList><RoleInstance><Configuration><ConfigName>test-deployment.config</ConfigName>
</Configuration></RoleInstance></RoleInstanceList></Container></GoalState>"""


class UploadLogsTests(unittest.TestCase):
    def setUp(self):
        self.urllib3 = mock.MagicMock()
        spec = importlib.util.spec_from_file_location("aks_log_collector_send", SCRIPT)
        self.sender = importlib.util.module_from_spec(spec)
        with mock.patch.dict("sys.modules", {"urllib3": self.urllib3}):
            spec.loader.exec_module(self.sender)
        self.sender.MAX_RETRIES = 2
        self.sender.SLEEP_SECONDS = 0
        self.http = self.urllib3.PoolManager.return_value
        self.tempdir = tempfile.TemporaryDirectory()
        self.addCleanup(self.tempdir.cleanup)
        self.bundle = Path(self.tempdir.name) / "private.zip"
        self.payload = b"test bundle containing an I/O export manifest"
        self.bundle.write_bytes(self.payload)

    def upload(self, path=None):
        output = io.StringIO()
        with contextlib.redirect_stdout(output), self.assertRaises(SystemExit) as result:
            if path is None:
                self.sender.upload_logs()
            else:
                self.sender.upload_logs(path)
        return result.exception.code, output.getvalue()

    def test_acknowledges_exact_private_bundle_and_request_correlation(self):
        self.http.request.side_effect = [
            mock.Mock(data=GOAL_STATE),
            mock.Mock(status=200),
        ]
        code, output = self.upload(str(self.bundle))
        self.assertEqual(code, 0)
        put = self.http.request.call_args
        self.assertEqual(put.args[0], "PUT")
        self.assertEqual(put.args[1], "http://168.63.129.16:32526/vmAgentLog")
        self.assertEqual(put.kwargs["body"], self.payload)
        self.assertIn("bundle_sha256=" + hashlib.sha256(self.payload).hexdigest(), output)
        self.assertIn("correlation_id=" + put.kwargs["headers"]["x-ms-client-correlationid"], output)
        self.assertIn("http_status=200", output)
        self.assertIn("Successfully uploaded guest agent logs", output)

    def test_preserves_the_legacy_default_path(self):
        self.http.request.side_effect = [
            mock.Mock(data=GOAL_STATE),
            mock.Mock(status=200),
        ]
        with mock.patch("builtins.open", mock.mock_open(read_data=self.payload)) as opened:
            code, _ = self.upload()
        self.assertEqual(code, 0)
        opened.assert_called_once_with("/var/lib/waagent/logcollector/logs.zip", "rb")

    def test_cli_uses_the_private_bundle_path(self):
        self.http.request.side_effect = [
            mock.Mock(data=GOAL_STATE),
            mock.Mock(status=200),
        ]
        with mock.patch.dict("sys.modules", {"urllib3": self.urllib3}):
            with mock.patch("sys.argv", [str(SCRIPT), str(self.bundle)]):
                with contextlib.redirect_stdout(io.StringIO()), self.assertRaises(SystemExit) as result:
                    runpy.run_path(str(SCRIPT), run_name="__main__")
        self.assertEqual(result.exception.code, 0)
        self.assertEqual(self.http.request.call_args.kwargs["body"], self.payload)

    def test_failure_is_not_reported_as_acknowledgment(self):
        self.http.request.side_effect = [
            mock.Mock(data=GOAL_STATE), mock.Mock(status=503, data=b"unavailable"),
            mock.Mock(data=GOAL_STATE), mock.Mock(status=503, data=b"unavailable"),
        ]
        code, output = self.upload(str(self.bundle))
        self.assertEqual(code, 1)
        self.assertEqual(output.count("http_status=503"), 2)
        self.assertNotIn("Successfully uploaded", output)
        self.assertIn("Failed to upload guest agent logs after 2 retries", output)

    def test_missing_private_bundle_never_falls_back_to_shared_zip(self):
        self.http.request.return_value = mock.Mock(data=GOAL_STATE)
        code, output = self.upload(str(self.bundle.with_name("missing.zip")))
        self.assertEqual(code, 1)
        self.assertTrue(all(call.args[0] == "GET" for call in self.http.request.call_args_list))
        self.assertNotIn("Successfully uploaded", output)
        self.assertIn("encountered exception", output)


if __name__ == "__main__":
    unittest.main()
