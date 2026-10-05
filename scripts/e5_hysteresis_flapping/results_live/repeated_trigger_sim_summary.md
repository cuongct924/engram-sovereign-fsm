In-process repeated-trigger adversary, 1200 blocks, warning-level bursts (TestE5c_RepeatedTriggerAdversary).
tailored = burst as long as the committed effective threshold; fixed_k0 = k0 bad blocks, one healthy block between bursts;
patient = waits until the flap level is back to 0, then k0 bad blocks.

| config | strategy | bad blocks | demotions | bad blocks per demotion | outside ANCHORED |
|---|---|---:|---:|---:|---:|
| static2 | tailored | 600 | 300 | 2.00 | 50.0% |
| static2 | fixed_k0 | 600 | 300 | 2.00 | 50.0% |
| static2 | patient | 600 | 300 | 2.00 | 50.0% |
| static4 | tailored | 800 | 200 | 4.00 | 33.3% |
| static4 | fixed_k0 | 800 | 200 | 4.00 | 33.3% |
| static4 | patient | 800 | 200 | 4.00 | 33.3% |
| static8 | tailored | 960 | 120 | 8.00 | 20.0% |
| static8 | fixed_k0 | 960 | 120 | 8.00 | 20.0% |
| static8 | patient | 960 | 120 | 8.00 | 20.0% |
| adaptive2-8_w16 | tailored | 958 | 121 | 7.92 | 20.2% |
| adaptive2-8_w16 | fixed_k0 | 800 | 1 | 800.00 | 0.2% |
| adaptive2-8_w16 | patient | 120 | 60 | 2.00 | 10.0% |
| adaptive2-8_w64 | tailored | 958 | 121 | 7.92 | 20.2% |
| adaptive2-8_w64 | fixed_k0 | 800 | 1 | 800.00 | 0.2% |
| adaptive2-8_w64 | patient | 36 | 18 | 2.00 | 3.0% |
| adaptive2-8_w128 | tailored | 958 | 121 | 7.92 | 20.2% |
| adaptive2-8_w128 | fixed_k0 | 800 | 1 | 800.00 | 0.2% |
| adaptive2-8_w128 | patient | 20 | 10 | 2.00 | 1.7% |
| adaptive2-8_w256 | tailored | 958 | 121 | 7.92 | 20.2% |
| adaptive2-8_w256 | fixed_k0 | 800 | 1 | 800.00 | 0.2% |
| adaptive2-8_w256 | patient | 10 | 5 | 2.00 | 0.8% |
