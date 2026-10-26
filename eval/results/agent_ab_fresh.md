# Agent localization A/B (n = 20 issues with all 3 runs)

| Toolkit | Acc@1 | Acc@5 | Tool calls | read calls | Files opened | Wrong files opened | Tool output (k chars) | gl/graphify calls |
|---|---|---|---|---|---|---|---|---|
| grep | 85% | 90% | 4.6 | 1.8 | 1.4 | 0.6 | 9.1 | 0.0 |
| graphify | 85% | 90% | 5.8 | 1.9 | 1.5 | 0.7 | 8.9 | 0.0 |
| gl | 85% | 90% | 4.4 | 1.4 | 1.2 | 0.6 | 8.1 | 0.3 |

## Paired differences vs grep-only (mean, 95% bootstrap CI; McNemar p for accuracy)

| Toolkit | ΔAcc@1 | ΔAcc@5 | ΔTool calls | ΔWrong files opened | ΔTool output (k chars) |
|---|---|---|---|---|---|
| graphify | +0.0 [+0.0, +0.0] p=1.00 | +0.0 [+0.0, +0.0] p=1.00 | +1.1 [-0.1, +2.9] | +0.1 [-0.1, +0.4] | -0.2 [-1.4, +1.3] |
| gl | +0.0 [+0.0, +0.0] p=1.00 | +0.0 [+0.0, +0.0] p=1.00 | -0.2 [-0.9, +0.7] | +0.0 [-0.3, +0.3] | -1.0 [-2.6, +0.5] |

Accuracy deltas are in percentage points; tool output is in thousands of characters (≈ tokens × 4) returned to the agent by tools.
