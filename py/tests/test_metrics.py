import math

from groundline.metrics import ClaimJudgment, Verdict, grounding_report, ndcg_at_k


def test_ndcg_perfect_and_reversed():
    qrels = {"a": 3, "b": 2, "c": 1}
    assert ndcg_at_k(["a", "b", "c"], qrels) == 1.0
    assert ndcg_at_k(["c", "b", "a"], qrels) < 1.0


def test_ndcg_no_relevant_docs():
    assert ndcg_at_k(["a"], {}) == 0.0


def test_ndcg_known_value():
    # one relevant doc at rank 2: 1/log2(3)
    assert math.isclose(ndcg_at_k(["x", "a"], {"a": 1}), 1 / math.log2(3))


def test_grounding_report():
    S, N, C, U = Verdict.SUPPORTED, Verdict.NOT_FOUND, Verdict.CONTRADICTED, Verdict.UNVERIFIABLE
    r = grounding_report([
        ClaimJudgment("a1", "x", S),
        ClaimJudgment("a1", "y", S),
        ClaimJudgment("a1", "opinion", U),
        ClaimJudgment("a2", "x", S),
        ClaimJudgment("a2", "y", C),
        ClaimJudgment("a3", "z", N),
        ClaimJudgment("a4", "only opinion", U),
    ])
    assert (r.claims, r.supported, r.not_found, r.contradicted) == (5, 3, 1, 1)
    assert r.answers == 3  # a4 has no verifiable claims
    assert r.fully_grounded == 1
    assert math.isclose(r.support_rate, 0.6)
