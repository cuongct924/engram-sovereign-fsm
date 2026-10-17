# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 477 | S4_p2p_eclipse_partial | 6 |
| 497 | S4_p2p_eclipse_partial | 6 |
| 512 | S4_p2p_eclipse_partial | 6 |
| 526 | S4_p2p_eclipse_partial | 6 |
| 541 | S4_p2p_eclipse_partial | 6 |
| 555 | S4_p2p_eclipse_partial | 6 |
| 570 | S4_p2p_eclipse_partial | 6 |
| 582 | S4_p2p_eclipse_partial | 6 |
| 595 | S4_p2p_eclipse_partial | 6 |
| 700 | S5_anchor_isolation | 6 |
| 710 | S5_anchor_isolation | 6 |
| 720 | S5_anchor_isolation | 6 |
| 730 | S5_anchor_isolation | 6 |
| 741 | S5_anchor_isolation | 6 |
| 751 | S5_anchor_isolation | 6 |
| 761 | S5_anchor_isolation | 6 |
| 771 | S5_anchor_isolation | 6 |
| 782 | S5_anchor_isolation | 6 |
| 792 | S5_anchor_isolation | 6 |
| 802 | S5_anchor_isolation | 6 |
| 813 | S5_anchor_isolation | 6 |
| 827 | S5_anchor_isolation | 6 |
| 837 | S5_anchor_isolation | 6 |
| 851 | S5_anchor_isolation | 6 |
| 865 | S5_anchor_isolation | 6 |
| 878 | S5_anchor_isolation | 6 |
