// circuit/reanchoring_recursion_spike/src/main.nr
//
// Step 2 of the recursion feasibility spike (see the approved plan): this is
// the REAL leaf circuit, copied VERBATIM from circuit/reanchoring/src/main.nr
// (same "kept in sync by hand" pattern circuit/reanchoring_witness/ already
// uses in this repo) -- not modified in any way, so measurements taken here
// are directly comparable to Table 6B's existing N=4 baseline. Proven with
// `bb prove --verifier_target noir-recursive` (unlike the deployed circuit,
// which uses bb's default target) so it can be verified recursively by
// circuit/reanchoring_recursion_spike_aggregator/.
//
// Step 1's trivial `assert(x != 0)` smoke test (which lived in this same
// file/package) already confirmed the recursion mechanism itself works with
// this toolchain -- this replaces it with the real header-chain logic to get
// real, comparable benchmark numbers.

global N: u32 = 4;

struct Header {
    prev_hash: Field,
    fsm_state: Field,        // 2: SOVEREIGN, 3: RECOVERING
    withdrawal_locked: Field, // 1: true, 0: false
    state_root: Field,
}

fn hash_header(h: Header) -> Field {
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
    headers: [Header; N],
) {
    // (a) Header continuity (E6's C1): each header's prev_hash must be the
    // Poseidon2 hash of the previous header.
    for i in 1..N {
        assert(headers[i].prev_hash == hash_header(headers[i - 1]));
    }

    // (b)/(c) FSM legality + withdrawal-lock invariant (E6's C2/C3): every
    // header in the interval must be SOVEREIGN/RECOVERING with withdrawals
    // locked.
    for i in 0..N {
        assert((headers[i].fsm_state == 2) | (headers[i].fsm_state == 3));
        assert(headers[i].withdrawal_locked == 1);
    }

    // (d) Root binding (E6's C4/C5): the interval starts at rt_last and ends
    // at rt_new.
    assert(headers[0].prev_hash == rt_last);
    assert(headers[N - 1].state_root == rt_new);
}

#[test]
fn valid_chain_satisfies_circuit() {
    let h0 = Header { prev_hash: 111, fsm_state: 2, withdrawal_locked: 1, state_root: 1000 };
    let h1 = Header { prev_hash: hash_header(h0), fsm_state: 2, withdrawal_locked: 1, state_root: 1001 };
    let h2 = Header { prev_hash: hash_header(h1), fsm_state: 3, withdrawal_locked: 1, state_root: 1002 };
    let h3 = Header { prev_hash: hash_header(h2), fsm_state: 3, withdrawal_locked: 1, state_root: 1003 };
    main(111, 1003, [h0, h1, h2, h3]);
}
