"""Analyse the agent localization experiment (grep vs graphify vs gl toolkits).

    python -m groundline.agentab RESULTS.json GOLD.jsonl RUNS_DIR OUT_PREFIX

Inputs: the workflow's per-run answers (JSON list of {id, cond, out}) and the
harness call logs in eval/agent_runs/<id>__<cond>.jsonl. Gold files come from
the SWE-bench patches in data/swebench/lite.jsonl.
"""
from __future__ import annotations

import json
import random
import sys
from math import comb
from pathlib import Path
from statistics import mean

ROOT = Path(__file__).resolve().parents[2]
CONDS = ("grep", "graphify", "gl")


def norm(p: str) -> str:
    p = p.strip().split("::")[0].split(":")[0]
    return p[2:] if p.startswith("./") else p


def load(results_path: Path, gold_path: Path, runs_dir: Path) -> list[dict]:
    gold = {}
    for line in open(gold_path):
        r = json.loads(line)
        gold[r["instance_id"]] = set(r["gold_files"])
    rows = []
    for r in json.loads(results_path.read_text()):
        files = [norm(f) for f in r["out"]["files"]]
        log = runs_dir / f"{r['id']}__{r['cond']}.jsonl"
        calls = [json.loads(x) for x in open(log)] if log.exists() else []
        reads = [c["read_path"] for c in calls if c["tool"] == "read"]
        g = gold[r["id"]]
        rows.append({
            "id": r["id"], "cond": r["cond"], "gold": sorted(g), "answer": files,
            "acc1": int(bool(files) and files[0] in g),
            "acc5": int(any(f in g for f in files[:5])),
            "calls": len(calls),
            "reads": len(reads),
            "files_read": len(set(reads)),
            "wasted_files_read": len({p for p in reads if norm(p) not in g}),
            "tool_chars": sum(c["out_chars"] for c in calls),
            "special_calls": sum(c["tool"] in ("gl", "graphify") for c in calls),
            "confidence": r["out"].get("confidence"),
        })
    return rows


def mcnemar_p(b: int, c: int) -> float:
    """Exact two-sided McNemar test on discordant pairs b, c."""
    n = b + c
    if n == 0:
        return 1.0
    k = min(b, c)
    return min(1.0, 2 * sum(comb(n, i) for i in range(k + 1)) / 2 ** n)


def boot_ci(diffs: list[float], iters: int = 10_000, seed: int = 0) -> tuple[float, float]:
    rng = random.Random(seed)
    ms = sorted(mean(rng.choices(diffs, k=len(diffs))) for _ in range(iters))
    return ms[int(0.025 * iters)], ms[int(0.975 * iters)]


def main() -> None:
    results, gold_path, runs_dir, prefix = sys.argv[1:5]
    rows = load(Path(results), Path(gold_path), Path(runs_dir))
    by = {(r["id"], r["cond"]): r for r in rows}
    ids = sorted({r["id"] for r in rows if all((r["id"], c) in by for c in CONDS)})
    metrics = ["acc1", "acc5", "calls", "reads", "files_read", "wasted_files_read", "tool_chars", "special_calls"]

    lines = [f"# Agent localization A/B (n = {len(ids)} issues with all 3 runs)", "",
             "| Toolkit | Acc@1 | Acc@5 | Tool calls | read calls | Files opened | Wrong files opened |"
             " Tool output (k chars) | gl/graphify calls |",
             "|---|---|---|---|---|---|---|---|---|"]
    summary = {}
    for c in CONDS:
        rs = [by[(i, c)] for i in ids]
        s = {m: mean(r[m] for r in rs) for m in metrics}
        summary[c] = s
        lines.append(f"| {c} | {100 * s['acc1']:.0f}% | {100 * s['acc5']:.0f}% | {s['calls']:.1f} | {s['reads']:.1f} | "
                     f"{s['files_read']:.1f} | {s['wasted_files_read']:.1f} | {s['tool_chars'] / 1000:.1f} | "
                     f"{s['special_calls']:.1f} |")

    lines += ["", "## Paired differences vs grep-only (mean, 95% bootstrap CI; McNemar p for accuracy)", "",
              "| Toolkit | ΔAcc@1 | ΔAcc@5 | ΔTool calls | ΔWrong files opened | ΔTool output (k chars) |",
              "|---|---|---|---|---|---|"]
    paired = {}
    for c in ("graphify", "gl"):
        cells, paired[c] = [], {}
        for m in ("acc1", "acc5", "calls", "wasted_files_read", "tool_chars"):
            d = [by[(i, c)][m] - by[(i, "grep")][m] for i in ids]
            lo, hi = boot_ci(d)
            scale = 100 if m.startswith("acc") else (1 / 1000 if m == "tool_chars" else 1)
            cell = f"{mean(d) * scale:+.1f} [{lo * scale:+.1f}, {hi * scale:+.1f}]"
            if m.startswith("acc"):
                b = sum(by[(i, c)][m] > by[(i, "grep")][m] for i in ids)
                w = sum(by[(i, c)][m] < by[(i, "grep")][m] for i in ids)
                cell += f" p={mcnemar_p(b, w):.2f}"
            cells.append(cell)
            paired[c][m] = {"mean": mean(d), "ci95": [lo, hi]}
        lines.append(f"| {c} | " + " | ".join(cells) + " |")

    lines += ["", "Accuracy deltas are in percentage points; tool output is in thousands of characters "
              "(≈ tokens × 4) returned to the agent by tools."]
    out = ROOT / "eval" / "results"
    (out / f"{prefix}.md").write_text("\n".join(lines) + "\n")
    (out / f"{prefix}.json").write_text(json.dumps({"n": len(ids), "summary": summary, "paired": paired,
                                                   "runs": rows}, indent=1))
    print("\n".join(lines))


if __name__ == "__main__":
    main()
