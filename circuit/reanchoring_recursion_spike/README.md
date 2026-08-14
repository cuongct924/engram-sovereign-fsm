# Recursive/Aggregated ZK Re-anchoring Proofs — Feasibility Spike Results

Real, measured results for the plan approved 2026-08-09 (see
`/Users/cuongct090_04/.claude/plans/prancy-tinkering-coral.md`). Every number
below is from an actual `nargo compile`/`nargo execute`/`bb prove`/`bb verify`
run on this machine (`nargo 1.0.0-beta.22`, `bb 5.0.0-nightly.20260522`), not
estimated or extrapolated.

## Step 1 — Dependency compatibility: CONFIRMED WORKING

`bb_proof_verification` pinned to `v5.0.0-nightly.20260522`
(`AztecProtocol/aztec-packages`, exact tag match for the locally installed
`bb` build) compiles, links, and works correctly against this toolchain —
the earlier concern about a v5-vs-v6 version gap turned out to be moot once
the actual matching-date tag was found (aztec-packages tags its monorepo
with the same version string `bb --version` reports). `verify_honk_proof`
correctly accepted a real recursively-proven leaf proof inside a second
circuit's witness execution and its own `bb prove`/`bb verify` round-trip.

## Step 2 — Real spike with the actual header-chain circuit

Leaf circuit = `circuit/reanchoring_recursion_spike/src/main.nr`, a byte-for-byte
copy of the deployed `circuit/reanchoring/src/main.nr` (N=4, unmodified),
proven with `bb prove --verifier_target noir-recursive` instead of the
deployed circuit's default target. Aggregator =
`circuit/reanchoring_recursion_spike_aggregator/src/main.nr`, verifying M
leaf proofs via `verify_honk_proof` with the leaf VK as a compile-time
`global` constant (never a witness input — the plan's one hard security
requirement), chaining `rt_last`/`rt_new` across leaves, exposing exactly 2
public Fields (`rt_start`, `rt_end`) — confirmed to land as exactly a 64-byte
`public_inputs` file, i.e. the SAME shape `x/sovereignty/keeper/msg_server.go`'s
`SubmitRecoveryProof` already hardcodes.

### Measured numbers

| M (leaves) | Headers covered | ACIR opcodes | Circuit size | Prove time | Verify time | Proof size |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 4  | 58 | 703,582   | ~191s¹ | 24ms | 14,656 B |
| 2 | 8  | 85 | 1,469,553 | 74s    | 60ms | 14,656 B |
| 4 | 16 | 89 | 3,001,406 | 175s   | 20ms | 14,656 B |

¹ Likely inflated by a one-time CRS (structured reference string) growth —
`~/.bb-crs/bn254_g1.dat` was observed being extended during these runs (a
larger circuit needs more SRS points than the smaller circuits this cache
had previously been sized for); M=2/M=4 ran against an already-larger
cached CRS. Reported as-measured rather than silently adjusted, but the
directionally-obvious conclusion below does not depend on this caveat.

### Direct comparison to the existing non-recursive baseline (Table 6B)

| Headers covered | Direct N (no recursion) — ACIR opcodes / circuit size / prove / verify | Recursive M-leaf aggregate — ACIR opcodes / circuit size / prove / verify |
| ---: | --- | --- |
| 8  | N=8: 95 / 1,106 / **0.113s** / 22ms   | M=2: 85 / 1,469,553 / **74s** / 60ms |
| 16 | N=16: 191 / 2,338 / **0.146s** / 22ms | M=4: 89 / 3,001,406 / **175s** / 20ms |

Verify time stays flat and cheap either way (this repo's existing E6 finding
holds for the recursive case too — good, since that's the number that runs
in every validator's `DeliverTx`). **Prove time and circuit size do not** —
proving the SAME 8 or 16 headers via recursive aggregation costs roughly
**650x–1,200x more prove time** and **~1,300x more circuit size** than
proving them directly in one N-sized circuit. This matches the plan's own
prediction ("verifying a Honk proof inside a circuit is much more expensive
per gate than the header-hashing work alone") — now with real numbers behind
it, not just the expectation.

## Recommendation: NO-GO for recursive aggregation in this system

Table 6B already shows the existing non-recursive design handles N=256
headers in a single proof for 0.684s prove / 22ms verify / 39,298 circuit
size — cheaper on every axis than this spike's M=4 (16 headers, 175s,
3,001,406 circuit size). To match N=256's coverage with recursive
aggregation would need on the order of M=64 leaf proofs (64×N=4), and the
circuit-size trend measured here (roughly doubling per doubling of M, already
at 3M for M=4) makes that almost certainly computationally infeasible on
ordinary validator hardware (extrapolating the trend puts M=64 in the
hundreds-of-millions-of-circuit-size range).

**The root cause is structural, not a tuning problem**: `verify_honk_proof`'s
in-circuit cost (checking pairing/polynomial-commitment arithmetic for an
entire inner UltraHonk proof) vastly dominates this circuit's actual
per-header cost (12 ACIR opcodes/header, per Table 6A). Recursion pays this
fixed, large cost on **every** leaf verified, which is the wrong trade for a
circuit whose native per-header cost is already this cheap — recursion is
the right tool when the thing being aggregated is itself expensive to
re-verify from scratch (e.g. rollups aggregating many independent proofs
that would otherwise each need separate on-chain settlement), not when the
alternative is "just include more of the same cheap items in one proof."

**Actionable conclusion**: the two lower-effort options discussed before this
spike (raise N_MAX; max-N + padding/count) remain the right direction for
closing the real, measured E2 S7 staleness-race gap
(`docs/EXPERIMENT.md:105-107`) — raising N directly gets *more* proving
efficiency per header as N grows (Table 6A's marginal cost is flat and R²=1.0000),
the opposite of recursion's behavior measured here.

## Verification performed

- `nargo compile && nargo test` clean in both spike packages.
- Real `bb prove --verifier_target noir-recursive` for all 4 leaf proofs,
  each independently `bb verify`'d successfully.
- Real `bb prove` (default target) for the M=1/M=2/M=4 aggregator proofs,
  each independently `bb verify`'d successfully — genuine round trips, not
  mocked.
- `go build ./... && go vet ./... && go test ./...` — unaffected, unmodified
  (this spike touched no Go code, no `x/sovereignty/keeper/zk_assets/vk`, no
  `circuit/reanchoring/` or `circuit/reanchoring_witness/`), confirmed still
  passing after this work.
