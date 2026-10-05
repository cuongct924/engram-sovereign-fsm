# Columns of collusion_suite_*.csv

- `pass`: AND of every entry of `checks` except those ending in `_whole_run`. It includes `no_honest_transition`
  (no FSM transition inside the attack window). On this testbed that check fails even in the no-adversary control
  `c0` (first run under the name `b0`, see README_c0_rename.md) because of sensor noise, so `pass=False` does **not**
  mean the attack succeeded. The paper does not use this column.
- `safety_replay_ok` (`*_annotated.csv`, and in files written after the column was added): AND of `safety_held`,
  `liveness` and `replay_clean`. This is what the paper's evaluation rests on, together with identical AppHashes.
- `replay_violations`: blocks whose committed state differs from the offline recomputation, or whose aggregate lies
  outside the honest range. It is 0 in every case with at most two Byzantine validators. In
  `collusion_suite_20261005T100618.csv` (the threshold sweep b9) the `b3` cells have 5-42 violations on purpose: with
  three Byzantine validators the aggregate leaves the honest range, so `safety_replay_ok` is false there by design.
- The raw `collusion_suite_<stamp>.csv` files are unchanged; `annotate_suite_csv.py` writes the `_annotated.csv` copies.
- The repeats of a case in the suite share one chain; the cells of the b9 sweep use a fresh chain per trial.
