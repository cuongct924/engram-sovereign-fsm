# Max-N=256 + padding/count — real measured results

Real numbers for the max-N=256 + padding/count redesign of
`circuit/reanchoring/src/main.nr`, approved plan (2026-08-09). Every number
below is from an actual `nargo compile`/`nargo execute`/`bb prove`/
`bb verify` run on this machine (`nargo 1.0.0-beta.22`,
`bb 5.0.0-nightly.20260522`), at four real `count` values.

## Why this replaces the old Table 6B scaling methodology

The old design swept `global N` at compile time (recompiling the circuit for
each N = 4, 8, 16, ..., 256) to produce Table 6B's scaling curve. The new
design compiles **once**, for a fixed `N_MAX=256`; what varies per-proof is
the runtime witness `count` (1..=256), not the circuit itself. There is no
longer a meaningful "sweep N and recompile" scaling curve to produce for
this circuit — the circuit's cost (opcodes, circuit size, prove/verify time)
is now **constant regardless of the real interval length**, which is itself
the headline real finding below.

## Real measured numbers (count ∈ {1, 4, 130, 256})

| count | Real headers used | Prove | Verify | Proof size | Public inputs |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1 | 1.054s | 23.5ms | 14,656 B | 96 B |
| 4 | 4 | 1.065s | 22.2ms | 14,656 B | 96 B |
| 130 | 130 | 0.993s | 21.9ms | 14,656 B | 96 B |
| 256 (N_MAX) | 256 | 1.059s | 22.8ms | 14,656 B | 96 B |

Circuit shape (fixed, independent of `count` — same compiled circuit every
time): **6,143 ACIR opcodes, 47,613 Honk circuit size.**

VK is byte-identical across all four `count` values (confirmed via `diff`)
— expected, since it's the same compiled circuit regardless of witness data.

## Comparison to the old fixed-N-per-compile design (Table 6B)

| | Old (fixed-N=256, no padding) | New (N_MAX=256, count=256) |
| --- | ---: | ---: |
| ACIR opcodes | 3,071 | 6,143 (2.0x) |
| Circuit size | 39,298 | 47,613 (1.21x) |
| Prove | 0.684s | 1.059s (1.55x) |
| Verify | 22ms | 22.8ms (~flat) |
| Proof size | 14,656 B | 14,656 B (same) |

The new design costs real, measured overhead (~2x opcodes, ~1.5x prove time)
at the N_MAX ceiling itself — the price of the padding-gate logic (`active`
boolean + OR-gate per assertion) and the dynamic `headers[count-1]` array
index (a selector chain over all 256 slots). This is the real trade-off
flagged in the approved plan, now quantified: **every proof pays this fixed
~1.06s/47,613-circuit-size cost regardless of how many real headers it
covers** (confirmed: count=1's prove time is statistically indistinguishable
from count=256's — 1.054s vs 1.059s).

## Why this is still the right trade

This ~1.06s fixed cost is still comfortably under Engram's block time (well
under 2s), and buys a real, previously-impossible capability: **any**
interval length from 1 to 256 headers is provable in a single proof,
submittable immediately (no waiting to accumulate an exact or minimum
count). This closes both problems the old fixed-N=4 design had: the E2 S7
race (interval outgrowing a small fixed N's coverage) and the trailing-
remainder liveness gap (a segment shorter than the old fixed N could never
be proven at all).

## Verification performed

- `nargo test` clean in both `circuit/reanchoring/` (8 tests: count=1/4/130/256
  satisfy the circuit; count=0, count>N_MAX, wrong fsm_state, and a
  padding-cannot-extend-real-coverage attack all correctly `should_fail`)
  and `circuit/reanchoring_witness/` (hash cross-check against the real
  circuit's fixture).
- Real `bb prove`/`bb verify` round trips at count=1, 4, 130, 256 — genuine,
  not mocked.
- The regenerated embedded VK (`x/sovereignty/keeper/zk_assets/vk`) verified
  through the real Go code path: `go test ./tests/benchmark/... -bench
  BenchmarkVerifyZKProof` against a real proof from the new circuit —
  19.2ms/op, consistent with the direct `bb verify` measurements above.
- `go build ./... && go vet ./... && go test ./...` clean across the full
  repo after the `x/sovereignty/keeper/msg_server.go` public-input layout
  update (64 → 96 bytes: `rt_last‖rt_new‖count`).
