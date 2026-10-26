from groundline.pilot import auroc, gate, wilson, windows


def claim(answer, verdict, p):
    return {"answer_id": answer, "claim": "c", "verdict": verdict, "p_entail": p}


def test_windows_short_passage_is_one_window():
    assert windows("One. Two.") == ["One. Two."]


def test_windows_overlap():
    w = windows("A one. B two. C three. D four.", size=3)
    assert w == ["A one. B two. C three.", "B two. C three. D four."]


def test_wilson_bounds():
    lo, hi = wilson(9, 10)
    assert 0.5 < lo < 0.9 < hi <= 1.0
    assert wilson(0, 0) == (0.0, 0.0)


def test_auroc():
    assert auroc([0.9, 0.8], [0.1, 0.2]) == 1.0
    assert auroc([0.5], [0.5]) == 0.5


def test_gate():
    claims = [
        claim("a", "supported", 0.9),
        claim("a", "not_found", 0.2),
        claim("b", "supported", 0.1),
        claim("b", "unverifiable", 0.0),
    ]
    g = gate(claims, 0.5)
    assert g["claims_kept"] == 1
    assert g["precision_kept_supported"] == 1.0
    assert g["recall_supported_retained"] == 0.5
    assert g["unsupported_removed"] == 1.0
    assert g["abstention_rate"] == 0.5
    assert g["fully_grounded_rate_after"] == 1.0


def test_mcnemar_and_norm():
    from groundline.agentab import mcnemar_p, norm
    assert mcnemar_p(0, 0) == 1.0
    assert mcnemar_p(6, 0) == 2 / 64
    assert norm("./django/views/debug.py::technical_404_response") == "django/views/debug.py"
