// circuit/reanchoring/src/main.nr
//
// Recovery proof circuit -- proves a SOVEREIGN/RECOVERING interval of up to
// N_MAX headers is well-formed before re-anchoring to ANCHORED, mirroring
// VerifyZkProof's intent (spec/core/EngramTendermint.tla:257-260) and
// spec/README.md's formal relation x = (rt_last, rt_new, n) -- `count`
// below IS that `n`: a real public input, not implicit in a fixed circuit
// size. N_MAX=256 is a compile-time ceiling (chosen because Engram blocks
// are produced well under 2s, so an unhealthy interval can accumulate
// headers fast -- see the design note this replaced for the full
// reasoning); `count` (1..=N_MAX) is the ACTUAL number of real headers in
// THIS proof, checked at runtime. Slots at index >= count are padding: no
// constraint is enforced on them (see the `active` gate below), so their
// values are irrelevant and can be anything, including all-zero.
//
// This replaces the earlier fixed-N=4 design (global N: u32 = 4, every
// proof had to cover EXACTLY 4 headers) -- that design had two real
// problems, found by actually operating it: (1) docs/EXPERIMENT.md's E2 S7
// showed a genuinely unhealthy interval outgrows what fixed-size rolling
// checkpoints can keep pace with (29/52 real proof attempts rejected); (2)
// a trailing remainder shorter than N could never be proven at all if the
// interval stopped growing before a full N-block chunk accumulated -- a
// real liveness gap, not just a performance one. A prior recursive-proof
// (proof-of-proofs) alternative was investigated for real and found to be
// a clear NO-GO (circuit/reanchoring_recursion_spike/RESULTS.md) --
// verifying even one inner proof recursively costs orders of magnitude
// more than just proving more headers directly in this same circuit shape.

global N_MAX: u32 = 256;

struct Header {
    prev_hash: Field,
    fsm_state: Field,        // 2: SOVEREIGN, 3: RECOVERING
    withdrawal_locked: Field, // 1: true, 0: false
    state_root: Field,
}

fn hash_header(h: Header) -> Field {
    // Real Poseidon2 sponge (BN254 state width 4, rate 3, capacity 1 --
    // Barretenberg's own Poseidon2 instantiation, driven directly through the
    // real POSEIDON2_PERMUTATION black-box gate via std::hash::poseidon2_permutation).
    // Noir's stdlib at the pinned 1.0.0-beta.22 toolchain (see CLAUDE.md) has
    // no public single-call "hash this array" wrapper for Poseidon2 -- only
    // the raw permutation is exported -- so this absorbs the header's 4
    // fields with a standard, length-domain-separated (IV = input_length <<
    // 64) two-block absorb/squeeze. This mirrors the shape of Noir's own
    // internal Poseidon2Hasher (std/hash/poseidon2.nr, used for HashMap
    // bucketing) but does NOT reuse its body: that helper's tail-block
    // indexing re-adds inputs[0] instead of the true remaining element for
    // any length not a multiple of RATE=3, which is fine for its bucket-hash
    // use case but wrong for a cryptographic per-field absorption -- fixed
    // here so all 4 fields are absorbed exactly once.
    let inputs = [h.prev_hash, h.fsm_state, h.withdrawal_locked, h.state_root];
    let iv: Field = 4 * 18446744073709551616; // (input_length << 64), input_length = 4
    let mut state = [0, 0, 0, iv];
    state[0] += inputs[0];
    state[1] += inputs[1];
    state[2] += inputs[2];
    state = std::hash::poseidon2_permutation(state);
    state[0] += inputs[3];
    state = std::hash::poseidon2_permutation(state);
    state[0]
}

fn main(
    rt_last: pub Field,   // Root of the last anchored block
    rt_new: pub Field,    // Root of the proposed recovery block
    count: pub u32,       // spec/README.md's `n`: real number of headers in this proof, 1..=N_MAX
    headers: [Header; N_MAX],
) {
    assert(count >= 1);
    assert(count <= N_MAX);

    // (a) Header continuity (E6's C1): each REAL header's prev_hash must be
    // the Poseidon2 hash of the previous REAL header. Gated: for i >= count
    // (padding), `active` is false, so the assertion is vacuously satisfied
    // regardless of headers[i]'s actual (irrelevant) contents.
    for i in 1..N_MAX {
        let active = i < count;
        assert(!active | (headers[i].prev_hash == hash_header(headers[i - 1])));
    }

    // (b)/(c) FSM legality + withdrawal-lock invariant (E6's C2/C3): every
    // REAL header in the interval must be SOVEREIGN/RECOVERING with
    // withdrawals locked. Same padding gate as above.
    for i in 0..N_MAX {
        let active = i < count;
        assert(!active | ((headers[i].fsm_state == 2) | (headers[i].fsm_state == 3)));
        assert(!active | (headers[i].withdrawal_locked == 1));
    }

    // (d) Root binding (E6's C4/C5): the interval starts at rt_last and ends
    // at rt_new. headers[0] is always real (count >= 1 checked above), so
    // this stays unconditional. headers[count - 1] is a dynamic (witness-
    // valued) array index, not a compile-time constant -- Noir compiles
    // this to a selector chain over all N_MAX slots.
    assert(headers[0].prev_hash == rt_last);
    assert(headers[count - 1].state_root == rt_new);
}

// build_chain builds a real, valid, hash-linked chain of exactly COUNT
// headers (COUNT is a compile-time numeric generic, e.g. build_chain::<4>()),
// padded with zero-valued Header entries up to N_MAX, for use as test
// fixtures below -- mirrors the shape prove_and_submit.sh builds for real
// chain data (real headers first, zero-padding after), just generated
// in-circuit instead of from real Go/RPC data.
fn build_chain<let COUNT: u32>() -> ([Header; N_MAX], Field, Field) {
    let mut headers = [Header { prev_hash: 0, fsm_state: 0, withdrawal_locked: 0, state_root: 0 }; N_MAX];
    let rt_last = 111;
    headers[0] = Header { prev_hash: rt_last, fsm_state: 2, withdrawal_locked: 1, state_root: 1000 };
    for i in 1..COUNT {
        let fsm = if i % 2 == 0 { 2 } else { 3 };
        headers[i] = Header {
            prev_hash: hash_header(headers[i - 1]),
            fsm_state: fsm,
            withdrawal_locked: 1,
            state_root: 1000 + i as Field,
        };
    }
    let rt_new = headers[COUNT - 1].state_root;
    (headers, rt_last, rt_new)
}

#[test]
fn count_1_satisfies_circuit() {
    let (headers, rt_last, rt_new) = build_chain::<1>();
    main(rt_last, rt_new, 1, headers);
}

#[test]
fn count_4_satisfies_circuit() {
    // The old fixed-N=4 design's exact case -- must still work identically.
    let (headers, rt_last, rt_new) = build_chain::<4>();
    main(rt_last, rt_new, 4, headers);
}

#[test]
fn count_130_satisfies_circuit() {
    let (headers, rt_last, rt_new) = build_chain::<130>();
    main(rt_last, rt_new, 130, headers);
}

#[test]
fn count_n_max_satisfies_circuit() {
    let (headers, rt_last, rt_new) = build_chain::<N_MAX>();
    main(rt_last, rt_new, N_MAX, headers);
}

#[test(should_fail)]
fn count_zero_fails_circuit() {
    let (headers, rt_last, rt_new) = build_chain::<1>();
    main(rt_last, rt_new, 0, headers);
}

#[test(should_fail)]
fn count_above_n_max_fails_circuit() {
    let (headers, rt_last, rt_new) = build_chain::<N_MAX>();
    main(rt_last, rt_new, N_MAX + 1, headers);
}

#[test(should_fail)]
fn wrong_fsm_state_fails_circuit() {
    let (mut headers, rt_last, rt_new) = build_chain::<4>();
    headers[0].fsm_state = 1; // 1 = ANCHORED, not allowed
    main(rt_last, rt_new, 4, headers);
}

#[test(should_fail)]
fn padding_cannot_extend_real_coverage() {
    // A proof claiming count=8 but whose headers[4..8) are NOT a real
    // continuation of the count=4 chain (still zero-valued padding) must be
    // rejected -- confirms the gate genuinely enforces continuity up to the
    // CLAIMED count, not just up to whatever happens to be non-zero.
    let (headers, rt_last, _) = build_chain::<4>();
    main(rt_last, 1000, 8, headers);
}
