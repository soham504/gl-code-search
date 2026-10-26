# Agent localization A/B (n = 20 issues with all 3 runs)

| Toolkit | Acc@1 | Acc@5 | Tool calls | read calls | Files opened | Wrong files opened | Tool output (k chars) | gl/graphify calls |
|---|---|---|---|---|---|---|---|---|
| grep | 100% | 100% | 4.3 | 0.8 | 0.7 | 0.1 | 2.9 | 0.0 |
| graphify | 100% | 100% | 3.0 | 0.6 | 0.6 | 0.1 | 3.1 | 0.0 |
| gl | 100% | 100% | 3.0 | 0.7 | 0.6 | 0.1 | 2.9 | 0.0 |

## Paired differences vs grep-only (mean, 95% bootstrap CI; McNemar p for accuracy)

| Toolkit | ΔAcc@1 | ΔAcc@5 | ΔTool calls | ΔWrong files opened | ΔTool output (k chars) |
|---|---|---|---|---|---|
| graphify | +0.0 [+0.0, +0.0] p=1.00 | +0.0 [+0.0, +0.0] p=1.00 | -1.2 [-2.4, -0.5] | +0.0 [+0.0, +0.0] | +0.2 [-0.4, +0.9] |
| gl | +0.0 [+0.0, +0.0] p=1.00 | +0.0 [+0.0, +0.0] p=1.00 | -1.2 [-2.4, -0.5] | +0.0 [+0.0, +0.0] | -0.0 [-0.6, +0.7] |

Accuracy deltas are in percentage points; tool output is in thousands of characters (≈ tokens × 4) returned to the agent by tools.
