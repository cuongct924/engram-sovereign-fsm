package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMisbehaviorTypeName(t *testing.T) {
	require.Equal(t, "DuplicateVote", MisbehaviorTypeName(1))
	require.Equal(t, "LightClientAttack", MisbehaviorTypeName(2))
	require.Equal(t, "Unknown", MisbehaviorTypeName(0))
	require.Equal(t, "Unknown", MisbehaviorTypeName(42))
}
