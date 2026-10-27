# File localization across benchmarks

Gold = first non-test source file changed by the reference fix found in the ranking. Δ is gl − BM25 in percentage points with a paired 95% bootstrap CI.

| Benchmark | n | Repos | BM25 Acc@1 | gl Acc@1 | ΔAcc@1 [95% CI] | BM25 Acc@5 | gl Acc@5 | ΔAcc@5 [95% CI] |
|---|---|---|---|---|---|---|---|---|
| SWE-bench Lite | 300 | 12 | 25.7% | **48.3%** | +22.7 [+17.3, +28.0] | 55.0% | **72.0%** | +17.0 [+12.7, +21.3] |
| SWE-bench Verified | 500 | 12 | 24.0% | **49.2%** | +25.2 [+21.0, +29.4] | 54.8% | **76.0%** | +21.2 [+17.4, +25.2] |
| SWE-bench (full test) | 2294 | 12 | 25.9% | **48.4%** | +22.5 [+20.5, +24.4] | 56.2% | **75.0%** | +18.8 [+17.1, +20.5] |
| SWE-Gym (sample) | 479 | 11 | 18.0% | **41.3%** | +23.4 [+19.2, +27.6] | 52.2% | **73.7%** | +21.5 [+17.5, +25.7] |
| LocBench | 560 | 165 | 25.7% | **49.3%** | +23.6 [+19.5, +27.7] | 55.0% | **75.0%** | +20.0 [+16.6, +23.4] |
| SWE-bench-Live (sample) | 500 | 133 | 28.0% | **43.4%** | +15.4 [+11.4, +19.4] | 55.8% | **68.8%** | +13.0 [+9.8, +16.4] |
| SWE-rebench (sample) | 500 | 351 | 38.4% | **58.8%** | +20.4 [+16.2, +24.6] | 77.0% | **85.2%** | +8.2 [+5.6, +11.0] |
| SWE-bench Multilingual | 299 | 41 | 18.7% | **36.1%** | +17.4 [+13.0, +22.1] | 43.8% | **62.9%** | +19.1 [+14.4, +24.1] |
| SWE-PolyBench (sample) | 500 | 19 | 13.2% | **27.4%** | +14.2 [+10.8, +17.6] | 33.4% | **50.8%** | +17.4 [+13.6, +21.4] |
| Fresh post-cutoff issues (ours) | 28 | 7 | 28.6% | **50.0%** | +21.4 [+0.0, +42.9] | 57.1% | **71.4%** | +14.3 [+0.0, +32.1] |

Total instance evaluations: 5960 (benchmarks overlap: Lite and most of Verified are inside SWE-bench full).

## By language (pooled over benchmarks, excluding Lite/full duplicates)

| Language | n | BM25 Acc@1 | gl Acc@1 | BM25 Acc@5 | gl Acc@5 |
|---|---|---|---|---|---|
| python | 2601 | 26.8% | 48.7% | 58.6% | 75.5% |
| javascript | 264 | 14.4% | 28.8% | 34.5% | 54.2% |
| typescript | 195 | 8.7% | 14.4% | 24.6% | 36.4% |
| java | 85 | 20.0% | 38.8% | 49.4% | 69.4% |
| rust | 46 | 15.2% | 32.6% | 47.8% | 65.2% |
| c | 46 | 34.8% | 39.1% | 50.0% | 69.6% |
| ruby | 44 | 18.2% | 34.1% | 38.6% | 56.8% |
| php | 43 | 16.3% | 46.5% | 53.5% | 69.8% |
| go | 41 | 14.6% | 41.5% | 46.3% | 70.7% |
