# E11 -- liveness under peripheral degradation (hard-gated vs. Engram vs. soft)

Cells are mean [min, max] over trials. `Rate` is blocks/s in the fault window as a share of the
same trial's healthy rate. `Stalled` counts trials whose longest block gap reached 30 s or whose chain
never resumed within the 120 s recovery window. `Wd. after lock` is withdrawals committed from the
moment Engram's circuit breaker locks (per fault: Checkpoint-submission pause 125 s, DA outage 40 s, Both 40 s after onset) until the fault ends, the same window for every system; `Wd. before lock` is those
committed earlier in the fault window (exposure before detection). `Wd. while locked` counts
withdrawals committed while the FSM state was SOVEREIGN/RECOVERING: a check of the breaker itself,
always 0 for hard-gated because its FSM never leaves ANCHORED.

| Fault | System | Trials | Blocks in fault | Rate vs healthy | Longest gap (s) | Max round | Resume after heal (s) | Stalled | Wd. after lock | Wd. before lock | Wd. while locked |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Checkpoint-submission pause | hard-gated | 3 | 81.0 [79, 82] | 63.6 [56, 78]% | n/a | 13.0 [13, 13] | n/a | 3/3 | 0.0 [0, 0] | 59.0 [59, 59] | 0 |
| Checkpoint-submission pause | hard-gated-oob | 3 | 89.0 [89, 89] | 61.2 [60, 63]% | 6.7 [7, 7] | 12.0 [12, 12] | 53.9 [48, 57] | 0/3 | 3.0 [3, 3] | 58.7 [58, 59] | 0 |
| Checkpoint-submission pause | engram | 3 | 141.7 [139, 145] | 96.1 [94, 98]% | 6.3 [6, 6] | 2.3 [2, 3] | 0.8 [0, 1] | 0/3 | 0.0 [0, 0] | 57.7 [55, 59] | 0 |
| Checkpoint-submission pause | soft | 3 | 143.0 [142, 144] | 97.0 [96, 98]% | 6.4 [6, 6] | 2.3 [2, 3] | 0.8 [0, 1] | 0/3 | 27.7 [27, 29] | 59.7 [58, 61] | 243 |
| DA outage | hard-gated | 3 | 25.7 [23, 28] | 23.5 [22, 25]% | n/a | 21.3 [21, 22] | n/a | 3/3 | 5.3 [3, 7] | 18.0 [17, 19] | 0 |
| DA outage | hard-gated-oob | 3 | 29.3 [29, 30] | 19.8 [19, 21]% | 6.7 [7, 7] | 20.7 [20, 21] | 22.7 [21, 24] | 0/3 | 7.7 [7, 8] | 18.7 [18, 19] | 0 |
| DA outage | engram | 3 | 150.0 [149, 151] | 100.5 [99, 103]% | 1.4 [1, 1] | 0.0 [0, 0] | 1.1 [1, 2] | 0/3 | 0.0 [0, 0] | 18.3 [18, 19] | 0 |
| DA outage | soft | 3 | 148.7 [146, 152] | 98.2 [96, 100]% | 1.4 [1, 1] | 0.0 [0, 0] | 1.0 [1, 1] | 0/3 | 72.3 [72, 73] | 19.0 [19, 19] | 395 |
| Both | hard-gated | 3 | 26.7 [24, 28] | 17.5 [16, 18]% | n/a | 21.0 [21, 21] | n/a | 3/3 | 5.7 [3, 7] | 18.7 [18, 19] | 0 |
| Both | hard-gated-oob | 3 | 30.3 [27, 33] | 20.2 [17, 22]% | 6.7 [7, 7] | 20.3 [20, 21] | 63.3 [51, 74] | 0/3 | 8.7 [7, 10] | 18.3 [18, 19] | 0 |
| Both | engram | 3 | 149.3 [148, 151] | 96.9 [95, 98]% | 1.4 [1, 1] | 0.0 [0, 0] | 1.6 [1, 2] | 0/3 | 0.0 [0, 0] | 17.0 [15, 18] | 0 |
| Both | soft | 3 | 150.3 [149, 151] | 96.9 [96, 97]% | 1.3 [1, 1] | 0.0 [0, 0] | 0.8 [0, 2] | 0/3 | 72.3 [72, 73] | 19.0 [19, 19] | 398 |
