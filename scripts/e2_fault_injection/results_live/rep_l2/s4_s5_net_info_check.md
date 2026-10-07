# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1321 | S4_p2p_eclipse_partial | 6 |
| 1334 | S4_p2p_eclipse_partial | 6 |
| 1345 | S4_p2p_eclipse_partial | 6 |
| 1359 | S4_p2p_eclipse_partial | 6 |
| 1371 | S4_p2p_eclipse_partial | 6 |
| 1385 | S4_p2p_eclipse_partial | 6 |
| 1396 | S4_p2p_eclipse_partial | 6 |
| 1408 | S4_p2p_eclipse_partial | 6 |
| 1419 | S4_p2p_eclipse_partial | 6 |
| 1433 | S4_p2p_eclipse_partial | 6 |
| 1446 | S4_p2p_eclipse_partial | 6 |
| 1978 | S5_anchor_isolation | 6 |
| 1993 | S5_anchor_isolation | error: timed out |
| 2011 | S5_anchor_isolation | error: timed out |
| 2029 | S5_anchor_isolation | error: timed out |
| 2048 | S5_anchor_isolation | error: timed out |
| 2066 | S5_anchor_isolation | error: timed out |
| 2084 | S5_anchor_isolation | error: timed out |
| 2102 | S5_anchor_isolation | error: timed out |
| 2121 | S5_anchor_isolation | error: timed out |
| 2139 | S5_anchor_isolation | error: timed out |
| 2153 | S5_anchor_isolation | 6 |
| 2165 | S5_anchor_isolation | 6 |
