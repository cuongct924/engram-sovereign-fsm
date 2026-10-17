# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 477 | S4_p2p_eclipse_partial | 6 |
| 495 | S4_p2p_eclipse_partial | 6 |
| 511 | S4_p2p_eclipse_partial | 6 |
| 524 | S4_p2p_eclipse_partial | 6 |
| 537 | S4_p2p_eclipse_partial | 6 |
| 551 | S4_p2p_eclipse_partial | 6 |
| 563 | S4_p2p_eclipse_partial | 6 |
| 577 | S4_p2p_eclipse_partial | 6 |
| 589 | S4_p2p_eclipse_partial | 6 |
| 600 | S4_p2p_eclipse_partial | 6 |
| 688 | S5_anchor_isolation | error: timed out |
| 703 | S5_anchor_isolation | error: timed out |
| 717 | S5_anchor_isolation | error: Remote end closed connection without response |
| 736 | S5_anchor_isolation | error: timed out |
| 754 | S5_anchor_isolation | error: timed out |
| 772 | S5_anchor_isolation | error: timed out |
| 790 | S5_anchor_isolation | error: timed out |
| 809 | S5_anchor_isolation | error: timed out |
| 827 | S5_anchor_isolation | error: timed out |
| 845 | S5_anchor_isolation | error: timed out |
| 857 | S5_anchor_isolation | 6 |
| 869 | S5_anchor_isolation | 6 |
