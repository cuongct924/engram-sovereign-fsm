// Package benchmark implements docs/EXPERIMENT.md's E7 (Consensus Overhead of
// the Extended Proposal) as real Go benchmarks -- run via
// `go test ./tests/benchmark/... -bench=. -benchmem -run=^$`.
//
// V0-V5 (docs/EXPERIMENT.md's E7 variant table) don't correspond to separate
// code paths in this repo -- x/sovereignty/proposal.go's ExtendedProposal and
// ProcessProposal always carry/validate the full set of fields at once, since
// that is what a real node does. So:
//   - Proposal size overhead (BenchmarkProposalSize) is measured directly by
//     json.Marshal-ing progressively larger real Go structs (da.Receipt,
//     vigilante.Receipt, the real PeripheralMetrics P2P fields, a bool),
//     which is a faithful reproduction of V0-V5's cumulative wire size.
//   - CPU cost is decomposed into the real validation sub-steps
//     (CalculateNextState, da.VerifyReceipt, vigilante.VerifyReceipt) that
//     ProcessProposal actually calls, plus one end-to-end benchmark of the
//     real NewProcessProposalHandler for the fully-featured (V5) case.
//     scripts/e7_consensus_overhead composes V1-V4's cumulative CPU cost from
//     these sub-benchmarks rather than this file re-implementing partial
//     ProcessProposal variants that don't exist on a real node.
//
// P2P health (V4) is NOT part of the real wire-format ExtendedProposal today
// -- it's validated from the leader's own locally-tracked keeper.Metrics, not
// carried in the proposal (see proposal.go's currentFSMInput). The
// P2PDigestSizeEstimate struct below exists only to estimate what a P2P
// digest's marginal size WOULD be if added to the wire format, using the
// exact same 6 fields spec/core/EngramFSM.tla's IsP2PQualityHealthy reads
// from PeripheralMetrics -- this is a documented estimate, not a measurement
// of code that ships today.
package benchmark

import (
	"encoding/json"
	"testing"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/iden3/go-merkletree-sql/v2/db/memory"

	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/vigilante"
)

// P2PDigestSizeEstimate mirrors the 6 fields IsP2PQualityHealthy reads from
// PeripheralMetrics (spec/core/EngramFSM.tla:76-81) -- see the package doc
// above for why this is an estimate, not a measurement of shipped code.
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
	k := keeper.NewKeeper(storeService, cdc, memory.NewMemoryStorage())
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
		BTCReceipt: vigilante.Receipt{CheckpointBlockHeight: 0, CheckpointBlockHash: vigilante.ExpectedBlockHash(0)},
		ZKProofRef: false,
	}
}

// BenchmarkProposalSize reports the real JSON-encoded byte size of each
// V0-V5 cumulative payload (docs/EXPERIMENT.md's E7 table).
func BenchmarkProposalSize(b *testing.B) {
	daReceipt := da.Receipt{PublishedBlockHeight: 12345, Attestation: true}
	btcReceipt := vigilante.Receipt{CheckpointBlockHeight: 6789, CheckpointBlockHash: vigilante.ExpectedBlockHash(6789)}
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
			FSMState   string            `json:"fsm_state"`
			DAReceipt  da.Receipt        `json:"da_receipt"`
			BTCReceipt vigilante.Receipt `json:"btc_receipt"`
		}{types.StateAnchored, daReceipt, btcReceipt}},
		{"V4_PlusP2PDigest", struct {
			FSMState   string                `json:"fsm_state"`
			DAReceipt  da.Receipt            `json:"da_receipt"`
			BTCReceipt vigilante.Receipt     `json:"btc_receipt"`
			P2PDigest  P2PDigestSizeEstimate `json:"p2p_digest"`
		}{types.StateAnchored, daReceipt, btcReceipt, p2pDigest}},
		{"V5_PlusZKProofRef", sovereignty.ExtendedProposal{
			FSMState:   types.StateAnchored,
			DAReceipt:  daReceipt,
			BTCReceipt: btcReceipt,
			ZKProofRef: false,
		}},
	}

	for _, v := range variants {
		v := v
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

// BenchmarkProcessProposal measures the real, full end-to-end
// NewProcessProposalHandler cost (spec/core/EngramTendermint.tla's
// IsValidProposal, all checks) -- the V5 (fully-featured) CPU cost.
func BenchmarkProcessProposal(b *testing.B) {
	k, ctx := newTestKeeperCtx(b)
	ext := healthyExtendedProposal(k, ctx)
	tx, err := sovereignty.EncodeExtendedProposal(ext)
	if err != nil {
		b.Fatal(err)
	}
	handler := sovereignty.NewProcessProposalHandler(k)
	req := &abci.RequestProcessProposal{Txs: [][]byte{tx}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := handler(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCalculateNextState isolates the FSM-state cross-check cost
// (V1's marginal CPU cost -- every variant pays this, since fsm_state is the
// baseline field).
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

// BenchmarkDAVerifyReceipt isolates da.VerifyReceipt's cost (V2's marginal
// CPU cost on top of V1).
func BenchmarkDAVerifyReceipt(b *testing.B) {
	receipt := da.Receipt{PublishedBlockHeight: 100, Attestation: true}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = da.VerifyReceipt(receipt, types.StateAnchored, true, 100, 1, 0)
	}
}

// BenchmarkBTCVerifyReceipt isolates vigilante.VerifyReceipt's cost (V3's
// marginal CPU cost on top of V2).
func BenchmarkBTCVerifyReceipt(b *testing.B) {
	receipt := vigilante.Receipt{CheckpointBlockHeight: 100, CheckpointBlockHash: vigilante.ExpectedBlockHash(100)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vigilante.VerifyReceipt(receipt, 100, 100, 0)
	}
}
