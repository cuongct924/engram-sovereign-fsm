# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1327 | S4_p2p_eclipse_partial | 6 |
| 1338 | S4_p2p_eclipse_partial | 6 |
| 1353 | S4_p2p_eclipse_partial | 6 |
| 1365 | S4_p2p_eclipse_partial | 6 |
| 1376 | S4_p2p_eclipse_partial | 6 |
| 1387 | S4_p2p_eclipse_partial | 6 |
| 1397 | S4_p2p_eclipse_partial | 6 |
| 1412 | S4_p2p_eclipse_partial | 6 |
| 1423 | S4_p2p_eclipse_partial | 6 |
| 1434 | S4_p2p_eclipse_partial | 6 |
| 1446 | S4_p2p_eclipse_partial | 6 |
| 1999 | S5_anchor_isolation | 6 |
| 2011 | S5_anchor_isolation | 6 |
| 2023 | S5_anchor_isolation | 6 |
| 2035 | S5_anchor_isolation | 6 |
| 2048 | S5_anchor_isolation | 6 |
| 2060 | S5_anchor_isolation | 6 |
| 2072 | S5_anchor_isolation | 6 |
| 2084 | S5_anchor_isolation | 6 |
| 2096 | S5_anchor_isolation | 6 |
| 2108 | S5_anchor_isolation | 6 |
| 2121 | S5_anchor_isolation | 6 |
| 2133 | S5_anchor_isolation | 6 |
| 2145 | S5_anchor_isolation | 6 |
| 2157 | S5_anchor_isolation | 6 |
| 2170 | S5_anchor_isolation | 6 |
| 2182 | S5_anchor_isolation | 6 |
