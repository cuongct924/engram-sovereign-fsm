package types

// DefaultGenesis returns the FSM's initial state, matching FSMInit
// (spec/core/EngramFSM.tla:143-165): state=ANCHORED, all counters at 0,
// reanchoring_proof_valid=FALSE, and DefaultParams()'s HysteresisWait.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		FsmState:              StateAnchored,
		SafeBlocksCounter:     0,
		SuspiciousDuration:    0,
		ReanchoringProofValid: false,
		HysteresisWaitLimit:   DefaultParams().HysteresisWait,
		InitialMetrics:        &PeripheralMetrics{},
	}
}
