#!/usr/bin/env python3
"""Fetch the AKS CSE protected settings and materialize boothook.sh.

Obtains `commandToExecute` the way waagent's CustomScript handler would, then
writes /opt/azure/containers/boothook.sh. It does NOT execute boothook.sh and
does NOT touch aks-node-controller.

Channel order -- HostGAPlugin first, WireServer as fallback:

    goalstate (WireServer :80)            cheap; yields ContainerId, ConfigName,
                                          ExtensionsConfig URL, Certificates URL
      |
      +-- 1. HostGAPlugin  :32526/vmSettings          <-- preferred
      +-- 2. WireServer    <ExtensionsConfig URL>     <-- fallback

Both carry the same CMS-encrypted protectedSettings and normally answer in well
under a second. HostGAPlugin is preferred because it is the channel that keeps
working when the direct-to-storage path is broken -- the same failure that costs
waagent ~31s per process before it flips its own default channel.

Neither path fetches the InVMArtifactsProfileBlob. That blob only decides
`on_hold`, waagent treats its failure as non-fatal, and downloading it is
exactly where those 31s go.

Certificates: a transport keypair is minted by default, so this does not depend
on waagent having registered. If the fabric exchange fails we fall back to a
key waagent already decrypted, when one is present.

Nothing that could carry the TLS bootstrap token is logged. The decrypted
command is printed only under --unsafe-print-command.

Exit codes:
     0  boothook.sh written
     1  fatal error
    10  no CustomScript settings on either channel before --timeout
    11  commandToExecute did not match the expected template (nothing written)
"""

import argparse
import base64
import errno
import gzip
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
import uuid
import xml.etree.ElementTree as ET

WIRESERVER_IP = "168.63.129.16"
GOALSTATE_URL = "http://{0}/machine/?comp=goalstate".format(WIRESERVER_IP)

# hostplugin.py:39,42 -- HostGAPlugin listens on 32526.
HOSTGAPLUGIN_PORT = 32526
VMSETTINGS_URL = "http://{0}:{1}/vmSettings".format(WIRESERVER_IP, HOSTGAPLUGIN_PORT)

# wire.py:58 (WireServer) and hostplugin.py:48 (HostGAPlugin) -- different versions.
WIRE_PROTOCOL_VERSION = "2012-11-30"
HOSTGAPLUGIN_API_VERSION = "2015-09-01"

CUSTOM_SCRIPT_HANDLER = "Microsoft.Azure.Extensions.CustomScript"

# goal_state.py:560 -- the agent tries AES128_CBC first, then falls back.
CIPHERS = ("AES128_CBC", "DES_EDE3_CBC")
# goal_state.py:575 -- anything else is not a PFX-bearing blob.
EXPECTED_CERT_FORMAT = "Pkcs7BlobWithPfxContents"

DEFAULT_OUTPUT = "/opt/azure/containers/boothook.sh"
DEFAULT_WORK_DIR = "/run/aks-cse-early"
WAAGENT_LIB_DIR = "/var/lib/waagent"

HTTP_TIMEOUT = 10
HTTP_RETRIES = 3
HTTP_RETRY_WAIT = 1
POLL_INTERVAL = 2

# Exact shape emitted by cseScriptlessPhase2Template (pkg/agent/baker.go:55-58).
# Matching strictly is a safety feature: if the server-side template changes, or
# this node is not on the scriptless-phase2 path, we bail out and write nothing
# so CSE keeps handling provisioning exactly as it does today.
#
# The gzipped boothook payload is carried in the leading redirect of
# commandToExecute (see cseScriptlessPhase2Template in pkg/agent/baker.go):
#
#   echo '<payload>' | base64 -d | gzip -d > /opt/azure/containers/boothook.sh && ...
#
# Only that prefix is matched. Whatever the chain does afterwards -- run the
# boothook, guard it behind a marker, or go straight to provision-wait -- does not
# change the payload, so ignoring the tail keeps this working as the CSE command
# evolves. Anchoring at the start and restricting the payload to base64 characters
# is what makes it safe: nothing from the command is ever executed, the payload is
# only decoded and written to disk.
COMMAND_RE = re.compile(
    r"^echo '(?P<payload>[A-Za-z0-9+/=\s]+)' \| base64 -d \| gzip -d"
    r" > /opt/azure/containers/boothook\.sh"
)

_START = time.time()


def log(msg):
    sys.stderr.write("[{0:7.3f}s] {1}\n".format(time.time() - _START, msg))
    sys.stderr.flush()


class Bail(Exception):
    """Fatal error carrying an explicit exit code."""

    def __init__(self, message, code=1):
        super(Bail, self).__init__(message)
        self.code = code


# --------------------------------------------------------------------------
# HTTP
# --------------------------------------------------------------------------

def http_get(url, headers=None, quiet_url=False, retries=HTTP_RETRIES):
    """GET a URL with retries, returning the body as bytes.

    quiet_url suppresses the query string in errors -- goal state config URLs
    carry SAS tokens.
    """
    shown = url.split("?")[0] + "?<redacted>" if quiet_url else url
    request = urllib.request.Request(url, headers=headers or {})
    last = None
    for attempt in range(1, retries + 1):
        try:
            with urllib.request.urlopen(request, timeout=HTTP_TIMEOUT) as response:
                return response.read()
        except Exception as exc:  # noqa: BLE001 - urllib raises a wide range
            last = exc
            if attempt < retries:
                time.sleep(HTTP_RETRY_WAIT)
    raise Bail("GET {0} failed after {1} attempts: {2}".format(shown, retries, last))


def wire_headers():
    return {
        "x-ms-agent-name": "WALinuxAgent",
        "x-ms-version": WIRE_PROTOCOL_VERSION,
        "Content-Type": "text/xml;charset=utf-8",
    }


# --------------------------------------------------------------------------
# openssl
# --------------------------------------------------------------------------

def openssl(args, stdin=None, stdout_path=None):
    """Run openssl, returning stdout bytes or writing them to stdout_path."""
    out = open(stdout_path, "wb") if stdout_path else subprocess.PIPE
    try:
        proc = subprocess.Popen(
            ["openssl"] + args,
            stdin=subprocess.PIPE if stdin is not None else None,
            stdout=out,
            stderr=subprocess.PIPE,
        )
        stdout, stderr = proc.communicate(input=stdin)
        if proc.returncode != 0:
            raise Bail("openssl {0} failed (rc={1}): {2}".format(
                args[0], proc.returncode, stderr.decode("utf-8", "replace").strip()))
        return stdout
    finally:
        if stdout_path:
            out.close()


def pem_to_der_b64(pem_text):
    """Strip PEM armour, returning the base64 DER body on a single line."""
    return "".join(
        line.strip() for line in pem_text.splitlines() if line and not line.startswith("-----")
    )


def cert_thumbprint(cert_path):
    """SHA1 fingerprint, colons stripped, uppercased -- matches goal_state.py."""
    out = openssl(["x509", "-in", cert_path, "-fingerprint", "-noout"]).decode()
    return out.strip().split("=")[1].replace(":", "").upper()


def pubkey_of_cert(cert_path):
    return openssl(["x509", "-in", cert_path, "-pubkey", "-noout"]).decode()


def pubkey_of_key(key_path):
    """cryptutil.py:57 -- try `rsa` first, fall back to `pkey` on older openssl."""
    try:
        return openssl(["rsa", "-in", key_path, "-pubout"]).decode()
    except Bail:
        return openssl(["pkey", "-in", key_path, "-pubout"]).decode()


# --------------------------------------------------------------------------
# goal state
# --------------------------------------------------------------------------

class GoalState(object):
    def __init__(self, container_id, role_config_name, extensions_config_url, certificates_url):
        self.container_id = container_id
        self.role_config_name = role_config_name
        self.extensions_config_url = extensions_config_url
        self.certificates_url = certificates_url


def fetch_goal_state():
    """Fetch and parse the WireServer goal state.

    Cheap (a single :80 GET) and needed regardless of channel: HostGAPlugin
    requires ContainerId/ConfigName as headers, and the Certificates URL only
    ever comes from here.
    """
    doc = ET.fromstring(http_get(GOALSTATE_URL, wire_headers()))

    role_instance = doc.find(".//RoleInstance")
    if role_instance is None:
        raise Bail("goal state has no RoleInstance")
    config = role_instance.find("Configuration")
    if config is None:
        raise Bail("goal state RoleInstance has no Configuration")

    def text_of(parent, tag):
        node = parent.find(tag) if parent is not None else None
        return node.text.strip() if node is not None and node.text else None

    return GoalState(
        container_id=text_of(doc.find(".//Container"), "ContainerId"),
        role_config_name=text_of(config, "ConfigName"),
        extensions_config_url=text_of(config, "ExtensionsConfig"),
        certificates_url=text_of(config, "Certificates"),
    )


# --------------------------------------------------------------------------
# channel 1: HostGAPlugin /vmSettings
# --------------------------------------------------------------------------

def settings_from_vmsettings(goal_state):
    """Read protected settings from HostGAPlugin. Returns (protected, thumbprint)."""
    if not goal_state.container_id or not goal_state.role_config_name:
        raise Bail("goal state is missing ContainerId/ConfigName; cannot call HostGAPlugin")

    headers = {
        "x-ms-version": HOSTGAPLUGIN_API_VERSION,
        "x-ms-containerid": goal_state.container_id,
        "x-ms-host-config-name": goal_state.role_config_name,
        "x-ms-correlationid": str(uuid.uuid4()),
    }
    body = json.loads(http_get(VMSETTINGS_URL, headers).decode("utf-8"))

    for extension in body.get("extensionGoalStates", []):
        if extension.get("name") != CUSTOM_SCRIPT_HANDLER:
            continue
        for settings in extension.get("settings", []):
            protected = settings.get("protectedSettings")
            thumbprint = settings.get("protectedSettingsCertThumbprint")
            if protected and thumbprint:
                log("HostGAPlugin: found {0} v{1} (seqNo={2})".format(
                    CUSTOM_SCRIPT_HANDLER, extension.get("version"),
                    extension.get("settingsSeqNo")))
                return protected, thumbprint.upper()
    return None


# --------------------------------------------------------------------------
# channel 2: WireServer ExtensionsConfig
# --------------------------------------------------------------------------

def settings_from_extensions_config(goal_state):
    """Read protected settings from the WireServer ExtensionsConfig XML."""
    if not goal_state.extensions_config_url:
        raise Bail("goal state has no ExtensionsConfig URL")

    doc = ET.fromstring(
        http_get(goal_state.extensions_config_url, wire_headers(), quiet_url=True))

    for plugin in doc.findall(".//PluginSettings/Plugin"):
        if plugin.get("name") != CUSTOM_SCRIPT_HANDLER:
            continue
        runtime = plugin.find("RuntimeSettings")
        if runtime is None or not runtime.text:
            continue
        for entry in json.loads(runtime.text).get("runtimeSettings", []):
            handler = entry.get("handlerSettings", {})
            protected = handler.get("protectedSettings")
            thumbprint = handler.get("protectedSettingsCertThumbprint")
            if protected and thumbprint:
                log("WireServer: found {0} (seqNo={1})".format(
                    CUSTOM_SCRIPT_HANDLER, runtime.get("seqNo")))
                return protected, thumbprint.upper()
    return None


CHANNELS = (
    ("HostGAPlugin", settings_from_vmsettings),
    ("WireServer", settings_from_extensions_config),
)


def wait_for_settings(timeout):
    """Poll both channels, preferring HostGAPlugin, until settings appear."""
    deadline = time.time() + timeout
    attempt = 0
    while True:
        attempt += 1
        goal_state = None
        try:
            goal_state = fetch_goal_state()
        except Exception as exc:  # noqa: BLE001
            # Boothooks run at cloud-init init-local, which can be earlier than
            # the point where 168.63.129.16 is routable. A goal state failure is
            # therefore expected on the first attempts and must not be fatal.
            log("goal state unavailable: {0!r}".format(exc))

        if goal_state is not None:
            for name, reader in CHANNELS:
                try:
                    found = reader(goal_state)
                except Bail as exc:
                    log("{0} channel unavailable: {1}".format(name, exc))
                    continue
                except Exception as exc:  # noqa: BLE001
                    log("{0} channel error: {1!r}".format(name, exc))
                    continue
                if found:
                    return found[0], found[1], goal_state, name
                log("{0}: no {1} settings published yet".format(name, CUSTOM_SCRIPT_HANDLER))

        if time.time() >= deadline:
            raise Bail("no {0} settings on any channel after {1}s".format(
                CUSTOM_SCRIPT_HANDLER, timeout), code=10)
        log("attempt {0} found nothing; retrying".format(attempt))
        time.sleep(POLL_INTERVAL)


# --------------------------------------------------------------------------
# certificates
# --------------------------------------------------------------------------

def mint_transport_keypair(work_dir):
    """Mint a transport keypair exactly as cryptutil.py:40-48 does.

    The fabric encrypts the certificates response to whatever public cert we
    present, so this needs no coordination with waagent. We keep the artifacts
    in our own work dir and never write into /var/lib/waagent.
    """
    key_path = os.path.join(work_dir, "TransportPrivate.pem")
    crt_path = os.path.join(work_dir, "TransportCert.pem")
    if os.path.exists(key_path) and os.path.exists(crt_path):
        return key_path, crt_path
    log("minting transport keypair")
    openssl([
        "req", "-x509", "-nodes", "-subj", "/CN=LinuxTransport",
        "-days", "730", "-newkey", "rsa:2048",
        "-keyout", key_path, "-out", crt_path,
    ])
    os.chmod(key_path, 0o600)
    os.chmod(crt_path, 0o600)
    return key_path, crt_path


def build_p7m(work_dir, data_b64):
    """Recreate the MIME envelope from goal_state.py:596-604 byte for byte.

    filename/name are the p7m's own absolute path; openssl does not care, but
    mirroring the agent avoids surprises.
    """
    p7m_path = os.path.join(work_dir, "Certificates.p7m")
    body = (
        "MIME-Version:1.0\n"
        "Content-Disposition: attachment; filename=\"{0}\"\n"
        "Content-Type: application/x-pkcs7-mime; name=\"{1}\"\n"
        "Content-Transfer-Encoding: base64\n"
        "\n"
        "{2}"
    ).format(p7m_path, p7m_path, data_b64)
    with open(p7m_path, "w") as handle:
        handle.write(body)
    os.chmod(p7m_path, 0o600)
    return p7m_path


def pfx_to_pem(pfx_path, pem_path):
    """goal_state.py:606-620 -- -nomacver first, then without it."""
    for nomacver in (True, False):
        args = ["pkcs12", "-nodes", "-password", "pass:", "-in", pfx_path, "-out", pem_path]
        if nomacver:
            args.append("-nomacver")
        try:
            openssl(args)
            return
        except Bail as exc:
            log("pkcs12 conversion failed (-nomacver={0}): {1}".format(nomacver, exc))
            if os.path.exists(pem_path):
                os.remove(pem_path)
    raise Bail("cannot convert the certificates PFX to PEM")


def split_pem(pem_path, work_dir):
    """Split a PEM bag into (key_paths, cert_paths)."""
    keys, certs, buffer, index = [], [], [], 0
    with open(pem_path) as handle:
        for line in handle:
            buffer.append(line)
            if re.match(r"[-]+END.*KEY[-]+", line):
                path, bucket = os.path.join(work_dir, "{0}.prv".format(index)), keys
            elif re.match(r"[-]+END.*CERTIFICATE[-]+", line):
                path, bucket = os.path.join(work_dir, "{0}.crt".format(index)), certs
            else:
                continue
            with open(path, "w") as out:
                out.write("".join(buffer))
            os.chmod(path, 0o600)
            bucket.append(path)
            buffer = []
            index += 1
    return keys, certs


def fetch_private_key(work_dir, certificates_url, thumbprint):
    """Download the goal state certificates and return the key for thumbprint.

    Keys and certs arrive as an unordered bag, so they are paired by public key
    (goal_state.py:625-666), never by position.
    """
    if not certificates_url:
        raise Bail("goal state has no Certificates URL")

    key_path, crt_path = mint_transport_keypair(work_dir)
    with open(crt_path) as handle:
        transport_cert_b64 = pem_to_der_b64(handle.read())

    pfx_path = os.path.join(work_dir, "Certificates.pfx")
    pem_path = os.path.join(work_dir, "Certificates.pem")

    last_error = None
    for cipher in CIPHERS:
        headers = {
            "x-ms-agent-name": "WALinuxAgent",
            "x-ms-version": WIRE_PROTOCOL_VERSION,
            "x-ms-guest-agent-public-x509-cert": transport_cert_b64,
            "x-ms-cipher-name": cipher,
        }
        try:
            doc = ET.fromstring(http_get(certificates_url, headers, quiet_url=True))
        except Bail as exc:
            last_error = exc
            continue

        data_node = doc.find(".//Data")
        if data_node is None or not data_node.text:
            raise Bail("the Certificates response has no Data element")

        format_node = doc.find(".//Format")
        if format_node is not None and format_node.text \
                and format_node.text.strip() != EXPECTED_CERT_FORMAT:
            raise Bail("unexpected Certificates format: {0}".format(format_node.text.strip()))

        p7m_path = build_p7m(work_dir, data_node.text.strip())
        try:
            openssl(["cms", "-decrypt", "-in", p7m_path, "-inkey", key_path,
                     "-recip", crt_path], stdout_path=pfx_path)
        except Bail as exc:
            log("transport decryption failed (cipher={0}): {1}".format(cipher, exc))
            last_error = exc
            if os.path.exists(pfx_path):
                os.remove(pfx_path)
            continue

        pfx_to_pem(pfx_path, pem_path)
        keys, certs = split_pem(pem_path, work_dir)

        by_pubkey = {pubkey_of_key(key): key for key in keys}
        for cert in certs:
            if cert_thumbprint(cert) != thumbprint:
                continue
            match = by_pubkey.get(pubkey_of_cert(cert))
            if match:
                log("matched private key for thumbprint {0}".format(thumbprint))
                return match
            raise Bail("certificate {0} has no matching private key".format(thumbprint))
        raise Bail("thumbprint {0} not in the goal state certificates".format(thumbprint))

    raise Bail("could not download certificates with any cipher: {0}".format(last_error))


def waagent_private_key(thumbprint):
    path = os.path.join(WAAGENT_LIB_DIR, "{0}.prv".format(thumbprint))
    return path if os.path.exists(path) else None


def resolve_private_key(work_dir, goal_state, thumbprint):
    """Mint our own certs first; fall back to a key waagent already decrypted."""
    try:
        return fetch_private_key(work_dir, goal_state.certificates_url, thumbprint)
    except Bail as exc:
        log("minted-cert path failed: {0}".format(exc))
        fallback = waagent_private_key(thumbprint)
        if not fallback:
            raise
        log("falling back to waagent key {0}".format(fallback))
        return fallback


# --------------------------------------------------------------------------
# payload
# --------------------------------------------------------------------------

def decrypt_protected_settings(protected_b64, key_path):
    """CMS-decrypt the protected settings blob and return the parsed JSON."""
    der = base64.b64decode(protected_b64)
    plaintext = openssl(["cms", "-decrypt", "-inform", "DER", "-inkey", key_path], stdin=der)
    return json.loads(plaintext.decode("utf-8"))


def extract_boothook(command_to_execute):
    """Pull the gzipped boothook out of commandToExecute and decompress it."""
    match = COMMAND_RE.match(command_to_execute.strip())
    if not match:
        raise Bail(
            "commandToExecute does not carry a scriptless-phase2 boothook payload; "
            "refusing to guess (nothing was written)", code=11)
    payload = re.sub(r"\s+", "", match.group("payload"))
    return gzip.decompress(base64.b64decode(payload))


def write_atomic(path, data):
    """Write 0600 via temp+rename so readers never observe a torn file."""
    directory = os.path.dirname(path) or "."
    try:
        os.makedirs(directory, 0o755)
    except OSError as exc:
        if exc.errno != errno.EEXIST:
            raise
    fd, tmp = tempfile.mkstemp(dir=directory, prefix=".boothook.")
    try:
        os.fchmod(fd, 0o600)
        with os.fdopen(fd, "wb") as handle:
            handle.write(data)
            handle.flush()
            os.fsync(handle.fileno())
        os.rename(tmp, path)
        tmp = None
    finally:
        if tmp and os.path.exists(tmp):
            os.remove(tmp)


# --------------------------------------------------------------------------

def parse_args():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--output", default=DEFAULT_OUTPUT,
                        help="where to write boothook.sh (default: %(default)s)")
    parser.add_argument("--work-dir", default=DEFAULT_WORK_DIR,
                        help="scratch dir for certs, 0700, tmpfs (default: %(default)s)")
    parser.add_argument("--timeout", type=int, default=180,
                        help="seconds to wait for settings to appear (default: %(default)s)")
    parser.add_argument("--channel", choices=("auto", "hostgaplugin", "wireserver"),
                        default="auto",
                        help="restrict the settings channel; auto prefers HostGAPlugin "
                             "and falls back to WireServer (default: %(default)s)")
    parser.add_argument("--settings-file",
                        help="skip both channels and read a waagent *.settings file "
                             "(offline testing; needs the matching waagent key)")
    parser.add_argument("--unsafe-print-command", action="store_true",
                        help="print the decrypted commandToExecute; it embeds the TLS "
                             "bootstrap token, so use only for interactive debugging")
    parser.add_argument("--keep-work-dir", action="store_true",
                        help="do not delete the scratch dir on exit")
    return parser.parse_args()


def load_from_settings_file(path):
    with open(path) as handle:
        handler = json.load(handle)["runtimeSettings"][0]["handlerSettings"]
    return handler["protectedSettings"], handler["protectedSettingsCertThumbprint"].upper()


def run(args):
    global CHANNELS
    if args.channel == "hostgaplugin":
        CHANNELS = (("HostGAPlugin", settings_from_vmsettings),)
    elif args.channel == "wireserver":
        CHANNELS = (("WireServer", settings_from_extensions_config),)

    os.umask(0o077)
    os.makedirs(args.work_dir, 0o700, exist_ok=True)
    os.chmod(args.work_dir, 0o700)

    if args.settings_file:
        log("reading settings from {0}".format(args.settings_file))
        protected, thumbprint = load_from_settings_file(args.settings_file)
        key_path = waagent_private_key(thumbprint)
        if not key_path:
            raise Bail("no waagent key for {0}; --settings-file cannot fetch "
                       "certificates on its own".format(thumbprint))
    else:
        protected, thumbprint, goal_state, channel = wait_for_settings(args.timeout)
        log("settings retrieved via {0} channel".format(channel))
        key_path = resolve_private_key(args.work_dir, goal_state, thumbprint)

    settings = decrypt_protected_settings(protected, key_path)
    command = settings.get("commandToExecute")
    if not command:
        raise Bail("no commandToExecute in the decrypted protected settings")
    log("decrypted protected settings ({0} bytes of commandToExecute)".format(len(command)))

    if args.unsafe_print_command:
        log("!!! printing decrypted command -- contains the bootstrap token !!!")
        print(command)

    boothook = extract_boothook(command)
    write_atomic(args.output, boothook)
    log("wrote {0} ({1} bytes, 0600)".format(args.output, len(boothook)))
    return 0


def main():
    args = parse_args()
    try:
        return run(args)
    except Bail as exc:
        log("ERROR: {0}".format(exc))
        return exc.code
    except Exception as exc:  # noqa: BLE001
        log("ERROR: unexpected failure: {0!r}".format(exc))
        return 1
    finally:
        if not args.keep_work_dir and os.path.isdir(args.work_dir):
            shutil.rmtree(args.work_dir, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
