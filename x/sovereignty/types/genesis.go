package types

// DefaultGenesis returns the FSM's initial state, matching FSMInit
// (spec/core/EngramFSM.tla:143-165): state=ANCHORED, all counters at 0,
// reanchoring_proof_valid=FALSE, and DefaultParams().
func DefaultGenesis() *GenesisState {
	return DefaultGenesisWithParams(DefaultParams())
}

// DefaultGenesisWithParams is DefaultGenesis with p instead of
// DefaultParams() -- for cmd/engramd/main.go's genesis generation to bake
// in an ENGRAM_PARAM_* override (validated by the caller).
func DefaultGenesisWithParams(p Params) *GenesisState {
	return &GenesisState{
		FsmState:              StateAnchored,
		SafeBlocksCounter:     0,
		SuspiciousDuration:    0,
		ReanchoringProofValid: false,
		InitialMetrics:        &PeripheralMetrics{},
		Params:                p.ToGenesisParams(),
	}
}
