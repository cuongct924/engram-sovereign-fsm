---
name: experiment-results-refresh
description: Workflow to run immediately after any live experiment re-run under scripts/e{2..10}_*/ (a live_*.py or live_*.sh script that writes to that experiment's results_live/ directory) against the real Docker cluster. Use whenever the same experiment+scenario+param combination gets measured again -- e.g. after a topology change, a bug fix, or a routine spot-check. Not for one-off ad hoc debug captures (CLAUDE.md's own add/rebuild/revert convention already covers those), and not for in-process `go test` output (no results_live/ directory, out of scope here).
---

# After a live experiment re-run: retire stale data, don't just pile it on

`scripts/e{2..10}_*/results_live/` accumulates a new timestamped CSV+`_summary.md` pair every run,
forever, by design (the scripts never overwrite). That's correct for a single session, but across
many re-runs (topology changes, bug fixes, routine spot-checks) it silently builds up a mix of
current and stale measurements in the same directory, indistinguishable by filename alone. A
reader who opens `results_live/` months later — or a paper reviewer checking raw data behind a
table — has no way to tell which files backed the number actually cited in
`docs/EXPERIMENT.md` and which are leftovers from a state the repo has since moved past.

Run this workflow every time a live experiment script is re-run, not just when asked:

## 1. Identify what the new run supersedes

Same experiment, same scenario/param combination (e.g. E5's `hw10_noisy_da`, E8's
`byzantine_a4_forge_btc_hash`) as an existing file already in that experiment's `results_live/` →
the new run is a candidate replacement for the old one, not an addition to it.

## 2. Decide: delete, or keep as an explicit baseline

Delete the old pair (`.csv` + `_summary.md`) if **both**:
- `grep` confirms the old filename isn't cited anywhere in `docs/EXPERIMENT.md` (if it is, update
  the citation to the new file first, in the same pass — never leave a citation pointing at a
  deleted file)
- it isn't marked as an intentionally-retained reference (see below)

Keep it, uncited or not, if it's a genuine **baseline**: a result deliberately retained for
before/after comparison, or a contaminated/invalid run kept for the record with an explicit note
explaining why (e.g. E5's retracted `hw2_noisy_da_20260812T123903` run — "kept at
`results_live/...` for the record, not deleted"). If you're about to delete something like that,
stop and ask rather than guessing whether it's disposable.

When genuinely unsure whether a file is superseded or a baseline, treat it as a baseline (don't
delete) and flag the ambiguity instead of guessing.

## 3. Regenerate figures from the current file set

Check whether that experiment has a `live_figure_builder.py` (E5, E9, E2 have one as of this
writing; not every experiment does — E7/E8 present tables, not figures). If it does, re-run it
after step 2's cleanup, not before — most of these builders glob-discover their inputs by filename
pattern (see E5's `find_summary_paths()`) rather than taking an explicit file list, so a stale file
still sitting in `results_live/` at figure-build time silently leaks into the regenerated
`.png`/`.pdf` even though it was about to be deleted.

## 4. Update docs/EXPERIMENT.md in the same pass

Point citations at the new file(s), and correct any numbers/prose describing the superseded
result — a stale table sitting next to fresh figures is worse than either alone. Follow
`docs-style`'s rules for how to phrase the update (state the current fact, don't narrate the
before/after unless the divergence itself is the finding worth keeping, e.g. a contaminated-run
retraction).

## Worked example

Re-running E5's live spot-check at 5 `HysteresisWait` values (2026-08-13) superseded E5's old
2026-08-08 `hw2`/`hw10` files (both scenario+param combinations re-measured) — those got deleted,
`docs/EXPERIMENT.md`'s E5 table and `figure4_hysteresis_live.{png,pdf}` were regenerated from all 10
current files via `live_figure_builder.py`. In the same pass, E3/E8/E9's own 2026-08-07/08-08
`results_live/` files turned out to be superseded too (each experiment's docs section already cited
a same-topology re-run instead) and were deleted — except none of them were baselines, so nothing
was held back. E9's deleted 2026-08-08 run got one extra step: its raw data showed a contaminated
starting state (already `SOVEREIGN` before fault injection began) that the existing "bugs found"
writeup didn't explain, so that finding was written into `docs/EXPERIMENT.md` before the file was
deleted — the data went away, but the reason it was invalid didn't.
