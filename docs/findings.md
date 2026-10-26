# Do coding agents need better search? (2026-10-26)

**Question.** Coding agents (Claude Code, Cursor, Codex) find code mostly with grep. Tools like Graphify (a code knowledge graph) claim large token savings, and vendors say semantic search reduces wasted file reads. We tested two things:
- Does a better *search tool* make an agent better at **localization**, i.e. finding the files a bug fix must change?
- Does a code-aware ranker beat plain BM25 *without* any LLM?

Ground truth throughout is **the files changed by the real fix**, never our own labels.

## 1. Offline: `gl` vs plain BM25 (no LLM)
`gl` is a Go BM25 ranker built for code. It splits `camelCase` and `snake_case` names, indexes path and symbol fields, down-weights test files, and boosts files named in stack traces, file paths, dotted modules and backticked symbols in the issue. The design was fixed before the full runs; the only data seen while building it was a 20-issue smoke test.

| Benchmark | n | System | Acc@1 | Acc@5 | Acc@10 | p50 latency (index + rank) |
|---|---|---|---|---|---|---|
| SWE-bench Lite | 300 | BM25 (English analyzer) | 27.3% | 54.7% | 67.7% | 0.9 s |
| SWE-bench Lite | 300 | **gl** | **48.7%** | **71.7%** | **79.0%** | 1.1 s |
| Fresh issues (opened after 2026-07-14, 7 repos) | 28 | BM25 | 28.6% | 57.1% | 75.0% | 0.18 s |
| Fresh issues | 28 | **gl** | **50.0%** | **71.4%** | **85.7%** | 0.21 s |

Acc@k is the share of issues where a gold file is in the top k.

**Ablation on Lite (Acc@1):**

| Configuration | Acc@1 |
|---|---|
| Plain BM25 | 27.3 |
| Code-aware analyzer only, no issue signals | 39.3 |
| Full `gl` | 48.7 |
| Full minus test-file penalty | 45.0 (−3.7) |
| Full minus stack-trace and path signals | 47.0 (−1.7) |
| Full minus symbol signals | 47.0 (−1.7) |

The last three rows remove one signal at a time from the full system. For reference, published file-level numbers on SWE-bench Lite are about 34% for Lucene BM25 and about 70% for Agentless with GPT-4o, though metric definitions vary by paper ([Meta-RAG, arXiv 2508.02611](https://arxiv.org/abs/2508.02611)).

## 2. Agents: grep vs grep + Graphify vs grep + gl
**Setup:**
- The same agent (Claude Opus 5.5 subagent) was given the same prompt in each condition. Only the toolkit changed.
- Every repo access went through `harness/tool.py`. It enforces the toolkit and applies identical output caps of 150 lines and 12k characters.
- Every call was logged. All 380 tool calls across both experiments went through the harness.
- The prompt and workflow are in `harness/agent_ab.workflow.js`.
- 20 issues × 3 toolkits per experiment.

### 2a. SWE-bench Lite: the benchmark is memorised
| Toolkit | Acc@1 | Tool calls | gl/Graphify calls |
|---|---|---|---|
| grep | 100% | 4.3 | – |
| grep + Graphify | 100% | 3.0 | 0 |
| grep + gl | 100% | 3.0 | 0 |

Agents grepped the right file on their *first* search, for example `grep … django/views/debug.py`, even though the issue never named it. On famous repos a frontier model knows the codebase, so no search tool can show an effect. **SWE-bench Lite cannot evaluate agent search tooling.**

### 2b. Fresh post-cutoff issues: tools offered, tools ignored
| Toolkit | Acc@1 | Acc@5 | Tool calls | Wrong files opened | Tool output (k chars) | gl/Graphify calls |
|---|---|---|---|---|---|---|
| grep | 85% | 90% | 4.6 | 0.6 | 9.1 | – |
| grep + Graphify | 85% | 90% | 5.8 | 0.7 | 8.9 | **0.0** |
| grep + gl | 85% | 90% | 4.4 | 0.6 | 8.1 | **0.3** |

Paired against grep only:
- **Accuracy:** no change for either tool (McNemar p = 1.0).
- **Graphify:** Δ tool calls +1.1, 95% CI [−0.1, +2.9].
- **gl:** Δ tool output −1.0k characters, 95% CI [−2.6, +0.5].

## What we learned
1. **A code-aware ranker nearly doubles BM25's top-1 accuracy (27% → 49%) without an LLM, and the gain holds on unseen issues.** That suits uses with no frontier model in the loop: an issue-triage bot, CI, or a first stage for small or local models.
2. **A frontier agent with grep is already a strong localizer (85% Acc@1 on unseen issues).** That's well above any ranker alone. On these mid-sized repos, better search tools made no measurable difference.
3. **Agents don't adopt tools just because they're available.** Graphify was never called. `gl` was used in 5 of 20 runs, mostly as `gl symbol`, a grep substitute. Vendor claims of token savings assume the agent uses the tool; in a neutral prompt it doesn't. This matches [arXiv 2605.15184](https://arxiv.org/abs/2605.15184), which found that the harness matters as much as the retriever.
4. **Misses were reasoning errors, not search errors.** All three toolkits failed on the same 3 issues: a fix spanning two files, and two cases where the agent fixed a plausible neighbouring file.
5. **Contamination is the first thing to rule out.** Any agent-tooling claim measured on SWE-bench (Lite or Verified) for frontier models should be treated with suspicion.

## Limitations
- **Small sample:** 20 issues per experiment. CIs are wide. "No difference" means no difference *detectable at this n*, not proven equality.
- **One model, one prompt:** the tools were offered, not required. A system prompt that pushes the tool, as Graphify's installer does, may change adoption. That is the obvious next experiment.
- **Narrow scope:** mid-sized Python repos (beets, xarray, WeasyPrint, isort, tox, networkx, sqlfluff). Gains may appear on much larger or unfamiliar monorepos.
- **Gold labels:** fresh-issue gold comes from fix commits referencing `Fixes #N`, filtered to 1–3 non-test `.py` files. It is not hand-verified.
- **Graphify was used in code-only mode** (no LLM doc extraction), through its CLI rather than its MCP server.
- **Cost:** about 4.8M subagent tokens across both experiments.
