package types

import "time"

// EvidenceRecord mirrors one RequestFinalizeBlock.Misbehavior entry
// (CometBFT's DuplicateVote/LightClientAttack). JSON-encoded, not a new
// proto message. Safe to commit: deterministic agreed block data, unlike
// a node's own local sensor readings.
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
