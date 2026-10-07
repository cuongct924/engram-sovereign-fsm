# E11 -- liveness under peripheral degradation (hard-gated vs. Engram vs. soft)

Cells are mean [min, max] over trials. `Rate` is blocks/s in the fault window as a share of the
same trial's healthy rate. `Stalled` counts trials whose longest block gap reached 30 s or whose chain
never resumed within the 120 s recovery window. `Wd. after lock` is withdrawals committed from the
moment Engram's circuit breaker locks (per fault: Checkpoint-submission pause 117 s, DA outage 41 s, Both 43 s after onset) until the fault ends, the same window for every system; `Wd. before lock` is those
committed earlier in the fault window (exposure before detection). `Wd. while locked` counts
withdrawals committed while the FSM state was SOVEREIGN/RECOVERING: a check of the breaker itself,
always 0 for hard-gated because its FSM never leaves ANCHORED.

| Fault | System | Trials | Blocks in fault | Rate vs healthy | Longest gap (s) | Max round | Resume after heal (s) | Stalled | Wd. after lock | Wd. before lock | Wd. while locked |
|---|---|---|---|---|---|---|---|---|---|---|---|
| Checkpoint-submission pause | hard-gated | 3 | 73.7 [73, 74] | 55.7 [53, 57]% | n/a | 12.0 [12, 12] | n/a | 3/3 | 8.0 [6, 9] | 55.0 [54, 56] | 0 |
| Checkpoint-submission pause | hard-gated-oob | 3 | 89.0 [89, 89] | 61.2 [60, 63]% | 6.7 [7, 7] | 12.0 [12, 12] | 53.9 [48, 57] | 0/3 | 6.0 [6, 6] | 55.7 [55, 56] | 0 |
| Checkpoint-submission pause | engram | 3 | 147.0 [147, 147] | 101.8 [102, 102]% | 2.4 [2, 2] | 0.7 [0, 1] | 0.8 [0, 1] | 0/3 | 0.0 [0, 0] | 52.3 [45, 56] | 0 |
| Checkpoint-submission pause | soft | 3 | 144.0 [143, 145] | 99.0 [98, 100]% | 2.5 [2, 2] | 0.7 [0, 1] | 1.0 [1, 1] | 0/3 | 30.7 [30, 31] | 56.3 [56, 57] | 272 |
| DA outage | hard-gated | 3 | 23.0 [22, 24] | 15.7 [14, 17]% | n/a | 21.3 [21, 22] | n/a | 3/3 | 3.0 [3, 3] | 19.0 [18, 20] | 0 |
| DA outage | hard-gated-oob | 3 | 29.3 [29, 30] | 19.8 [19, 21]% | 6.7 [7, 7] | 20.7 [20, 21] | 22.7 [21, 24] | 0/3 | 7.0 [6, 8] | 19.3 [19, 20] | 0 |
| DA outage | engram | 3 | 149.7 [147, 153] | 96.3 [93, 99]% | 1.4 [1, 1] | 0.0 [0, 0] | 1.3 [1, 1] | 0/3 | 0.0 [0, 0] | 18.3 [18, 19] | 0 |
| DA outage | soft | 3 | 150.7 [149, 152] | 98.5 [98, 100]% | 1.5 [1, 2] | 0.0 [0, 0] | 0.8 [0, 1] | 0/3 | 72.3 [72, 73] | 19.3 [19, 20] | 398 |
| Both | hard-gated | 3 | 23.7 [23, 24] | 17.1 [16, 19]% | n/a | 21.3 [21, 22] | n/a | 3/3 | 3.0 [3, 3] | 19.3 [19, 20] | 0 |
| Both | hard-gated-oob | 3 | 30.3 [27, 33] | 20.2 [17, 22]% | 6.7 [7, 7] | 20.3 [20, 21] | 63.3 [51, 74] | 0/3 | 8.0 [7, 9] | 19.0 [18, 20] | 0 |
| Both | engram | 3 | 153.0 [150, 155] | 99.4 [97, 104]% | 1.4 [1, 1] | 0.0 [0, 0] | 1.2 [0, 2] | 0/3 | 0.0 [0, 0] | 18.7 [18, 20] | 0 |
| Both | soft | 3 | 151.0 [150, 152] | 98.1 [97, 99]% | 1.3 [1, 1] | 0.0 [0, 0] | 1.1 [1, 2] | 0/3 | 70.7 [70, 71] | 20.3 [20, 21] | 402 |
