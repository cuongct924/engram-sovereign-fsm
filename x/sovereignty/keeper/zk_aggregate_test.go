package keeper

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"testing"

	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
)

// TestVerifyZKProofWasm_AggregatorProof checks the embedded verifier accepts a real
// circuit/reanchoring_aggregator proof (two chained 256-header chunks, count=512, K=4) against the
// aggregator's own VK, rejects it under the leaf VK, and rejects a tampered copy. Regenerate
// zk_assets/testdata/aggregate and zk_assets/vk_aggregate whenever either circuit changes
// (scripts/e6_zk_reanchoring_benchmark/benchmark_aggregate.sh 512 leaves them in circuit/*/target).
func TestVerifyZKProofWasm_AggregatorProof(t *testing.T) {
	read := func(name string) []byte {
		b, err := os.ReadFile("zk_assets/testdata/aggregate/" + name)
		require.NoError(t, err)
		return b
	}
	vk, proof, inputs := read("vk"), read("proof"), read("public_inputs")

	ok, err := verifyZKProofWasm(vk, proof, inputs)
	require.NoError(t, err)
	require.True(t, ok)

	ok, _ = verifyZKProofWasm(embeddedVK, proof, inputs)
	require.False(t, ok, "aggregator proof must not verify under the leaf VK")

	bad := append([]byte(nil), proof...)
	bad[len(bad)/2] ^= 0x01
	ok, _ = verifyZKProofWasm(vk, bad, inputs)
	require.False(t, ok)
}

func recoveryInputsWithCount(count uint32) []byte {
	in := make([]byte, recoveryPublicInputsLen)
	binary.BigEndian.PutUint32(in[recoveryCountOffset:recoveryCountOffset+4], count)
	return in
}

// A single leaf proof covers at most N_MAX headers; only a longer claimed interval may use the
// aggregator VK, and malformed inputs must never reach it.
func TestRecoveryVK_SelectsByClaimedCount(t *testing.T) {
	require.Equal(t, embeddedVK, recoveryVK(recoveryInputsWithCount(1)))
	require.Equal(t, embeddedVK, recoveryVK(recoveryInputsWithCount(recoveryLeafNMax)))
	require.Equal(t, embeddedAggregateVK, recoveryVK(recoveryInputsWithCount(recoveryLeafNMax+1)))
	require.Equal(t, embeddedAggregateVK, recoveryVK(recoveryInputsWithCount(1024)))
	require.Equal(t, embeddedVK, recoveryVK(nil))
	require.Equal(t, embeddedVK, recoveryVK(make([]byte, recoveryPublicInputsLen-1)))
	require.NotEqual(t, embeddedVK, embeddedAggregateVK)
}

// VerifyZKProof must route a count>N_MAX proof to the aggregator VK on its own, accept the real
// aggregate, and fail closed when the proof or the claimed count is tampered with.
func TestVerifyZKProof_AggregateRoutedByCount(t *testing.T) {
	read := func(name string) []byte {
		b, err := os.ReadFile("zk_assets/testdata/aggregate/" + name)
		require.NoError(t, err)
		return b
	}
	proof, inputs := read("proof"), read("public_inputs")
	require.Equal(t, uint32(512), binary.BigEndian.Uint32(inputs[recoveryCountOffset:recoveryCountOffset+4]))

	k := &Keeper{}
	require.True(t, k.VerifyZKProof(proof, inputs), "real aggregate must verify via the count-selected VK")

	bad := append([]byte(nil), proof...)
	bad[len(bad)/2] ^= 0x01
	require.False(t, k.VerifyZKProof(bad, inputs))

	// Claiming a count within N_MAX sends the same proof to the leaf VK, where it cannot verify.
	shrunk := append([]byte(nil), inputs...)
	binary.BigEndian.PutUint32(shrunk[recoveryCountOffset:recoveryCountOffset+4], 100)
	require.False(t, k.VerifyZKProof(proof, shrunk))
}

// aggregateFixtureState seeds a keeper with the chain state the testdata aggregate proof was built
// against: scripts/e6_zk_reanchoring_benchmark/generate_chain.go's chunk mode (512 headers, heights
// 1..512, height-derived block hashes, 7 deterministic validators, power 10 each).
func aggregateFixtureState(t *testing.T, inputs []byte, lastAnchoredHeight uint64) (*MsgServerImpl, context.Context) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	srv := &MsgServerImpl{Keeper: NewKeeper(storeService, nil)}

	// Mirrors generate_chain.go's heightBlockHash.
	blockHash := func(height uint64) []byte {
		h := make([]byte, 32)
		for i := range h {
			h[i] = byte(height>>(8*(i%8))) ^ byte(i*7+1)
		}
		return h
	}
	privs := make([]cmted25519.PrivKey, 7)
	for i := range privs {
		privs[i] = cmted25519.GenPrivKeyFromSecret([]byte(fmt.Sprintf("engram-chunk-fixture-validator-%d", i)))
	}
	sort.Slice(privs, func(i, j int) bool {
		return bytes.Compare(privs[i].PubKey().Address(), privs[j].PubKey().Address()) < 0
	})
	vals := make([]*cmttypes.Validator, len(privs))
	for i := range privs {
		vals[i] = cmttypes.NewValidator(privs[i].PubKey(), 10)
	}

	require.NoError(t, srv.LastAnchoredRoot.Set(ctx, inputs[0:32]))
	require.NoError(t, srv.LastAnchoredHeight.Set(ctx, lastAnchoredHeight))
	require.NoError(t, srv.HeaderHistory.Set(ctx, 512, types.RecoveryHeader{FsmState: types.StateRecovering, Accumulator: inputs[32:64]}))
	require.NoError(t, srv.BlockHashByHeight.Set(ctx, 1, blockHash(1)))
	require.NoError(t, srv.BlockHashByHeight.Set(ctx, 512, blockHash(512)))
	require.NoError(t, srv.ValidatorSetHash.Set(ctx, cmttypes.NewValidatorSet(vals).Hash()))
	return srv, ctx
}

// A real 512-header aggregate must pass every SubmitRecoveryProof binding (rt_last, rt_new found in
// history, count == interval length, boundary block hashes, validator set) and advance the
// checkpoint to its last header; shifting the tracked interval makes the same proof stale.
func TestSubmitRecoveryProof_AcceptsRealAggregate(t *testing.T) {
	read := func(name string) []byte {
		b, err := os.ReadFile("zk_assets/testdata/aggregate/" + name)
		require.NoError(t, err)
		return b
	}
	proof, inputs := read("proof"), read("public_inputs")

	srv, ctx := aggregateFixtureState(t, inputs, 0)
	_, err := srv.SubmitRecoveryProof(ctx, &types.MsgSubmitRecoveryProofRequest{ZkProof: proof, PublicInputs: inputs})
	require.NoError(t, err)

	root, err := srv.LastAnchoredRoot.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, inputs[32:64], root, "checkpoint root must advance to the aggregate's rt_new")
	height, err := srv.LastAnchoredHeight.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(512), height)
	latched, err := srv.RealProofSubmittedHeight.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(512), latched)

	// Same proof against an interval that already advanced: count != 512 - lastAnchoredHeight.
	srv, ctx = aggregateFixtureState(t, inputs, 10)
	_, err = srv.SubmitRecoveryProof(ctx, &types.MsgSubmitRecoveryProofRequest{ZkProof: proof, PublicInputs: inputs})
	require.ErrorIs(t, err, types.ErrInvalidZKProof)
}

// Validator-side cost of one recovery proof through the embedded WASM verifier: a 256-header leaf
// proof vs. a 512-header aggregate. A chain of n leaf proofs costs n times the first.
func BenchmarkVerifyRecoveryProofWasm(b *testing.B) {
	read := func(path string) []byte {
		data, err := os.ReadFile(path)
		require.NoError(b, err)
		return data
	}
	k := &Keeper{}
	cases := map[string][2][]byte{
		"leaf":      {read("zk_assets/testdata/proof_valid"), read("zk_assets/testdata/inputs_valid")},
		"aggregate": {read("zk_assets/testdata/aggregate/proof"), read("zk_assets/testdata/aggregate/public_inputs")},
	}
	for _, name := range []string{"leaf", "aggregate"} {
		c := cases[name]
		require.True(b, k.VerifyZKProof(c[0], c[1]), name)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				k.VerifyZKProof(c[0], c[1])
			}
		})
	}
}
