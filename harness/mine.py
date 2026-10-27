"""Mine fresh (post-training-cutoff) localization instances from git history.

For each repo: walk first-parent commits since SINCE. A commit is a candidate
if its message says "fixes/closes/resolves #N", or if it is a squash-merged
PR "(#P)" whose PR closes exactly one issue (looked up via the GitHub API).
Keep candidates that change 1-3 non-test source files, then fetch the issue
and keep it if it was opened on/after ISSUE_SINCE with a substantive body.

    python3 harness/mine.py [--max-calls N] [--exclude existing.jsonl] > data/fresh/instances2.jsonl

Uses `gh auth token` when available (5,000 API calls/hour), else anonymous (60/hour).
"""
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REPOS_DIR = ROOT / "data" / "fresh" / "repos"
SINCE = "2026-07-15"
ISSUE_SINCE = "2026-07-01"

REPOS = [
    # Python
    "encode/httpx", "Textualize/textual", "Textualize/rich", "sqlfluff/sqlfluff", "pyvista/pyvista",
    "networkx/networkx", "marshmallow-code/marshmallow", "pre-commit/pre-commit", "mkdocs/mkdocs",
    "python-poetry/poetry", "tox-dev/tox", "beetbox/beets", "streamlink/streamlink", "Kozea/WeasyPrint",
    "pallets/click", "jazzband/pip-tools", "python-attrs/attrs", "PyCQA/isort", "pydata/xarray",
    "pydantic/pydantic", "psf/black", "joke2k/faker", "arrow-py/arrow",
    # JavaScript / TypeScript
    "eslint/eslint", "prettier/prettier", "honojs/hono", "colinhacks/zod", "TanStack/query",
    "excalidraw/excalidraw", "date-fns/date-fns", "chartjs/Chart.js",
    # Go
    "spf13/cobra", "charmbracelet/bubbletea", "junegunn/fzf", "go-chi/chi", "urfave/cli", "jesseduffield/lazygit",
    # Rust
    "nushell/nushell", "sharkdp/bat", "biomejs/biome", "astral-sh/uv",
]
SRC_EXT = {".py", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".java", ".go", ".rs", ".rb", ".php", ".c", ".h",
           ".cc", ".cpp", ".hpp", ".kt", ".scala", ".cs"}
TEST_RE = re.compile(r"(^|/)(tests?|testing|__tests__|spec|specs|testdata|fixtures?)/|(^|/)test_[^/]*$|"
                     r"_test\.[a-z]+$|_spec\.rb$|\.(test|spec)\.[a-z]+$|Tests?\.(java|cs|kt|scala)$|(^|/)conftest\.py$")
FIX_RE = re.compile(r"\b(?:fix(?:es|ed)?|close[sd]?|resolve[sd]?)\s*:?\s+#(\d+)", re.I)
PR_RE = re.compile(r"\(#(\d+)\)\s*$")


def token() -> str | None:
    try:
        return subprocess.run(["gh", "auth", "token"], capture_output=True, text=True, check=True).stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        return None


class GitHub:
    def __init__(self, max_calls: int):
        self.tok, self.calls, self.max_calls = token(), 0, max_calls

    def _req(self, url: str, data: bytes | None = None) -> dict | None:
        if self.calls >= self.max_calls:
            raise SystemExit(f"API budget of {self.max_calls} calls exhausted")
        self.calls += 1
        headers = {"Accept": "application/vnd.github+json", "User-Agent": "gl-code-search-mine"}
        if self.tok:
            headers["Authorization"] = f"Bearer {self.tok}"
        try:
            with urllib.request.urlopen(urllib.request.Request(url, data=data, headers=headers), timeout=30) as r:
                return json.load(r)
        except urllib.error.HTTPError as e:
            print(f"# {url}: HTTP {e.code}", file=sys.stderr)
            if e.code in (401, 403, 429):
                raise SystemExit("rate limited or unauthorized") from e
            return None

    def issue(self, repo: str, n: int) -> dict | None:
        return self._req(f"https://api.github.com/repos/{repo}/issues/{n}")

    def closing_issue(self, repo: str, pr: int) -> int | None:
        """The single issue a PR closes, or None (needs a token: GraphQL)."""
        if not self.tok:
            return None
        owner, name = repo.split("/")
        q = {"query": "query($o:String!,$n:String!,$p:Int!){repository(owner:$o,name:$n){pullRequest(number:$p)"
                      "{closingIssuesReferences(first:2){nodes{number}}}}}",
             "variables": {"o": owner, "n": name, "p": pr}}
        d = self._req("https://api.github.com/graphql", json.dumps(q).encode())
        try:
            nodes = d["data"]["repository"]["pullRequest"]["closingIssuesReferences"]["nodes"]
        except (TypeError, KeyError):
            return None
        return nodes[0]["number"] if len(nodes) == 1 else None


def git(gitdir: Path, *args: str) -> str:
    return subprocess.run(["git", "--git-dir", str(gitdir), *args], capture_output=True, text=True,
                          check=True).stdout


def clone(repo: str) -> Path:
    d = REPOS_DIR / (repo.replace("/", "_") + ".git")
    if not d.exists():
        url = f"https://github.com/{repo}.git"
        shallow = subprocess.run(["git", "clone", "-q", "--bare", "--shallow-since=2026-05-01", url, str(d)],
                                 capture_output=True, text=True)
        if shallow.returncode != 0:  # e.g. no commits in the window: fall back to a full clone
            subprocess.run(["rm", "-rf", str(d)])
            subprocess.run(["git", "clone", "-q", "--bare", url, str(d)], check=True, capture_output=True, text=True)
    return d


def candidates(repo: str, gitdir: Path):
    log = git(gitdir, "log", "--first-parent", f"--since={SINCE}", "--format=%H%x00%P%x00%B%x01", "HEAD")
    for entry in log.split("\x01"):
        parts = entry.strip("\n").split("\x00")
        if len(parts) != 3 or not parts[1]:
            continue
        sha, parents, msg = parts
        issues = sorted({int(n) for n in FIX_RE.findall(msg)})
        pr = PR_RE.search(msg.splitlines()[0]) if msg else None
        if len(issues) > 1 or (not issues and not pr):
            continue
        parent = parents.split()[0]
        try:
            changed = git(gitdir, "diff", "--name-only", parent, sha).split()
        except subprocess.CalledProcessError:
            continue
        src = [f for f in changed if Path(f).suffix in SRC_EXT and not TEST_RE.search(f)]
        if 1 <= len(src) <= 3:
            yield {"repo": repo, "fix_commit": sha, "base_commit": parent, "issue": issues[0] if issues else None,
                   "pr": int(pr.group(1)) if pr else None, "gold_files": src}


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--max-calls", type=int, default=1500)
    ap.add_argument("--exclude", action="append", default=[], help="JSONL of instances to skip")
    args = ap.parse_args()
    seen = set()
    for p in args.exclude:
        seen |= {json.loads(line)["instance_id"] for line in open(p)}

    REPOS_DIR.mkdir(parents=True, exist_ok=True)
    gh = GitHub(args.max_calls)
    print(f"# GitHub API: {'token' if gh.tok else 'anonymous'}", file=sys.stderr)
    by_repo: dict[str, list] = {}
    for repo in REPOS:
        try:
            by_repo[repo] = list(candidates(repo, clone(repo)))
        except subprocess.CalledProcessError as e:
            print(f"# {repo}: {(e.stderr or '').strip()[:200]}", file=sys.stderr)
    print(f"# {sum(map(len, by_repo.values()))} candidate commits", file=sys.stderr)

    order = []  # round-robin across repos so no single repo dominates
    while any(by_repo.values()):
        for repo in list(by_repo):
            if by_repo[repo]:
                order.append(by_repo[repo].pop(0))

    kept = 0
    try:
        for c in order:
            n = c["issue"] or gh.closing_issue(c["repo"], c["pr"])
            if not n:
                continue
            iid = f"{c['repo'].replace('/', '__')}-{n}"
            if iid in seen:
                continue
            issue = gh.issue(c["repo"], n)
            if not issue or "pull_request" in issue or issue["created_at"] < ISSUE_SINCE:
                continue
            body = issue.get("body") or ""
            if len(body) < 150:
                continue
            seen.add(iid)
            langs = sorted({Path(f).suffix.lstrip(".") for f in c["gold_files"]})
            c.update(instance_id=iid, issue=n, created_at=issue["created_at"], lang=langs[0],
                     problem_statement=f"{issue['title']}\n\n{body}")
            print(json.dumps(c), flush=True)
            kept += 1
    finally:
        print(f"# {gh.calls} API calls, {kept} instances kept", file=sys.stderr)


if __name__ == "__main__":
    main()
