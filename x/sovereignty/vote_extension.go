package sovereignty

import (
	"sort"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewExtendVoteHandler builds the ONLY ABCI++ hook still allowed to read
// this validator's own live sensors or hit its own bitcoind/Celestia RPC
// (RefreshMetrics -- moved here from ProcessProposal; see
// NewProcessProposalHandler's doc for the AppHash-divergence risk this
// closes). The resulting PeripheralMetrics snapshot becomes this
// validator's signed ABCI++ vote extension for the block CometBFT is about
// to precommit. It is real, protobuf-marshaled bytes (not this package's
// usual JSON keeper codec) because vote extensions are gossiped with every
// precommit and are size-sensitive.
//
// PrepareProposal aggregates >=2f+1 of these at the NEXT height into an
// AggregatedHealthCertificate (AggregateHealthCertificates); from that point
// on, ProcessProposal and CommitFSMTransition read only that certificate,
// never a live sensor.
func NewExtendVoteHandler(k *keeper.Keeper, s *Sensors) sdk.ExtendVoteHandler {
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
		if err := RefreshMetrics(ctx, k, s); err != nil {
			return nil, err
		}
		metrics, err := k.Metrics.Get(ctx)
		if err != nil {
			metrics = &types.PeripheralMetrics{}
		}
		ext, err := proto.Marshal(metrics)
		if err != nil {
			return nil, err
		}
		return &abci.ResponseExtendVote{VoteExtension: ext}, nil
	}
}

// NewVerifyVoteExtensionHandler only checks that a received vote extension
// decodes as a well-formed PeripheralMetrics -- it deliberately does NOT
// re-verify the sensor readings themselves (that would reintroduce the
// exact live-RPC non-determinism ProcessProposal used to have). Signature
// authenticity against the genesis validator set is checked later, once,
// by AggregateHealthCertificates -- doing it twice would be redundant, not
// safer.
func NewVerifyVoteExtensionHandler(k *keeper.Keeper) sdk.VerifyVoteExtensionHandler {
	accept := &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_ACCEPT}
	reject := &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}

	return func(ctx sdk.Context, req *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
		var metrics types.PeripheralMetrics
		if err := proto.Unmarshal(req.VoteExtension, &metrics); err != nil {
			return reject, nil
		}
		return accept, nil
	}
}

// certificatesFromExtendedCommit extracts one SignedHealthCertificate per
// validator that actually precommitted the previous block (BlockIDFlagCommit
// -- ABCI 2.0 requires every other vote to carry an empty extension), from
// the ExtendedCommitInfo only the proposer receives (RequestPrepareProposal's
// LocalLastCommit). Called from PrepareProposal only.
func certificatesFromExtendedCommit(extCommit abci.ExtendedCommitInfo) []*types.SignedHealthCertificate {
	certs := make([]*types.SignedHealthCertificate, 0, len(extCommit.Votes))
	for _, v := range extCommit.Votes {
		if v.BlockIdFlag != cmtproto.BlockIDFlagCommit {
			continue
		}
		certs = append(certs, &types.SignedHealthCertificate{
			ValidatorAddress:   v.Validator.Address,
			Round:              extCommit.Round,
			MetricsExtension:   v.VoteExtension,
			ExtensionSignature: v.ExtensionSignature,
		})
	}
	return certs
}

// validatorTotalPower sums k.ValidatorPubKeys' genesis-fixed power (this app
// has no x/staking rotation -- see types.ValidatorInfo's doc).
func validatorTotalPower(ctx sdk.Context, k *keeper.Keeper) (int64, error) {
	var total int64
	err := k.ValidatorPubKeys.Walk(ctx, nil, func(_ string, vi types.ValidatorInfo) (stop bool, err error) {
		total += vi.Power
		return false, nil
	})
	return total, err
}

// AggregateHealthCertificates is the pure, deterministic function every
// validator (leader in PrepareProposal, follower in ProcessProposal,
// CommitFSMTransition) calls over the SAME certs bytes embedded in Txs[0] --
// unlike the non-determinism this replaces, no live sensor or RPC read
// happens here.
//
// Verifies each certificate's ExtensionSignature against the genesis
// validator registry and CometBFT's own vote-extension canonical sign bytes
// (cmttypes.VoteExtensionSignBytes), then computes a Byzantine-tolerant
// per-field aggregate over every certificate that verifies: a power-weighted
// median for numeric gauges (btc_gap, da_gap, ...) and a power-weighted
// majority for booleans -- the same aggregation technique production ABCI++
// oracle vote extensions use (e.g. Skip Connect's median price
// aggregation), chosen because exact equality across independently-sampled
// sensor readings isn't expected even among honest validators.
//
// Returns ok=false if verified voting power doesn't reach >=2f+1 of the
// total genesis power -- callers must fall back to the previously committed
// state, never invent a value.
func AggregateHealthCertificates(ctx sdk.Context, k *keeper.Keeper, chainID string, height int64, certs []*types.SignedHealthCertificate) (*types.PeripheralMetrics, bool) {
	totalPower, err := validatorTotalPower(ctx, k)
	if err != nil || totalPower == 0 {
		return nil, false
	}
	quorum := totalPower*2/3 + 1

	seen := make(map[string]bool, len(certs))
	metrics := make([]*types.PeripheralMetrics, 0, len(certs))
	powers := make([]int64, 0, len(certs))
	var verifiedPower int64

	for _, cert := range certs {
		addr := string(cert.ValidatorAddress)
		if seen[addr] {
			continue // one certificate per validator; extras never add power
		}
		vi, err := k.ValidatorPubKeys.Get(ctx, addr)
		if err != nil {
			continue // not a genesis validator -- never counted
		}

		signBytes := cmttypes.VoteExtensionSignBytes(chainID, &cmtproto.Vote{
			Height:    height,
			Round:     cert.Round,
			Extension: cert.MetricsExtension,
		})
		if !ed25519.PubKey(vi.PubKey).VerifySignature(signBytes, cert.ExtensionSignature) {
			continue // forged/mismatched signature -- excluded, not fatal
		}

		var m types.PeripheralMetrics
		if err := proto.Unmarshal(cert.MetricsExtension, &m); err != nil {
			continue
		}

		seen[addr] = true
		metrics = append(metrics, &m)
		powers = append(powers, vi.Power)
		verifiedPower += vi.Power
	}

	if verifiedPower < quorum {
		return nil, false
	}

	return &types.PeripheralMetrics{
		BtcGap:                weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.BtcGap }), powers),
		IsBtcSpvFailed:        weightedMajorityBool(mapBool(metrics, func(m *types.PeripheralMetrics) bool { return m.IsBtcSpvFailed }), powers),
		DaGap:                 weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.DaGap }), powers),
		IsDasFailed:           weightedMajorityBool(mapBool(metrics, func(m *types.PeripheralMetrics) bool { return m.IsDasFailed }), powers),
		IsAttestationFailed:   weightedMajorityBool(mapBool(metrics, func(m *types.PeripheralMetrics) bool { return m.IsAttestationFailed }), powers),
		SubnetDiversity:       weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.SubnetDiversity }), powers),
		ActiveAnchors:         weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.ActiveAnchors }), powers),
		CleanPeers:            weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.CleanPeers }), powers),
		PeerChurnRate:         weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.PeerChurnRate }), powers),
		AvgPeerTenure:         weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.AvgPeerTenure }), powers),
		PeerLatency:           weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.PeerLatency }), powers),
		ConfirmedAnchorHeight: weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.ConfirmedAnchorHeight }), powers),
		DaVerifiedHeight:      weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.DaVerifiedHeight }), powers),
		BtcCurrentHeight:      weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.BtcCurrentHeight }), powers),
	}, true
}

func mapUint64(metrics []*types.PeripheralMetrics, f func(*types.PeripheralMetrics) uint64) []uint64 {
	out := make([]uint64, len(metrics))
	for i, m := range metrics {
		out[i] = f(m)
	}
	return out
}

func mapBool(metrics []*types.PeripheralMetrics, f func(*types.PeripheralMetrics) bool) []bool {
	out := make([]bool, len(metrics))
	for i, m := range metrics {
		out[i] = f(m)
	}
	return out
}

// weightedMedianUint64 returns the value at the cumulative half-power point
// once vals are sorted -- deterministic for any input order since ties in
// value are broken by stable sort and cumulative power is monotone.
func weightedMedianUint64(vals []uint64, powers []int64) uint64 {
	if len(vals) == 0 {
		return 0
	}
	idx := make([]int, len(vals))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return vals[idx[a]] < vals[idx[b]] })

	var total int64
	for _, p := range powers {
		total += p
	}
	half := total / 2

	var cum int64
	for _, i := range idx {
		cum += powers[i]
		if cum > half {
			return vals[i]
		}
	}
	return vals[idx[len(idx)-1]]
}

// weightedMajorityBool returns true iff the power backing `true` exceeds
// half the total power among the included certificates.
func weightedMajorityBool(vals []bool, powers []int64) bool {
	var truePower, total int64
	for i, v := range vals {
		total += powers[i]
		if v {
			truePower += powers[i]
		}
	}
	return truePower*2 > total
}
