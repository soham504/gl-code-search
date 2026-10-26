# File localization across benchmarks

Gold = first non-test source file changed by the reference fix found in the ranking. Δ is gl − BM25 in percentage points with a paired 95% bootstrap CI.

| Benchmark | n | Repos | BM25 Acc@1 | gl Acc@1 | ΔAcc@1 [95% CI] | BM25 Acc@5 | gl Acc@5 | ΔAcc@5 [95% CI] |
|---|---|---|---|---|---|---|---|---|
| SWE-bench Lite | 300 | 12 | 25.7% | **48.3%** | +22.7 [+17.3, +28.0] | 55.0% | **72.0%** | +17.0 [+12.7, +21.3] |
| SWE-bench Verified | 500 | 12 | 24.0% | **49.2%** | +25.2 [+21.0, +29.4] | 54.8% | **76.0%** | +21.2 [+17.4, +25.2] |
| SWE-bench (full test) | 2294 | 12 | 25.9% | **48.4%** | +22.5 [+20.5, +24.4] | 56.2% | **75.0%** | +18.8 [+17.1, +20.5] |
| Fresh post-cutoff issues (ours) | 28 | 7 | 28.6% | **50.0%** | +21.4 [+0.0, +42.9] | 57.1% | **71.4%** | +14.3 [+0.0, +32.1] |

Total instance evaluations: 3122 (benchmarks overlap: Lite and most of Verified are inside SWE-bench full).

## By language (pooled over benchmarks, excluding Lite/full duplicates)

| Language | n | BM25 Acc@1 | gl Acc@1 | BM25 Acc@5 | gl Acc@5 |
|---|---|---|---|---|---|
| python | 528 | 24.2% | 49.2% | 54.9% | 75.8% |
