// circuit/reanchoring_witness/src/main.nr
//
// Off-chain witness helper for circuit/reanchoring/src/main.nr: NOT a proof
// circuit (only `nargo execute` + println, never a proof). Builds a real
// prev_hash-linked witness from real chain data (HeaderHistory, dumped via
// `engramd query sovereignty recovery-headers`) WITHOUT a Go-side
// Poseidon/pedersen_hash implementation.
//
// Links only the REAL `count` headers (1..=N_MAX) -- padding is applied by
// prove_and_submit.sh when it writes the real circuit's Prover.toml. That
// script feeds rt_last + raw {fsm_state, withdrawal_locked, state_root} per
// header (no prev_hash -- this crate derives it), runs `nargo execute witness
// --show-output`, and greps the printed, linked chain into the real Prover.toml.

global N_MAX: u32 = 256;

struct Header {
    prev_hash: Field,
    fsm_state: Field,
    withdrawal_locked: Field,
    state_root: Field,
}

fn hash_header(h: Header) -> Field {
    // Byte-identical copy of the real circuit's hash_header (Poseidon2
    // two-block absorb/squeeze), hand-kept in sync; N_MAX must match too.
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

// raw_* are the per-header values tracked by the Go keeper (types.RecoveryHeader),
// without prev_hash (this crate derives it). Only the first `count` are real;
// the rest are ignored. rt_last seeds the chain, matching the real circuit's
// headers[0].prev_hash == rt_last binding.
fn main(
    rt_last: pub Field,
    count: pub u32,
    raw_fsm_state: [Field; N_MAX],
    raw_withdrawal_locked: [Field; N_MAX],
    raw_state_root: [Field; N_MAX],
) {
    let mut prev_hash = rt_last;
    for i in 0..N_MAX {
        if i < count {
            let h = Header {
                prev_hash,
                fsm_state: raw_fsm_state[i],
                withdrawal_locked: raw_withdrawal_locked[i],
                state_root: raw_state_root[i],
            };
            println(h);
            prev_hash = hash_header(h);
        }
    }
}

#[test]
fn hash_header_matches_main_circuit_fixture() {
    // Cross-checks against the real circuit's fixture -- this hash_header is
    // a hand-kept copy (Noir has no cross-crate import for one function), so a
    // failure here means they've drifted and this output would no longer link
    // into a witness the real circuit accepts.
    let h0 = Header { prev_hash: 111, fsm_state: 2, withdrawal_locked: 1, state_root: 1000 };
    assert(hash_header(h0) == 0x168c1f5895ad57e0d8257da085aab977ec14ed66699553157cd0bcffcd02bb3c);
}
