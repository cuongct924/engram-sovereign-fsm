package types

// PeripheralMetrics is generated from
// proto/engram/sovereignty/v1/state.proto (state.pb.go) -- do not redeclare
// it here. Field names follow protoc-gen-go casing (BtcGap, IsDasFailed,
// ...), not the TLA+ spelling.

// PeripheralSnapshot is the coarse three-boolean view used by callers that
// only need pass/fail health per peripheral (not the raw readings).
type PeripheralSnapshot struct {
	BitcoinFinalityGap uint64 // BTC blocks since the last confirmed anchor
	DAReceiptValidated bool   // Celestia DA attestation
	P2PQualityHealthy  bool   // Tri-interface Profiler result
}

type PeripheralSensorEngine interface {
	GetLocalSnapshot() (PeripheralSnapshot, error)
	InjectFaultScenario(scenarioID string) // feeds E2-E9 directly
}
