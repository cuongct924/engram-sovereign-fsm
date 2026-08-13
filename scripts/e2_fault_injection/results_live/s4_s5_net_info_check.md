# S4/S5 real /net_info peer-count cross-check

Query.State never carries ActiveAnchors/p2p_healthy (documented stale field, logger.py's _decode_query_state) -- this cross-checks engram-node01's real connected-peer count via CometBFT's own /net_info RPC during the isolation window, independent of the app-level FSM state.

| t (s) | phase | n_peers |
|---:|---|---:|
| 350 | S4_p2p_eclipse_partial | 3 |
| 362 | S4_p2p_eclipse_partial | 3 |
| 374 | S4_p2p_eclipse_partial | 3 |
| 391 | S4_p2p_eclipse_partial | 3 |
| 406 | S4_p2p_eclipse_partial | 3 |
| 418 | S4_p2p_eclipse_partial | 3 |
| 434 | S4_p2p_eclipse_partial | 3 |
| 450 | S4_p2p_eclipse_partial | 3 |
| 464 | S4_p2p_eclipse_partial | 3 |
| 477 | S4_p2p_eclipse_partial | 3 |
| 531 | S5_anchor_isolation | 3 |
| 543 | S5_anchor_isolation | 3 |
| 555 | S5_anchor_isolation | 3 |
| 567 | S5_anchor_isolation | 3 |
| 579 | S5_anchor_isolation | 3 |
| 591 | S5_anchor_isolation | 3 |
| 603 | S5_anchor_isolation | 3 |
| 616 | S5_anchor_isolation | 3 |
| 628 | S5_anchor_isolation | 3 |
| 640 | S5_anchor_isolation | 3 |
| 652 | S5_anchor_isolation | 3 |
| 664 | S5_anchor_isolation | 3 |
| 676 | S5_anchor_isolation | 3 |
| 689 | S5_anchor_isolation | 3 |
| 701 | S5_anchor_isolation | 3 |
| 713 | S5_anchor_isolation | 3 |
