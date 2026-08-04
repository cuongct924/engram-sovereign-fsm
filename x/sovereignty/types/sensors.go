package types

// PeripheralMetrics itself is now generated from
// proto/engram/sovereignty/v1/state.proto (see state.pb.go) -- do not
// redeclare it here. Field names follow protoc-gen-go casing (BtcGap, DaGap,
// IsDasFailed, IsAttestationFailed, ...), not the TLA+ variable spelling.

// PeripheralSnapshot is the coarse three-boolean view used by callers that
// only need pass/fail health per peripheral (not the raw readings).
type PeripheralSnapshot struct {
	BitcoinFinalityGap uint64 // T_delta (số block BTC chưa có anchor)
	DAReceiptValidated bool   // Celestia DA attestation
	P2PQualityHealthy  bool   // Kết quả từ Tri-interface Profiler
}

type PeripheralSensorEngine interface {
	GetLocalSnapshot() (PeripheralSnapshot, error)
	InjectFaultScenario(scenarioID string) // Phục vụ trực tiếp cho E2-E9
}
