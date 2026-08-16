package main

import (
	"testing"

	"github.com/cometbft/cometbft/p2p"
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
