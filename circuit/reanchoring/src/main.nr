// circuit/reanchoring/src/main.nr
//
// Recovery proof circuit -- proves a SOVEREIGN/RECOVERING interval of N
// headers is well-formed before re-anchoring to ANCHORED, mirroring
// VerifyZkProof's intent (spec/core/EngramTendermint.tla:257-260). N is the
// number of sovereign blocks in the recovery interval; docs/EXPERIMENT.md's
// E6 Table 6B scales this from 10 to 5,000 by recompiling with different N
// (see scripts/zk_bench.sh).

global N: u32 = 4;

struct Header {
    prev_hash: Field,
    fsm_state: Field,        // 2: SOVEREIGN, 3: RECOVERING
    withdrawal_locked: Field, // 1: true, 0: false
    state_root: Field,
}

fn hash_header(h: Header) -> Field {
    std::hash::pedersen_hash([h.prev_hash, h.fsm_state, h.withdrawal_locked, h.state_root])
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

#[test(should_fail)]
fn wrong_fsm_state_fails_circuit() {
    let h0 = Header { prev_hash: 111, fsm_state: 1, withdrawal_locked: 1, state_root: 1000 }; // 1 = ANCHORED, not allowed
    let h1 = Header { prev_hash: hash_header(h0), fsm_state: 2, withdrawal_locked: 1, state_root: 1001 };
    let h2 = Header { prev_hash: hash_header(h1), fsm_state: 3, withdrawal_locked: 1, state_root: 1002 };
    let h3 = Header { prev_hash: hash_header(h2), fsm_state: 3, withdrawal_locked: 1, state_root: 1003 };
    main(111, 1003, [h0, h1, h2, h3]);
}

// dump_chain is a one-off generator, not a real test: it prints a long
// valid hash-chain to stdout so scripts/e6_zk_reanchoring_benchmark's
// benchmark script can slice prefixes of it into per-N Prover.toml files
// without reimplementing Noir's pedersen_hash in another language. Run via
// `nargo test dump_chain --show-output`, not part of CI.
global CHAIN_LEN: u32 = 256;

#[test]
fn dump_chain() {
    let mut chain = [Header { prev_hash: 0, fsm_state: 2, withdrawal_locked: 1, state_root: 0 }; CHAIN_LEN];
    chain[0] = Header { prev_hash: 111, fsm_state: 2, withdrawal_locked: 1, state_root: 1000 };
    for i in 1..CHAIN_LEN {
        let fsm = if i % 2 == 0 { 2 } else { 3 };
        chain[i] = Header {
            prev_hash: hash_header(chain[i - 1]),
            fsm_state: fsm,
            withdrawal_locked: 1,
            state_root: 1000 + i as Field,
        };
    }
    for i in 0..CHAIN_LEN {
        println(chain[i]);
    }
}
