# ANCHORED share under load: default against relaxed DA time limits

| DA limits | offered (tx/s) | runs | ANCHORED share | p95 latency (s) | peak validator CPU (%) | FSM states seen |
|---|---|---|---|---|---|---|
| default | 200 | 3 | 0.68 (0.58--0.74) | 1.6 (1.6--1.6) | 24 (21--30) | ANCHORED, SUSPICIOUS |
| default | 500 | 3 | 0.36 (0.34--0.38) | 1.9 (1.8--2.1) | 55 (35--73) | ANCHORED, SOVEREIGN, SUSPICIOUS |
| relaxed | 200 | 3 | 0.75 (0.73--0.78) | 1.9 (1.8--2.0) | 22 (20--25) | ANCHORED, SUSPICIOUS |
| relaxed | 500 | 3 | 0.80 (0.74--0.89) | 3.7 (3.1--4.6) | 44 (42--46) | ANCHORED, SUSPICIOUS |
