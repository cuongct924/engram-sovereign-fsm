# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1336 | S4_p2p_eclipse_partial | 6 |
| 1352 | S4_p2p_eclipse_partial | 6 |
| 1369 | S4_p2p_eclipse_partial | 6 |
| 1383 | S4_p2p_eclipse_partial | 6 |
| 1398 | S4_p2p_eclipse_partial | 6 |
| 1414 | S4_p2p_eclipse_partial | 6 |
| 1428 | S4_p2p_eclipse_partial | 6 |
| 1443 | S4_p2p_eclipse_partial | 6 |
| 1454 | S4_p2p_eclipse_partial | 6 |
| 1465 | S4_p2p_eclipse_partial | 6 |
| 1985 | S5_anchor_isolation | 6 |
| 1997 | S5_anchor_isolation | 6 |
| 2009 | S5_anchor_isolation | 6 |
| 2021 | S5_anchor_isolation | 6 |
| 2034 | S5_anchor_isolation | 6 |
| 2046 | S5_anchor_isolation | 6 |
| 2058 | S5_anchor_isolation | 6 |
| 2070 | S5_anchor_isolation | 6 |
| 2083 | S5_anchor_isolation | 6 |
| 2095 | S5_anchor_isolation | 6 |
| 2107 | S5_anchor_isolation | 6 |
| 2119 | S5_anchor_isolation | 6 |
| 2131 | S5_anchor_isolation | 6 |
| 2144 | S5_anchor_isolation | 6 |
| 2156 | S5_anchor_isolation | 6 |
| 2168 | S5_anchor_isolation | 6 |
