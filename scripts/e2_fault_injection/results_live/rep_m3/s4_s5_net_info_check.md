# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 1194 | S4_p2p_eclipse_partial | 6 |
| 1210 | S4_p2p_eclipse_partial | 6 |
| 1226 | S4_p2p_eclipse_partial | 6 |
| 1240 | S4_p2p_eclipse_partial | 6 |
| 1252 | S4_p2p_eclipse_partial | 6 |
| 1263 | S4_p2p_eclipse_partial | 6 |
| 1277 | S4_p2p_eclipse_partial | 6 |
| 1288 | S4_p2p_eclipse_partial | 6 |
| 1300 | S4_p2p_eclipse_partial | 6 |
| 1312 | S4_p2p_eclipse_partial | 6 |
| 1368 | S5_anchor_isolation | 6 |
| 1380 | S5_anchor_isolation | 6 |
| 1393 | S5_anchor_isolation | 6 |
| 1405 | S5_anchor_isolation | 6 |
| 1417 | S5_anchor_isolation | 6 |
| 1429 | S5_anchor_isolation | 6 |
| 1442 | S5_anchor_isolation | 6 |
| 1454 | S5_anchor_isolation | 6 |
| 1466 | S5_anchor_isolation | 6 |
| 1478 | S5_anchor_isolation | 6 |
| 1490 | S5_anchor_isolation | 6 |
| 1503 | S5_anchor_isolation | 6 |
| 1515 | S5_anchor_isolation | 6 |
| 1527 | S5_anchor_isolation | 6 |
| 1539 | S5_anchor_isolation | 6 |
| 1552 | S5_anchor_isolation | 6 |
