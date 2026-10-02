package sovereignty

import (
	"fmt"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Replay violation kinds (engramd debug replay-fsm's CSV `kind` column).
const (
	ViolationNoCertificate        = "no_certificate"
	ViolationSubQuorum            = "sub_quorum_certificate"
	ViolationFSMStateMismatch     = "fsm_state_mismatch"
	ViolationHealthyMismatch      = "healthy_mismatch"
	ViolationCriticalMismatch     = "critical_mismatch"
	ViolationNoHonestCertificate  = "no_honest_certificate"
	ViolationOutsideHonestRange   = "aggregate_outside_honest_range"
	ViolationWithdrawalWhileLock  = "withdrawal_while_locked"
	ViolationCommittedStateDiffer = "committed_state_differs"
)

// ReplayViolation is one way a committed block disagrees with the protocol's
// own semantics, found by re-deriving the transition from the block's bytes.
type ReplayViolation struct {
	Height int64
	Kind   string
	Detail string
}

// Replayer re-executes the committed chain's FSM transitions in memory:
// AggregateHealthCertificates + CalculateNextState + CommitFSMTransition over
// each block's Txs[0], starting from genesis. It shares those functions with
// the live handlers, so "AppHash agrees" (every validator ran the same code)
// is separated from "the committed transition is legal" (the code's own
// answer for those inputs). It applies no transactions: a SOVEREIGN recovery
// that relies on a real SubmitRecoveryProof tx cannot be reproduced.
type Replayer struct {
	k       *keeper.Keeper
	ctx     sdk.Context
	chainID string
	// honest holds the addresses (raw bytes as string) of the validators the
	// range check trusts; empty disables the honest-range check.
	honest map[string]bool
}

// NewReplayer seeds a fresh in-memory keeper from the genesis the live chain
// was started with (NewInitChainer's writes that the FSM transition reads).
func NewReplayer(chainID string, gs *types.GenesisState, validators []cmttypes.GenesisValidator, honestAddrs [][]byte) (*Replayer, error) {
	storeService, base := colltest.MockStore()
	k := keeper.NewKeeper(storeService, nil)
	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).
		WithContext(base).
		WithChainID(chainID).
		WithConsensusParams(cmtproto.ConsensusParams{Abci: &cmtproto.ABCIParams{VoteExtensionsEnableHeight: 1}})

	params := gs.Params.ToParams()
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("genesis params: %w", err)
	}
	k.Params = params
	for name, set := range map[string]error{
		"fsm_state":               k.FSMState.Set(ctx, gs.FsmState),
		"safe_blocks":             k.SafeBlocks.Set(ctx, gs.SafeBlocksCounter),
		"suspicious_duration":     k.SuspiciousDuration.Set(ctx, gs.SuspiciousDuration),
		"reanchoring_proof_valid": k.ReanchoringProofValid.Set(ctx, gs.ReanchoringProofValid),
	} {
		if set != nil {
			return nil, fmt.Errorf("seed %s: %w", name, set)
		}
	}
	for _, v := range validators {
		if err := k.ValidatorPubKeys.Set(ctx, string(v.PubKey.Address()), types.ValidatorInfo{PubKey: v.PubKey.Bytes(), Power: v.Power}); err != nil {
			return nil, err
		}
	}
	r := &Replayer{k: k, ctx: ctx, chainID: chainID, honest: map[string]bool{}}
	for _, a := range honestAddrs {
		r.honest[string(a)] = true
	}
	return r, nil
}

// FSMState is the replayed state after the last applied block.
func (r *Replayer) FSMState() string {
	s, err := r.k.FSMState.Get(r.ctx)
	if err != nil {
		return types.StateAnchored
	}
	return s
}

// ApplyBlock replays one committed block (appHash is the block header's
// app_hash, i.e. the state after height-1) and returns every violation found.
func (r *Replayer) ApplyBlock(height int64, appHash []byte, txs [][]byte) ([]ReplayViolation, error) {
	ctx := r.ctx.WithBlockHeader(cmtproto.Header{Height: height, AppHash: appHash})
	fail := func(kind, format string, args ...any) ReplayViolation {
		return ReplayViolation{Height: height, Kind: kind, Detail: fmt.Sprintf(format, args...)}
	}
	if len(txs) == 0 {
		return []ReplayViolation{fail(ViolationNoCertificate, "empty block")}, nil
	}
	ext, ok, err := DecodeAggregatedHealthCertificate(txs[0])
	if err != nil || !ok {
		return []ReplayViolation{fail(ViolationNoCertificate, "Txs[0] is not a health certificate (err=%v)", err)}, nil
	}

	var out []ReplayViolation
	metrics, aggOk := AggregateHealthCertificates(ctx, r.k, r.chainID, ext.Height, ext.Certificates)
	if !aggOk {
		if voteExtensionsAvailableAt(ctx, ext.Height) {
			out = append(out, fail(ViolationSubQuorum, "%d certificates below 2f+1", len(ext.Certificates)))
		}
		metrics = degradedMetrics(r.k.Params)
	}

	currState, in := fsmInputFromMetrics(ctx, r.k, metrics)
	expected := keeper.CalculateNextState(currState, in, r.k.Params)
	if ext.FsmState != expected {
		out = append(out, fail(ViolationFSMStateMismatch, "committed %s, protocol computes %s from %s", ext.FsmState, expected, currState))
	}
	if want := types.IsHealthyCondition(in.Metrics, r.k.Params); ext.Healthy != want {
		out = append(out, fail(ViolationHealthyMismatch, "committed %v, computed %v", ext.Healthy, want))
	}
	if want := types.IsCriticalCondition(in.Metrics, r.k.Params, in.SuspiciousDuration); ext.Critical != want {
		out = append(out, fail(ViolationCriticalMismatch, "committed %v, computed %v", ext.Critical, want))
	}

	if aggOk && len(r.honest) > 0 {
		out = append(out, r.checkHonestRange(ctx, height, ext, metrics)...)
	}

	if types.WithdrawLocked(ext.FsmState) && !r.k.Baseline.SkipsCircuitBreaker() {
		for _, tx := range txs[1:] {
			if containsWithdrawal(tx) {
				out = append(out, fail(ViolationWithdrawalWhileLock, "withdrawal tx committed in a %s block", ext.FsmState))
				break
			}
		}
	}

	// Apply the block's transition as the chain did, whatever was found, so
	// later heights are judged against the committed history.
	if err := CommitFSMTransition(ctx, r.k, ext); err != nil {
		return out, fmt.Errorf("height %d: %w", height, err)
	}
	return out, nil
}

// checkHonestRange is Lemma 1 turned into a check: the committed aggregate
// must lie inside the range of the honest validators' own signed
// observations included in this block's certificate.
func (r *Replayer) checkHonestRange(ctx sdk.Context, height int64, ext *types.AggregatedHealthCertificate, agg *types.PeripheralMetrics) []ReplayViolation {
	var honest []*types.PeripheralMetrics
	for _, c := range ext.Certificates {
		if !r.honest[string(c.ValidatorAddress)] {
			continue
		}
		// A certificate counts only if AggregateHealthCertificates counted it.
		m, _, ok := verifiedCertificate(ctx, r.k, r.chainID, ext.Height, c)
		if !ok {
			continue
		}
		honest = append(honest, m)
	}
	if len(honest) == 0 {
		return []ReplayViolation{{Height: height, Kind: ViolationNoHonestCertificate, Detail: "no honest certificate in the block"}}
	}
	if fields := metricsOutsideRange(agg, honest); len(fields) > 0 {
		return []ReplayViolation{{Height: height, Kind: ViolationOutsideHonestRange, Detail: fmt.Sprintf("fields %v outside the range of %d honest certificates", fields, len(honest))}}
	}
	return nil
}

// metricsOutsideRange names every field of agg that is not inside the
// [min, max] (numbers) or not held by any member (booleans) of honest.
func metricsOutsideRange(agg *types.PeripheralMetrics, honest []*types.PeripheralMetrics) []string {
	type numField struct {
		name string
		get  func(*types.PeripheralMetrics) uint64
	}
	type boolField struct {
		name string
		get  func(*types.PeripheralMetrics) bool
	}
	nums := []numField{
		{"btc_gap", func(m *types.PeripheralMetrics) uint64 { return m.BtcGap }},
		{"da_gap", func(m *types.PeripheralMetrics) uint64 { return m.DaGap }},
		{"subnet_diversity", func(m *types.PeripheralMetrics) uint64 { return m.SubnetDiversity }},
		{"active_anchors", func(m *types.PeripheralMetrics) uint64 { return m.ActiveAnchors }},
		{"clean_peers", func(m *types.PeripheralMetrics) uint64 { return m.CleanPeers }},
		{"peer_churn_rate", func(m *types.PeripheralMetrics) uint64 { return m.PeerChurnRate }},
		{"avg_peer_tenure", func(m *types.PeripheralMetrics) uint64 { return m.AvgPeerTenure }},
		{"peer_latency", func(m *types.PeripheralMetrics) uint64 { return m.PeerLatency }},
		{"confirmed_anchor_height", func(m *types.PeripheralMetrics) uint64 { return m.ConfirmedAnchorHeight }},
		{"da_verified_height", func(m *types.PeripheralMetrics) uint64 { return m.DaVerifiedHeight }},
		{"btc_current_height", func(m *types.PeripheralMetrics) uint64 { return m.BtcCurrentHeight }},
		{"da_contiguous_height", func(m *types.PeripheralMetrics) uint64 { return m.DaContiguousHeight }},
	}
	bools := []boolField{
		{"is_anchor_reverification_failed", func(m *types.PeripheralMetrics) bool { return m.IsAnchorReverificationFailed }},
		{"is_das_failed", func(m *types.PeripheralMetrics) bool { return m.IsDasFailed }},
		{"is_attestation_failed", func(m *types.PeripheralMetrics) bool { return m.IsAttestationFailed }},
	}

	var out []string
	for _, f := range nums {
		lo, hi := f.get(honest[0]), f.get(honest[0])
		for _, m := range honest[1:] {
			v := f.get(m)
			lo, hi = min(lo, v), max(hi, v)
		}
		if v := f.get(agg); v < lo || v > hi {
			out = append(out, f.name)
		}
	}
	for _, f := range bools {
		held := false
		for _, m := range honest {
			if f.get(m) == f.get(agg) {
				held = true
				break
			}
		}
		if !held {
			out = append(out, f.name)
		}
	}
	return out
}
