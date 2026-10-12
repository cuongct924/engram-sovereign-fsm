package sovereignty

import (
	"fmt"
	"sort"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// This file does not port a named spec/core/*.tla operator -- there isn't
// one to port. EngramFSM.tla's UpdateSensors (EngramFSM.tla:343-346) and the
// networkSensorVars it mutates (EngramVars.tla:156-187) model Bitcoin/DA/P2P
// health as a SINGLE global truth every validator's CalculateNextFSMState
// call reads identically; the spec never models a validator's local RPC
// view as distinct from any other's; that gap is exactly what this app's
// original ProcessProposal-reads-live-sensors implementation had gotten
// wrong (see root CLAUDE.md's "Never write live local sensor reads into
// committed/hashed state" and "Sensors observe, quorum certifies, the state
// machine decides"). This file is the concrete BFT mechanism that makes
// networkSensorVars' single-global-truth assumption actually true across
// independently-observing validators, rather than leaving it an unenforced
// hope: ExtendVote signs each validator's own local reading (never entering
// consensus state directly), AggregateHealthCertificates then derives one
// byte-identical PeripheralMetrics from >=2f+1 signed weight, which
// downstream CalculateNextFSMState callers (proposal.go) read exactly as
// EngramFSM.tla's UpdateSensors output. No spec/core/*.tla change is needed
// for this: the state-transition layer's abstraction (a single agreed
// sensor value) was already input-agnostic about how that value is
// produced -- this file is the missing real-world proof-of-mechanism for
// an assumption the spec always made, not a new protocol layer needing its
// own operators/invariants.

// NewExtendVoteHandler is the ONLY ABCI++ hook still allowed to read this
// validator's own live sensors or RPC (see NewProcessProposalHandler's doc
// for the AppHash-divergence risk this avoids). The resulting metrics
// snapshot becomes this validator's signed vote extension; PrepareProposal
// aggregates >=2f+1 of these into a certificate that everyone else reads
// instead of touching a live sensor again. byzantineBehavior (E8 only, "" on
// every real validator) lets a colluding node sign a false observation.
func NewExtendVoteHandler(k *keeper.Keeper, s *Sensors, byzantineBehavior string) sdk.ExtendVoteHandler {
	snap, syncMode, emptyMode := newSensorSnapshotCache(), syncSensorsFromEnv(), emptySensorsFromEnv()
	return func(ctx sdk.Context, req *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
		// ExtendVote runs before this validator can send its precommit, so its
		// duration is on every height's critical path; RQ4 reads this line.
		started := time.Now()
		defer func() {
			fmt.Printf("engramd: extend_vote height=%d ms=%.1f\n", req.Height, float64(time.Since(started).Microseconds())/1000)
		}()
		var metrics *types.PeripheralMetrics
		if emptyMode {
			metrics = &types.PeripheralMetrics{}
		} else if s != nil && !syncMode && hasLiveSources(s) {
			// RPC runs in sensor_snapshot.go's background loops; this path
			// only composes the latest observations.
			var err error
			if metrics, _, err = snapshotMetrics(ctx, k, s, snap); err != nil {
				return nil, err
			}
		} else {
			if err := RefreshMetrics(ctx, k, s); err != nil {
				return nil, err
			}
			var err error
			if metrics, err = k.Metrics.Get(ctx); err != nil {
				metrics = &types.PeripheralMetrics{}
			}
		}
		if byzantineBehavior != "" {
			metrics = applyByzantineObservation(byzantineBehavior, metrics)
		}
		ext, err := proto.Marshal(metrics)
		if err != nil {
			return nil, err
		}
		return &abci.ResponseExtendVote{VoteExtension: ext}, nil
	}
}

func hasLiveSources(s *Sensors) bool {
	return s.BTC.Source() != nil || s.DA.Source() != nil || s.Anchor != nil || s.DAPublisher != nil
}

// NewVerifyVoteExtensionHandler only checks that a vote extension decodes
// as a well-formed PeripheralMetrics -- it deliberately does not re-verify
// the sensor readings (that would reintroduce live-RPC non-determinism).
// Signature authenticity is checked once, later, by AggregateHealthCertificates.
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
// validator that actually precommitted the previous block (every other
// vote carries an empty extension), from the ExtendedCommitInfo only the
// proposer receives.
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

// certificatesBoundToCommit reports whether every certificate was signed in
// the round that committed the previous height, by a validator whose
// precommit is in that commit. Height binding alone lets a proposer mix in a
// validator's observation from an earlier round of the same height; the
// commit round and signer set are fixed by the block's LastCommit, which
// every validator verifies. Implementation-level strengthening: the spec
// abstracts the quorum subset (EngramServer.tla's `members`) as drawn from it.
func certificatesBoundToCommit(certs []*types.SignedHealthCertificate, last abci.CommitInfo) bool {
	committed := make(map[string]bool, len(last.Votes))
	for _, v := range last.Votes {
		if v.BlockIdFlag == cmtproto.BlockIDFlagCommit {
			committed[string(v.Validator.Address)] = true
		}
	}
	for _, c := range certs {
		if c.Round != last.Round || !committed[string(c.ValidatorAddress)] {
			return false
		}
	}
	return true
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
// validator calls over the SAME certs bytes embedded in Txs[0] -- no live
// sensor or RPC read here. Verifies each signature against the genesis
// validator registry, then takes a power-weighted median (gauges) or
// majority (booleans) over every cert that verifies -- exact equality
// across independently-sampled sensors isn't expected even among honest
// validators. Returns ok=false below >=2f+1 verified power; callers must
// fall back to previously committed state, never invent a value. Its output
// is what EngramFSM.tla:343-346's UpdateSensors is the spec-level
// abstraction of -- see this file's top-of-file comment.
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
		m, power, ok := verifiedCertificate(ctx, k, chainID, height, cert)
		if !ok {
			continue // not a genesis validator, forged signature, or undecodable -- excluded, not fatal
		}

		seen[addr] = true
		metrics = append(metrics, m)
		powers = append(powers, power)
		verifiedPower += power
	}

	if verifiedPower < quorum {
		return nil, false
	}

	return &types.PeripheralMetrics{
		BtcGap:                       weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.BtcGap }), powers),
		IsAnchorReverificationFailed: weightedMajorityBool(mapBool(metrics, func(m *types.PeripheralMetrics) bool { return m.IsAnchorReverificationFailed }), powers),
		DaGap:                        weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.DaGap }), powers),
		IsDasFailed:                  weightedMajorityBool(mapBool(metrics, func(m *types.PeripheralMetrics) bool { return m.IsDasFailed }), powers),
		IsAttestationFailed:          weightedMajorityBool(mapBool(metrics, func(m *types.PeripheralMetrics) bool { return m.IsAttestationFailed }), powers),
		SubnetDiversity:              weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.SubnetDiversity }), powers),
		ActiveAnchors:                weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.ActiveAnchors }), powers),
		CleanPeers:                   weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.CleanPeers }), powers),
		PeerChurnRate:                weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.PeerChurnRate }), powers),
		AvgPeerTenure:                weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.AvgPeerTenure }), powers),
		PeerLatency:                  weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.PeerLatency }), powers),
		ConfirmedAnchorHeight:        weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.ConfirmedAnchorHeight }), powers),
		DaVerifiedHeight:             weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.DaVerifiedHeight }), powers),
		BtcCurrentHeight:             weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.BtcCurrentHeight }), powers),
		DaContiguousHeight:           weightedMedianUint64(mapUint64(metrics, func(m *types.PeripheralMetrics) uint64 { return m.DaContiguousHeight }), powers),
	}, true
}

// verifiedCertificate checks cert's signer is a genesis validator and its
// extension signature covers (chainID, height, round, extension), returning
// the decoded metrics and the signer's voting power.
func verifiedCertificate(ctx sdk.Context, k *keeper.Keeper, chainID string, height int64, cert *types.SignedHealthCertificate) (*types.PeripheralMetrics, int64, bool) {
	vi, err := k.ValidatorPubKeys.Get(ctx, string(cert.ValidatorAddress))
	if err != nil {
		return nil, 0, false
	}
	signBytes := cmttypes.VoteExtensionSignBytes(chainID, &cmtproto.Vote{
		Height:    height,
		Round:     cert.Round,
		Extension: cert.MetricsExtension,
	})
	if !ed25519.PubKey(vi.PubKey).VerifySignature(signBytes, cert.ExtensionSignature) {
		return nil, 0, false
	}
	m := &types.PeripheralMetrics{}
	if err := proto.Unmarshal(cert.MetricsExtension, m); err != nil {
		return nil, 0, false
	}
	return m, vi.Power, true
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
