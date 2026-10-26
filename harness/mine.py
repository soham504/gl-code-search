"""Mine fresh (post-training-cutoff) localization instances from git history.

For each repo: find first-parent commits since SINCE whose message says
"fixes/closes/resolves #N", keep those that change 1-3 non-test .py files,
then fetch issue N (one unauthenticated GitHub API call each) and keep it if
it is a real issue opened on/after ISSUE_SINCE with a substantive body.

    python3 harness/mine.py > data/fresh/instances.jsonl
"""
from __future__ import annotations

import json
import re
import subprocess
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REPOS_DIR = ROOT / "data" / "fresh" / "repos"
SINCE = "2026-07-15"
ISSUE_SINCE = "2026-07-01"
MAX_API_CALLS = 55  # unauthenticated limit is 60/hour

REPOS = [
    "encode/httpx", "Textualize/textual", "Textualize/rich", "sqlfluff/sqlfluff", "pyvista/pyvista",
    "networkx/networkx", "marshmallow-code/marshmallow", "pre-commit/pre-commit", "mkdocs/mkdocs",
    "python-poetry/poetry", "tox-dev/tox", "beetbox/beets", "streamlink/streamlink", "Kozea/WeasyPrint",
    "pallets/click", "jazzband/pip-tools", "python-attrs/attrs", "PyCQA/isort", "pydata/xarray",
]
FIX_RE = re.compile(r"\b(?:fix(?:es|ed)?|close[sd]?|resolve[sd]?)\s*:?\s+#(\d+)", re.I)
TEST_RE = re.compile(r"(^|/)(tests?|testing)/|(^|/)test_[^/]*\.py$|_test\.py$|conftest\.py$")


def git(gitdir: Path, *args: str) -> str:
    return subprocess.run(["git", "--git-dir", str(gitdir), *args], capture_output=True, text=True,
                          check=True).stdout


def clone(repo: str) -> Path:
    d = REPOS_DIR / (repo.replace("/", "_") + ".git")
    if not d.exists():
        subprocess.run(["git", "clone", "-q", "--bare", f"https://github.com/{repo}.git", str(d)],
                       check=True, capture_output=True, text=True)
    return d


def candidates(repo: str, gitdir: Path):
    log = git(gitdir, "log", "--first-parent", f"--since={SINCE}", "--format=%H%x00%P%x00%B%x01", "HEAD")
    for entry in log.split("\x01"):
        parts = entry.strip("\n").split("\x00")
        if len(parts) != 3:
            continue
        sha, parents, msg = parts
        issues = sorted({int(n) for n in FIX_RE.findall(msg)})
        if len(issues) != 1 or not parents:
            continue
        parent = parents.split()[0]
        try:
            changed = git(gitdir, "diff", "--name-only", parent, sha).split()
        except subprocess.CalledProcessError:
            continue
        src = [f for f in changed if f.endswith(".py") and not TEST_RE.search(f)]
        if 1 <= len(src) <= 3 and all(not f.endswith(".rs") for f in changed):
            yield {"repo": repo, "fix_commit": sha, "base_commit": parent, "issue": issues[0], "gold_files": src}


def fetch_issue(repo: str, n: int) -> dict | None:
    req = urllib.request.Request(f"https://api.github.com/repos/{repo}/issues/{n}",
                                 headers={"Accept": "application/vnd.github+json", "User-Agent": "groundline-mine"})
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            return json.load(r)
    except urllib.error.HTTPError as e:
        print(f"# {repo}#{n}: HTTP {e.code}", file=sys.stderr)
        if e.code == 403:
            raise SystemExit("rate limited") from e
        return None


def main() -> None:
    REPOS_DIR.mkdir(parents=True, exist_ok=True)
    pool = []
    for repo in REPOS:
        try:
            pool.extend(candidates(repo, clone(repo)))
        except subprocess.CalledProcessError as e:
            print(f"# {repo}: {(e.stderr or '').strip()[:200]}", file=sys.stderr)
    print(f"# {len(pool)} candidate fix commits", file=sys.stderr)

    # Round-robin across repos so no single repo dominates the API budget.
    by_repo: dict[str, list] = {}
    for c in pool:
        by_repo.setdefault(c["repo"], []).append(c)
    order = []
    while any(by_repo.values()):
        for repo in list(by_repo):
            if by_repo[repo]:
                order.append(by_repo[repo].pop(0))

    calls = kept = 0
    for c in order:
        if calls >= MAX_API_CALLS:
            break
        issue = fetch_issue(c["repo"], c["issue"])
        calls += 1
        time.sleep(1)
        if not issue or "pull_request" in issue or issue["created_at"] < ISSUE_SINCE:
            continue
        body = issue.get("body") or ""
        if len(body) < 150:
            continue
        c.update(instance_id=f"{c['repo'].replace('/', '__')}-{c['issue']}",
                 created_at=issue["created_at"],
                 problem_statement=f"{issue['title']}\n\n{body}")
        print(json.dumps(c), flush=True)
        kept += 1
    print(f"# {calls} API calls, {kept} instances kept", file=sys.stderr)


if __name__ == "__main__":
    main()
