In-process model (real FSM), TestE8_HideOutagePlacement: can b Byzantine proposers hide a real DA outage?
Every placement of b Byzantine slots in the 7-slot proposer cycle; the outage starts on an ANCHORED chain; 300 blocks.

| b | down threshold k | placements | never detected | blocks to detection (min-max) | longest honest run (min-max) |
|---:|---:|---:|---:|---|---|
| 2 | 2 | 21 | 0 | 2-2 | 3-5 |
| 2 | 3 | 21 | 0 | 3-3 | 3-5 |
| 2 | 4 | 21 | 0 | 4-4 | 3-5 |
| 3 | 2 | 35 | 0 | 2-8 | 2-4 |
| 3 | 3 | 35 | 14 | 3-9 | 2-4 |
| 3 | 4 | 35 | 28 | 4-10 | 2-4 |
| 4 | 2 | 35 | 35 | -- | 1-3 |
| 4 | 3 | 35 | 35 | -- | 1-3 |
| 4 | 4 | 35 | 35 | -- | 1-3 |
