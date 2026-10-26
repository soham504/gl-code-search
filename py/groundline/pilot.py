"""Grounding pilot: build claims, score them with an NLI verifier, and report.

    python -m groundline.pilot claims   # answers + evidence -> claims.jsonl
    python -m groundline.pilot verify   # claims -> verifier.jsonl (NLI scores)
    python -m groundline.pilot report   # + judgments.jsonl -> results/pilot_report.{json,md}
"""
from __future__ import annotations

import argparse
import json
import math
import re
from collections import defaultdict
from pathlib import Path

from groundline.metrics import ClaimJudgment, Verdict, grounding_report

ROOT = Path(__file__).resolve().parents[2]
PILOT = ROOT / "eval" / "pilot"
RESULTS = ROOT / "eval" / "results"
DATASETS = ("fiqa", "scifact")
NLI_MODEL = "cross-encoder/nli-deberta-v3-base"
THRESHOLDS = (0.1, 0.2, 0.3, 0.5, 0.7, 0.9)


def read_jsonl(path: Path) -> list[dict]:
    with open(path) as f:
        return [json.loads(line) for line in f if line.strip()]


def write_jsonl(path: Path, rows) -> None:
    with open(path, "w") as f:
        for r in rows:
            f.write(json.dumps(r, ensure_ascii=False) + "\n")


def cmd_claims() -> None:
    rows = []
    for ds in DATASETS:
        evidence = {e["qid"]: e for e in read_jsonl(PILOT / f"{ds}_evidence.jsonl")}
        for ans in read_jsonl(PILOT / f"{ds}_answers.jsonl"):
            ev = evidence[ans["qid"]]
            passages = {p["ID"]: p for p in ev["passages"]}
            for i, s in enumerate(ans["sentences"]):
                p = passages[s["cite"]]
                rows.append({
                    "claim_id": f"{ds}:{ans['qid']}:{i}",
                    "answer_id": f"{ds}:{ans['qid']}",
                    "dataset": ds,
                    "source": ev["source"],
                    "query": ev["query"],
                    "claim": s["text"],
                    "cite": s["cite"],
                    "cite_relevant": p["Relevant"],
                    "passage": p["Text"],
                })
    write_jsonl(PILOT / "claims.jsonl", rows)
    print(f"wrote {len(rows)} claims")


def windows(passage: str, size: int = 3) -> list[str]:
    """Overlapping sentence windows, so a long passage fits the NLI model and a
    claim supported by one part is not diluted by the rest (SummaC-style)."""
    sents = [s for s in re.split(r"(?<=[.!?])\s+", passage.strip()) if s]
    if len(sents) <= size:
        return [passage]
    return [" ".join(sents[i:i + size]) for i in range(len(sents) - size + 1)]


def cmd_verify() -> None:
    import torch
    from transformers import AutoModelForSequenceClassification, AutoTokenizer

    tok = AutoTokenizer.from_pretrained(NLI_MODEL)
    model = AutoModelForSequenceClassification.from_pretrained(NLI_MODEL).eval()
    labels = {v.lower(): k for k, v in model.config.id2label.items()}
    out = []
    for c in read_jsonl(PILOT / "claims.jsonl"):
        wins = windows(c["passage"])
        enc = tok(wins, [c["claim"]] * len(wins), padding=True, truncation=True, max_length=512,
                  return_tensors="pt")
        with torch.no_grad():
            probs = model(**enc).logits.softmax(-1)
        out.append({
            "claim_id": c["claim_id"],
            "p_entail": round(probs[:, labels["entailment"]].max().item(), 4),
            "p_contra": round(probs[:, labels["contradiction"]].max().item(), 4),
        })
    write_jsonl(PILOT / "verifier.jsonl", out)
    print(f"scored {len(out)} claims with {NLI_MODEL}")


def wilson(k: int, n: int, z: float = 1.96) -> tuple[float, float]:
    if n == 0:
        return (0.0, 0.0)
    p = k / n
    d = 1 + z * z / n
    c = p + z * z / (2 * n)
    m = z * math.sqrt(p * (1 - p) / n + z * z / (4 * n * n))
    return (round((c - m) / d, 4), round((c + m) / d, 4))


def auroc(pos: list[float], neg: list[float]) -> float:
    if not pos or not neg:
        return float("nan")
    wins = sum((p > n) + 0.5 * (p == n) for p in pos for n in neg)
    return round(wins / (len(pos) * len(neg)), 4)


def summarize(claims: list[dict]) -> dict:
    rep = grounding_report(ClaimJudgment(c["answer_id"], c["claim"], Verdict(c["verdict"])) for c in claims)
    return {
        "claims": rep.claims,
        "support_rate": round(rep.support_rate, 4),
        "support_rate_95ci": wilson(rep.supported, rep.claims),
        "not_found_rate": round(rep.not_found / rep.claims, 4) if rep.claims else 0.0,
        "contradicted_rate": round(rep.contradicted / rep.claims, 4) if rep.claims else 0.0,
        "answers": rep.answers,
        "fully_grounded_rate": round(rep.fully_grounded_rate, 4),
        "unverifiable_claims": sum(c["verdict"] == "unverifiable" for c in claims),
    }


def gate(claims: list[dict], tau: float) -> dict:
    """Keep only claims the verifier scores >= tau; judge the survivors."""
    verifiable = [c for c in claims if c["verdict"] != "unverifiable"]
    kept = [c for c in verifiable if c["p_entail"] >= tau]
    supported = [c for c in verifiable if c["verdict"] == "supported"]
    kept_supported = [c for c in kept if c["verdict"] == "supported"]
    unsupported = len(verifiable) - len(supported)
    answers = {c["answer_id"] for c in verifiable}
    surviving = {c["answer_id"] for c in kept}
    after = summarize(kept) if kept else {}
    return {
        "tau": tau,
        "claims_kept": len(kept),
        "precision_kept_supported": round(len(kept_supported) / len(kept), 4) if kept else None,
        "recall_supported_retained": round(len(kept_supported) / len(supported), 4) if supported else None,
        "unsupported_removed": round(1 - (len(kept) - len(kept_supported)) / unsupported, 4) if unsupported else None,
        "abstention_rate": round(1 - len(surviving) / len(answers), 4) if answers else None,
        "fully_grounded_rate_after": after.get("fully_grounded_rate"),
    }


def cmd_report() -> None:
    claims = {c["claim_id"]: c for c in read_jsonl(PILOT / "claims.jsonl")}
    for v in read_jsonl(PILOT / "verifier.jsonl"):
        claims[v["claim_id"]].update(v)
    for j in read_jsonl(PILOT / "judgments.jsonl"):
        claims[j["claim_id"]]["verdict"] = j["verdict"]
    missing = [k for k, c in claims.items() if "verdict" not in c]
    if missing:
        raise SystemExit(f"{len(missing)} claims have no judgment, e.g. {missing[:3]}")
    rows = list(claims.values())

    by_ds = defaultdict(list)
    for c in rows:
        by_ds[c["dataset"]].append(c)

    def verifier_quality(cs: list[dict]) -> dict:
        v = [c for c in cs if c["verdict"] != "unverifiable"]
        return {"auroc_entail_vs_supported": auroc(
            [c["p_entail"] for c in v if c["verdict"] == "supported"],
            [c["p_entail"] for c in v if c["verdict"] != "supported"])}

    report = {
        "generator": "LLM agent, answer-box style, 3-5 single-citation sentences",
        "judge": "independent LLM agent, blind to verifier scores",
        "verifier": NLI_MODEL,
        "overall": {**summarize(rows), **verifier_quality(rows)},
        "by_dataset": {ds: {**summarize(cs), **verifier_quality(cs)} for ds, cs in by_ds.items()},
        "cited_passage_in_qrels": {ds: round(sum(c["cite_relevant"] for c in cs) / len(cs), 4)
                                   for ds, cs in by_ds.items()},
        "gate": {ds: [gate(cs, t) for t in THRESHOLDS] for ds, cs in [("all", rows), *by_ds.items()]},
    }
    RESULTS.mkdir(parents=True, exist_ok=True)
    (RESULTS / "pilot_report.json").write_text(json.dumps(report, indent=2))
    (RESULTS / "pilot_report.md").write_text(to_markdown(report))
    print(to_markdown(report))


def to_markdown(r: dict) -> str:
    def pct(x):
        return "–" if x is None else f"{100 * x:.1f}%"

    lines = ["# Grounding pilot results", "",
             f"- Generator: {r['generator']}", f"- Judge: {r['judge']}", f"- Verifier: `{r['verifier']}`", "",
             "## Baseline grounding (judge labels)", "",
             "| Slice | Claims | Supported (95% CI) | Not found | Contradicted | Fully grounded answers |"
             " Verifier AUROC |",
             "|---|---|---|---|---|---|---|"]
    for name, s in [("all", r["overall"]), *r["by_dataset"].items()]:
        lo, hi = s["support_rate_95ci"]
        lines.append(f"| {name} | {s['claims']} | {pct(s['support_rate'])} ({pct(lo)}–{pct(hi)}) | "
                     f"{pct(s['not_found_rate'])} | {pct(s['contradicted_rate'])} | "
                     f"{pct(s['fully_grounded_rate'])} | {s['auroc_entail_vs_supported']} |")
    lines += ["", "## Gate: drop claims with verifier entailment < τ", ""]
    for name, rows in r["gate"].items():
        lines += [f"### {name}", "",
                  "| τ | Kept | Precision (kept supported) | Supported retained | Unsupported removed |"
                  " Abstained answers | Fully grounded after |",
                  "|---|---|---|---|---|---|---|"]
        for g in rows:
            lines.append(f"| {g['tau']} | {g['claims_kept']} | {pct(g['precision_kept_supported'])} | "
                         f"{pct(g['recall_supported_retained'])} | {pct(g['unsupported_removed'])} | "
                         f"{pct(g['abstention_rate'])} | {pct(g['fully_grounded_rate_after'])} |")
        lines.append("")
    return "\n".join(lines)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("cmd", choices=["claims", "verify", "report"])
    {"claims": cmd_claims, "verify": cmd_verify, "report": cmd_report}[ap.parse_args().cmd]()


if __name__ == "__main__":
    main()
