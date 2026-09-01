# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 388 | S4_p2p_eclipse_partial | 6 |
| 406 | S4_p2p_eclipse_partial | 6 |
| 422 | S4_p2p_eclipse_partial | 6 |
| 441 | S4_p2p_eclipse_partial | 6 |
| 458 | S4_p2p_eclipse_partial | 6 |
| 476 | S4_p2p_eclipse_partial | 6 |
| 493 | S4_p2p_eclipse_partial | 6 |
| 505 | S4_p2p_eclipse_partial | 6 |
| 552 | S5_anchor_isolation | 6 |
| 564 | S5_anchor_isolation | 6 |
| 576 | S5_anchor_isolation | 6 |
| 589 | S5_anchor_isolation | 6 |
| 601 | S5_anchor_isolation | 6 |
| 613 | S5_anchor_isolation | 6 |
| 626 | S5_anchor_isolation | 6 |
| 638 | S5_anchor_isolation | 6 |
| 650 | S5_anchor_isolation | 6 |
| 663 | S5_anchor_isolation | 6 |
| 675 | S5_anchor_isolation | 6 |
| 687 | S5_anchor_isolation | 6 |
| 699 | S5_anchor_isolation | 6 |
| 711 | S5_anchor_isolation | 6 |
| 724 | S5_anchor_isolation | 6 |
| 736 | S5_anchor_isolation | 6 |
