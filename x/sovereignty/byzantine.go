package sovereignty

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Collusion hooks for docs/EXPERIMENT.md's E8 f=2 suite (B1-B9). They extend
// ENGRAM_BYZANTINE_BEHAVIOR beyond proposal.go's single-proposer values:
// several nodes can each carry a behavior, and one node can combine several
// separated by ';'. None of them can forge a signature or a receipt hash that
// honest validators recompute -- they only lie about what Byzantine nodes may
// legitimately choose: their own observation, which certificates to include,
// and which claim to propose.
const (
	// byzantineFalseObservationPrefix overrides fields of the metrics this
	// validator signs in ExtendVote (the vote-extension hook, not the proposer).
	byzantineFalseObservationPrefix = "false_observation:"
	// byzantineBiasCertificate keeps exactly a quorum of certificates but
	// picks the most extreme ones (":best" = healthiest, default ":worst").
	byzantineBiasCertificate = "bias_certificate"
	// byzantineOmitCertsPrefix keeps only the first k certificates.
	byzantineOmitCertsPrefix = "omit_certs:"
	// byzantineStaleReceipt replays an authentic receipt from staleReceiptLag
	// heights ago.
	byzantineStaleReceipt = "stale_receipt"
	// byzantineIncludeWithdrawal proposes a withdrawal tx even while the
	// proposal's own claimed state locks withdrawals (B5).
	byzantineIncludeWithdrawal = "include_withdrawal"

	staleReceiptLag = 50
	// byzantineWithdrawalTx is containsWithdrawal's marker, made unique per
	// height so it is never deduplicated away.
	byzantineWithdrawalTx = "TX_WITHDRAWAL|byzantine|"
)

// splitByzantineBehaviors splits a ';'-separated ENGRAM_BYZANTINE_BEHAVIOR.
func splitByzantineBehaviors(spec string) []string {
	var out []string
	for _, p := range strings.Split(spec, ";") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func hasByzantineBehavior(parts []string, name string) bool {
	for _, p := range parts {
		if p == name || strings.HasPrefix(p, name+":") {
			return true
		}
	}
	return false
}

// applyFalseObservation overrides the named fields of m (proto field names,
// e.g. {"btc_gap":6,"is_das_failed":true}) and returns the lying copy.
func applyFalseObservation(m *types.PeripheralMetrics, overrides string) (*types.PeripheralMetrics, error) {
	var patch map[string]json.RawMessage
	if err := json.Unmarshal([]byte(overrides), &patch); err != nil {
		return nil, fmt.Errorf("false_observation json: %w", err)
	}
	base, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(m)
	if err != nil {
		return nil, err
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(base, &merged); err != nil {
		return nil, err
	}
	for k, v := range patch {
		merged[k] = v
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	out := &types.PeripheralMetrics{}
	if err := (protojson.UnmarshalOptions{}).Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("false_observation fields: %w", err)
	}
	return out, nil
}

// applyByzantineObservation is NewExtendVoteHandler's hook; a malformed spec
// leaves the honest reading unchanged (and says so) rather than crashing the node.
func applyByzantineObservation(behavior string, m *types.PeripheralMetrics) *types.PeripheralMetrics {
	for _, p := range splitByzantineBehaviors(behavior) {
		if !strings.HasPrefix(p, byzantineFalseObservationPrefix) {
			continue
		}
		lied, err := applyFalseObservation(m, strings.TrimPrefix(p, byzantineFalseObservationPrefix))
		if err != nil {
			fmt.Printf("engramd: BYZANTINE ExtendVote ignored: %v\n", err)
			return m
		}
		fmt.Printf("engramd: BYZANTINE ExtendVote false_observation applied: %v\n", p)
		return lied
	}
	return m
}

// healthBadness orders certificates from healthy (low) to degraded (high).
// Gaps are capped so a huge lie cannot overflow the sum; only the order matters.
func healthBadness(m *types.PeripheralMetrics) int64 {
	capGap := func(v uint64) int64 {
		if v > 1<<40 {
			return 1 << 40
		}
		return int64(v)
	}
	s := capGap(m.BtcGap) + capGap(m.DaGap) + capGap(m.PeerChurnRate) + capGap(m.PeerLatency)
	for _, failed := range []bool{m.IsAnchorReverificationFailed, m.IsDasFailed, m.IsAttestationFailed} {
		if failed {
			s += 1 << 41
		}
	}
	s -= capGap(m.CleanPeers) + capGap(m.SubnetDiversity) + capGap(m.ActiveAnchors) + capGap(m.AvgPeerTenure)
	return s
}

// biasCertificates keeps just enough certificates for quorum, choosing the
// most extreme ones. Honest quorum members still sit in the kept set whenever
// b < quorum, which is exactly why the weighted median stays honest-bounded.
func biasCertificates(ctx sdk.Context, k *keeper.Keeper, certs []*types.SignedHealthCertificate, healthiest bool) []*types.SignedHealthCertificate {
	totalPower, err := validatorTotalPower(ctx, k)
	if err != nil || totalPower == 0 {
		return certs
	}
	quorum := totalPower*2/3 + 1

	type scored struct {
		cert  *types.SignedHealthCertificate
		score int64
		power int64
	}
	ranked := make([]scored, 0, len(certs))
	for _, c := range certs {
		vi, err := k.ValidatorPubKeys.Get(ctx, string(c.ValidatorAddress))
		if err != nil {
			continue
		}
		var m types.PeripheralMetrics
		if err := proto.Unmarshal(c.MetricsExtension, &m); err != nil {
			continue
		}
		score := healthBadness(&m)
		if healthiest {
			score = -score
		}
		ranked = append(ranked, scored{c, score, vi.Power})
	}
	sort.SliceStable(ranked, func(a, b int) bool {
		if ranked[a].score != ranked[b].score {
			return ranked[a].score > ranked[b].score
		}
		return string(ranked[a].cert.ValidatorAddress) < string(ranked[b].cert.ValidatorAddress)
	})

	var kept []*types.SignedHealthCertificate
	var power int64
	for _, r := range ranked {
		if power >= quorum {
			break
		}
		kept = append(kept, r.cert)
		power += r.power
	}
	if power < quorum {
		return certs
	}
	return kept
}

// applyByzantineCertificates is NewPrepareProposalHandler's hook on the
// certificate set the leader aggregates and embeds (B2/B3).
func applyByzantineCertificates(ctx sdk.Context, k *keeper.Keeper, behavior string, certs []*types.SignedHealthCertificate) []*types.SignedHealthCertificate {
	parts := splitByzantineBehaviors(behavior)
	in := len(certs)
	for _, p := range parts {
		switch {
		case p == byzantineBiasCertificate || strings.HasPrefix(p, byzantineBiasCertificate+":"):
			certs = biasCertificates(ctx, k, certs, strings.HasSuffix(p, ":best"))
		case strings.HasPrefix(p, byzantineOmitCertsPrefix):
			keep, err := strconv.Atoi(strings.TrimPrefix(p, byzantineOmitCertsPrefix))
			if err == nil && keep >= 0 && keep < len(certs) {
				certs = certs[:keep]
			}
		}
	}
	if len(certs) != in {
		fmt.Printf("engramd: BYZANTINE PrepareProposal certificates %d -> %d behavior=%s\n", in, len(certs), behavior)
	}
	return certs
}

// applyByzantineStaleReceipt rolls cert's DA and BTC receipts back to
// authentic-but-old values (B6). Both are recomputed by honest validators'
// freshness / monotonicity checks, so the proposal must be rejected.
func applyByzantineStaleReceipt(cert *types.AggregatedHealthCertificate) {
	sub := func(h uint64) uint64 {
		if h < staleReceiptLag {
			return 0
		}
		return h - staleReceiptLag
	}
	cert.DaPublishedBlockHeight = sub(cert.DaPublishedBlockHeight)
	cert.BtcCheckpointBlockHeight = sub(cert.BtcCheckpointBlockHeight)
	hash := anchor.ExpectedBlockHash(cert.BtcCheckpointBlockHeight)
	cert.BtcCheckpointBlockHashTag = hash.Tag
	cert.BtcCheckpointBlockHashHeight = hash.Height
}

// byzantineWithdrawalPayload is the withdrawal tx include_withdrawal injects
// when the proposal's claimed state locks withdrawals.
func byzantineWithdrawalPayload(height int64) []byte {
	return []byte(byzantineWithdrawalTx + strconv.FormatInt(height, 10))
}
