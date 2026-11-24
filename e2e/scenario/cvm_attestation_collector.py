import json
import os
import re
import sys

from src.attestation_provider import MAAProvider
from src.encoder import Encoder
from src.imds_client import ImdsClient
from src.isolation import IsolationType
from src.report_parser import ReportParser
from src.tpm_wrapper import TssWrapper


def safe_text(value):
    text = re.sub(r"\beyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b",
                  "[redacted JWT]", str(value))
    text = re.sub(r"[A-Za-z0-9_+/=-]{80,}", "[redacted encoded data]", text)
    text = re.sub(r"[\x00-\x1f\x7f]", " ", text)
    return " ".join(text.split())[:1024]


def report_phase(name):
    print(f"[cvm-collector] phase={name}", file=sys.stderr, flush=True)


class DiagnosticLogger:
    def debug(self, *_args, **_kwargs):
        pass

    def info(self, message, *_args, **_kwargs):
        if message == "Sending attestation request to provider..." or message.startswith("Retrying in "):
            print(f"[cvm-collector] phase=maa-request {safe_text(message)}",
                  file=sys.stderr, flush=True)

    def warning(self, message, *_args, **_kwargs):
        print(f"[cvm-collector] warning: {safe_text(message)}",
              file=sys.stderr, flush=True)

    def error(self, message, *_args, **_kwargs):
        if message.startswith("Request payload:"):
            return
        if message.startswith("Failed to verify evidence due to invalid collateral, error:"):
            message = "phase=maa-request HTTP 400: " + message
        print(f"[cvm-collector] error: {safe_text(message)}",
              file=sys.stderr, flush=True)


def main():
    platform, nonce, token_path, endpoint = sys.argv[1:]
    isolation_type = {
        "sevsnp": IsolationType.SEV_SNP,
        "tdx": IsolationType.TDX,
    }[platform]
    claims = {"user-claims": {"nonce": nonce}}
    logger = DiagnosticLogger()
    report_phase("tpm-report")
    tss = TssWrapper(logger)
    hcl_report = tss.get_hcl_report(claims)
    report_type = ReportParser.extract_report_type(hcl_report)
    print("[cvm-collector] platform-report: " + json.dumps({"reportType": report_type.name}),
          flush=True)
    if report_type != isolation_type:
        raise RuntimeError(f"expected {isolation_type.name} report, got {report_type.name}")

    runtime_data = ReportParser.extract_runtimes_data(hcl_report)
    hardware_report = ReportParser.extract_hw_report(hcl_report)
    encoded_report = Encoder.base64url_encode(hardware_report)
    imds = ImdsClient(logger)
    report_phase("imds-evidence")
    if isolation_type == IsolationType.TDX:
        encoded_evidence = imds.get_td_quote(encoded_report)
    else:
        cert_chain = imds.get_vcek_certificate()
        snp_report = json.dumps({
            "SnpReport": encoded_report,
            "VcekCertChain": Encoder.base64url_encode(cert_chain),
        }).encode("utf-8")
        encoded_evidence = Encoder.base64url_encode(snp_report)

    report_phase("maa-request")
    provider = MAAProvider(logger, isolation_type, endpoint)
    token = provider.attest_platform(encoded_evidence, Encoder.base64url_encode(runtime_data))
    if not token:
        raise RuntimeError("attestation provider returned no token")

    report_phase("token-write")
    fd = os.open(token_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as token_file:
        token_file.write(token)
        token_file.write("\n")


try:
    main()
except Exception as exc:
    print(f"attestation collector failed: {type(exc).__name__}: {safe_text(exc)}", file=sys.stderr)
    sys.exit(1)
