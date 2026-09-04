// circuit/reanchoring_witness/src/main.nr
//
// Off-chain witness helper for circuit/reanchoring/src/main.nr: NOT a proof
// circuit (only `nargo execute` + println). Builds a real prev_hash-linked
// witness from real chain data without a Go-side hash_header implementation
// -- prove_and_submit.sh greps the printed output into the real Prover.toml.

global N_MAX: u32 = 256;

struct Header {
    prev_hash: Field,
    fsm_state: Field,
    withdrawal_locked: Field,
    state_root: Field,
}

fn hash_header(h: Header) -> Field {
    // Byte-identical, hand-kept copy of the real circuit's hash_header;
    // N_MAX must stay in sync too.
    let state = [h.prev_hash, h.fsm_state, h.withdrawal_locked, h.state_root];
    std::hash::poseidon2_permutation(state)[0]
}

// raw_* are the per-header values tracked by types.RecoveryHeader, without
// prev_hash (derived here). Only the first `count` are real.
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
    // rt_new must be hash_header(headers[count-1]), not the raw state_root
    // field -- prev_hash already holds that value after the loop above.
    println(f"rt_new_hash={prev_hash}");
}

#[test]
fn hash_header_matches_main_circuit_fixture() {
    // Cross-checks against the real circuit's fixture -- a failure here
    // means the two hand-kept copies have drifted.
    let h0 = Header { prev_hash: 111, fsm_state: 2, withdrawal_locked: 1, state_root: 1000 };
    assert(hash_header(h0) == 0x1d480f47dfb5f6045ee6b8393646287ac84f230ee5faa8a8d77547383b275f95);
}
