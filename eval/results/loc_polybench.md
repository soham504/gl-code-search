## polybench: file localization, n=500

| System | Acc@1 | Acc@3 | Acc@5 | Acc@10 | p50 index+rank | p99 |
|---|---|---|---|---|---|---|
| bm25_plain | 13.2% | 24.4% | 33.4% | 44.0% | 526ms | 8.939s |
| gl_text_only | 17.8% | 32.0% | 41.2% | 51.8% | 629ms | 11.704s |
| gl_no_test_penalty | 23.0% | 36.4% | 44.4% | 54.4% | 629ms | 11.705s |
| gl_no_trace_paths | 24.0% | 39.4% | 49.0% | 61.2% | 630ms | 11.707s |
| gl_no_symbols | 26.8% | 40.2% | 49.8% | 61.6% | 631ms | 11.71s |
| gl_full | 27.4% | 41.6% | 50.8% | 62.4% | 630ms | 11.707s |
