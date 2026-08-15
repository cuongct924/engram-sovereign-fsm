// Package benchmark implements E7 (Consensus Overhead of the Extended
// Proposal) as Go benchmarks: `go test ./tests/benchmark/... -bench=. -benchmem`.
//
// V0-V5 don't map to separate code paths -- the real ExtendedProposal always
// carries the full field set. So size is measured by json.Marshal-ing
// progressively larger structs (BenchmarkProposalSize), and CPU cost by the
// real sub-steps ProcessProposal calls (CalculateNextState, da.VerifyReceipt,
// anchor.VerifyReceipt) plus one end-to-end benchmark for V5.
// scripts/e7_consensus_overhead composes V1-V4 from these.
//
// P2P health (V4) isn't on the real wire today (validated from the leader's
// keeper.Metrics, see currentFSMInput); P2PDigestSizeEstimate only estimates
// what a digest would add -- an estimate, not a measurement of shipped code.
package benchmark

import (
	"encoding/json"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// P2PDigestSizeEstimate mirrors IsP2PQualityHealthy's 6 fields
// (spec/core/EngramFSM.tla:76-81) -- an estimate, not shipped code.
type P2PDigestSizeEstimate struct {
	SubnetDiversity uint64 `json:"subnet_diversity"`
	ActiveAnchors   uint64 `json:"active_anchors"`
	CleanPeers      uint64 `json:"clean_peers"`
	PeerChurnRate   uint64 `json:"peer_churn_rate"`
	AvgPeerTenure   uint64 `json:"avg_peer_tenure"`
	PeerLatency     uint64 `json:"peer_latency"`
}

func newTestKeeperCtx(tb testing.TB) (*keeper.Keeper, sdk.Context) {
	tb.Helper()
	storeService, ctx := colltest.MockStore()
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	k := keeper.NewKeeper(storeService, cdc)
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx)
	return k, sdkCtx
}

func healthyExtendedProposal(k *keeper.Keeper, ctx sdk.Context) sovereignty.ExtendedProposal {
	p := k.Params
	metrics := &types.PeripheralMetrics{
		SubnetDiversity: p.MinSubnetDiversity,
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		AvgPeerTenure:   p.MinAvgTenure,
	}
	_ = k.Metrics.Set(ctx, metrics)

	return sovereignty.ExtendedProposal{
		FSMState:   types.StateAnchored,
		DAReceipt:  da.Receipt{PublishedBlockHeight: 0, Attestation: true},
		BTCReceipt: anchor.Receipt{CheckpointBlockHeight: 0, CheckpointBlockHash: anchor.ExpectedBlockHash(0)},
		ZKProofRef: nil,
	}
}

// Real JSON byte size of each V0-V5 cumulative payload (E7's table).
func BenchmarkProposalSize(b *testing.B) {
	daReceipt := da.Receipt{PublishedBlockHeight: 12345, Attestation: true}
	btcReceipt := anchor.Receipt{CheckpointBlockHeight: 6789, CheckpointBlockHash: anchor.ExpectedBlockHash(6789)}
	p2pDigest := P2PDigestSizeEstimate{SubnetDiversity: 4, ActiveAnchors: 3, CleanPeers: 10, PeerChurnRate: 1, AvgPeerTenure: 120, PeerLatency: 50}

	variants := []struct {
		name    string
		payload any
	}{
		{"V0_Vanilla_NoExtension", struct{}{}},
		{"V1_FSMStateOnly", struct {
			FSMState string `json:"fsm_state"`
		}{types.StateAnchored}},
		{"V2_PlusDAReceipt", struct {
			FSMState  string     `json:"fsm_state"`
			DAReceipt da.Receipt `json:"da_receipt"`
		}{types.StateAnchored, daReceipt}},
		{"V3_PlusBTCReceipt", struct {
			FSMState   string         `json:"fsm_state"`
			DAReceipt  da.Receipt     `json:"da_receipt"`
			BTCReceipt anchor.Receipt `json:"btc_receipt"`
		}{types.StateAnchored, daReceipt, btcReceipt}},
		{"V4_PlusP2PDigest", struct {
			FSMState   string                `json:"fsm_state"`
			DAReceipt  da.Receipt            `json:"da_receipt"`
			BTCReceipt anchor.Receipt        `json:"btc_receipt"`
			P2PDigest  P2PDigestSizeEstimate `json:"p2p_digest"`
		}{types.StateAnchored, daReceipt, btcReceipt, p2pDigest}},
		{"V5_PlusZKProofRef", sovereignty.ExtendedProposal{
			FSMState:   types.StateAnchored,
			DAReceipt:  daReceipt,
			BTCReceipt: btcReceipt,
			ZKProofRef: nil,
		}},
	}

	for _, v := range variants {
		b.Run(v.name, func(b *testing.B) {
			encoded, err := json.Marshal(v.payload)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportMetric(float64(len(encoded)), "bytes/op")
			for i := 0; i < b.N; i++ {
				_, _ = json.Marshal(v.payload)
			}
		})
	}
}

// End-to-end NewProcessProposalHandler cost (IsValidProposal, all checks) --
// the V5 CPU cost.
func BenchmarkProcessProposal(b *testing.B) {
	k, ctx := newTestKeeperCtx(b)
	ext := healthyExtendedProposal(k, ctx)
	tx, err := sovereignty.EncodeExtendedProposal(ext)
	if err != nil {
		b.Fatal(err)
	}
	handler := sovereignty.NewProcessProposalHandler(k, nil)
	req := &abci.RequestProcessProposal{Txs: [][]byte{tx}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := handler(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

// Isolates CalculateNextState's cost -- V1's marginal CPU; every variant pays
// it since fsm_state is the baseline field.
func BenchmarkCalculateNextState(b *testing.B) {
	k, ctx := newTestKeeperCtx(b)
	metrics := &types.PeripheralMetrics{
		SubnetDiversity: k.Params.MinSubnetDiversity,
		ActiveAnchors:   k.Params.MinAnchorPeers,
		CleanPeers:      k.Params.MinPeers,
		AvgPeerTenure:   k.Params.MinAvgTenure,
	}
	in := keeper.FSMInput{Metrics: metrics}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = keeper.CalculateNextState(types.StateAnchored, in, k.Params)
	}
	_ = ctx
}

// Isolates da.VerifyReceipt -- V2's marginal CPU on top of V1.
func BenchmarkDAVerifyReceipt(b *testing.B) {
	receipt := da.Receipt{PublishedBlockHeight: 100, Attestation: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = da.VerifyReceipt(receipt, types.StateAnchored, true, 100, 1, 0)
	}
}

// Isolates anchor.VerifyReceipt -- V3's marginal CPU on top of V2.
func BenchmarkBTCVerifyReceipt(b *testing.B) {
	receipt := anchor.Receipt{CheckpointBlockHeight: 100, CheckpointBlockHash: anchor.ExpectedBlockHash(100)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = anchor.VerifyReceipt(receipt, types.StateAnchored, true, 100, 100, 0, 2)
	}
}
