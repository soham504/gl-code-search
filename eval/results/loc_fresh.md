## fresh: file localization, n=28

| System | Acc@1 | Acc@3 | Acc@5 | Acc@10 | p50 index+rank | p99 |
|---|---|---|---|---|---|---|
| bm25_plain | 28.6% | 46.4% | 57.1% | 75.0% | 321ms | 652ms |
| gl_text_only | 39.3% | 53.6% | 60.7% | 78.6% | 393ms | 762ms |
| gl_no_test_penalty | 42.9% | 64.3% | 67.9% | 78.6% | 394ms | 762ms |
| gl_no_trace_paths | 50.0% | 64.3% | 67.9% | 82.1% | 394ms | 762ms |
| gl_no_symbols | 46.4% | 64.3% | 71.4% | 85.7% | 394ms | 762ms |
| gl_full | 50.0% | 67.9% | 71.4% | 85.7% | 394ms | 762ms |
