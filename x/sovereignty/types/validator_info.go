package types

// ValidatorInfo is the genesis-fixed validator identity this app needs to
// verify ABCI++ vote-extension signatures in ProcessProposal (see
// x/sovereignty/vote_extension.go). ABCI only hands a validator's raw
// address (a pubkey hash) to VerifyVoteExtension/ExtendedVoteInfo, never the
// pubkey itself, so the pubkey must be captured once at InitChain from
// RequestInitChain.Validators (app/app.go's newInitChainer) -- this app has
// no x/staking module, so there is no other source and no post-genesis
// rotation (CLAUDE.md: "the validator set comes only from genesis").
type ValidatorInfo struct {
	PubKey []byte `json:"pub_key"` // raw ed25519 public key bytes
	Power  int64  `json:"power"`
}
