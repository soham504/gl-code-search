# gl-code-search: code search for bug localization, and does it help agents?

[![ci](https://github.com/soham504/gl-code-search/actions/workflows/ci.yml/badge.svg)](https://github.com/soham504/gl-code-search/actions/workflows/ci.yml) · MIT licensed

Given a bug report, which files need to change? Engineers triaging an issue and AI coding agents both start here.

This repo contains:
- **`gl`**: a code-aware search engine in Go that ranks files for an issue, in about 0.2–1 s on CPU with no LLM;
- an **evaluation harness** that tests whether better search tools (ours, and the popular [Graphify](https://github.com/Graphify-Labs/graphify) knowledge graph) make coding agents better.

Ground truth everywhere is the files changed by the real fix, never our own labels. **Full write-up: [docs/findings.md](docs/findings.md).**

## Results
| | Plain BM25 | **gl** |
|---|---|---|
| SWE-bench Lite (n=300), Acc@1 / Acc@5 | 27.3% / 54.7% | **48.7% / 71.7%** |
| Fresh issues opened after 2026-07-14 (n=28), Acc@1 / Acc@5 | 28.6% / 57.1% | **50.0% / 71.4%** |

**Agent A/B (20 issues × 3 toolkits):** grep vs grep + Graphify vs grep + gl.
- **On SWE-bench Lite**, every toolkit scored 100%. The agent already knew where the bugs were, so the benchmark is memorised and can't measure search tooling.
- **On fresh issues the model can't have memorised**, every toolkit scored 85% Acc@1. The agent almost never called the extra tools: Graphify 0 times, gl 0.3 times per run.

Better search did not make a frontier agent better at localization. It does make a non-LLM triage pipeline nearly 2× better at top-1.

## Use it
```sh
go install github.com/soham504/gl-code-search/go/cmd/gl@latest   # Go 1.22+
cd your-repo
gl locate < issue.txt                # ranked likely files, with reasons
gl search "date field serialization" # ranked files + best-matching lines
gl symbol QuerySet                   # where it's defined
gl -k 5 -markdown locate < issue.txt # issue-comment-ready list (also -json)
```
Languages: Python, JS/TS, Go, Rust, Java, Kotlin, Scala, C/C++, C#, Ruby, PHP. Symbol extraction is regex-based: Python, JS/TS, Go, Rust and Ruby work well; it finds Java/C# types but not methods.

The first run on a repo analyses every file. Later runs reuse an on-disk cache (`~/Library/Caches/groundline` or `$XDG_CACHE_HOME/groundline`) keyed by file size and mtime, so only edited files are re-analysed. On Django's 2,600 files that's 3.9 s cold and 0.25 s warm.

**For coding agents (MCP).** `gl mcp` serves `locate`, `search` and `symbol` over stdio, and re-scans on every call so results stay fresh while the agent edits.
- Claude Code: `claude mcp add gl -- gl mcp`
- Cursor (`.cursor/mcp.json`): `{"mcpServers": {"gl": {"command": "gl", "args": ["mcp"]}}}`

Our A/B found that agents rarely choose this tool unprompted (see findings), so tell the agent to start with `locate` if you want it used.

**Issue triage bot (GitHub Action).** `action.yml` comments the top-k likely files on every new issue. Copy [docs/examples/issue-triage.yml](docs/examples/issue-triage.yml) into the target repo. This is the setting where `gl` measurably helps, because no LLM is in the loop. The Action hasn't been run on GitHub yet; the comment output was tested locally.

## Layout
| Path | What |
|---|---|
| `go/internal/code` | Code-aware analyzer (`camelCase`/`snake_case` splitting), symbol index, issue signals (stack traces, paths, modules, backticked symbols), test-file penalty, git-object loader |
| `go/internal/index` | Inverted index + BM25 shared by all tools |
| `go/cmd/gl` | CLI and MCP server: `locate`, `search`, `symbol`, `mcp`, with the analysis cache in `go/internal/code/cache.go` |
| `action.yml` | GitHub Action: issue-triage comment |
| `harness/datasets.py` | Downloads and normalises 8 public localization benchmarks |
| `go/cmd/locbench` | Offline localization benchmark with ablations, reading repos at the base commit straight from git objects |
| `harness/tool.py` | Logged tool dispatcher that enforces each toolkit with identical output caps |
| `harness/mine.py` | Mines fresh post-cutoff instances from git history (`Fixes #N` commits) |
| `harness/agent_ab.workflow.js` | The exact agent prompt and workflow used for the A/B |
| `py/groundline/agentab.py` | Paired A/B analysis: bootstrap CIs and McNemar tests |
| `eval/results/` | All reports: `loc_lite.md`, `loc_fresh.md`, `agent_ab_lite.md`, `agent_ab_fresh.md`, plus per-run JSON |
| `go/cmd/bench`, `go/cmd/searchd` | Earlier BM25-on-BEIR work (SciFact nDCG@10 0.694, FiQA 0.249) and a small results page |
| `docs/archive/` | An earlier, self-graded grounding pilot, kept for the record and superseded |

## Reproduce
```sh
brew install go python@3.12
make test                                   # Go + Python unit tests
# SWE-bench Lite offline benchmark (clones 12 repos, ~2 GB)
python3 -m venv .venv && .venv/bin/pip install pyarrow pytest -e .
# see docs/findings.md for the data steps; then:
go build -o bin/locbench ./go/cmd/locbench && ./bin/locbench -data data/swebench/lite.jsonl -repos data/repos
python3 harness/mine.py > data/fresh/instances.jsonl   # fresh instances (unauthenticated GitHub API, 55 calls)
./bin/locbench -data data/fresh/instances.jsonl -repos data/fresh/repos
```
