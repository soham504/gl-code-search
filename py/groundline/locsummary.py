"""Summarise localization results across benchmarks.

    python -m groundline.locsummary   # reads eval/results/loc_*.jsonl -> eval/results/loc_summary.md

For each benchmark: Acc@1/Acc@5 for plain BM25 and gl, and the paired gl - BM25
difference with a 95% bootstrap CI (resampling instances). Also a per-language
breakdown pooled over all benchmarks.
"""
from __future__ import annotations

import json
import random
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
RESULTS = ROOT / "eval" / "results"
ORDER = ["lite", "verified", "swebench_full", "swe_gym", "locbench", "swebench_live", "swe_rebench",
         "multilingual", "polybench", "fresh"]
LABEL = {
    "lite": "SWE-bench Lite", "verified": "SWE-bench Verified", "swebench_full": "SWE-bench (full test)",
    "swe_gym": "SWE-Gym (sample)", "locbench": "LocBench", "swebench_live": "SWE-bench-Live (sample)",
    "swe_rebench": "SWE-rebench (sample)", "multilingual": "SWE-bench Multilingual",
    "polybench": "SWE-PolyBench (sample)", "fresh": "Fresh post-cutoff issues (ours)",
}


def hit(rank: int, k: int) -> int:
    return int(0 < rank <= k)


def pct(xs: list[int], n: int) -> float:
    return 100 * sum(xs) / n


def paired_ci(a: list[int], b: list[int], iters: int = 5000, seed: int = 0) -> tuple[float, float]:
    rng = random.Random(seed)
    n = len(a)
    diffs = [x - y for x, y in zip(a, b)]
    ms = sorted(sum(rng.choices(diffs, k=n)) / n for _ in range(iters))
    return ms[int(0.025 * iters)], ms[int(0.975 * iters)]


def main() -> None:
    rows_by = {}
    for name in ORDER:
        p = RESULTS / f"loc_{name}.jsonl"
        if p.exists():
            rows_by[name] = [json.loads(line) for line in open(p)]

    out = ["# File localization across benchmarks", "",
           "Gold = first non-test source file changed by the reference fix found in the ranking. "
           "Δ is gl − BM25 in percentage points with a paired 95% bootstrap CI.", "",
           "| Benchmark | n | Repos | BM25 Acc@1 | gl Acc@1 | ΔAcc@1 [95% CI] | BM25 Acc@5 | gl Acc@5 | ΔAcc@5 [95% CI] |",
           "|---|---|---|---|---|---|---|---|---|"]
    pooled = defaultdict(lambda: {"bm1": [], "gl1": [], "bm5": [], "gl5": []})
    total = 0
    for name, rows in rows_by.items():
        bm1 = [hit(r["bm25_plain"], 1) for r in rows]
        gl1 = [hit(r["gl_full"], 1) for r in rows]
        bm5 = [hit(r["bm25_plain"], 5) for r in rows]
        gl5 = [hit(r["gl_full"], 5) for r in rows]
        n = len(rows)
        total += n
        lo1, hi1 = paired_ci(gl1, bm1)
        lo5, hi5 = paired_ci(gl5, bm5)
        out.append(f"| {LABEL[name]} | {n} | {len({r['repo'] for r in rows})} | {pct(bm1, n):.1f}% | **{pct(gl1, n):.1f}%** | "
                   f"{pct(gl1, n) - pct(bm1, n):+.1f} [{100 * lo1:+.1f}, {100 * hi1:+.1f}] | {pct(bm5, n):.1f}% | "
                   f"**{pct(gl5, n):.1f}%** | {pct(gl5, n) - pct(bm5, n):+.1f} [{100 * lo5:+.1f}, {100 * hi5:+.1f}] |")
        if name in ("lite", "swebench_full"):  # Lite is a subset of full: pool only once
            continue
        for r in rows:
            d = pooled[r.get("lang") or "python"]
            d["bm1"].append(hit(r["bm25_plain"], 1))
            d["gl1"].append(hit(r["gl_full"], 1))
            d["bm5"].append(hit(r["bm25_plain"], 5))
            d["gl5"].append(hit(r["gl_full"], 5))

    out += ["", f"Total instance evaluations: {total} (benchmarks overlap: Lite and most of Verified are "
            "inside SWE-bench full).", "",
            "## By language (pooled over benchmarks, excluding Lite/full duplicates)", "",
            "| Language | n | BM25 Acc@1 | gl Acc@1 | BM25 Acc@5 | gl Acc@5 |", "|---|---|---|---|---|---|"]
    for lang, d in sorted(pooled.items(), key=lambda kv: -len(kv[1]["gl1"])):
        n = len(d["gl1"])
        if n < 10:
            continue
        cells = " | ".join(f"{pct(d[key], n):.1f}%" for key in ("bm1", "gl1", "bm5", "gl5"))
        out.append(f"| {lang} | {n} | {cells} |")

    (RESULTS / "loc_summary.md").write_text("\n".join(out) + "\n")
    print("\n".join(out))


if __name__ == "__main__":
    main()
