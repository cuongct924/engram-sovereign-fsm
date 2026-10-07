# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1287 | S4_p2p_eclipse_partial | 6 |
| 1298 | S4_p2p_eclipse_partial | 6 |
| 1310 | S4_p2p_eclipse_partial | 6 |
| 1323 | S4_p2p_eclipse_partial | 6 |
| 1338 | S4_p2p_eclipse_partial | 6 |
| 1350 | S4_p2p_eclipse_partial | 6 |
| 1362 | S4_p2p_eclipse_partial | 6 |
| 1374 | S4_p2p_eclipse_partial | 6 |
| 1385 | S4_p2p_eclipse_partial | 6 |
| 1398 | S4_p2p_eclipse_partial | 6 |
| 1409 | S4_p2p_eclipse_partial | 6 |
| 1920 | S5_anchor_isolation | 6 |
| 1933 | S5_anchor_isolation | 6 |
| 1945 | S5_anchor_isolation | 6 |
| 1957 | S5_anchor_isolation | 6 |
| 1969 | S5_anchor_isolation | 6 |
| 1982 | S5_anchor_isolation | 6 |
| 1994 | S5_anchor_isolation | 6 |
| 2006 | S5_anchor_isolation | 6 |
| 2018 | S5_anchor_isolation | 6 |
| 2031 | S5_anchor_isolation | 6 |
| 2043 | S5_anchor_isolation | 6 |
| 2055 | S5_anchor_isolation | 6 |
| 2067 | S5_anchor_isolation | 6 |
| 2080 | S5_anchor_isolation | 6 |
| 2092 | S5_anchor_isolation | 6 |
| 2104 | S5_anchor_isolation | 6 |
