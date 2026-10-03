# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 482 | S4_p2p_eclipse_partial | 6 |
| 494 | S4_p2p_eclipse_partial | 6 |
| 507 | S4_p2p_eclipse_partial | 6 |
| 519 | S4_p2p_eclipse_partial | 6 |
| 533 | S4_p2p_eclipse_partial | 6 |
| 545 | S4_p2p_eclipse_partial | 6 |
| 557 | S4_p2p_eclipse_partial | 6 |
| 569 | S4_p2p_eclipse_partial | 6 |
| 580 | S4_p2p_eclipse_partial | 6 |
| 592 | S4_p2p_eclipse_partial | 6 |
| 606 | S4_p2p_eclipse_partial | 6 |
| 715 | S5_anchor_isolation | 6 |
| 728 | S5_anchor_isolation | 6 |
| 740 | S5_anchor_isolation | 6 |
| 753 | S5_anchor_isolation | 6 |
| 765 | S5_anchor_isolation | 6 |
| 777 | S5_anchor_isolation | 6 |
| 790 | S5_anchor_isolation | 6 |
| 802 | S5_anchor_isolation | 6 |
| 814 | S5_anchor_isolation | 6 |
| 827 | S5_anchor_isolation | 6 |
| 839 | S5_anchor_isolation | 6 |
| 851 | S5_anchor_isolation | 6 |
| 863 | S5_anchor_isolation | 6 |
| 875 | S5_anchor_isolation | 6 |
| 888 | S5_anchor_isolation | 6 |
| 900 | S5_anchor_isolation | 6 |
