#!/usr/bin/env python3
"""Exercise prerequisite repository setup without changing active package sources."""

import base64
import configparser
import functools
import http.server
import os
from pathlib import Path
import ssl
import subprocess
import sys
import tempfile
import threading


def run(*args, **kwargs):
    try:
        return subprocess.run(args, check=True, text=True, capture_output=True, timeout=60, **kwargs).stdout
    except subprocess.CalledProcessError as error:
        print(error.stdout, flush=True)
        print(error.stderr, flush=True)
        raise


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def fields(path):
    return dict(
        line.split(":", 1) for line in path.read_text().splitlines() if ":" in line
    )


def key_fingerprints(path, env):
    output = run("gpg", "--batch", "--with-colons", "--show-keys", str(path), env=env)
    return {line.split(":")[9] for line in output.splitlines() if line.startswith("fpr:")}


def validate_ubuntu(work, endpoint, version, codename, env):
    sources = work / "sources"
    require(
        {p.name for p in sources.iterdir()}
        == {"ubuntu.sources", "microsoft-prod.sources", "microsoft-prod-testing.sources"},
        "Unexpected active APT source files",
    )
    ubuntu = {key: value.strip() for key, value in fields(sources / "ubuntu.sources").items()}
    require(ubuntu == {
        "Types": "deb",
        "URIs": endpoint + "/ubuntu",
        "Suites": f"{codename} {codename}-updates {codename}-backports {codename}-security",
        "Components": "main universe restricted multiverse",
        "Signed-By": "/usr/share/keyrings/ubuntu-archive-keyring.gpg",
    }, f"Unexpected Ubuntu source configuration: {ubuntu}")
    expected_keys = [work / "keyrings" / (name + ".gpg") for name in ("microsoft.asc", "msopentech.asc")]
    for name in ("microsoft-prod", "microsoft-prod-testing"):
        actual = {key: value.strip() for key, value in fields(sources / (name + ".sources")).items()}
        require(actual == {
            "Types": "deb", "URIs": endpoint + f"/microsoft/ubuntu/{version}/prod",
            "Suites": codename, "Components": "main", "Arch": "amd64",
            "Signed-By": " ".join(str(key) for key in expected_keys),
        }, f"Unexpected Microsoft source configuration: {actual}")
    for name, installed in zip(("microsoft.asc", "msopentech.asc"), expected_keys):
        expected = key_fingerprints(work / "www" / "keys" / name, env)
        require(bool(expected), f"Empty fixture signing key: {name}")
        require(key_fingerprints(installed, env) == expected, f"Incorrect installed signing key: {name}")


def validate_azurelinux(work, endpoint):
    repos = ("amd", "base", "cloud-native", "extended", "ms-non-oss", "ms-oss", "nvidia")
    require(
        {p.name for p in (work / "repos").iterdir()}
        == {f"azurelinux-{repo}.repo" for repo in repos},
        "Unexpected active Azure Linux repository files",
    )
    for repo in repos:
        parser = configparser.ConfigParser(interpolation=None)
        parser.read(work / "repos" / f"azurelinux-{repo}.repo")
        section = f"azurelinux-official-{repo}"
        require(parser.sections() == [section], f"Unexpected sections for {repo}")
        actual = dict(parser[section])
        require(actual == {
            "name": f"Azure Linux Official {repo} $releasever $basearch",
            "baseurl": endpoint + f"/azurelinux/$releasever/prod/{repo}/$basearch",
            "gpgkey": "file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY",
            "gpgcheck": "1", "repo_gpgcheck": "1", "enabled": "1",
            "skip_if_unavailable": "True", "sslverify": "1",
        }, f"Unexpected Azure Linux repository configuration: {actual}")
    require(Path("/etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY").stat().st_size > 0,
            "Missing baked Microsoft RPM signing key")


def main():
    release = dict(line.split("=", 1) for line in Path("/etc/os-release").read_text().splitlines() if "=" in line)
    distro = release["ID"].strip('"')
    require(distro in ("ubuntu", "azurelinux"), f"Unsupported fixture distro: {distro}")
    with tempfile.TemporaryDirectory(prefix="ab-repodepot-e2e-") as directory:
        work = Path(directory)
        # Published VHDs may predate prerequisite mode; exercise this checkout's script.
        initializer = work / "init-aks-cloud.sh"
        initializer.write_text(base64.b64decode(sys.argv[1], validate=True).decode().replace("\r\n", "\n"))
        for name in ("www/keys", "sources", "keyrings", "repos", "backup", "gnupg", "bin"):
            (work / name).mkdir(parents=True, mode=0o700)
        # Fail explicitly if prerequisite mode ever starts refreshing package indexes.
        for command in ("apt-get", "dnf", "tdnf", "crontab", "systemctl"):
            guard = work / "bin" / command
            guard.write_text(
                "#!/bin/sh\n"
                f"touch '{work / 'unexpected-command'}'\n"
                f"echo 'Unexpected prerequisite command: {command}' >&2\nexit 97\n"
            )
            guard.chmod(0o700)
        env = dict(os.environ, GNUPGHOME=str(work / "gnupg"))
        if distro == "ubuntu":
            run("gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase", "",
                "--quick-generate-key", "AgentBaker RepoDepot E2E", "rsa2048", "sign", "0", env=env)
            key = run("gpg", "--batch", "--armor", "--export", env=env)
            run("gpgconf", "--homedir", str(work / "gnupg"), "--kill", "gpg-agent")
            for name in ("microsoft.asc", "msopentech.asc"):
                (work / "www/keys" / name).write_text(key)
            version = release["VERSION_ID"].strip('"')
            (work / "www/microsoft/ubuntu" / version / "prod").mkdir(parents=True)

        run("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
            "-subj", "/CN=AgentBaker RepoDepot E2E", "-addext", "subjectAltName=IP:127.0.0.1",
            "-keyout", str(work / "tls.key"), "-out", str(work / "tls.crt"))
        handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=str(work / "www"))
        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(work / "tls.crt", work / "tls.key")
        server.socket = context.wrap_socket(server.socket, server_side=True)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        endpoint = f"https://127.0.0.1:{server.server_port}"
        try:
            untrusted_env = dict(os.environ, NO_PROXY="127.0.0.1", no_proxy="127.0.0.1")
            for name in ("CURL_CA_BUNDLE", "SSL_CERT_FILE", "SSL_CERT_DIR"):
                untrusted_env.pop(name, None)
            untrusted = subprocess.run(
                ["curl", "--fail", "--silent", "--show-error", endpoint + "/"],
                env=untrusted_env, capture_output=True, timeout=30,
            )
            require(untrusted.returncode == 60, "Fixture HTTPS must fail certificate verification without its CA")
            (work / "wgetrc").write_text(f"ca_certificate = {work / 'tls.crt'}\n")
            env.update({
                "LOCATION": "eastus2", "REPO_DEPOT_ENDPOINT": endpoint + "/ubuntu",
                "AKS_CLOUD_PREREQUISITES_DONE": "false",
                "APT_SOURCES_LIST": str(work / "sources.list"),
                "APT_SOURCES_LIST_D_DIR": str(work / "sources"),
                "APT_BACKUP_DIR": str(work / "backup"), "APT_KEYRINGS_DIR": str(work / "keyrings"),
                "SSL_CERT_TARGET": str(work / "openssl-bundle.pem"),
                "YUM_REPOS_DIR": str(work / "repos"),
                "CURL_CA_BUNDLE": str(work / "tls.crt"), "WGETRC": str(work / "wgetrc"),
                "NO_PROXY": "127.0.0.1,168.63.129.16", "no_proxy": "127.0.0.1,168.63.129.16",
                "PATH": str(work / "bin") + ":" + os.environ["PATH"],
            })
            # Only this invocation trusts the fixture CA; never change the system trust store for the server.
            run("curl", "--fail", "--silent", "--show-error", endpoint + "/", env=env)
            result = subprocess.run(
                ["bash", str(initializer), "prerequisites"],
                env=env, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=180,
            )
            print(result.stdout, flush=True)
            require(result.returncode == 0, f"Prerequisite setup failed: {result.returncode}")
            require(not (work / "unexpected-command").exists(), "Prerequisites refreshed indexes or scheduled tasks")
            if distro == "ubuntu":
                validate_ubuntu(work, endpoint, version, release["VERSION_CODENAME"].strip('"'), env)
            else:
                validate_azurelinux(work, endpoint)
            print("RepoDepot HTTPS fixture and repository configuration validated", flush=True)
        finally:
            server.shutdown()
            server.server_close()
            thread.join()
            if distro == "ubuntu":
                run("gpgconf", "--homedir", str(work / "gnupg"), "--kill", "gpg-agent")


if __name__ == "__main__":
    main()
