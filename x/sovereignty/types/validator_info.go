package types

// ValidatorInfo is the genesis-fixed validator identity needed to verify
// ABCI++ vote-extension signatures (vote_extension.go). ABCI only hands
// VerifyVoteExtension a validator's address, never its pubkey, so this is
// captured once at InitChain (no x/staking, no post-genesis rotation).
type ValidatorInfo struct {
	PubKey []byte `json:"pub_key"` // raw ed25519 public key bytes
	Power  int64  `json:"power"`
}
