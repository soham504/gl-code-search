SWE-bench Lite file localization, n=300 (gold file in top k)

| System | Acc@1 | Acc@3 | Acc@5 | Acc@10 | p50 build+rank | p99 |
|---|---|---|---|---|---|---|
| bm25_plain | 27.3% | 46.7% | 54.7% | 67.7% | 905ms | 2.629s |
| gl_text_only | 39.3% | 56.0% | 63.3% | 72.7% | 1.106s | 3.102s |
| gl_no_test_penalty | 45.0% | 61.3% | 66.7% | 75.3% | 1.106s | 3.102s |
| gl_no_trace_paths | 47.0% | 63.7% | 70.3% | 78.3% | 1.106s | 3.102s |
| gl_no_symbols | 47.0% | 64.0% | 70.3% | 78.0% | 1.106s | 3.102s |
| gl_full | 48.7% | 65.0% | 71.7% | 79.0% | 1.106s | 3.102s |
