package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithdrawLocked(t *testing.T) {
	for _, s := range []string{StateSovereign, StateRecovering} {
		require.True(t, WithdrawLocked(s), s)
	}
	for _, s := range []string{StateAnchored, StateSuspicious} {
		require.False(t, WithdrawLocked(s), s)
	}
}
