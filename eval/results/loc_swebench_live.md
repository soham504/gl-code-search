## swebench_live: file localization, n=500

| System | Acc@1 | Acc@3 | Acc@5 | Acc@10 | p50 index+rank | p99 |
|---|---|---|---|---|---|---|
| bm25_plain | 28.0% | 44.6% | 55.8% | 69.8% | 406ms | 2.833s |
| gl_text_only | 30.6% | 53.0% | 61.0% | 71.4% | 513ms | 4.502s |
| gl_no_test_penalty | 37.8% | 57.2% | 64.8% | 73.0% | 514ms | 4.503s |
| gl_no_trace_paths | 41.4% | 59.6% | 67.8% | 78.8% | 514ms | 4.505s |
| gl_no_symbols | 41.2% | 60.6% | 67.8% | 78.4% | 515ms | 4.505s |
| gl_full | 43.4% | 62.4% | 68.8% | 79.2% | 515ms | 4.507s |
