package types

import "time"

// EvidenceRecord mirrors one entry of RequestFinalizeBlock.Misbehavior
// (CometBFT's stock evidence pool: DuplicateVote or LightClientAttack).
// Plain JSON-encoded (collections.NewJSONValueCodec), not a new proto
// message. Safe to commit -- unlike a validator's own local sensor
// readings -- because Misbehavior is deterministic, already-agreed block
// data, identical across every honest validator.
type EvidenceRecord struct {
	Type             string    `json:"type"` // "DuplicateVote" | "LightClientAttack" | "Unknown"
	ValidatorAddress []byte    `json:"validator_address"`
	ValidatorPower   int64     `json:"validator_power"`
	OffenseHeight    int64     `json:"offense_height"`
	OffenseTime      time.Time `json:"offense_time"`
	DetectedAtHeight int64     `json:"detected_at_height"` // detection latency = DetectedAtHeight - OffenseHeight
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
