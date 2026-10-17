# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 481 | S4_p2p_eclipse_partial | 6 |
| 502 | S4_p2p_eclipse_partial | 6 |
| 520 | S4_p2p_eclipse_partial | 6 |
| 533 | S4_p2p_eclipse_partial | 6 |
| 548 | S4_p2p_eclipse_partial | 6 |
| 565 | S4_p2p_eclipse_partial | 6 |
| 577 | S4_p2p_eclipse_partial | 6 |
| 590 | S4_p2p_eclipse_partial | 6 |
| 601 | S4_p2p_eclipse_partial | 6 |
| 709 | S5_anchor_isolation | 6 |
| 721 | S5_anchor_isolation | 6 |
| 733 | S5_anchor_isolation | 6 |
| 746 | S5_anchor_isolation | 6 |
| 758 | S5_anchor_isolation | 6 |
| 770 | S5_anchor_isolation | 6 |
| 782 | S5_anchor_isolation | 6 |
| 794 | S5_anchor_isolation | 6 |
| 807 | S5_anchor_isolation | 6 |
| 819 | S5_anchor_isolation | 6 |
| 831 | S5_anchor_isolation | 6 |
| 843 | S5_anchor_isolation | 6 |
| 855 | S5_anchor_isolation | 6 |
| 868 | S5_anchor_isolation | 6 |
| 880 | S5_anchor_isolation | 6 |
| 892 | S5_anchor_isolation | 6 |
