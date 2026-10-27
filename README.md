<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/banner-dark.svg">
    <img alt="gl code-search: paste a bug report, get the files that need fixing" src="docs/assets/banner-light.svg" width="100%">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/soham504/gl-code-search/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/soham504/gl-code-search/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="Go 1.22+" src="https://img.shields.io/badge/go-1.22%2B-00ADD8?logo=go&logoColor=white">
  <img alt="MCP server" src="https://img.shields.io/badge/MCP-server-6250d6">
  <img alt="No LLM" src="https://img.shields.io/badge/LLM%20calls-0-2a78d6">
  <a href="LICENSE"><img alt="MIT" src="https://img.shields.io/badge/license-MIT-59636e"></a>
</p>

<p align="center">
  <b><a href="#quickstart">Quickstart</a></b> ·
  <b><a href="#results">Results</a></b> ·
  <b><a href="#does-better-search-help-ai-coding-agents">Agents A/B</a></b> ·
  <b><a href="#use-it-with-your-tools">MCP &amp; GitHub Action</a></b> ·
  <b><a href="docs/findings.md">Full write-up</a></b>
</p>

**`gl` finds the files a bug report is about.** Point it at a repository and give it an issue. It ranks the source files most likely to need changes, in milliseconds on a laptop, with no LLM, no GPU and no index server.

Across **10 benchmarks, 5,960 issues and about 750 repos**, it roughly doubles plain BM25's top-1 accuracy, every time scored against the files the real fix changed. It's built for places with no frontier model in the loop: triage bots, CI, small local models, and as a pre-fetched shortlist inside agent harnesses. The repo also reports an honest A/B showing that *offering* agents a better search tool doesn't change their accuracy.

<p align="center">
  <img alt="Real gl output on a fresh beets issue" src="docs/assets/terminal.svg" width="88%">
  <br><sub>Real output on <a href="https://github.com/beetbox/beets/issues/7036">beets #7036</a>, an issue opened after the model's training cutoff. The fix changed <code>beets/plugins.py</code> and <code>beets/events.py</code>, ranked #2 and #3.</sub>
</p>

## Quickstart

```sh
go install github.com/soham504/gl-code-search/go/cmd/gl@latest     # Go 1.22+

cd your-repo
gl locate < issue.txt                 # ranked likely files, with the reasons
gl search "date field serialization"  # code-aware ranked search with matching lines
gl symbol QuerySet                    # where it's defined
gl -k 5 -markdown locate < issue.txt  # ready to paste into an issue (also: -json)
```

Works on Python, JS/TS, Go, Rust, Java, Kotlin, Scala, C/C++, C#, Ruby and PHP. The first run analyses the repo. After that an on-disk cache re-analyses only changed files: **3.9 s → 0.25 s** on Django's 2,600 files.

## Results

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/results-dark.svg">
    <img alt="Dot plot: gl roughly doubles BM25's top-1 accuracy on every benchmark" src="docs/assets/results-light.svg" width="100%">
  </picture>
</p>

| Benchmark | Issues | Repos | BM25 Acc@1 | **gl Acc@1** | Gain (95% CI) | BM25 Acc@5 | **gl Acc@5** |
|---|---|---|---|---|---|---|---|
| SWE-bench Verified | 500 | 12 | 24.0% | **49.2%** | +25.2 [+21.0, +29.4] | 54.8% | **76.0%** |
| SWE-bench (full test) | 2,294 | 12 | 25.9% | **48.4%** | +22.5 [+20.5, +24.4] | 56.2% | **75.0%** |
| SWE-Gym (sample) | 479 | 11 | 18.0% | **41.3%** | +23.4 [+19.2, +27.6] | 52.2% | **73.7%** |
| LocBench | 560 | 165 | 25.7% | **49.3%** | +23.6 [+19.5, +27.7] | 55.0% | **75.0%** |
| SWE-bench-Live (sample) | 500 | 133 | 28.0% | **43.4%** | +15.4 [+11.4, +19.4] | 55.8% | **68.8%** |
| SWE-rebench (sample) | 500 | 351 | 38.4% | **58.8%** | +20.4 [+16.2, +24.6] | 77.0% | **85.2%** |
| SWE-bench Multilingual (9 languages) | 299 | 41 | 18.7% | **36.1%** | +17.4 [+13.0, +22.1] | 43.8% | **62.9%** |
| SWE-PolyBench (sample; mostly JS/TS) | 500 | 19 | 13.2% | **27.4%** | +14.2 [+10.8, +17.6] | 33.4% | **50.8%** |
| Fresh issues, opened after the model's cutoff | 28 | 7 | 28.6% | **50.0%** | +21.4 [+0.0, +42.9] | 57.1% | **71.4%** |

Acc@k is the share of issues where a file changed by the real fix is in the top k. Ground truth is always the actual fix, never our own labels. Samples are seeded (500 instances); 21 SWE-Gym instances were skipped because their commits are no longer fetchable. The 28-issue fresh set is small and should be read as a sanity check.

**By language (pooled):**
- **Large gains:** Python 27 → 49%, Go 15 → 42%, PHP 16 → 47%, Java 20 → 39%, Rust 15 → 33%, Ruby 18 → 34%, JavaScript 14 → 29%.
- **Weak spots:** C 35 → 39% (C functions have no keyword for the regex symbol extractor to match) and TypeScript 9 → 14%.

Full tables are in [`eval/results/loc_summary.md`](eval/results/loc_summary.md).

<details>
<summary><b>Where the gain comes from</b> (ablation on SWE-bench Lite)</summary>

| Configuration | Acc@1 |
|---|---|
| Plain BM25 (English analyzer) | 25.7% |
| Code-aware analyzer only (`camelCase`/`snake_case` splitting, symbol and path fields) | 39.0% |
| **Full `gl`** | **48.3%** |
| Full, without the test-file penalty | 44.7% |
| Full, without stack-trace and path signals | 46.7% |
| Full, without symbol signals | 46.3% |
</details>

## Does better search help AI coding agents?

We gave the same agent the same 20 issues three times: with **grep only**, with **grep + [Graphify](https://github.com/Graphify-Labs/graphify)** (a popular code knowledge graph), and with **grep + `gl`**. All 380 tool calls went through a logged harness with identical output caps.

| | grep | grep + Graphify | grep + gl |
|---|---|---|---|
| SWE-bench Lite: Acc@1 | 100% | 100% | 100% |
| **Fresh post-cutoff issues: Acc@1** | **85%** | **85%** | **85%** |
| Fresh: calls to the extra tool per run | – | 0.0 | 0.3 |

> [!IMPORTANT]
> **On SWE-bench, agents had memorised the repos.** They grepped straight to the right file on their first search, so SWE-bench can't evaluate agent search tools.
>
> **On fresh issues, agents ignored the extra tools,** and accuracy was identical. All three toolkits missed the same 3 issues, which were reasoning errors, not search errors.
>
> `gl` nearly doubles top-1 accuracy where no LLM is in the loop (triage bots, CI, small local models). A frontier agent with grep was already at 85%. Details, statistics and limitations are in [docs/findings.md](docs/findings.md).

## Use it with your tools

<details open>
<summary><b>Coding agents (MCP)</b></summary>

`gl mcp` serves `locate`, `search` and `symbol` over stdio. It re-scans on every call (cheap thanks to the cache), so results stay fresh while the agent edits.

```sh
claude mcp add gl -- gl mcp                                        # Claude Code
```
```json
{ "mcpServers": { "gl": { "command": "gl", "args": ["mcp"] } } }
```
<sub>Cursor: <code>.cursor/mcp.json</code>. Our A/B shows agents rarely pick a new tool unprompted, so tell yours to start with <code>locate</code>.</sub>
</details>

<details>
<summary><b>Claude Code hook: shortlist injected before the agent starts</b></summary>

Agents rarely choose to call a new tool, so the hook doesn't wait for one. `gl hook` runs on every prompt. When the ranking is confident, it adds the top files to the model's context (about 300 tokens, no tool call). Otherwise it does nothing.

```json
{ "hooks": { "UserPromptSubmit": [ { "hooks": [ { "type": "command", "command": "gl hook" } ] } ] } }
```
<sub>Add to <code>.claude/settings.json</code>. Whether this helps agents is exactly what the pilot experiment measures; see findings.</sub>
</details>

<details>
<summary><b>Issue triage bot (GitHub Action)</b></summary>

This posts the top-k likely files as a comment on every new issue. It's where `gl` measurably helps, because no LLM is in the loop.

```yaml
# .github/workflows/issue-triage.yml in the repo you want triaged
on: { issues: { types: [opened] } }
permissions: { issues: write, contents: read }
jobs:
  suggest-files:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: soham504/gl-code-search@main
        with: { k: "5" }
```
<sub>The comment format is tested locally; the Action hasn't been run on a real repository yet.</sub>
</details>

## How it works

```mermaid
flowchart LR
    I["Issue text"] --> Q["Signals: stack-trace frames, file paths, dotted modules, backticked symbols"]
    I --> A
    R[("Repository")] --> A["Code-aware analyzer: camelCase / snake_case splitting, stemming, symbol + path fields"]
    A --> X[("BM25 index + symbol table, cached by file mtime")]
    X --> S["Score = BM25 + signal boosts − test-file penalty"]
    Q --> S
    S --> O["Ranked files + reasons"]
    O --> C1["CLI"] & C2["MCP server"] & C3["GitHub Action"]
```

<details>
<summary><b>Reproduce everything</b></summary>

```sh
make test                                                   # Go + Python unit tests
python3 -m venv .venv && .venv/bin/pip install pyarrow -e '.[dev]'
.venv/bin/python harness/datasets.py                        # 8 public benchmarks → data/bench/
go build -o bin/locbench ./go/cmd/locbench
./bin/locbench -data data/bench/swebench_verified.jsonl -repos data/repos -name verified -fetch -j 8
python3 harness/mine.py > data/fresh/instances.jsonl        # fresh post-cutoff issues from git history
.venv/bin/python -m groundline.locsummary                   # cross-benchmark table
.venv/bin/python -m groundline.readme_assets                # regenerate the charts on this page
```
The agent A/B prompt and workflow are in [`harness/agent_ab.workflow.js`](harness/agent_ab.workflow.js); the dispatcher that enforces each toolkit is [`harness/tool.py`](harness/tool.py).
</details>

<details>
<summary><b>Repository layout</b></summary>

| Path | What |
|---|---|
| `go/internal/code` | Analyzer, symbol extraction, issue signals, ranking, analysis cache, git-object loader |
| `go/internal/index` | Inverted index + BM25 |
| `go/cmd/gl` | CLI and MCP server |
| `go/cmd/locbench` | Offline localization benchmark with ablations; fetches single commits on demand |
| `action.yml` | GitHub Action |
| `harness/` | Dataset normalisation, fresh-issue mining, agent tool dispatcher, A/B workflow |
| `py/groundline/` | A/B statistics, cross-benchmark summary, chart generation |
| `eval/results/` | Every report and per-instance result |
| `docs/findings.md` | Full write-up with limitations |
| `go/cmd/bench`, `go/cmd/searchd`, `docs/archive/` | Earlier work: BM25 on BEIR, a self-graded grounding pilot (superseded) |
</details>

## Related work

IR-based bug localization is an established research area. `gl` builds on it rather than inventing it:
- **BugLocator** (Zhou et al., ICSE 2012) ranks files by bug-report similarity.
- **BRTracer** (Wong et al., ICSME 2014) adds stack traces and segmentation.
- **Locus** (Wen et al., ASE 2016) adds change histories.
- **ABLoTS** (2024 replication) learns to combine signals.
- In the LLM era, **Agentless** (Xia et al., 2024) localizes hierarchically with LLMs, **Aider's repo map** injects a tree-sitter + PageRank summary into every prompt, and **SweRank** (2025) trains code rankers.

What's different here:
- **No LLM or training at all.** It's one Go binary, with multi-language support and an incremental cache.
- **Evaluated at scale.** 10 public and fresh benchmarks, with confidence intervals and ablations.
- **Shipped as agent infrastructure.** MCP, a context-injection hook and an Action.
- **Measured against agents.** A controlled A/B shows whether agents benefit at all.

## Limitations

- **Small agent study:** 20 issues per condition and one model; tools were offered, not required.
- **Simple symbol extraction:** it's regex-based. It finds types but not methods in Java and C#.
- **Fresh-issue gold isn't hand-checked:** it comes from `Fixes #N` commits.

See [docs/findings.md](docs/findings.md#limitations) for the rest.

<sub>MIT licensed. Built with Go, plain BM25, and careful measurement.</sub>
