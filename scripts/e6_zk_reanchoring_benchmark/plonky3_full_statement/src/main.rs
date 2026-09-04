//! Plonky3 "full statement" recovery-proof circuit: header-chain continuity
//! (a raw Poseidon2 t=4 permutation, not a mocked hash), FSM-state/
//! withdrawal-lock legality, and root binding to rt_last/rt_new -- the same
//! (a)-(d) conditions `circuit/reanchoring/src/main.nr`'s
//! `verify_header_chain` proves, ported to a Plonky3 AIR instead of
//! Noir/UltraHonk.
//!
//! ed25519 boundary-signature verification ((e) in main.nr) is NOT
//! implemented here -- see this directory's benchmark report for why and
//! what a real implementation needs.
//!
//! # Design
//!
//! One trace row = one header = one raw Poseidon2 permutation over the
//! header's 4 fixed fields (`prev_hash`, `fsm_state`, `withdrawal_locked`,
//! `state_root`), matching main.nr's `hash_header` (see header.rs's module
//! doc for the field-choice caveat: BabyBear here vs. BN254 Fr there).
//! `p3-poseidon2-air::Poseidon2Air` supplies the permutation's own
//! constraints wholesale (reused via `p3_uni_stark::SubAirBuilder`, not
//! reimplemented, matching the previous SHA-256 version's pattern of
//! delegating to `p3-sha256-air::Sha256Air`); this AIR only adds what
//! main.nr's `hash_header` doesn't already cover:
//!
//! - (a) row-to-row chaining: next row's `inputs[0]` (its `prev_hash`) must
//!   equal this row's digest (`ending_full_rounds[last].post[0]`) -- a
//!   genuine AIR *transition* constraint, since `Poseidon2Air` itself has
//!   none (each row is an independent permutation instance).
//! - (b) `fsm_state` in `{2, 3}`.
//! - (c) `withdrawal_locked` forced to exactly `1`.
//! - (d) root binding: first row's `prev_hash` = `rt_last`; last row's
//!   digest = `rt_new` (both baked into the AIR instance as constants --
//!   previously 256-bit arrays under the SHA-256 version, now a single
//!   Field element each, since Poseidon2 operates natively on field
//!   elements rather than bytes -- no more bit-decomposition/big-endian
//!   byte-encoding machinery is needed anywhere in this AIR).
//!
//! rt_last/rt_new are baked into the AIR instance as constants (`Air::eval`
//! compares trace columns against them), not routed through
//! `StarkGenericConfig`'s public-values slot: this crate reuses
//! `p3_examples::proofs::prove_monty31_keccak`, which hardcodes `&[]` for
//! public values. Both prover and verifier build the identical
//! `HeaderChainAir{poseidon2, rt_last, rt_new}` instance, so the same
//! binding property holds -- see the benchmark report for why this
//! simplification was made and what wiring true public inputs would need.

use clap::Parser;
use core::borrow::Borrow;
use p3_air::{Air, AirBuilder, BaseAir, WindowAccess};
use p3_challenger::FieldChallenger;
use p3_commit::PolynomialSpace;
use p3_field::ExtensionField;
use p3_field::PrimeCharacteristicRing;
use p3_field::extension::BinomialExtensionField;
use p3_examples::proofs::{prove_monty31_keccak, report_result};
use p3_matrix::dense::RowMajorMatrix;
use p3_poseidon2_air::Poseidon2Air;
use p3_uni_stark::{StarkGenericConfig, SubAirBuilder};
mod header;
use header::{
    F, HALF_FULL_ROUNDS, HEADER_ROW_WIDTH, HeaderPoseidon2LinearLayers, PARTIAL_ROUNDS,
    Poseidon2ColsT, SBOX_DEGREE, SBOX_REGISTERS, WIDTH, build_header_chain, build_trace,
    hash_header, round_constants,
};

type Challenge = BinomialExtensionField<F, 4>;

type HeaderPoseidon2Air = Poseidon2Air<
    F,
    HeaderPoseidon2LinearLayers,
    WIDTH,
    SBOX_DEGREE,
    SBOX_REGISTERS,
    HALF_FULL_ROUNDS,
    PARTIAL_ROUNDS,
>;

/// The AIR: header-chain continuity + FSM/withdrawal-lock legality + root
/// binding, for a fixed `n`-header chain whose endpoints are baked in as
/// `rt_last`/`rt_new`. See module doc for the exact statement.
#[derive(Debug, Clone)]
pub struct HeaderChainAir {
    poseidon2: HeaderPoseidon2Air,
    rt_last: F,
    rt_new: F,
}

impl HeaderChainAir {
    /// Build the AIR instance for a deterministic n-header chain
    /// (`header::build_header_chain`), baking in its real rt_last/rt_new.
    fn for_n(n: usize) -> Self {
        let constants = round_constants();
        let headers = build_header_chain(n);
        let rt_last = headers[0].prev_hash;
        let rt_new = hash_header(headers.last().expect("n >= 1"), &constants);
        Self {
            poseidon2: Poseidon2Air::new(constants),
            rt_last,
            rt_new,
        }
    }
}

impl<F: Sync> BaseAir<F> for HeaderChainAir {
    fn width(&self) -> usize {
        HEADER_ROW_WIDTH
    }
}

impl<AB: AirBuilder<F = F>> Air<AB> for HeaderChainAir {
    fn eval(&self, builder: &mut AB) {
        // Delegate to Poseidon2Air's own (unmodified, upstream-tested)
        // constraints for the header's raw t=4 permutation -- reused via
        // p3_uni_stark::SubAirBuilder, not reimplemented.
        {
            let mut sub =
                SubAirBuilder::<AB, HeaderPoseidon2Air, ()>::new(builder, 0..HEADER_ROW_WIDTH);
            self.poseidon2.eval(&mut sub);
        }

        // Snapshot the current + next row views (Var: Copy) before taking
        // further `&mut` sub-builders (when_transition/when_first_row/
        // when_last_row all need `&mut self`).
        let (inputs, digest, next_prev_hash) = {
            let main = builder.main();
            let cur = main.current_slice();
            let nxt = main.next_slice();
            let cur_cols: &Poseidon2ColsT<AB::Var> = cur.borrow();
            let nxt_cols: &Poseidon2ColsT<AB::Var> = nxt.borrow();
            (
                cur_cols.inputs,
                cur_cols.ending_full_rounds[HALF_FULL_ROUNDS - 1].post[0],
                nxt_cols.inputs[0],
            )
        };

        // (b) FSM-state legality: fsm_state in {2, 3} -- main.nr's condition (b).
        builder.assert_zero((inputs[1] - AB::F::TWO) * (inputs[1] - AB::F::from_u8(3)));

        // (c) withdrawal_locked forced true -- main.nr's condition (c).
        builder.assert_one(inputs[2]);

        // (a) Header continuity: next row's prev_hash = this row's digest.
        builder.when_transition().assert_eq(next_prev_hash, digest);

        // (d) Root binding: first row's prev_hash = rt_last; last row's digest = rt_new.
        builder.when_first_row().assert_eq(inputs[0], self.rt_last);
        builder.when_last_row().assert_eq(digest, self.rt_new);
    }
}

impl<Domain, EF, Challenger, Pcs, SC> p3_examples::airs::ExampleHashAir<F, SC> for HeaderChainAir
where
    Domain: PolynomialSpace<Val = F>,
    EF: ExtensionField<F>,
    Challenger: FieldChallenger<F>,
    Pcs: p3_commit::Pcs<EF, Challenger, Domain = Domain>,
    SC: StarkGenericConfig<Pcs = Pcs, Challenge = EF, Challenger = Challenger>,
{
    fn generate_trace_rows(&self, num_hashes: usize, extra_capacity_bits: usize) -> RowMajorMatrix<F> {
        let headers = build_header_chain(num_hashes);
        let constants = round_constants();
        build_trace(&headers, &constants, extra_capacity_bits)
    }
}

#[derive(Parser, Debug)]
struct Args {
    /// Number of chained headers (rows). Must be a power of two.
    #[arg(long)]
    n: usize,
}

#[cfg(test)]
mod tests {
    use super::*;
    use p3_air::check_constraints;

    /// Cheap sanity check (no STARK proving): the AIR must accept a
    /// genuinely valid n-header trace under symbolic constraint evaluation.
    #[test]
    fn valid_trace_satisfies_constraints_n2() {
        let air = HeaderChainAir::for_n(2);
        let constants = round_constants();
        let headers = build_header_chain(2);
        let trace: RowMajorMatrix<F> = build_trace(&headers, &constants, 0);
        check_constraints(&air, &trace, &[]);
    }

    #[test]
    fn valid_trace_satisfies_constraints_n8() {
        let air = HeaderChainAir::for_n(8);
        let constants = round_constants();
        let headers = build_header_chain(8);
        let trace: RowMajorMatrix<F> = build_trace(&headers, &constants, 0);
        check_constraints(&air, &trace, &[]);
    }

    #[test]
    #[should_panic(expected = "constraints not satisfied")]
    fn tampered_fsm_state_is_rejected() {
        // Corrupt header 0's fsm_state to a value outside {2, 3} and
        // regenerate its (fully self-consistent) round trace from that --
        // must be rejected by the fsm_state-legality constraint. Poseidon2
        // has no bit-decomposed bytes to flip a single bit of (unlike the
        // old SHA-256 version): the header field is a raw Field element, so
        // the direct/legible corruption is to set it outside {2, 3} itself.
        let air = HeaderChainAir::for_n(2);
        let constants = round_constants();
        let mut headers = build_header_chain(2);
        headers[0].fsm_state = F::from_u8(5);
        let trace: RowMajorMatrix<F> = build_trace(&headers, &constants, 0);
        check_constraints(&air, &trace, &[]);
    }

    #[test]
    #[should_panic(expected = "constraints not satisfied")]
    fn broken_chain_link_is_rejected() {
        // Corrupt header 1's prev_hash so it no longer equals header 0's
        // real digest, and regenerate its (fully self-consistent) round
        // trace from that -- the transition constraint must reject it.
        let air = HeaderChainAir::for_n(2);
        let constants = round_constants();
        let mut headers = build_header_chain(2);
        headers[1].prev_hash += F::ONE;
        let trace: RowMajorMatrix<F> = build_trace(&headers, &constants, 0);
        check_constraints(&air, &trace, &[]);
    }
}

fn main() {
    use tracing_subscriber::layer::SubscriberExt;
    use tracing_subscriber::util::SubscriberInitExt;
    let env_filter = tracing_subscriber::EnvFilter::builder()
        .with_default_directive(tracing::level_filters::LevelFilter::INFO.into())
        .from_env_lossy();
    tracing_subscriber::Registry::default()
        .with(env_filter)
        .with(tracing_forest::ForestLayer::default())
        .init();

    let args = Args::parse();
    assert!(args.n.is_power_of_two(), "N must be a power of two");

    let air = HeaderChainAir::for_n(args.n);
    let dft = p3_dft::Radix2Bowers;

    let result = prove_monty31_keccak::<F, Challenge, _, _>(&air, dft, args.n);
    report_result(result);
}
