# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 476 | S4_p2p_eclipse_partial | 6 |
| 493 | S4_p2p_eclipse_partial | 6 |
| 512 | S4_p2p_eclipse_partial | 6 |
| 526 | S4_p2p_eclipse_partial | 6 |
| 541 | S4_p2p_eclipse_partial | 6 |
| 554 | S4_p2p_eclipse_partial | 6 |
| 568 | S4_p2p_eclipse_partial | 6 |
| 583 | S4_p2p_eclipse_partial | 6 |
| 598 | S4_p2p_eclipse_partial | 6 |
| 712 | S5_anchor_isolation | 6 |
| 723 | S5_anchor_isolation | 6 |
| 733 | S5_anchor_isolation | 6 |
| 746 | S5_anchor_isolation | 6 |
| 758 | S5_anchor_isolation | 6 |
| 772 | S5_anchor_isolation | 6 |
| 786 | S5_anchor_isolation | 6 |
| 800 | S5_anchor_isolation | 6 |
| 810 | S5_anchor_isolation | 6 |
| 824 | S5_anchor_isolation | 6 |
| 837 | S5_anchor_isolation | 6 |
| 852 | S5_anchor_isolation | 6 |
| 866 | S5_anchor_isolation | 6 |
| 879 | S5_anchor_isolation | 6 |
| 893 | S5_anchor_isolation | 6 |
