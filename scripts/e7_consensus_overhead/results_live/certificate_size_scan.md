# Txs[0] byte accounting, last 300 blocks up to 511 (300 with a certificate)

| FSM state | certificates in quorum | blocks | Txs[0] bytes (mean) | per validator (mean) | fixed part (mean) | address | round | extension | signature |
|---|---|---|---|---|---|---|---|---|---|
| ANCHORED | 6 | 1 | 782 | 118.7 | 70 | 22.0 | 0.0 | 28.7 | 66.0 |
| ANCHORED | 7 | 126 | 894 | 117.8 | 70 | 22.0 | 0.0 | 27.8 | 66.0 |
| RECOVERING | 7 | 123 | 926 | 117.5 | 103 | 22.0 | 0.0 | 27.5 | 66.0 |
| SOVEREIGN | 7 | 34 | 889 | 117.2 | 69 | 22.0 | 0.0 | 27.2 | 66.0 |
| SUSPICIOUS | 7 | 16 | 895 | 117.6 | 72 | 22.0 | 0.0 | 27.6 | 66.0 |

Each validator's entry is address + round + extension + signature (bytes per validator, including field tags and length prefixes).
The fixed part is the marker plus the leader's claimed aggregation (state, receipts, flags). The block's last_commit is separate.
