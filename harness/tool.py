"""Tool dispatcher for the localization agent experiment.

Agents reach the repository only through this script, so every condition
gets identical output caps and every call is logged:

    python3 harness/tool.py RUN_ID TOOL [ARGS...]

RUN_ID is "<instance_id>__<condition>", condition in {grep, graphify, gl}.
Logs go to eval/agent_runs/<RUN_ID>.jsonl.
"""
from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MAX_LINES = 150
MAX_CHARS = 12_000

BASE_TOOLS = {"issue", "grep", "find", "read", "ls"}
CONDITIONS = {
    "grep": BASE_TOOLS,
    "graphify": BASE_TOOLS | {"graphify"},
    "gl": BASE_TOOLS | {"gl"},
}
GRAPHIFY_SUBCOMMANDS = {"query", "explain", "path", "affected", "god-nodes", "report"}
GL_SUBCOMMANDS = {"locate", "search", "symbol"}


def cap(text: str) -> str:
    lines = text.splitlines()
    out = "\n".join(lines[:MAX_LINES])
    note = ""
    if len(lines) > MAX_LINES:
        note = f"\n[truncated: showing {MAX_LINES} of {len(lines)} lines]"
    if len(out) > MAX_CHARS:
        out, note = out[:MAX_CHARS], f"\n[truncated at {MAX_CHARS} chars]"
    return out + note


def safe_rel(p: str) -> str:
    if p.startswith("/") or ".." in Path(p).parts:
        raise SystemExit(f"error: path must be relative to the repo root and stay inside it: {p}")
    return p


def run(cmd: list[str], cwd: Path, stdin: str | None = None) -> str:
    r = subprocess.run(cmd, cwd=cwd, input=stdin, capture_output=True, text=True, timeout=120)
    return (r.stdout or "") + (r.stderr if r.returncode not in (0, 1) else "")


def main() -> None:
    if len(sys.argv) < 3:
        raise SystemExit(__doc__)
    run_id, tool, args = sys.argv[1], sys.argv[2], sys.argv[3:]
    instance, _, cond = run_id.rpartition("__")
    if cond not in CONDITIONS:
        raise SystemExit(f"error: unknown condition in run id {run_id}")
    allowed = CONDITIONS[cond]
    if tool not in allowed:
        raise SystemExit(f"error: tool '{tool}' is not available in this run. Available: {sorted(allowed)}")
    work = ROOT / "data" / "work" / instance
    graph = ROOT / "data" / "graphs" / instance / "graphify-out"

    if tool == "issue":
        out = (ROOT / "harness" / "issues" / f"{instance}.txt").read_text()
    elif tool == "ls":
        out = run(["ls", "-p", safe_rel(args[0]) if args else "."], work)
    elif tool == "find":
        if not args:
            raise SystemExit("usage: find NAME_GLOB  (e.g. find '*query*.py')")
        out = run(["find", ".", "-name", args[0], "-not", "-path", "./.git/*"], work)
    elif tool == "grep":
        flags = [a for a in args if a in ("-i", "-w", "-l")]
        rest = [a for a in args if a not in flags]
        if not rest:
            raise SystemExit("usage: grep [-i] [-w] [-l] REGEX [SUBPATH]")
        sub = safe_rel(rest[1]) if len(rest) > 1 else "."
        out = run(["/usr/bin/grep", "-rnE", "--include=*.py", *flags, "--", rest[0], sub], work)
    elif tool == "read":
        if not args:
            raise SystemExit("usage: read PATH [START_LINE] [END_LINE]")
        path = work / safe_rel(args[0])
        lines = path.read_text(errors="replace").splitlines()
        start = int(args[1]) if len(args) > 1 else 1
        end = int(args[2]) if len(args) > 2 else start + MAX_LINES - 1
        out = "\n".join(f"{i}: {lines[i - 1]}" for i in range(max(1, start), min(end, len(lines)) + 1))
        out += f"\n[{args[0]}: {len(lines)} lines total]"
    elif tool == "gl":
        if not args or args[0] not in GL_SUBCOMMANDS:
            raise SystemExit("usage: gl locate | gl search QUERY... | gl symbol NAME")
        stdin = (ROOT / "harness" / "issues" / f"{instance}.txt").read_text() if args[0] == "locate" else None
        out = run([str(ROOT / "bin" / "gl"), "-root", str(work), *args], work, stdin)
    elif tool == "graphify":
        if not args or args[0] not in GRAPHIFY_SUBCOMMANDS:
            raise SystemExit('usage: graphify query "QUESTION" | explain NAME | path A B | affected NAME | god-nodes | report')
        if args[0] == "report":
            out = (graph / "GRAPH_REPORT.md").read_text()
        else:
            out = run([str(ROOT / ".venv-graphify" / "bin" / "graphify"), *args, "--graph", str(graph / "graph.json")], work)
    else:  # unreachable: guarded by `allowed`
        raise SystemExit(f"error: {tool}")

    out = cap(out)
    print(out)
    log = ROOT / "eval" / "agent_runs" / f"{run_id}.jsonl"
    log.parent.mkdir(parents=True, exist_ok=True)
    with open(log, "a") as f:
        f.write(json.dumps({"tool": tool, "args": args, "out_chars": len(out),
                            "read_path": args[0] if tool == "read" and args else None}) + "\n")


if __name__ == "__main__":
    main()
