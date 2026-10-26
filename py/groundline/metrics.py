"""Evaluation metrics for retrieval and answer grounding.

Grounding metrics follow the definitions in Xu et al., "Measuring Google AI
Overviews" (IMC 2026, arXiv:2605.14021): a claim is *supported* if its cited
source entails it, and an answer is *fully grounded* if every verifiable claim
is supported.
"""
from __future__ import annotations

import math
from dataclasses import dataclass
from enum import Enum
from typing import Iterable, Mapping, Sequence


def dcg(gains: Sequence[float]) -> float:
    return sum(g / math.log2(i + 2) for i, g in enumerate(gains))


def ndcg_at_k(ranked: Sequence[str], qrels: Mapping[str, float], k: int = 10) -> float:
    """nDCG@k for one query. `qrels` maps doc id to graded relevance."""
    ideal = dcg(sorted(qrels.values(), reverse=True)[:k])
    if ideal == 0:
        return 0.0
    return dcg([qrels.get(d, 0.0) for d in ranked[:k]]) / ideal


class Verdict(str, Enum):
    SUPPORTED = "supported"
    NOT_FOUND = "not_found"        # claim absent from the cited source
    CONTRADICTED = "contradicted"  # cited source says the opposite
    UNVERIFIABLE = "unverifiable"  # opinion, or source could not be fetched


@dataclass(frozen=True)
class ClaimJudgment:
    answer_id: str
    claim: str
    verdict: Verdict


@dataclass(frozen=True)
class GroundingReport:
    claims: int
    supported: int
    not_found: int
    contradicted: int
    answers: int
    fully_grounded: int

    @property
    def support_rate(self) -> float:
        return self.supported / self.claims if self.claims else 0.0

    @property
    def fully_grounded_rate(self) -> float:
        return self.fully_grounded / self.answers if self.answers else 0.0


def grounding_report(judgments: Iterable[ClaimJudgment]) -> GroundingReport:
    """Aggregate per-claim verdicts. Unverifiable claims are excluded from the
    denominators, matching the IMC study's "verifiable claims" framing."""
    counts = {v: 0 for v in Verdict}
    answer_ok: dict[str, bool] = {}
    for j in judgments:
        counts[j.verdict] += 1
        if j.verdict is Verdict.UNVERIFIABLE:
            continue
        answer_ok[j.answer_id] = answer_ok.get(j.answer_id, True) and j.verdict is Verdict.SUPPORTED
    verifiable = counts[Verdict.SUPPORTED] + counts[Verdict.NOT_FOUND] + counts[Verdict.CONTRADICTED]
    return GroundingReport(
        claims=verifiable,
        supported=counts[Verdict.SUPPORTED],
        not_found=counts[Verdict.NOT_FOUND],
        contradicted=counts[Verdict.CONTRADICTED],
        answers=len(answer_ok),
        fully_grounded=sum(answer_ok.values()),
    )
