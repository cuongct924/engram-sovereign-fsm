// circuit/reanchoring_witness/src/main.nr
//
// Off-chain witness-generation helper for circuit/reanchoring/src/main.nr's
// real Header.prev_hash chain -- this is NOT a proof circuit itself: no
// proof is ever generated from this crate, only `nargo execute` + println
// output. It exists solely so a real prev_hash-linked witness can be built
// from real chain data (x/sovereignty/keeper.HeaderHistory, dumped via
// `engramd query sovereignty recovery-headers`) WITHOUT a Go-side
// Poseidon/pedersen_hash implementation -- computing hash_header identically
// to the real circuit is the only thing that matters here, so this crate
// deliberately mirrors main.nr's Header/hash_header verbatim (kept in sync
// by hand -- same "compile-time constant kept in sync manually across
// files" pattern already used by circuit/reanchoring/src/main.nr's own E6
// benchmark scripting; N below MUST match circuit/reanchoring/src/main.nr's
// global N).
//
// scripts/reanchoring_prover.sh (A6) writes this crate's Prover.toml from
// real chain data (rt_last plus raw {fsm_state, withdrawal_locked,
// state_root} per header, deliberately WITHOUT prev_hash -- that's exactly
// what this crate derives), runs `nargo execute witness --show-output`, and
// greps the printed, now-linked chain into the REAL circuit's Prover.toml.

global N: u32 = 4;

struct Header {
    prev_hash: Field,
    fsm_state: Field,
    withdrawal_locked: Field,
    state_root: Field,
}

fn hash_header(h: Header) -> Field {
    // Kept byte-for-byte identical to circuit/reanchoring/src/main.nr's
    // hash_header (see that file's comment for the full rationale of this
    // Poseidon2 sponge construction) -- must stay in sync by hand.
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

// raw_fsm_state / raw_withdrawal_locked / raw_state_root are the per-header
// fields as tracked by the Go keeper (types.RecoveryHeader) -- no prev_hash,
// since that's what this crate derives. rt_last seeds the chain, matching
// circuit/reanchoring/src/main.nr's own headers[0].prev_hash == rt_last
// binding.
fn main(
    rt_last: pub Field,
    raw_fsm_state: [Field; N],
    raw_withdrawal_locked: [Field; N],
    raw_state_root: [Field; N],
) {
    let mut prev_hash = rt_last;
    for i in 0..N {
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

#[test]
fn hash_header_matches_main_circuit_fixture() {
    // Cross-check against circuit/reanchoring/src/main.nr's own
    // valid_chain_satisfies_circuit fixture -- this crate's hash_header is a
    // hand-kept copy of the real circuit's (Noir has no cross-crate import
    // for a single tiny function without a shared library package), so a
    // failure here means the two have drifted and this crate's output would
    // no longer link into a witness the real circuit accepts.
    let h0 = Header { prev_hash: 111, fsm_state: 2, withdrawal_locked: 1, state_root: 1000 };
    assert(hash_header(h0) == 0x168c1f5895ad57e0d8257da085aab977ec14ed66699553157cd0bcffcd02bb3c);
}
