# LIVE recovery-flow observation (real 4-node docker testnet)

Window: 60 min requested, 4.4 min actually run.

Samples: 216, CSV: `recovery_flow_20260808T104651.csv`

## Real transitions observed

| t (s) | Node | From | To |
|---:|---|---|---|
| 261 | engram-node01 | SOVEREIGN | RECOVERING |
| 261 | engram-node02 | SOVEREIGN | RECOVERING |
| 261 | engram-node03 | SOVEREIGN | RECOVERING |
| 261 | engram-node04 | SOVEREIGN | RECOVERING |
| 266 | engram-node01 | RECOVERING | ANCHORED |
| 266 | engram-node02 | RECOVERING | ANCHORED |
| 266 | engram-node03 | RECOVERING | ANCHORED |
| 266 | engram-node04 | RECOVERING | ANCHORED |

## ANCHORED reached

- engram-node01: reached ANCHORED at t=266s
- engram-node02: reached ANCHORED at t=266s
- engram-node03: reached ANCHORED at t=266s
- engram-node04: reached ANCHORED at t=266s
