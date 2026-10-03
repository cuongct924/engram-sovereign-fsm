package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cometbft/cometbft/p2p"
	sovereigntytypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

func TestParsePersistentPeerIDs(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want map[p2p.ID]bool
	}{
		{
			name: "single entry",
			raw:  "deadbeef@1.2.3.4:26656",
			want: map[p2p.ID]bool{"deadbeef": true},
		},
		{
			name: "multiple entries",
			raw:  "aaa@1.2.3.4:26656,bbb@5.6.7.8:26656",
			want: map[p2p.ID]bool{"aaa": true, "bbb": true},
		},
		{
			name: "whitespace around entries is trimmed",
			raw:  " aaa@1.2.3.4:26656 , bbb@5.6.7.8:26656 ",
			want: map[p2p.ID]bool{"aaa": true, "bbb": true},
		},
		{
			name: "empty string yields empty map",
			raw:  "",
			want: map[p2p.ID]bool{},
		},
		{
			name: "entry with no @ is skipped, not erroring (best-effort parsing)",
			raw:  "no-at-sign,aaa@1.2.3.4:26656",
			want: map[p2p.ID]bool{"aaa": true},
		},
		{
			name: "entry starting with @ is skipped (empty ID)",
			raw:  "@1.2.3.4:26656,aaa@1.2.3.4:26656",
			want: map[p2p.ID]bool{"aaa": true},
		},
		{
			name: "blank entries between commas are skipped",
			raw:  "aaa@1.2.3.4:26656,,bbb@5.6.7.8:26656",
			want: map[p2p.ID]bool{"aaa": true, "bbb": true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePersistentPeerIDs(tc.raw)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for id := range tc.want {
				if !got[id] {
					t.Errorf("missing expected id %q in %v", id, got)
				}
			}
		})
	}
}

func TestCountsAsChurn(t *testing.T) {
	persistentPeerIDs := map[p2p.ID]bool{"aaa": true, "bbb": true}

	cases := []struct {
		name string
		id   p2p.ID
		want bool
	}{
		{name: "known persistent peer does not count as churn", id: "aaa", want: false},
		{name: "another known persistent peer does not count as churn", id: "bbb", want: false},
		{name: "unknown peer counts as churn", id: "ccc", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := countsAsChurn(tc.id, persistentPeerIDs); got != tc.want {
				t.Errorf("countsAsChurn(%q, %v) = %v, want %v", tc.id, persistentPeerIDs, got, tc.want)
			}
		})
	}

	t.Run("empty persistentPeerIDs always counts as churn", func(t *testing.T) {
		if got := countsAsChurn("aaa", map[p2p.ID]bool{}); !got {
			t.Errorf("countsAsChurn with empty persistentPeerIDs = %v, want true", got)
		}
	})
}

func TestDefaultHome(t *testing.T) {
	home := defaultHome()
	if home == "" {
		t.Fatal("expected a non-empty default home path")
	}
}

func TestTestnetInitFilesPeerHosts(t *testing.T) {
	hosts := []string{"10.0.0.1", "10.0.0.2", "validator-3.example.org"}
	dir := t.TempDir()
	require.NoError(t, testnetInitFiles(dir, 3, "engram-test-1", "engram-node", 26656, hosts))

	cfg, err := loadConfig(filepath.Join(dir, "engram-node02"))
	require.NoError(t, err)
	require.Equal(t, "10.0.0.2:26656", cfg.P2P.ExternalAddress)
	peers := strings.Split(cfg.P2P.PersistentPeers, ",")
	require.Len(t, peers, 2)
	require.Contains(t, peers[0], "@10.0.0.1:26656")
	require.Contains(t, peers[1], "@validator-3.example.org:26656")
	require.NotContains(t, cfg.P2P.PersistentPeers, "172.40.")
}

func TestTestnetInitFilesPeerHostsCountMustMatch(t *testing.T) {
	err := testnetInitFiles(t.TempDir(), 3, "engram-test-1", "engram-node", 26656, []string{"a", "b"})
	require.ErrorContains(t, err, "exactly 3 entries")
}

func TestTestnetInitFilesDefaultsToPairwiseLinks(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, testnetInitFiles(dir, 2, "engram-test-1", "engram-node", 26656, nil))
	cfg, err := loadConfig(filepath.Join(dir, "engram-node01"))
	require.NoError(t, err)
	require.Contains(t, cfg.P2P.PersistentPeers, "@172.40.0.3:26656")
	require.Empty(t, cfg.P2P.ExternalAddress)
}

func TestResolveBaselineModeAcceptsEnvVanilla(t *testing.T) {
	mode, err := resolveBaselineMode(false, false, false, "vanilla")
	require.NoError(t, err)
	require.Equal(t, sovereigntytypes.BaselineEngram, mode, "vanilla bypasses the hooks, so the baseline policy stays the default")

	_, err = resolveBaselineMode(false, false, false, "bogus")
	require.Error(t, err)
}

func TestEngramConsensusParamsVoteExtensionsToggle(t *testing.T) {
	t.Setenv("ENGRAM_DISABLE_VOTE_EXTENSIONS", "")
	require.Equal(t, int64(1), engramConsensusParams().ABCI.VoteExtensionsEnableHeight)

	t.Setenv("ENGRAM_DISABLE_VOTE_EXTENSIONS", "1")
	require.Equal(t, int64(0), engramConsensusParams().ABCI.VoteExtensionsEnableHeight, "pure-vanilla genesis has no vote extensions")
}
