//! Host-side (non-circuit) header chain construction and the real Poseidon2
//! permutation, used both to build the witness trace and to compute the
//! fixed rt_last/rt_new constants the AIR checks against.
//!
//! Mirrors `circuit/reanchoring/src/main.nr`'s `Header` struct and
//! `hash_header` -- a raw Poseidon2 t=4 permutation over the header's 4
//! fixed fields, taking output[0] as the digest (no sponge, no
//! absorb/squeeze -- matches main.nr's `poseidon2_permutation(state)[0]`)
//! -- but is a fresh, from-scratch implementation (not a port): the real
//! circuit runs over BN254 Fr via gnark-crypto's
//! `poseidon2.NewPermutation(4, 8, 56)`, while this benchmark stays on
//! BabyBear (this crate's whole STARK pipeline -- `prove_monty31_keccak`,
//! `Radix2Bowers`, etc. -- is Monty31-specific). The round *counts* (8
//! full, 56 partial) are carried over from the real circuit's parameters;
//! the round *constants* and the width-4 internal linear layer below are
//! this benchmark's own, generated via `p3-poseidon2-air`'s own
//! `RoundConstants` machinery, not hand-derived to match gnark-crypto's.

use core::borrow::Borrow;

use p3_baby_bear::BabyBear;
use p3_field::PrimeCharacteristicRing;
use p3_matrix::dense::RowMajorMatrix;
use p3_poseidon2::GenericPoseidon2LinearLayers;
use p3_poseidon2_air::{RoundConstants, generate_trace_rows, num_cols};
use rand::SeedableRng;
use rand::rngs::SmallRng;

pub type F = BabyBear;

pub const WIDTH: usize = 4;
pub const SBOX_DEGREE: u64 = 7;
pub const SBOX_REGISTERS: usize = 1;
pub const HALF_FULL_ROUNDS: usize = 4; // RF = 8, matching main.nr's gnark-crypto poseidon2.NewPermutation(4, 8, 56).
pub const PARTIAL_ROUNDS: usize = 56; // RP = 56, ditto.

pub const HEADER_ROW_WIDTH: usize =
    num_cols::<WIDTH, SBOX_DEGREE, SBOX_REGISTERS, HALF_FULL_ROUNDS, PARTIAL_ROUNDS>();

pub type Poseidon2ColsT<T> = p3_poseidon2_air::Poseidon2Cols<
    T,
    WIDTH,
    SBOX_DEGREE,
    SBOX_REGISTERS,
    HALF_FULL_ROUNDS,
    PARTIAL_ROUNDS,
>;

/// Width-4 Poseidon2 internal (partial-round) linear layer: `(1 + diag(V))`
/// with `V = [-2, 1, 2, 4]` -- distinct, nonzero diagonal entries, following
/// the same "-2 first, small ints/powers-of-2" pattern BabyBear's own tuned
/// width-16/24/32 diagonals use (`baby-bear/src/poseidon2.rs`). Plonky3
/// ships no built-in BabyBear width-4 config (only 16/24/32), so this crate
/// defines its own rather than leaving `GenericPoseidon2LinearLayers::
/// internal_linear_layer` unimplemented; the external layer is the crate's
/// own default (`mds_light_permutation` + `MDSMat4`), unoverridden.
#[derive(Debug, Clone, Copy, Default)]
pub struct HeaderPoseidon2LinearLayers;

impl GenericPoseidon2LinearLayers<WIDTH> for HeaderPoseidon2LinearLayers {
    fn internal_linear_layer<R: PrimeCharacteristicRing>(state: &mut [R; WIDTH]) {
        let sum = state[0].dup() + state[1].dup() + state[2].dup() + state[3].dup();
        state[0] = sum.dup() - state[0].double();
        state[1] = sum.dup() + state[1].dup();
        state[2] = sum.dup() + state[2].double();
        state[3] = sum + state[3].double().double();
    }
}

/// Fixed-seed round constants, shared by chain construction, the AIR
/// instance, and trace generation -- `RoundConstants::from_rng` with a
/// fixed seed, the same idiom `examples/prove_prime_field_31.rs`'s
/// `ProofOptions::Poseidon2Permutations` arm uses to build its own
/// (width-16) `VectorizedPoseidon2Air`, not hand-rolled constants.
pub fn round_constants() -> RoundConstants<F, WIDTH, HALF_FULL_ROUNDS, PARTIAL_ROUNDS> {
    RoundConstants::from_rng(&mut SmallRng::seed_from_u64(1))
}

#[derive(Debug, Clone, Copy)]
pub struct Header {
    pub prev_hash: F,
    pub fsm_state: F,         // 2 = SOVEREIGN, 3 = RECOVERING
    pub withdrawal_locked: F, // always 1 on a real header
    pub state_root: F,
}

impl Header {
    fn as_input(&self) -> [F; WIDTH] {
        [
            self.prev_hash,
            self.fsm_state,
            self.withdrawal_locked,
            self.state_root,
        ]
    }
}

/// hash_header via `p3-poseidon2-air`'s own trace generator (reused as-is,
/// not reimplemented): builds a length-1 trace for this single input and
/// reads back the permutation's `output[0]`, matching main.nr's
/// `poseidon2_permutation(state)[0]`.
pub fn hash_header(
    h: &Header,
    constants: &RoundConstants<F, WIDTH, HALF_FULL_ROUNDS, PARTIAL_ROUNDS>,
) -> F {
    let trace: RowMajorMatrix<F> = generate_trace_rows::<
        F,
        HeaderPoseidon2LinearLayers,
        WIDTH,
        SBOX_DEGREE,
        SBOX_REGISTERS,
        HALF_FULL_ROUNDS,
        PARTIAL_ROUNDS,
    >(vec![h.as_input()], constants, 0);
    let cols: &Poseidon2ColsT<F> = trace.values[..].borrow();
    cols.ending_full_rounds[HALF_FULL_ROUNDS - 1].post[0]
}

/// Build a real, hash-linked chain of `n` headers -- deterministic in `n`
/// alone (fixed round constants + fixed seed values below), so the AIR's
/// baked rt_last/rt_new constants (computed once by the caller) always
/// match what this regenerates for the STARK trace.
pub fn build_header_chain(n: usize) -> Vec<Header> {
    let constants = round_constants();
    let mut headers = Vec::with_capacity(n);
    let mut prev = F::from_u32(0x1111_1111);
    for i in 0..n {
        let fsm_state = if i % 2 == 0 { F::from_u8(2) } else { F::from_u8(3) };
        let state_root = F::from_u32(1000 + i as u32);
        let h = Header {
            prev_hash: prev,
            fsm_state,
            withdrawal_locked: F::ONE,
            state_root,
        };
        prev = hash_header(&h, &constants);
        headers.push(h);
    }
    headers
}

/// Populate a `[Poseidon2Cols; 1]`-per-row trace for the header chain, using
/// `p3-poseidon2-air`'s own public `generate_trace_rows` (reused as-is, not
/// reimplemented) -- one call across all N headers, since N is already
/// required to be a power of two.
pub fn build_trace(
    headers: &[Header],
    constants: &RoundConstants<F, WIDTH, HALF_FULL_ROUNDS, PARTIAL_ROUNDS>,
    extra_capacity_bits: usize,
) -> RowMajorMatrix<F> {
    let inputs: Vec<[F; WIDTH]> = headers.iter().map(Header::as_input).collect();
    generate_trace_rows::<
        F,
        HeaderPoseidon2LinearLayers,
        WIDTH,
        SBOX_DEGREE,
        SBOX_REGISTERS,
        HALF_FULL_ROUNDS,
        PARTIAL_ROUNDS,
    >(inputs, constants, extra_capacity_bits)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn build_header_chain_is_really_hash_linked() {
        let constants = round_constants();
        let headers = build_header_chain(4);
        for i in 1..4 {
            assert_eq!(headers[i].prev_hash, hash_header(&headers[i - 1], &constants));
        }
    }
}
