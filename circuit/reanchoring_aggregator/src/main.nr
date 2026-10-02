// Chunk aggregator: verifies up to K proofs of circuit/reanchoring (each covering
// up to 256 headers) and chains them into ONE proof with the SAME 7 public inputs
// as a single leaf, so SubmitRecoveryProof's 224-byte layout is unchanged.
//
// Slots i >= n_real are padding: verify_honk_proof is unconditional, so a padding
// slot carries a copy of a real, valid leaf proof, and every constraint that uses
// the slot's values is gated off by `active`.
mod leaf_vk;

use bb_proof_verification::{verify_honk_proof, UltraHonkZKProof};
use leaf_vk::{LEAF_KEY_HASH, LEAF_VK};

global K: u32 = 4;
global LEAF_PUBLIC_INPUTS: u32 = 7;

fn main(
    rt_last: pub Field,
    rt_new: pub Field,
    count: pub u32,
    first_height: pub u64,
    first_block_hash: pub Field,
    last_block_hash: pub Field,
    validator_set_hash: pub Field,
    n_real: u32,
    leaf_proofs: [UltraHonkZKProof; K],
    // Each leaf's public inputs, in circuit/reanchoring's order:
    // rt_last, rt_new, count, first_height, first_block_hash, last_block_hash, validator_set_hash.
    leaf_pi: [[Field; LEAF_PUBLIC_INPUTS]; K],
) {
    assert(n_real >= 1);
    assert(n_real <= K);

    let mut total_count: Field = 0;
    let mut out_rt_new: Field = 0;
    let mut out_last_block_hash: Field = 0;

    for i in 0..K {
        verify_honk_proof(LEAF_VK, leaf_proofs[i], leaf_pi[i], LEAF_KEY_HASH);

        let active = i < n_real;
        let is_last = i == n_real - 1;
        if i > 0 {
            // Chunk i continues exactly where chunk i-1 ended, under the same validator set.
            assert(!active | (leaf_pi[i][0] == leaf_pi[i - 1][1]));
            assert(!active | (leaf_pi[i][3] == leaf_pi[i - 1][3] + leaf_pi[i - 1][2]));
            assert(!active | (leaf_pi[i][6] == leaf_pi[0][6]));
        }
        total_count += (active as Field) * leaf_pi[i][2];
        out_rt_new += (is_last as Field) * leaf_pi[i][1];
        out_last_block_hash += (is_last as Field) * leaf_pi[i][5];
    }

    assert(rt_last == leaf_pi[0][0]);
    assert(first_height as Field == leaf_pi[0][3]);
    assert(first_block_hash == leaf_pi[0][4]);
    assert(validator_set_hash == leaf_pi[0][6]);
    assert(rt_new == out_rt_new);
    assert(last_block_hash == out_last_block_hash);
    assert(count as Field == total_count);
}
