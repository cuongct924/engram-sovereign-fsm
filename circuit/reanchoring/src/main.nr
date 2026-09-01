// circuit/reanchoring/src/main.nr
//
// Recovery proof circuit: proves a SOVEREIGN/RECOVERING interval of up to
// N_MAX headers is well-formed before re-anchoring to ANCHORED, mirroring
// VerifyZkProof (spec/core/EngramTendermint.tla:257-260), AND that the
// interval's two boundary blocks were really committed by >=2/3 of the
// genesis validator set (circuit/README.md's Scope section). For design
// rationale, measurements, and the fixed-N=4 / recursion history see
// circuit/reanchoring/README.md.
//
// Implementation: `count` (1..=N_MAX) is a real public input -- the ACTUAL
// number of real headers in this proof -- not implicit in the circuit size;
// N_MAX=256 is a compile-time ceiling. Slots at index >= count are padding:
// no constraint is enforced on them (see the `active` gate below).

use bignum::{bignum::BigNum, ED25519_Fq as Fq};
use edwards_bignum::ed25519::Ed25519;
use edwards_bignum::ScalarField;
use ed25519::ed25519_verify;
use ed25519::compress;
use sha256::digest;

global N_MAX: u32 = 256;
global NUM_VALIDATORS: u32 = 7;
global CHAIN_ID: [u8; 12] = [
    0x65, 0x6e, 0x67, 0x72, 0x61, 0x6d, 0x2d, 0x64, 0x65, 0x76, 0x2d, 0x31,
]; // "engram-dev-1"

struct Header {
    prev_hash: Field,
    fsm_state: Field,        // 2: SOVEREIGN, 3: RECOVERING
    withdrawal_locked: Field, // 1: true, 0: false
    state_root: Field,
}

// hash_header chains a Header via SHA-256, not Poseidon2: Poseidon2 t=4 over
// BN254 has no independently-verified Go implementation (gnark-crypto's
// bn254 poseidon2 package supports only t=2,3; the Horizen Labs reference
// implementation has a documented history of bugs specifically in the t=4
// external linear layer) -- an unacceptable correctness risk for a value
// the Go keeper must reproduce bit-for-bit every block
// (x/sovereignty/types.HashHeader, called from CommitFSMTransition). SHA-256
// has no tunable parameters, so Go's crypto/sha256 and this circuit
// (noir-lang/sha256) are guaranteed to agree -- cross-verified byte-for-byte
// against Go's stdlib before this change landed. Costs more per header than
// Poseidon2 would have (see circuit/reanchoring/README.md's benchmark
// section); the trade is deliberate.
fn hash_header(h: Header) -> Field {
    let mut bytes: [u8; 128] = [0; 128];
    let b0 = h.prev_hash.to_be_bytes::<32>();
    let b1 = h.fsm_state.to_be_bytes::<32>();
    let b2 = h.withdrawal_locked.to_be_bytes::<32>();
    let b3 = h.state_root.to_be_bytes::<32>();
    for i in 0..32 {
        bytes[i] = b0[i];
        bytes[32 + i] = b1[i];
        bytes[64 + i] = b2[i];
        bytes[96 + i] = b3[i];
    }
    reduce_to_field(digest(bytes))
}

// reduce_to_field is the big-endian byte-to-Field fold shared by
// hash_header's digest reduction and the boundary-binding block/validator-set
// hash checks below -- Field arithmetic reduces mod the BN254 scalar field
// automatically, matching Go's x/sovereignty/types.ReduceToField exactly
// (both are mod-p reduction of the same big-endian 32-byte integer).
fn reduce_to_field(bytes: [u8; 32]) -> Field {
    let mut acc: Field = 0;
    for i in 0..32 {
        acc = acc * 256 + bytes[i] as Field;
    }
    acc
}

fn le_bytes_to_field(bytes: [u8; 32]) -> Field {
    let mut acc: Field = 0;
    for i in 0..32 {
        acc += (bytes[i] as Field) * (256 as Field).pow_32(i as Field);
    }
    acc
}

// build_simple_validator_bytes reconstructs the exact protobuf encoding of
// tendermint.types.SimpleValidator{PubKey, VotingPower} that
// types.Validator.Bytes() produces and ValidatorSet.Hash() hashes as each
// Merkle leaf -- see crypto/merkle.HashFromByteSlices in the
// engram-consensus-core fork. voting_power is constrained to < 128 (a
// single-byte protobuf varint) -- true for every validator in this
// deployment's genesis; a real deployment with larger per-validator power
// would need a general varint encoder here, a documented scope limit.
fn build_simple_validator_bytes(pubkey_compressed: [u8; 32], voting_power: u8) -> [u8; 38] {
    assert(voting_power < 128);
    let mut out: [u8; 38] = [0; 38];
    out[0] = 0x0a;
    out[1] = 0x22;
    out[2] = 0x0a;
    out[3] = 0x20;
    for i in 0..32 {
        out[4 + i] = pubkey_compressed[i];
    }
    out[36] = 0x10;
    out[37] = voting_power;
    out
}

fn sha256_leaf(data: [u8; 38]) -> [u8; 32] {
    let mut preimage: [u8; 39] = [0; 39];
    preimage[0] = 0; // leafPrefix
    for i in 0..38 {
        preimage[1 + i] = data[i];
    }
    digest(preimage)
}

fn sha256_inner(left: [u8; 32], right: [u8; 32]) -> [u8; 32] {
    let mut preimage: [u8; 65] = [0; 65];
    preimage[0] = 1; // innerPrefix
    for i in 0..32 {
        preimage[1 + i] = left[i];
        preimage[33 + i] = right[i];
    }
    digest(preimage)
}

// validator_set_hash_7 reproduces types.ValidatorSet.Hash() for EXACTLY 7
// validators -- crypto/merkle.HashFromByteSlices's split-point recursion
// (largest power of two < n) is hardcoded for n=7 rather than implemented
// generically, since this deployment's genesis validator set is immutable
// for the network's lifetime (CLAUDE.md: "no staking-driven rotation") and
// the tree SHAPE is a fixed, compile-time-known function of n. A future
// validator-count change requires recompiling this circuit (and its VK)
// with a re-derived shape -- not a supported runtime parameter. Takes each
// validator's ALREADY-COMPUTED leaf hash
// (sha256_leaf(build_simple_validator_bytes(pubkey, power))).
fn validator_set_hash_7(leaf: [[u8; 32]; 7]) -> [u8; 32] {
    let ll = sha256_inner(leaf[0], leaf[1]);
    let lr = sha256_inner(leaf[2], leaf[3]);
    let l = sha256_inner(ll, lr);
    let rl = sha256_inner(leaf[4], leaf[5]);
    let r = sha256_inner(rl, leaf[6]);
    sha256_inner(l, r)
}

// encode_varint_u64 is a protobuf-style base-128 varint ENCODER (not a
// parser): given a value known at witness-build time, deterministically
// produces its wire bytes. Bounded to 5 bytes (covers any u32, comfortably
// covers real block heights/part-counts for the lifetime of this testnet).
fn encode_varint_u64<let MAX: u32>(v: Field) -> BoundedVec<u8, MAX> {
    let mut out: BoundedVec<u8, MAX> = BoundedVec::new();
    let mut remaining = v;
    let mut done = false;
    for _ in 0..MAX {
        if !done {
            let byte = remaining as u64 % 128;
            remaining = (remaining as u64 / 128) as Field;
            if remaining as u64 == 0 {
                out.push(byte as u8);
                done = true;
            } else {
                out.push((byte + 128) as u8);
            }
        }
    }
    out
}

// build_vote_sign_bytes reconstructs the exact bytes a CometBFT validator
// ed25519-signs for a Precommit vote (types.VoteSignBytes: MarshalDelimited
// of the protobuf CanonicalVote{Type, Height, BlockID, Timestamp, ChainID},
// round omitted since proto3 elides zero-valued scalar fields -- only
// round==0 precommits are supported by this circuit, a documented scope
// limit, not a silent gap).
//
// height and block_hash are the values the boundary-binding check BINDS to
// public inputs; part_set_header and timestamp are real vote fields that
// must be included in what was signed but whose specific values don't need
// public verification (BlockID.Hash alone identifies the block -- see
// circuit/README.md), so they're taken as opaque witness bytes. chain_id is
// a compile-time constant, NOT taken from the witness, so a colluding
// validator subset cannot pre-sign a vote for a different chain and replay
// it here.
fn build_vote_sign_bytes<let MAX_LEN: u32, let MAX_TS_LEN: u32>(
    height: u64,
    block_hash: [u8; 32],
    ps_total: u32,
    ps_hash: [u8; 32],
    timestamp_content: BoundedVec<u8, MAX_TS_LEN>,
    chain_id: [u8; 12],
) -> BoundedVec<u8, MAX_LEN> {
    let mut body: BoundedVec<u8, MAX_LEN> = BoundedVec::new();

    // field 1: type = Precommit (2), varint wiretype
    body.push(0x08);
    body.push(0x02);

    // field 2: height, fixed64 (little-endian), wiretype 1
    body.push(0x11);
    let mut h = height;
    for _ in 0..8 {
        body.push((h % 256) as u8);
        h /= 256;
    }

    // field 4: block_id (embedded CanonicalBlockID), wiretype 2
    // sub: field1 hash (32 bytes, wiretype2) + field2 part_set_header (wiretype2)
    let ps_total_varint: BoundedVec<u8, 5> = encode_varint_u64(ps_total as Field);
    let ps_header_len = 1 + ps_total_varint.len() + 2 + 32;
    let block_id_len = 2 + 32 + 1 + 1 + ps_header_len;
    body.push(0x22);
    let block_id_len_varint: BoundedVec<u8, 5> = encode_varint_u64(block_id_len as Field);
    body.extend_from_bounded_vec(block_id_len_varint);
    body.push(0x0a);
    body.push(0x20);
    body.extend_from_array(block_hash);
    body.push(0x12);
    let ps_header_len_varint: BoundedVec<u8, 5> = encode_varint_u64(ps_header_len as Field);
    body.extend_from_bounded_vec(ps_header_len_varint);
    body.push(0x08);
    body.extend_from_bounded_vec(ps_total_varint);
    body.push(0x12);
    body.push(0x20);
    body.extend_from_array(ps_hash);

    // field 5: timestamp (embedded google.protobuf.Timestamp), wiretype 2,
    // opaque content (see doc comment above).
    body.push(0x2a);
    let ts_len_varint: BoundedVec<u8, 5> = encode_varint_u64(timestamp_content.len() as Field);
    body.extend_from_bounded_vec(ts_len_varint);
    body.extend_from_bounded_vec(timestamp_content);

    // field 6: chain_id (string), wiretype 2.
    body.push(0x32);
    body.push(chain_id.len() as u8);
    body.extend_from_array(chain_id);

    // Outer MarshalDelimited wrapper: varint(len(body)) || body.
    let mut out: BoundedVec<u8, MAX_LEN> = BoundedVec::new();
    let len_varint: BoundedVec<u8, 5> = encode_varint_u64(body.len() as Field);
    out.extend_from_bounded_vec(len_varint);
    out.extend_from_bounded_vec(body);
    out
}

// ValidatorSetWitness is the genesis-fixed validator set (immutable for the
// network's lifetime, CLAUDE.md: "no staking-driven rotation"), shared by
// both boundary-block checks in main() below.
struct ValidatorSetWitness {
    pubkey_x: [[u8; 32]; 7], // affine x, little-endian (compress()'s convention)
    pubkey_y: [[u8; 32]; 7],
    power: [u8; 7],
}

// BoundaryBlockWitness is the per-block signature data for one of the
// interval's two boundary blocks.
struct BoundaryBlockWitness {
    ps_total: u32,
    ps_hash: [u8; 32],
    // Per-validator, NOT shared: every honest validator signs the same
    // ps_total/ps_hash/block_hash but their OWN local wall-clock timestamp
    // (CometBFT's CanonicalVote.timestamp), and ed25519 verification is
    // exact-byte -- reusing one validator's real timestamp bytes to verify
    // another's real signature always fails except by coincidence (proven
    // live: with a single shared timestamp only the ONE validator it was
    // taken from ever verified, capping signed_power at 1/7 of total power,
    // permanently short of 2/3 quorum -- not a rare edge case, the design
    // could never pass with >1 real signer).
    timestamp_content: [BoundedVec<u8, 16>; 7],
    signed: [bool; 7],
    sig_r_x: [[u8; 32]; 7],
    sig_r_y: [[u8; 32]; 7],
    sig_s: [[u8; 32]; 7], // scalar s, little-endian (fits one Field: < 2^253)
}

// verify_boundary_block asserts that `height`/`block_hash` and
// `validator_set_hash` are backed by >=2/3 real ed25519 precommit signatures
// from the genesis validator set, over the real CometBFT sign-bytes for that
// exact (height, block_hash) pair. This is the circuit side of "boundary
// height, block/commit hash, validator-set hash" binding -- see
// circuit/README.md's Scope section for what this does and does not prove
// relative to a full in-circuit history verification (every header
// individually, not just the two endpoints).
fn verify_boundary_block(
    height: u64,
    block_hash: [u8; 32],
    validator_set_hash: [u8; 32],
    validators: ValidatorSetWitness,
    block: BoundaryBlockWitness,
) {
    let mut leaves: [[u8; 32]; 7] = [[0; 32]; 7];
    let mut total_power: Field = 0;
    let mut signed_power: Field = 0;
    for i in 0..NUM_VALIDATORS {
        // Built per-validator, from THEIR OWN timestamp_content -- see
        // BoundaryBlockWitness.timestamp_content's doc for why a shared
        // message can never work here.
        let message: BoundedVec<u8, 160> = build_vote_sign_bytes(
            height,
            block_hash,
            block.ps_total,
            block.ps_hash,
            block.timestamp_content[i],
            CHAIN_ID,
        );

        let pubkey_point = Ed25519::from((
            Fq::from_le_bytes(validators.pubkey_x[i]),
            Fq::from_le_bytes(validators.pubkey_y[i]),
        ));
        let pubkey_compressed = compress(pubkey_point);
        leaves[i] =
            sha256_leaf(build_simple_validator_bytes(pubkey_compressed, validators.power[i]));
        total_power += validators.power[i] as Field;

        let sig_point = Ed25519::from((
            Fq::from_le_bytes(block.sig_r_x[i]),
            Fq::from_le_bytes(block.sig_r_y[i]),
        ));
        let sig_scalar = ScalarField::<64>::from(le_bytes_to_field(block.sig_s[i]));
        let valid = ed25519_verify(pubkey_point, sig_scalar, sig_point, message);
        if block.signed[i] & valid {
            signed_power += validators.power[i] as Field;
        }
    }

    assert(validator_set_hash_7(leaves) == validator_set_hash);
    // >= 2/3 majority, avoiding division: signed_power * 3 >= total_power * 2
    assert((signed_power * 3) as u64 >= (total_power * 2) as u64);
}

// verify_header_chain is main()'s (a)-(d): chain continuity, FSM/withdrawal-
// lock legality, and root binding -- everything EXCEPT boundary-signature
// binding (e). Split out so tests exercising only header-chain semantics
// don't pay for 14 ed25519 verifications (~257,588 gates each) they don't
// need: nargo test runs a full witness-solve per #[test], and this circuit's
// own #[test] suite calls this ~10x more often than it calls the full
// main() (see the two dedicated *_with_boundary tests below) -- production
// main()'s behavior/public interface is unchanged, this is a pure
// refactor for testability.
fn verify_header_chain(rt_last: Field, rt_new: Field, count: u32, headers: [Header; N_MAX]) {
    assert(count >= 1);
    assert(count <= N_MAX);

    // (a) Header continuity (E6's C1): each REAL header's prev_hash must be
    // the hash of the previous REAL header. Gated: for i >= count (padding)
    // `active` is false, so the assert holds vacuously regardless of
    // headers[i]'s (irrelevant) contents.
    for i in 1..N_MAX {
        let active = i < count;
        assert(!active | (headers[i].prev_hash == hash_header(headers[i - 1])));
    }

    // (b)/(c) FSM legality + withdrawal-lock (E6's C2/C3): every REAL header
    // must be SOVEREIGN/RECOVERING with withdrawals locked. Same gate.
    for i in 0..N_MAX {
        let active = i < count;
        assert(!active | ((headers[i].fsm_state == 2) | (headers[i].fsm_state == 3)));
        assert(!active | (headers[i].withdrawal_locked == 1));
    }

    // (d) Root binding (E6's C4/C5): the interval starts at rt_last and ends
    // at rt_new. headers[0] is always real (count >= 1 above), so this is
    // unconditional; headers[count-1] is a dynamic (witness-valued) index,
    // which Noir compiles to a selector chain over all N_MAX slots.
    //
    // rt_new is bound to hash_header(headers[count-1]), NOT to
    // headers[count-1].state_root directly: state_root is a free witness
    // field, so binding rt_new to it literally would let a prover satisfy
    // this assert for ANY (rt_last, rt_new) pair already recorded on-chain
    // by fabricating a single header (count=1) whose state_root field is
    // just set to rt_new -- the continuity check above never touches that
    // header's OWN fields, only how it's referenced by the NEXT one. Forcing
    // rt_new to be the hash OUTPUT means reproducing it requires knowing the
    // real header content back to rt_last, chained through hash_header at
    // every step -- see x/sovereignty/preblock.go's accumulator, which is
    // the only thing capable of producing that output honestly.
    assert(headers[0].prev_hash == rt_last);
    assert(hash_header(headers[count - 1]) == rt_new);
}

fn main(
    rt_last: pub Field,   // Root of the last anchored block
    rt_new: pub Field,    // Root of the proposed recovery block
    count: pub u32,       // spec/README.md's `n`: real number of headers in this proof, 1..=N_MAX
    headers: [Header; N_MAX],
    // --- Boundary-block binding (circuit/README.md's Scope section) ---
    first_height: pub u64,       // real height of the interval's FIRST header
    first_block_hash: pub Field, // reduce_to_field of that block's real CometBFT hash
    last_block_hash: pub Field,  // reduce_to_field of the LAST header's real CometBFT hash
    validator_set_hash: pub Field, // reduce_to_field of the genesis ValidatorSet.Hash()
    first_block_hash_bytes: [u8; 32],
    last_block_hash_bytes: [u8; 32],
    validator_set_hash_bytes: [u8; 32],
    validators: ValidatorSetWitness,
    first_block: BoundaryBlockWitness,
    last_block: BoundaryBlockWitness,
) {
    verify_header_chain(rt_last, rt_new, count, headers);

    // (e) Boundary binding: the two REAL CometBFT blocks straddling the
    // interval (first_height and first_height+count-1) were really
    // committed by >=2/3 of the genesis validator set -- independent,
    // stronger evidence than "the accumulator, trusted to have been written
    // only by real consensus, says so" (see circuit/README.md's Scope
    // section). Ties the header-chain proof above to real consensus rather
    // than letting a self-consistent-but-fabricated chain and a real-but-
    // unrelated signed block pass as two independently-satisfiable proofs.
    assert(reduce_to_field(first_block_hash_bytes) == first_block_hash);
    assert(reduce_to_field(last_block_hash_bytes) == last_block_hash);
    assert(reduce_to_field(validator_set_hash_bytes) == validator_set_hash);
    verify_boundary_block(
        first_height,
        first_block_hash_bytes,
        validator_set_hash_bytes,
        validators,
        first_block,
    );
    let last_height = first_height + count as u64 - 1;
    verify_boundary_block(
        last_height,
        last_block_hash_bytes,
        validator_set_hash_bytes,
        validators,
        last_block,
    );
}

// build_chain builds a real, valid, hash-linked chain of exactly COUNT
// headers (compile-time generic, e.g. build_chain::<4>()), zero-padded to
// N_MAX, as a test fixture -- mirrors prove_and_submit.sh's real-header-then-
// padding shape, generated in-circuit instead of from Go/RPC data.
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
    let rt_new = hash_header(headers[COUNT - 1]);
    (headers, rt_last, rt_new)
}

// test_boundary_fixture returns a real, ground-truthed boundary-block
// witness (7 real ed25519 keypairs, real CometBFT vote sign-bytes, 5-of-7
// signatures -- >2/3 quorum) reused by every test below: these tests exist
// to check header-chain semantics, not boundary-signature semantics, so one
// fixed, valid boundary fixture (captured from a real run of
// types.VoteSignBytes/ValidatorSet.Hash against the engram-consensus-core
// fork, cross-checked in circuit/reanchoring_sigtest/) is reused for both
// the first and last block in every call.
fn test_boundary_fixture() -> (ValidatorSetWitness, [u8; 32], BoundaryBlockWitness, [u8; 32]) {
    let validators = ValidatorSetWitness {
        pubkey_x: [
            [
                0x1f, 0x47, 0xed, 0xb8, 0x72, 0xb0, 0x22, 0xfa, 0x38, 0xf7, 0x51, 0xde, 0xfc,
                0x31, 0x97, 0x3d, 0x60, 0xad, 0x21, 0xcf, 0x69, 0xee, 0x73, 0x53, 0xeb, 0xe5,
                0xfa, 0x06, 0x7b, 0x00, 0xb0, 0x47,
            ],
            [
                0x97, 0x93, 0xac, 0x71, 0x83, 0x09, 0x97, 0x31, 0x63, 0x21, 0x1b, 0x62, 0x9f,
                0xca, 0xb8, 0xbe, 0x17, 0x25, 0x40, 0x48, 0xde, 0x96, 0x14, 0xc1, 0x2c, 0x86,
                0x81, 0x11, 0x0c, 0xd2, 0x8c, 0x1b,
            ],
            [
                0x72, 0x34, 0x0b, 0x0e, 0x2c, 0xb8, 0x8f, 0x71, 0x14, 0x61, 0xdc, 0xc5, 0xd8,
                0x50, 0xf8, 0xcf, 0x43, 0x99, 0x9b, 0xef, 0xe1, 0x34, 0x69, 0x22, 0x91, 0xc0,
                0x15, 0x87, 0xc2, 0x6a, 0xd9, 0x12,
            ],
            [
                0x38, 0xab, 0x72, 0xbe, 0x32, 0x79, 0xc7, 0x67, 0xb4, 0x3e, 0xc7, 0x96, 0x7a,
                0x74, 0xfc, 0x55, 0xa4, 0x2f, 0xac, 0xab, 0x56, 0x93, 0x83, 0xa1, 0x4e, 0xc8,
                0x83, 0x41, 0x3c, 0xf0, 0xf7, 0x7d,
            ],
            [
                0x98, 0xdb, 0x8e, 0xe8, 0x95, 0xe1, 0xb1, 0x76, 0x50, 0x03, 0x42, 0x56, 0x72,
                0x35, 0x3e, 0x71, 0x4e, 0xa3, 0x9a, 0x00, 0x65, 0x4d, 0x42, 0x78, 0xf9, 0xca,
                0x6f, 0x94, 0x44, 0x0d, 0x3b, 0x51,
            ],
            [
                0x2a, 0xb4, 0x6b, 0x78, 0x60, 0x8b, 0xbd, 0x8c, 0x6f, 0x99, 0x3c, 0x71, 0x3a,
                0x53, 0x39, 0x29, 0xdc, 0x51, 0x75, 0xee, 0x3d, 0x91, 0x2a, 0xb8, 0xd8, 0x59,
                0xa8, 0x61, 0x88, 0xc3, 0x94, 0x4a,
            ],
            [
                0xae, 0x03, 0xd3, 0xa5, 0x3e, 0xbd, 0x98, 0x41, 0x87, 0xdf, 0xab, 0x39, 0x5c,
                0x8d, 0x01, 0x7d, 0xdc, 0x6f, 0x34, 0x6c, 0xbf, 0x5a, 0x6f, 0x7a, 0xcb, 0xc4,
                0x00, 0x87, 0x08, 0x88, 0x90, 0x05,
            ],
        ],
        pubkey_y: [
            [
                0x66, 0xe0, 0x9c, 0x9b, 0x4a, 0xa9, 0xb6, 0xdb, 0xd9, 0xc3, 0x20, 0xc9, 0x2c,
                0xcd, 0x03, 0xa5, 0x03, 0xb1, 0x25, 0x7a, 0xd4, 0xdc, 0x76, 0xbd, 0x4f, 0x22,
                0x24, 0x5a, 0x22, 0x85, 0xa8, 0x1e,
            ],
            [
                0x56, 0x5c, 0x50, 0x2a, 0x6e, 0xa7, 0xbb, 0x16, 0x1e, 0xef, 0x2e, 0xb5, 0xae,
                0x50, 0x65, 0xd1, 0xc5, 0xcb, 0xff, 0x99, 0xcc, 0xee, 0x7f, 0xec, 0x20, 0x8e,
                0x16, 0xf7, 0x73, 0x9e, 0xa9, 0x42,
            ],
            [
                0x63, 0x54, 0x75, 0xe5, 0x96, 0x89, 0xd6, 0x5a, 0xfb, 0x2d, 0x29, 0xec, 0xce,
                0x1e, 0xb3, 0x9b, 0x67, 0x8e, 0xbb, 0x19, 0x02, 0x2f, 0x38, 0x2c, 0x5d, 0x1f,
                0x1a, 0xf6, 0xff, 0x05, 0xba, 0x35,
            ],
            [
                0xee, 0xa5, 0x07, 0x86, 0x6b, 0xec, 0x23, 0xc9, 0x32, 0x48, 0x4f, 0x22, 0x13,
                0x13, 0x2b, 0x9c, 0xc4, 0xe0, 0x33, 0x71, 0x49, 0xf5, 0x2b, 0xed, 0xf1, 0xb9,
                0xbe, 0xd0, 0xf1, 0xb4, 0x52, 0x79,
            ],
            [
                0x75, 0x82, 0x9d, 0xbb, 0xa9, 0x18, 0x9b, 0xa5, 0xdd, 0xe7, 0x71, 0x35, 0xa5,
                0x42, 0xfb, 0x81, 0xf0, 0xff, 0xac, 0x02, 0xaa, 0x10, 0x58, 0x72, 0xa3, 0x48,
                0x51, 0x28, 0x90, 0xc4, 0xd5, 0x06,
            ],
            [
                0xe1, 0xd5, 0x98, 0xc5, 0xee, 0x14, 0x81, 0xb9, 0x6c, 0x67, 0x89, 0x10, 0xce,
                0x2a, 0xec, 0x44, 0x34, 0x7b, 0xa8, 0x17, 0x89, 0xba, 0x72, 0x62, 0x74, 0x18,
                0x01, 0x8e, 0x1c, 0xdb, 0x71, 0x10,
            ],
            [
                0xa1, 0xd1, 0xbd, 0xf1, 0x9e, 0x24, 0xb4, 0x65, 0x41, 0x0a, 0x68, 0x34, 0xd8,
                0x1e, 0x2e, 0xda, 0x77, 0xe0, 0x13, 0x48, 0x0b, 0x6e, 0x56, 0x2a, 0xd5, 0xf0,
                0xaf, 0xa0, 0x92, 0x8c, 0xec, 0x67,
            ],
        ],
        power: [10, 10, 10, 10, 10, 10, 10],
    };
    let validator_set_hash_bytes: [u8; 32] = [
        0x9d, 0xb1, 0x56, 0xd3, 0x03, 0x3f, 0x51, 0x79, 0x0c, 0xb2, 0xef, 0xeb, 0xf1, 0xe7, 0x28,
        0xd0, 0xe7, 0x7a, 0x10, 0x39, 0xf0, 0x94, 0x72, 0x18, 0xed, 0x0c, 0x12, 0xee, 0xc9, 0x8f,
        0xd4, 0xca,
    ];

    let mut block_hash_bytes: [u8; 32] = [0; 32];
    for i in 0..32 {
        block_hash_bytes[i] = (i + 1) as u8;
    }
    let mut ps_hash: [u8; 32] = [0; 32];
    for i in 0..32 {
        ps_hash[i] = (200 + i) as u8;
    }
    let mut one_timestamp_content: BoundedVec<u8, 16> = BoundedVec::new();
    one_timestamp_content.extend_from_array([0x08, 0xc0, 0xd9, 0xd5, 0xd4, 0x06]);
    // All 5 real signatures in this fixture were captured signing the SAME
    // fixed message (test_boundary_fixture's own doc: "one fixed, valid
    // boundary fixture"), so replicating that one timestamp 7x here is
    // correct for this fixture specifically -- unlike real production data,
    // where each validator's timestamp genuinely differs (see
    // BoundaryBlockWitness.timestamp_content's doc).
    let timestamp_content: [BoundedVec<u8, 16>; 7] =
        [one_timestamp_content, one_timestamp_content, one_timestamp_content, one_timestamp_content, one_timestamp_content, one_timestamp_content, one_timestamp_content];

    let block = BoundaryBlockWitness {
        ps_total: 1,
        ps_hash,
        timestamp_content,
        signed: [true, true, true, true, true, false, false], // 5/7 = >2/3
        sig_r_x: [
            [
                0x6c, 0x2e, 0x1f, 0x89, 0x72, 0x73, 0xf8, 0x94, 0x1d, 0xbb, 0x26, 0x58, 0x9a,
                0x3b, 0x8e, 0x38, 0xe9, 0x37, 0x3c, 0x6d, 0xe4, 0xf9, 0xe0, 0x83, 0x19, 0x39,
                0xb6, 0xee, 0x28, 0x33, 0xb1, 0x1d,
            ],
            [
                0xee, 0xed, 0x98, 0x45, 0x09, 0xde, 0x2a, 0x6d, 0x51, 0xd8, 0x8c, 0x76, 0x71,
                0x56, 0xa8, 0x17, 0x6f, 0x5c, 0xbc, 0x45, 0x3f, 0x79, 0xcb, 0x3d, 0x61, 0x25,
                0x70, 0xf1, 0x83, 0xb0, 0x09, 0x65,
            ],
            [
                0xbe, 0x06, 0x30, 0x32, 0x3d, 0xe1, 0x1f, 0xb9, 0x7e, 0xc9, 0x41, 0x4c, 0x2b,
                0xbe, 0xee, 0xdf, 0x71, 0x80, 0x7f, 0xc9, 0xf2, 0xd4, 0x18, 0xb1, 0x72, 0x9b,
                0x8b, 0xce, 0x1f, 0xce, 0x49, 0x77,
            ],
            [
                0x77, 0x35, 0x08, 0xc8, 0xef, 0x74, 0x20, 0x37, 0x97, 0x6c, 0x38, 0xb4, 0xf4,
                0x64, 0xb6, 0x35, 0x53, 0x62, 0xcb, 0x91, 0x83, 0x6b, 0xa4, 0xd8, 0x10, 0xbd,
                0x5b, 0x20, 0x17, 0xa3, 0xf2, 0x78,
            ],
            [
                0x8e, 0x9c, 0x84, 0xd4, 0x44, 0x97, 0xa3, 0x50, 0x29, 0x3c, 0x8d, 0x44, 0x8d,
                0xd6, 0x80, 0x6b, 0xf2, 0xfd, 0x0b, 0x8d, 0xa3, 0x5a, 0xbd, 0xb8, 0x6c, 0x5e,
                0xca, 0x0b, 0x20, 0x4e, 0x43, 0x60,
            ],
            [
                0xf6, 0x6b, 0x3f, 0x80, 0x5f, 0x70, 0x49, 0x77, 0x08, 0x19, 0x9c, 0xbc, 0x68,
                0x5e, 0x0c, 0x05, 0x8f, 0x2d, 0x7d, 0xac, 0xcf, 0x4e, 0x3d, 0x3e, 0x51, 0x5e,
                0x64, 0x8a, 0x72, 0x9d, 0xf4, 0x4b,
            ],
            [
                0x08, 0x55, 0xa9, 0x58, 0x40, 0x19, 0x89, 0x2b, 0x66, 0x81, 0x35, 0xee, 0x0c,
                0x36, 0x17, 0xff, 0x94, 0x40, 0xa6, 0xc3, 0x88, 0xce, 0xa4, 0xa5, 0x00, 0x88,
                0x81, 0x97, 0x38, 0xa2, 0x3a, 0x2d,
            ],
        ],
        sig_r_y: [
            [
                0xe3, 0x7f, 0x7f, 0xe4, 0x6d, 0x32, 0xc9, 0x8f, 0xed, 0xe2, 0xc5, 0x57, 0xb3,
                0x73, 0x2a, 0x5c, 0x82, 0xbb, 0x6d, 0x9f, 0xa0, 0x81, 0x3f, 0xa8, 0x22, 0xb3,
                0x30, 0xa0, 0x6d, 0x3f, 0x02, 0x63,
            ],
            [
                0xc0, 0x72, 0x30, 0x82, 0xce, 0x46, 0xa9, 0x92, 0xdf, 0xd1, 0x52, 0xe6, 0xe6,
                0x70, 0xeb, 0xf1, 0x66, 0x9f, 0xfb, 0x76, 0xf8, 0x65, 0xa1, 0x9a, 0x7b, 0x61,
                0x4a, 0x5b, 0xa6, 0xce, 0xcb, 0x2f,
            ],
            [
                0xf4, 0x5f, 0x48, 0xfc, 0x62, 0x72, 0xaf, 0x5e, 0x93, 0x7f, 0xbd, 0x19, 0x88,
                0xc2, 0x99, 0xbd, 0x2c, 0x0c, 0x74, 0x48, 0xc4, 0x14, 0x58, 0x6f, 0x99, 0x8f,
                0xe5, 0x9d, 0x75, 0xd1, 0xb7, 0x23,
            ],
            [
                0x49, 0xce, 0xa5, 0xf1, 0xc4, 0x14, 0x7d, 0xf1, 0xf5, 0xc7, 0xb5, 0x86, 0xc5,
                0x26, 0x46, 0x41, 0x15, 0x65, 0xe4, 0xae, 0x4a, 0xfd, 0xff, 0x86, 0x63, 0xa4,
                0x88, 0xc1, 0x9e, 0x3b, 0xef, 0x0c,
            ],
            [
                0x34, 0x58, 0x18, 0xc6, 0xc9, 0x58, 0x05, 0x75, 0x20, 0x23, 0xfa, 0x1a, 0x94,
                0x21, 0x4f, 0x27, 0x60, 0x6e, 0x48, 0x02, 0x37, 0x55, 0x68, 0xb1, 0xe9, 0x6e,
                0x90, 0x47, 0x72, 0x7c, 0x02, 0x5b,
            ],
            [
                0xca, 0xfb, 0x3f, 0xae, 0xdb, 0xa0, 0x81, 0xb6, 0x9d, 0xe9, 0x8f, 0xf7, 0xb5,
                0xb7, 0x7b, 0x17, 0xa9, 0x08, 0x9e, 0x83, 0x75, 0x27, 0x0c, 0xac, 0x16, 0xef,
                0xc8, 0x0c, 0xb6, 0xc6, 0xb2, 0x35,
            ],
            [
                0x01, 0x60, 0x0f, 0x7f, 0x4b, 0xea, 0xac, 0x6f, 0x84, 0x2a, 0xe3, 0xad, 0x66,
                0x75, 0xcc, 0x32, 0xe5, 0x9d, 0xa3, 0x62, 0x08, 0xbe, 0xb5, 0xaa, 0x91, 0x44,
                0x78, 0xa2, 0x7f, 0xdf, 0x1c, 0x0c,
            ],
        ],
        sig_s: [
            [
                0xfd, 0x96, 0x35, 0x51, 0x7e, 0x49, 0x7e, 0x7e, 0x16, 0x38, 0x6c, 0xc2, 0x78,
                0x29, 0x00, 0x2b, 0xc6, 0xdd, 0x43, 0x77, 0x80, 0x80, 0xac, 0x27, 0x46, 0x59,
                0xca, 0xed, 0x2f, 0x91, 0xdb, 0x09,
            ],
            [
                0x6d, 0x3e, 0x90, 0xe2, 0x38, 0x5e, 0xa2, 0x21, 0x71, 0xe2, 0xd7, 0x92, 0x1a,
                0xa2, 0xd1, 0x6b, 0xae, 0x20, 0x1a, 0x74, 0x24, 0x84, 0x8d, 0x03, 0x7d, 0x52,
                0xcc, 0xa1, 0xc4, 0xd9, 0x34, 0x01,
            ],
            [
                0x7e, 0x9c, 0xcd, 0x75, 0xc0, 0x77, 0x60, 0xf8, 0xbd, 0x8d, 0x75, 0xa4, 0xa0,
                0xf7, 0x83, 0xb6, 0xa4, 0x1a, 0xda, 0xa2, 0xff, 0x26, 0x9c, 0x33, 0x40, 0xcd,
                0x39, 0xa1, 0x33, 0x91, 0x18, 0x0a,
            ],
            [
                0x3f, 0xaa, 0x01, 0xc2, 0x33, 0x3b, 0x8b, 0x4d, 0xdd, 0xf0, 0x1b, 0x6d, 0x0a,
                0x6e, 0xbd, 0x21, 0xd1, 0x70, 0xad, 0x45, 0x69, 0x91, 0xac, 0x7c, 0xa6, 0x2a,
                0x96, 0xd5, 0x02, 0x69, 0x6a, 0x02,
            ],
            [
                0x55, 0x04, 0xa1, 0x53, 0xe5, 0x28, 0xf9, 0x06, 0xf3, 0xc2, 0xfa, 0xf7, 0xca,
                0x18, 0x11, 0xc3, 0x26, 0x82, 0x70, 0x1f, 0x13, 0x11, 0xf3, 0xc4, 0x17, 0xd2,
                0xb3, 0xb5, 0x62, 0x17, 0x7c, 0x04,
            ],
            [
                0x35, 0x9d, 0xf4, 0x0a, 0x7a, 0x4c, 0x09, 0x53, 0x3e, 0x61, 0x1d, 0x4a, 0xcd,
                0x95, 0x0a, 0x65, 0xac, 0x3f, 0x62, 0x17, 0xb0, 0xc4, 0xce, 0xcc, 0xcd, 0x6f,
                0x02, 0x5e, 0x07, 0x3c, 0x40, 0x0f,
            ],
            [
                0xf7, 0x5b, 0x2e, 0x78, 0x49, 0xfd, 0x38, 0xbd, 0x1b, 0xa1, 0x27, 0xb3, 0x51,
                0x1e, 0x3d, 0x42, 0x70, 0x10, 0x27, 0x61, 0x2d, 0xad, 0x07, 0x41, 0x8a, 0xfc,
                0x3c, 0xf6, 0xef, 0xc1, 0x2f, 0x0c,
            ],
        ],
    };

    (validators, validator_set_hash_bytes, block, block_hash_bytes)
}

// call_main_with_test_boundary calls the real main() with the given
// header-chain args plus test_boundary_fixture()'s ground-truthed boundary
// data (used identically as both the first and last block by every test
// below -- these tests exercise header-chain semantics, not per-height
// binding, so a fixed real height/hash pair is reused throughout).
fn call_main_with_test_boundary(rt_last: Field, rt_new: Field, count: u32, headers: [Header; N_MAX]) {
    let (validators, validator_set_hash_bytes, block, block_hash_bytes) = test_boundary_fixture();
    main(
        rt_last,
        rt_new,
        count,
        headers,
        12345,
        reduce_to_field(block_hash_bytes),
        reduce_to_field(block_hash_bytes),
        reduce_to_field(validator_set_hash_bytes),
        block_hash_bytes,
        block_hash_bytes,
        validator_set_hash_bytes,
        validators,
        block,
        block,
    );
}

// count_1_satisfies_circuit is the ONE positive test exercising the FULL
// main() (header chain + boundary-signature binding) -- everything below it
// tests verify_header_chain() directly, since header-chain semantics don't
// need the ~14 ed25519 verifications (~3.6M gates) boundary binding costs
// per witness-solve (this function's own doc).
#[test]
fn count_1_satisfies_circuit() {
    let (headers, rt_last, rt_new) = build_chain::<1>();
    call_main_with_test_boundary(rt_last, rt_new, 1, headers);
}

#[test]
fn count_4_satisfies_circuit() {
    // The old fixed-N=4 design's exact case -- must still work identically.
    let (headers, rt_last, rt_new) = build_chain::<4>();
    verify_header_chain(rt_last, rt_new, 4, headers);
}

#[test]
fn count_130_satisfies_circuit() {
    let (headers, rt_last, rt_new) = build_chain::<130>();
    verify_header_chain(rt_last, rt_new, 130, headers);
}

#[test]
fn count_n_max_satisfies_circuit() {
    let (headers, rt_last, rt_new) = build_chain::<N_MAX>();
    verify_header_chain(rt_last, rt_new, N_MAX, headers);
}

#[test(should_fail)]
fn fabricated_single_header_cannot_forge_arbitrary_rt_new() {
    // Regression for the rt_new-binding fix: with the OLD
    // `headers[count-1].state_root == rt_new` constraint, a prover could
    // satisfy this with count=1 by simply setting state_root := target
    // rt_new directly, without that value ever being the real hash_header
    // output of a header chained from rt_last. Confirms the new
    // `hash_header(headers[count-1]) == rt_new` binding rejects exactly
    // that forgery.
    let rt_last = 111;
    let mut headers = [Header { prev_hash: 0, fsm_state: 0, withdrawal_locked: 0, state_root: 0 }; N_MAX];
    let target_rt_new = 999999; // an arbitrary value NOT derived from hash_header
    headers[0] = Header { prev_hash: rt_last, fsm_state: 2, withdrawal_locked: 1, state_root: target_rt_new };
    verify_header_chain(rt_last, target_rt_new, 1, headers);
}

#[test(should_fail)]
fn count_zero_fails_circuit() {
    let (headers, rt_last, rt_new) = build_chain::<1>();
    verify_header_chain(rt_last, rt_new, 0, headers);
}

#[test(should_fail)]
fn count_above_n_max_fails_circuit() {
    let (headers, rt_last, rt_new) = build_chain::<N_MAX>();
    verify_header_chain(rt_last, rt_new, N_MAX + 1, headers);
}

#[test(should_fail)]
fn wrong_fsm_state_fails_circuit() {
    let (mut headers, rt_last, rt_new) = build_chain::<4>();
    headers[0].fsm_state = 1; // 1 = ANCHORED, not allowed
    verify_header_chain(rt_last, rt_new, 4, headers);
}

#[test(should_fail)]
fn padding_cannot_extend_real_coverage() {
    // A proof claiming count=8 but whose headers[4..8) are NOT a real
    // continuation of the count=4 chain (still zero padding) must be rejected
    // -- confirms the gate enforces continuity up to the CLAIMED count, not
    // just up to whatever is non-zero.
    let (headers, rt_last, _) = build_chain::<4>();
    verify_header_chain(rt_last, 1000, 8, headers);
}

#[test(should_fail)]
fn insufficient_boundary_quorum_fails_circuit() {
    // Only 3/7 signed (< 2/3) must be rejected, even with an otherwise
    // perfectly valid header chain and real signatures.
    let (headers, rt_last, rt_new) = build_chain::<1>();
    let (validators, validator_set_hash_bytes, mut block, block_hash_bytes) =
        test_boundary_fixture();
    block.signed = [true, true, true, false, false, false, false];
    main(
        rt_last,
        rt_new,
        1,
        headers,
        12345,
        reduce_to_field(block_hash_bytes),
        reduce_to_field(block_hash_bytes),
        reduce_to_field(validator_set_hash_bytes),
        block_hash_bytes,
        block_hash_bytes,
        validator_set_hash_bytes,
        validators,
        block,
        block,
    );
}
