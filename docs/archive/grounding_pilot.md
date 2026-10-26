# Grounding pilot: findings (2026-10-26)

**Question.** If an answer engine writes cited, answer-box-style responses over its own retrieved results, how many claims are not supported by their citation? Can a cheap NLI gate remove those claims without discarding the good ones?

## Setup
| Stage | What |
|---|---|
| Retrieval | Groundline Go BM25 (Snowball stemming, Lucene stopwords) over BEIR **FiQA** (57,638 StackExchange/Reddit finance posts) and **SciFact** (5,183 paper abstracts). The top 5 passages per query were truncated to 1,500 characters. |
| Queries | 40 per dataset, seeded random sample (seed 42) from the official test split |
| Generator | LLM agent writing 3–5 sentences per answer, each citing exactly one passage. It was told to be helpful and allowed to add general knowledge, mimicking answer-box behaviour. |
| Claims | 371 (1 sentence = 1 claim) |
| Judge | A separate LLM agent, **blind to verifier scores**. It labelled each claim supported / not found / contradicted / unverifiable against its cited passage only. |
| Verifier | `cross-encoder/nli-deberta-v3-base`, max entailment over 3-sentence windows of the passage (SummaC-style) |

## Retrieval sanity check (official qrels, full test sets)
| Dataset | Groundline BM25 nDCG@10 | BEIR paper BM25 | p50 / p99 latency |
|---|---|---|---|
| SciFact | **0.694** | 0.665 | 1.7 ms / 2.8 ms |
| FiQA | **0.249** | 0.236 | 19 ms / 36 ms |

## Results
| | All | FiQA (forum) | SciFact (science) |
|---|---|---|---|
| Claims supported by citation (95% CI) | 75.9% (71.3–80.0) | 79.4% (73.0–84.5) | 72.4% (65.5–78.4) |
| Not found in cited passage | 23.2% | 20.6% | 26.0% |
| Contradicted by cited passage | 0.8% | 0% | 1.7% |
| **Fully grounded answers** | **33.8%** | 35.0% | 32.5% |
| Verifier AUROC (supported vs. not) | 0.90 | 0.93 | 0.87 |

**Gate at τ = 0.5** (drop claims with entailment < 0.5):
- **Fully grounded answers: 33.8% → 92.2%**
- Precision of kept claims: 97.1%
- Unsupported claims removed: 93.3%
- Supported claims kept: 70.8%. The cost is that **29% of good claims are lost.**
- Answers left empty (abstained): 3.8%

The full τ sweep is in [`eval/results/pilot_report.md`](../eval/results/pilot_report.md). Verifier scores are strongly bimodal, so results barely change for τ between 0.1 and 0.7.

## What we learned
1. **There is plenty of headroom.** One claim in four was unsupported. Only one answer in three was fully grounded, which is close to the IMC 2026 AI Overviews figure (41.9%).
2. **A small off-the-shelf NLI model is a strong gate.** It removes about 93% of unsupported claims at a cost of about 29% of supported ones. The next target is that recall loss. Most dropped-but-supported claims are paraphrases spread across sentences.
3. **The forum hypothesis was not confirmed.** Forum evidence (FiQA) was *not* less grounded than scientific evidence. The difference runs the other way and the confidence intervals overlap. The verifier also worked *better* on forum text (AUROC 0.93 vs 0.87).
4. **Retrieval quality affects grounding.** Claims citing a passage that the qrels mark relevant were supported 82.4% of the time, versus 71.6% for other passages. Better retrieval should mean fewer ungrounded claims.

## Limitations (read before quoting numbers)
- **No human labels.** The judge is an LLM. The generator and judge come from the same model family, so their errors may be correlated. Next step: hand-label a 100-claim subset and report judge agreement (Cohen's κ).
- **The generator was nudged.** It was told it *may* add general knowledge. That inflates "not found" compared with a strictly extractive generator. The SciFact generator reported stating some study results from memory.
- **Truncation.** Cutting passages to 1,500 characters lowered support: 71.4% for truncated passages vs 78.5% for full ones. Some "not found" labels are artifacts of truncation.
- **Small and narrow sample.** 80 answers, two English domains, and sentence-level claims (no atomic claim splitting).
- **Latency.** The verifier takes about 0.24 s per claim on CPU, which is too slow for serving. It needs batching, a smaller model, or distillation.
