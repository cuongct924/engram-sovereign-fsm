package types

const (
	StateAnchored   = "ANCHORED"
	StateSuspicious = "SUSPICIOUS"
	StateSovereign  = "SOVEREIGN"
	StateRecovering = "RECOVERING"
)

// WithdrawLocked mirrors WithdrawLocked in spec/core/EngramFSM.tla:
//
//	WithdrawLocked == state \in {"SOVEREIGN", "RECOVERING"}
func WithdrawLocked(state string) bool {
	return state == StateSovereign || state == StateRecovering
}
