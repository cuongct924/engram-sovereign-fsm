# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 473 | S4_p2p_eclipse_partial | 6 |
| 486 | S4_p2p_eclipse_partial | 6 |
| 498 | S4_p2p_eclipse_partial | 6 |
| 510 | S4_p2p_eclipse_partial | 6 |
| 523 | S4_p2p_eclipse_partial | 6 |
| 535 | S4_p2p_eclipse_partial | 6 |
| 547 | S4_p2p_eclipse_partial | 6 |
| 560 | S4_p2p_eclipse_partial | 6 |
| 572 | S4_p2p_eclipse_partial | 6 |
| 582 | S4_p2p_eclipse_partial | 6 |
| 594 | S4_p2p_eclipse_partial | 6 |
| 681 | S5_anchor_isolation | 6 |
| 693 | S5_anchor_isolation | 6 |
| 705 | S5_anchor_isolation | 6 |
| 717 | S5_anchor_isolation | 6 |
| 729 | S5_anchor_isolation | 6 |
| 742 | S5_anchor_isolation | 6 |
| 754 | S5_anchor_isolation | 6 |
| 766 | S5_anchor_isolation | 6 |
| 778 | S5_anchor_isolation | 6 |
| 790 | S5_anchor_isolation | 6 |
| 803 | S5_anchor_isolation | 6 |
| 815 | S5_anchor_isolation | 6 |
| 827 | S5_anchor_isolation | 6 |
| 839 | S5_anchor_isolation | 6 |
| 851 | S5_anchor_isolation | 6 |
| 862 | S5_anchor_isolation | 6 |
