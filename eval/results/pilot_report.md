# Grounding pilot results

- Generator: LLM agent, answer-box style, 3-5 single-citation sentences
- Judge: independent LLM agent, blind to verifier scores
- Verifier: `cross-encoder/nli-deberta-v3-base`

## Baseline grounding (judge labels)

| Slice | Claims | Supported (95% CI) | Not found | Contradicted | Fully grounded answers | Verifier AUROC |
|---|---|---|---|---|---|---|
| all | 370 | 75.9% (71.3%–80.0%) | 23.2% | 0.8% | 33.8% | 0.8994 |
| fiqa | 189 | 79.4% (73.0%–84.5%) | 20.6% | 0.0% | 35.0% | 0.931 |
| scifact | 181 | 72.4% (65.5%–78.4%) | 26.0% | 1.7% | 32.5% | 0.8744 |

## Gate: drop claims with verifier entailment < τ

### all

| τ | Kept | Precision (kept supported) | Supported retained | Unsupported removed | Abstained answers | Fully grounded after |
|---|---|---|---|---|---|---|
| 0.1 | 216 | 96.8% | 74.4% | 92.1% | 2.5% | 92.3% |
| 0.2 | 209 | 97.1% | 72.2% | 93.3% | 3.8% | 92.2% |
| 0.3 | 208 | 97.1% | 71.9% | 93.3% | 3.8% | 92.2% |
| 0.5 | 205 | 97.1% | 70.8% | 93.3% | 3.8% | 92.2% |
| 0.7 | 199 | 97.0% | 68.7% | 93.3% | 3.8% | 92.2% |
| 0.9 | 190 | 97.9% | 66.2% | 95.5% | 6.2% | 94.7% |

### fiqa

| τ | Kept | Precision (kept supported) | Supported retained | Unsupported removed | Abstained answers | Fully grounded after |
|---|---|---|---|---|---|---|
| 0.1 | 108 | 98.2% | 70.7% | 94.9% | 2.5% | 94.9% |
| 0.2 | 103 | 98.1% | 67.3% | 94.9% | 5.0% | 94.7% |
| 0.3 | 102 | 98.0% | 66.7% | 94.9% | 5.0% | 94.7% |
| 0.5 | 100 | 98.0% | 65.3% | 94.9% | 5.0% | 94.7% |
| 0.7 | 98 | 98.0% | 64.0% | 94.9% | 5.0% | 94.7% |
| 0.9 | 95 | 97.9% | 62.0% | 94.9% | 5.0% | 94.7% |

### scifact

| τ | Kept | Precision (kept supported) | Supported retained | Unsupported removed | Abstained answers | Fully grounded after |
|---|---|---|---|---|---|---|
| 0.1 | 108 | 95.4% | 78.6% | 90.0% | 2.5% | 89.7% |
| 0.2 | 106 | 96.2% | 77.9% | 92.0% | 2.5% | 89.7% |
| 0.3 | 106 | 96.2% | 77.9% | 92.0% | 2.5% | 89.7% |
| 0.5 | 105 | 96.2% | 77.1% | 92.0% | 2.5% | 89.7% |
| 0.7 | 101 | 96.0% | 74.1% | 92.0% | 2.5% | 89.7% |
| 0.9 | 95 | 97.9% | 71.0% | 96.0% | 7.5% | 94.6% |
