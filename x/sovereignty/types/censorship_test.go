package types

import "testing"

func TestIsCensoring(t *testing.T) {
	cases := []struct {
		name     string
		queue    []string
		ignored  map[string]uint64
		included map[string]bool
		maxRound uint64
		want     bool
	}{
		{
			name:     "below threshold never censoring",
			queue:    []string{"tx1"},
			ignored:  map[string]uint64{"tx1": 0},
			included: map[string]bool{},
			maxRound: 1,
			want:     false,
		},
		{
			name:     "at threshold but included in current proposal is not censoring",
			queue:    []string{"tx1"},
			ignored:  map[string]uint64{"tx1": 1},
			included: map[string]bool{"tx1": true},
			maxRound: 1,
			want:     false,
		},
		{
			name:     "at threshold and absent is censoring",
			queue:    []string{"tx1"},
			ignored:  map[string]uint64{"tx1": 1},
			included: map[string]bool{},
			maxRound: 1,
			want:     true,
		},
		{
			name:     "above threshold and absent is censoring",
			queue:    []string{"tx1"},
			ignored:  map[string]uint64{"tx1": 2},
			included: map[string]bool{},
			maxRound: 1,
			want:     true,
		},
		{
			name:     "one clean, one censored -- exists quantifier",
			queue:    []string{"tx1", "tx2"},
			ignored:  map[string]uint64{"tx1": 0, "tx2": 5},
			included: map[string]bool{"tx1": true},
			maxRound: 1,
			want:     true,
		},
		{
			name:     "empty queue never censoring",
			queue:    nil,
			ignored:  map[string]uint64{},
			included: map[string]bool{},
			maxRound: 1,
			want:     false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsCensoring(c.queue, c.ignored, c.included, c.maxRound); got != c.want {
				t.Errorf("IsCensoring() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestNextIgnoredRounds(t *testing.T) {
	t.Run("included tx resets to zero", func(t *testing.T) {
		got := NextIgnoredRounds([]string{"tx1"}, map[string]uint64{"tx1": 3}, map[string]bool{"tx1": true}, 1)
		if got["tx1"] != 0 {
			t.Errorf("got %d, want 0", got["tx1"])
		}
	})

	t.Run("absent tx increments", func(t *testing.T) {
		got := NextIgnoredRounds([]string{"tx1"}, map[string]uint64{"tx1": 0}, map[string]bool{}, 1)
		if got["tx1"] != 1 {
			t.Errorf("got %d, want 1", got["tx1"])
		}
	})

	t.Run("absent tx caps at maxIgnoreRounds+1", func(t *testing.T) {
		got := NextIgnoredRounds([]string{"tx1"}, map[string]uint64{"tx1": 1}, map[string]bool{}, 1)
		if got["tx1"] != 2 {
			t.Errorf("got %d, want 2 (cap)", got["tx1"])
		}
		got2 := NextIgnoredRounds([]string{"tx1"}, map[string]uint64{"tx1": 2}, map[string]bool{}, 1)
		if got2["tx1"] != 2 {
			t.Errorf("got %d, want 2 (stays capped)", got2["tx1"])
		}
	})

	t.Run("unseen tx in queue starts from zero current value", func(t *testing.T) {
		got := NextIgnoredRounds([]string{"tx1"}, map[string]uint64{}, map[string]bool{}, 1)
		if got["tx1"] != 1 {
			t.Errorf("got %d, want 1", got["tx1"])
		}
	})
}
