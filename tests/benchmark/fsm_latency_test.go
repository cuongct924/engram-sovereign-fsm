// Package benchmark implements E7 (Consensus Overhead of the Extended
// Proposal) as Go benchmarks: `go test ./tests/benchmark/... -bench=. -benchmem`.
// V0-V5 don't map to separate code paths, so size is measured via
// json.Marshal on progressively larger structs and CPU cost via the real
// ProcessProposal sub-steps; P2P health (V4) is an estimate, not shipped code.
package benchmark

import (
	"encoding/json"
	"fmt"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"google.golang.org/protobuf/proto"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const benchmarkChainID = "sovereignty-benchmark"

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
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx).WithChainID(benchmarkChainID)
	return k, sdkCtx
}

// signedCertificates registers n equal-power validators and signs the same
// metrics for each -- AggregateHealthCertificates verifies a real ed25519
// signature per certificate, so the fixture needs real key material.
func signedCertificates(tb testing.TB, k *keeper.Keeper, ctx sdk.Context, height int64, metrics *types.PeripheralMetrics, n int) []*types.SignedHealthCertificate {
	tb.Helper()
	ext, err := proto.Marshal(metrics)
	if err != nil {
		tb.Fatal(err)
	}
	signBytes := cmttypes.VoteExtensionSignBytes(benchmarkChainID, &cmtproto.Vote{Height: height, Round: 0, Extension: ext})

	certs := make([]*types.SignedHealthCertificate, n)
	for i := 0; i < n; i++ {
		priv := ed25519.GenPrivKey()
		pub := priv.PubKey().(ed25519.PubKey)
		addr := pub.Address()
		if err := k.ValidatorPubKeys.Set(ctx, string(addr), types.ValidatorInfo{PubKey: pub.Bytes(), Power: 1}); err != nil {
			tb.Fatal(err)
		}
		sig, err := priv.Sign(signBytes)
		if err != nil {
			tb.Fatal(err)
		}
		certs[i] = &types.SignedHealthCertificate{ValidatorAddress: addr, Round: 0, MetricsExtension: ext, ExtensionSignature: sig}
	}
	return certs
}

func healthyAggregatedCertificate(tb testing.TB, k *keeper.Keeper, ctx sdk.Context) *types.AggregatedHealthCertificate {
	tb.Helper()
	return healthyAggregatedCertificateN(tb, k, ctx, 4)
}

// healthyAggregatedCertificateN is the same certificate carrying n signed
// extensions: the validators' count sets the number of ed25519 verifications
// ProcessProposal does.
func healthyAggregatedCertificateN(tb testing.TB, k *keeper.Keeper, ctx sdk.Context, n int) *types.AggregatedHealthCertificate {
	tb.Helper()
	p := k.Params
	metrics := &types.PeripheralMetrics{
		SubnetDiversity: p.MinSubnetDiversity,
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		AvgPeerTenure:   p.MinAvgTenure,
	}
	certs := signedCertificates(tb, k, ctx, -1, metrics, n)

	return &types.AggregatedHealthCertificate{
		Height:                       -1,
		Certificates:                 certs,
		FsmState:                     types.StateAnchored,
		Healthy:                      true,
		DaAttestation:                true,
		BtcCheckpointBlockHeight:     0,
		BtcCheckpointBlockHashTag:    anchor.ExpectedBlockHash(0).Tag,
		BtcCheckpointBlockHashHeight: anchor.ExpectedBlockHash(0).Height,
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
		{"V5_PlusZKProofRef", struct {
			FSMState   string         `json:"fsm_state"`
			DAReceipt  da.Receipt     `json:"da_receipt"`
			BTCReceipt anchor.Receipt `json:"btc_receipt"`
			ZKProofRef []byte         `json:"zk_proof_ref"`
		}{types.StateAnchored, daReceipt, btcReceipt, nil}},
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
// the V5 CPU cost. Every iteration must be ACCEPTED: a handler that rejects
// early is far cheaper and would make the number meaningless. The certificate
// carries n signed extensions (4 and 7 validators), and the handler verifies
// each ed25519 signature; ZK proof verification is a separate path, measured by
// BenchmarkVerifyZKProof.
func BenchmarkProcessProposal(b *testing.B) {
	for _, n := range []int{4, 7} {
		b.Run(fmt.Sprintf("validators=%d", n), func(b *testing.B) {
			k, ctx := newTestKeeperCtx(b)
			cert := healthyAggregatedCertificateN(b, k, ctx, n)
			tx, err := sovereignty.EncodeAggregatedHealthCertificate(cert)
			if err != nil {
				b.Fatal(err)
			}
			handler := sovereignty.NewProcessProposalHandler(k)
			req := &abci.RequestProcessProposal{Txs: [][]byte{tx}, Height: 0}

			resp, err := handler(ctx, req)
			if err != nil || !resp.IsAccepted() {
				b.Fatalf("benchmark proposal is not accepted (err=%v): the measurement would be of a rejection path", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if resp, err := handler(ctx, req); err != nil || !resp.IsAccepted() {
					b.Fatal("proposal rejected")
				}
			}
		})
	}
}

// Isolates the signature work inside ProcessProposal: AggregateHealthCertificates
// verifies every certificate's ed25519 signature and takes the weighted median.
func BenchmarkAggregateHealthCertificates(b *testing.B) {
	for _, n := range []int{4, 7} {
		b.Run(fmt.Sprintf("validators=%d", n), func(b *testing.B) {
			k, ctx := newTestKeeperCtx(b)
			cert := healthyAggregatedCertificateN(b, k, ctx, n)
			if _, ok := sovereignty.AggregateHealthCertificates(ctx, k, benchmarkChainID, cert.Height, cert.Certificates); !ok {
				b.Fatal("no quorum in the benchmark fixture")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = sovereignty.AggregateHealthCertificates(ctx, k, benchmarkChainID, cert.Height, cert.Certificates)
			}
		})
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
