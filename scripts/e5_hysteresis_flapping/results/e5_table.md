**E5 -- Hysteresis Sweep (measured, real tests/e2e data, 5 environments, 100-block window):**

| HYSTERESIS_WAIT | Environment | Reached ANCHORED | First at (blocks) | Final State | Flapping | ANCHORED uptime |
| ---: | --- | :---: | ---: | --- | ---: | ---: |
| 0 | stable | yes | 3 | ANCHORED | 0 | 33.3% |
| 0 | noisy_btc | yes | 3 | SOVEREIGN | 10 | 59.8% |
| 0 | noisy_da | yes | 3 | ANCHORED | 3 | 94.1% |
| 0 | noisy_p2p | yes | 3 | ANCHORED | 3 | 94.1% |
| 0 | combined_adversarial | yes | 3 | SOVEREIGN | 10 | 59.8% |
| 1 | stable | yes | 4 | ANCHORED | 0 | 25.0% |
| 1 | noisy_btc | yes | 4 | SOVEREIGN | 12 | 46.1% |
| 1 | noisy_da | yes | 4 | ANCHORED | 3 | 93.1% |
| 1 | noisy_p2p | yes | 4 | ANCHORED | 3 | 93.1% |
| 1 | combined_adversarial | yes | 4 | SOVEREIGN | 12 | 46.1% |
| 3 | stable | yes | 6 | ANCHORED | 0 | 16.7% |
| 3 | noisy_btc | yes | 6 | SOVEREIGN | 23 | 26.5% |
| 3 | noisy_da | yes | 6 | ANCHORED | 3 | 91.2% |
| 3 | noisy_p2p | yes | 6 | ANCHORED | 3 | 91.2% |
| 3 | combined_adversarial | yes | 6 | SOVEREIGN | 23 | 26.5% |
| 5 | stable | yes | 8 | ANCHORED | 0 | 12.5% |
| 5 | noisy_btc | yes | 33 | SOVEREIGN | 31 | 12.8% |
| 5 | noisy_da | yes | 10 | ANCHORED | 3 | 87.2% |
| 5 | noisy_p2p | yes | 10 | ANCHORED | 3 | 87.2% |
| 5 | combined_adversarial | yes | 33 | SOVEREIGN | 31 | 12.8% |
| 10 | stable | yes | 13 | ANCHORED | 0 | 7.7% |
| 10 | noisy_btc | yes | 38 | SOVEREIGN | 35 | 1.0% |
| 10 | noisy_da | yes | 34 | ANCHORED | 3 | 65.7% |
| 10 | noisy_p2p | yes | 34 | ANCHORED | 3 | 65.7% |
| 10 | combined_adversarial | yes | 38 | SOVEREIGN | 35 | 1.0% |
| 20 | stable | yes | 23 | ANCHORED | 0 | 4.3% |
| 20 | noisy_btc | NO | never | SOVEREIGN | 37 | 0.0% |
| 20 | noisy_da | yes | 46 | ANCHORED | 3 | 53.9% |
| 20 | noisy_p2p | yes | 46 | ANCHORED | 3 | 53.9% |
| 20 | combined_adversarial | NO | never | SOVEREIGN | 37 | 0.0% |
