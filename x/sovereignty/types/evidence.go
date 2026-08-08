package types

import "time"

// EvidenceRecord mirrors one entry of RequestFinalizeBlock.Misbehavior --
// CometBFT's stock, unmodified evidence pool's report of a detected
// DuplicateVote or LightClientAttack. Deliberately a plain JSON-encoded Go
// struct (collections.NewJSONValueCodec, matching PeripheralMetrics'
// existing pattern in keeper.go) rather than a new protobuf message -- no
// proto-gen round trip needed for this.
//
// Unlike x/sovereignty/keeper/sensors' live P2P/BTC/DA readings (which are
// each validator's own LOCAL view and must never be written into committed
// state, see preblock.go's NewPreBlocker doc for the real AppHash-divergence
// bug that taught this repo that lesson), Misbehavior is part of
// RequestFinalizeBlock itself -- deterministic, identical across every
// honest validator for a given block, exactly like req.Txs or req.Header.
// Recording it here is safe for the same reason CommitFSMTransition safely
// commits ext.BTCReceipt/ext.DAReceipt: it's already-agreed block data, not
// a fresh local read.
type EvidenceRecord struct {
	Type             string    `json:"type"` // "DuplicateVote" | "LightClientAttack" | "Unknown"
	ValidatorAddress []byte    `json:"validator_address"`
	ValidatorPower   int64     `json:"validator_power"`
	OffenseHeight    int64     `json:"offense_height"` // height the misbehavior occurred at
	OffenseTime      time.Time `json:"offense_time"`
	DetectedAtHeight int64     `json:"detected_at_height"` // height this record was committed at (E8's "detection latency" = DetectedAtHeight - OffenseHeight)
}

func MisbehaviorTypeName(t int32) string {
	switch t {
	case 1:
		return "DuplicateVote"
	case 2:
		return "LightClientAttack"
	default:
		return "Unknown"
	}
}
