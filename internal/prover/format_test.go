package prover

import (
	"strings"
	"testing"
)

const queryOut = `rt_last=5646870041200576471122970460328951287190189637856723436015228560911888792656
first_height=1323
validator_set_hash=ab
validator pub_key=aa power=10
block_hash height=1324 hash=bb
height=1324 fsm_state=2 withdrawal_locked=1 state_root=11
height=1325 fsm_state=2 withdrawal_locked=1 state_root=12
height=1326 fsm_state=3 withdrawal_locked=1 state_root=13
`

func TestParseHeaders(t *testing.T) {
	rt, hs, err := ParseHeaders(queryOut)
	if err != nil {
		t.Fatal(err)
	}
	if rt != "5646870041200576471122970460328951287190189637856723436015228560911888792656" {
		t.Fatalf("rt_last %q", rt)
	}
	if len(hs) != 3 || hs[0].Height != 1324 || hs[2].FSMState != "3" || hs[1].StateRoot != "12" {
		t.Fatalf("headers %+v", hs)
	}
	if _, _, err := ParseHeaders("height=1 fsm_state=2 withdrawal_locked=1 state_root=1\n"); err == nil {
		t.Fatal("no rt_last must be an error")
	}
	if _, _, err := ParseHeaders("rt_last=1\nheight=5 fsm_state=2\n"); err == nil {
		t.Fatal("an incomplete header line must be an error")
	}
}

// The output below is what nargo prints for circuit/reanchoring_witness on three headers (captured from a real run).
const nargoOut = `Header { prev_hash: 0x01, fsm_state: 0x02, withdrawal_locked: 0x01, state_root: 0x0b }
Header { prev_hash: 0x215e7bb6c51832572c2c7b4e72737a44a398b9183c9065fdf5227ab122306c35, fsm_state: 0x02, withdrawal_locked: 0x01, state_root: 0x0c }
Header { prev_hash: 0x2b0a4466d2ada8a4a9f2355a8561460a25b6c8ea7ba56706b5e0ee8fc4619d12, fsm_state: 0x03, withdrawal_locked: 0x01, state_root: 0x0d }
rt_new_hash=0x183d2d3b29467660fb1152b98f7d89bdb7fec5ce77f31172de0bafa4c0c37bb8
[reanchoring_witness] Circuit witness successfully solved
`

func TestParseLinked(t *testing.T) {
	l, rt, err := ParseLinked(nargoOut)
	if err != nil {
		t.Fatal(err)
	}
	if len(l) != 3 || l[0].PrevHash != "0x01" || l[2].FSMState != "0x03" || l[1].StateRoot != "0x0c" {
		t.Fatalf("linked %+v", l)
	}
	if rt != "0x183d2d3b29467660fb1152b98f7d89bdb7fec5ce77f31172de0bafa4c0c37bb8" {
		t.Fatalf("rt_new %q", rt)
	}
	if _, _, err := ParseLinked("Header { prev_hash: 0x01, fsm_state: 0x02, withdrawal_locked: 0x01, state_root: 0x0b }\n"); err == nil {
		t.Fatal("no rt_new_hash must be an error")
	}
}

func TestWitnessTomlPadsToNMax(t *testing.T) {
	_, hs, _ := ParseHeaders(queryOut)
	toml := WitnessToml("7", hs)
	if !strings.Contains(toml, `rt_last = "7"`) || !strings.Contains(toml, `count = "3"`) {
		t.Fatal(toml)
	}
	for _, line := range strings.Split(toml, "\n") {
		if strings.HasPrefix(line, "raw_fsm_state") {
			if n := strings.Count(line, `"`) / 2; n != NMax {
				t.Fatalf("raw_fsm_state has %d entries, want %d", n, NMax)
			}
			if !strings.HasPrefix(line, `raw_fsm_state = ["2","2","3","0",`) {
				t.Fatalf("raw_fsm_state starts %q", line[:40])
			}
		}
	}
}

func TestCircuitTomlLayout(t *testing.T) {
	boundary := "first_height = \"1324\"\nfirst_block_hash = \"9\"\n\n[first_block]\nx = \"1\"\n[last_block]\ny = \"2\"\n"
	linked := []Linked{{"0x01", "0x02", "0x01", "0x0b"}, {"0x05", "0x03", "0x01", "0x0c"}}
	toml := CircuitToml("7", "0x99", linked, boundary)
	// loose keys before any table (nargo misattributes later keys otherwise)
	if i, j := strings.Index(toml, "first_height"), strings.Index(toml, "[first_block]"); i < 0 || j < 0 || i > j {
		t.Fatalf("loose keys must precede the tables:\n%s", toml)
	}
	if strings.Count(toml, "[[headers]]") != NMax {
		t.Fatalf("want %d [[headers]] tables, got %d", NMax, strings.Count(toml, "[[headers]]"))
	}
	if strings.Count(toml, `prev_hash = "0"`) != NMax-2 {
		t.Fatalf("want %d padding slots", NMax-2)
	}
	if !strings.Contains(toml, `rt_new = "0x99"`) || !strings.Contains(toml, `count = "2"`) {
		t.Fatal(toml)
	}
	loose, tables := SplitBoundary(boundary)
	if strings.Contains(loose, "[") || !strings.HasPrefix(tables, "[first_block]") {
		t.Fatalf("split wrong: %q | %q", loose, tables)
	}
}

func TestAuditTomlHasNoPadding(t *testing.T) {
	a := AuditToml("1", "2", []Linked{{"0x01", "0x02", "0x01", "0x0b"}})
	if strings.Count(a, "[[headers]]") != 1 || !strings.Contains(a, `rt_new = "2"`) {
		t.Fatal(a)
	}
}
