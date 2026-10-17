# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 474 | S4_p2p_eclipse_partial | 6 |
| 490 | S4_p2p_eclipse_partial | 6 |
| 505 | S4_p2p_eclipse_partial | 6 |
| 517 | S4_p2p_eclipse_partial | 6 |
| 531 | S4_p2p_eclipse_partial | 6 |
| 542 | S4_p2p_eclipse_partial | 6 |
| 556 | S4_p2p_eclipse_partial | 6 |
| 570 | S4_p2p_eclipse_partial | 6 |
| 580 | S4_p2p_eclipse_partial | 6 |
| 593 | S4_p2p_eclipse_partial | 6 |
| 703 | S5_anchor_isolation | 6 |
| 715 | S5_anchor_isolation | 6 |
| 727 | S5_anchor_isolation | 6 |
| 740 | S5_anchor_isolation | 6 |
| 752 | S5_anchor_isolation | 6 |
| 764 | S5_anchor_isolation | 6 |
| 776 | S5_anchor_isolation | 6 |
| 788 | S5_anchor_isolation | 6 |
| 801 | S5_anchor_isolation | 6 |
| 813 | S5_anchor_isolation | 6 |
| 825 | S5_anchor_isolation | 6 |
| 837 | S5_anchor_isolation | 6 |
| 849 | S5_anchor_isolation | 6 |
| 861 | S5_anchor_isolation | 6 |
| 874 | S5_anchor_isolation | 6 |
| 886 | S5_anchor_isolation | 6 |
