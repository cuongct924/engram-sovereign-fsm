# Live repeated-trigger runs (fresh genesis per run; range over runs)

| config | runs | pulses sent | bridge down per pulse (s) | demotions | flaps | ANCHORED share (%) | reached SOVEREIGN |
|---|---|---|---|---|---|---|---|
| static2 | 3 | 21--22 | 16.4--17.3 | 1 | 21--34 | 0.2--1.7 | 3/3 |
| static4 | 3 | 21 | 16.8--17.0 | 1 | 0--23 | 1.7--2.1 | 3/3 |
| adaptiveW16 | 3 | 21 | 16.7--17.0 | 1 | 19--28 | 2.1--2.1 | 3/3 |
| adaptiveW64 | 3 | 21 | 17.0--17.2 | 1 | 5--29 | 1.9--2.1 | 3/3 |
| adaptiveK4W64 | 3 | 21--23 | 15.2--17.0 | 1 | 19--39 | 1.9--2.2 | 3/3 |

All 15 runs: nominal pulse 3.0 s; bridge down per pulse 15.2--17.3 s on average; SOVEREIGN reached in 15/15; demotions per run 1.
