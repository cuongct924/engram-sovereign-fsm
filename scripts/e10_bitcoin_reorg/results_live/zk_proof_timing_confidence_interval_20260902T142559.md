# E10 / item-15: real ZK recovery-proof round-trip timing (mean +- 95% CI)

Parsed from `/tmp/watch_and_prove.log` -- round-trip = one full query -> link chain -> boundary witness -> nargo execute -> bb prove -> tx-submit-recovery-proof cycle (`prove_and_submit.sh`), timed via `watch_and_prove.sh`'s own per-poll timestamps (no new instrumentation).

* Accepted: 5, Rejected: 31
* Mean +- 95% CI: 131.6s +- 52.3s (stdev=59.6s, n=5)
* Values (s): 25.0, 159.0, 161.0, 159.0, 154.0
