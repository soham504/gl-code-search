## swe_rebench: file localization, n=500

| System | Acc@1 | Acc@3 | Acc@5 | Acc@10 | p50 index+rank | p99 |
|---|---|---|---|---|---|---|
| bm25_plain | 38.4% | 67.6% | 77.0% | 85.2% | 69ms | 2.731s |
| gl_text_only | 44.4% | 71.0% | 79.0% | 87.6% | 84ms | 3.262s |
| gl_no_test_penalty | 52.8% | 75.2% | 81.6% | 88.6% | 84ms | 3.262s |
| gl_no_trace_paths | 56.2% | 78.6% | 84.4% | 90.8% | 85ms | 3.262s |
| gl_no_symbols | 56.4% | 78.0% | 84.6% | 90.8% | 85ms | 3.262s |
| gl_full | 58.8% | 79.4% | 85.2% | 91.2% | 85ms | 3.262s |
