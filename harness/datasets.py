"""Download localization benchmarks from Hugging Face and normalise them to
data/bench/<name>.jsonl rows of:

    {instance_id, repo, base_commit, problem_statement, gold_files, created_at, lang}

gold_files are the non-test source files touched by the reference patch.

    .venv/bin/python harness/datasets.py
"""
from __future__ import annotations

import json
import re
import urllib.request
from collections import Counter
from pathlib import Path

import pyarrow.parquet as pq

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "data" / "bench"

DATASETS = {
    "swebench_verified": ("princeton-nlp/SWE-bench_Verified", "test"),
    "swebench_full": ("princeton-nlp/SWE-bench", "test"),
    "swebench_live": ("SWE-bench-Live/SWE-bench-Live", "full"),
    "swe_rebench": ("nebius/SWE-rebench", "test"),
    "swe_gym": ("SWE-Gym/SWE-Gym", "train"),
    "locbench": ("czlll/Loc-Bench_V1", "test"),
    "swebench_multilingual": ("SWE-bench/SWE-bench_Multilingual", "test"),
    "swe_polybench": ("AmazonScience/SWE-PolyBench", "test"),
}
SRC_EXT = {".py": "python", ".js": "javascript", ".jsx": "javascript", ".ts": "typescript", ".tsx": "typescript",
           ".java": "java", ".go": "go", ".rs": "rust", ".rb": "ruby", ".php": "php", ".c": "c", ".h": "c",
           ".cc": "cpp", ".cpp": "cpp", ".hpp": "cpp", ".kt": "kotlin", ".scala": "scala", ".cs": "csharp"}
TEST_RE = re.compile(r"(^|/)(tests?|testing|__tests__|spec|specs)/|(^|/)test_[^/]*$|_test\.[a-z]+$|"
                     r"\.(test|spec)\.[a-z]+$|Test\.java$|Tests?\.cs$|conftest\.py$")
DIFF_RE = re.compile(r"^diff --git a/(\S+) b/", re.M)


def parquet_urls(repo: str, split: str) -> list[str]:
    with urllib.request.urlopen(f"https://huggingface.co/api/datasets/{repo}/parquet") as r:
        return json.load(r)["default"][split]


def gold(patch: str) -> list[str]:
    files = sorted(set(DIFF_RE.findall(patch or "")))
    return [f for f in files if Path(f).suffix in SRC_EXT and not TEST_RE.search(f)]


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    for name, (repo, split) in DATASETS.items():
        rows = []
        for i, url in enumerate(parquet_urls(repo, split)):
            local = OUT / f"{name}_{i}.parquet"
            if not local.exists():
                urllib.request.urlretrieve(url, local)
            rows += pq.read_table(local).to_pylist()
        out, langs = [], Counter()
        for r in rows:
            g = gold(r.get("patch", ""))
            if not g or not r.get("base_commit") or not r.get("problem_statement"):
                continue
            lang = Counter(SRC_EXT[Path(f).suffix] for f in g).most_common(1)[0][0]
            langs[lang] += 1
            created = r.get("created_at")
            out.append({"instance_id": r["instance_id"], "repo": r["repo"], "base_commit": r["base_commit"],
                        "problem_statement": r["problem_statement"], "gold_files": g, "lang": lang,
                        "created_at": str(created)[:10] if created else None})
        with open(OUT / f"{name}.jsonl", "w") as f:
            for o in out:
                f.write(json.dumps(o) + "\n")
        dates = sorted(o["created_at"] for o in out if o["created_at"])
        print(f"{name:24s} n={len(out):5d} repos={len({o['repo'] for o in out}):4d} "
              f"dates={dates[0] if dates else '?'}..{dates[-1] if dates else '?'} langs={dict(langs.most_common(4))}")


if __name__ == "__main__":
    main()
