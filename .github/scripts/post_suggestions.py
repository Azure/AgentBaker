#!/usr/bin/env python3
"""Post GitHub suggested-change review comments for previousLatestVersion
findings, so a developer can fix them with GitHub's native "Add suggestion
to batch" / "Commit suggestion" button instead of hand-editing the file.

Takes the {line, new_line} list produced by validate_previous_version.py's
--suggestions output and turns each row into an inline PR review comment on
`--file`, containing a ```suggestion fenced block. Every comment is tagged
with a hidden HTML marker so re-runs (e.g. after a Renovate PR is rebased)
first delete this script's own prior comments before posting fresh ones --
otherwise every push would pile up another stale round of suggestions.
"""

import argparse
import json
import os
import urllib.error
import urllib.request

MARKER = "<!-- PREVIOUS_LATEST_VERSION_SUGGESTION -->"
API = "https://api.github.com"


def api_request(method: str, url: str, token: str, body: dict = None) -> dict:
    data = json.dumps(body).encode("utf-8") if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Accept", "application/vnd.github+json")
    req.add_header("X-GitHub-Api-Version", "2022-11-28")
    if data is not None:
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req) as resp:
            raw = resp.read()
            return json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        detail = e.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{method} {url} -> {e.code}: {detail}") from e


def delete_stale_suggestions(repo: str, pr: int, token: str) -> None:
    """Delete this script's own previously-posted suggestion comments (identified
    by MARKER) so repeated pushes to the same PR don't pile up stale suggestions
    for values that have since changed or been fixed."""
    # Collect every marked comment ID across all pages before deleting any of
    # them. Deleting while paginating would shift later pages' contents into
    # earlier page numbers, causing some marked comments to be skipped.
    stale_ids = []
    page = 1
    while True:
        comments = api_request(
            "GET", f"{API}/repos/{repo}/pulls/{pr}/comments?per_page=100&page={page}", token
        )
        if not comments:
            break
        stale_ids.extend(c["id"] for c in comments if MARKER in c.get("body", ""))
        page += 1

    for comment_id in stale_ids:
        api_request("DELETE", f"{API}/repos/{repo}/pulls/comments/{comment_id}", token)


def post_suggestions(repo: str, pr: int, commit_sha: str, file_path: str, suggestions: list, token: str) -> None:
    if not suggestions:
        return
    comments = []
    for row in suggestions:
        body = (
            "Best-effort recommendation from the `previousLatestVersion` check: "
            "the highest build found upstream for this release.\n\n"
            f"```suggestion\n{row['new_line']}\n```\n\n"
            f"{MARKER}"
        )
        # "side" must be explicit: without it GitHub rejects the whole review
        # payload for a line-anchored comment. These suggestions always target
        # the new (proposed) file content, so it's always RIGHT.
        comments.append({"path": file_path, "line": row["line"], "side": "RIGHT", "body": body})

    review_body = api_request(
        "POST",
        f"{API}/repos/{repo}/pulls/{pr}/reviews",
        token,
        {
            "commit_id": commit_sha,
            "event": "COMMENT",
            "body": (
                f"Posted {len(comments)} suggested fix(es) for `previousLatestVersion` "
                "below -- click a suggestion's \"Add suggestion to batch\" (or "
                "\"Commit suggestion\") button to apply it directly to this PR."
            ),
            "comments": comments,
        },
    )
    print(f"Posted review {review_body.get('id')} with {len(comments)} suggestion(s).")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", required=True, help="owner/repo")
    parser.add_argument("--pr", required=True, type=int)
    parser.add_argument("--commit-sha", required=True)
    parser.add_argument("--file", required=True, help="path (as it appears in the PR diff) to components.json")
    parser.add_argument("--suggestions", required=True, help="path to the JSON produced by --suggestions")
    args = parser.parse_args()

    token = os.environ.get("GITHUB_TOKEN")
    if not token:
        print("::error::GITHUB_TOKEN is not set")
        return 1

    suggestions = []
    if os.path.exists(args.suggestions):
        with open(args.suggestions, "r", encoding="utf-8") as f:
            suggestions = json.load(f)

    # Always clean up prior suggestions first, even when this run has none --
    # findings from an earlier push may since have been fixed or superseded.
    delete_stale_suggestions(args.repo, args.pr, token)
    post_suggestions(args.repo, args.pr, args.commit_sha, args.file, suggestions, token)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
