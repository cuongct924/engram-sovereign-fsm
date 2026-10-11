// Package invariants checks that values which several files must agree on do agree. The circuit has the chain id, the number of validators and
// the capacity compiled in, and the images pin the tool versions the verification keys were made with: a deployment that drifts from any of them
// fails late and expensively (every proof of a cloud deployment failed once because its chain id differed from the circuit's), so the test fails
// in CI instead.
package invariants

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/internal/prover"
)

func read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func find(t *testing.T, text, pattern string) string {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("pattern %q not found", pattern)
	}
	return m[1]
}

// circuitChainID decodes the CHAIN_ID byte array of circuit/reanchoring/src/main.nr.
func circuitChainID(t *testing.T) string {
	t.Helper()
	body := find(t, read(t, "circuit/reanchoring/src/main.nr"), `(?s)global CHAIN_ID: \[u8; \d+\] = \[(.*?)\];`)
	var out []byte
	for _, tok := range regexp.MustCompile(`0x[0-9a-fA-F]{2}`).FindAllString(body, -1) {
		n, err := strconv.ParseUint(tok[2:], 16, 8)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, byte(n))
	}
	return string(out)
}

func TestChainIDAgreesWithTheCircuit(t *testing.T) {
	want := circuitChainID(t)
	if want == "" {
		t.Fatal("no chain id in the circuit")
	}
	checks := map[string]string{
		"cmd/engramd/reanchor_witness_cli.go":            `"chain-id", "([^"]+)"`,
		"cmd/engramd/main.go":                            `ChainID:\s+"([^"]+)"`,
		"cmd/engramprover/main.go":                       `"ENGRAM_CHAIN_ID", "([^"]+)"`,
		"deploy/ansible/group_vars/all.yml":              `(?m)^engram_chain_id: (\S+)`,
		"scripts/reanchoring_prover/prove_and_submit.sh": `ENGRAM_CHAIN_ID:-([^}]+)\}`,
	}
	for file, pattern := range checks {
		if got := find(t, read(t, file), pattern); got != want {
			t.Errorf("%s uses chain id %q, the circuit has %q", file, got, want)
		}
	}
	if !strings.Contains(read(t, "deploy/ansible/roles/controller/tasks/main.yml"), "engram_chain_id == '"+want+"'") {
		t.Errorf("the controller role's check does not name the circuit's chain id %q", want)
	}
}

func TestValidatorCountAgreesWithTheCircuit(t *testing.T) {
	want := find(t, read(t, "circuit/reanchoring/src/main.nr"), `global NUM_VALIDATORS: u32 = (\d+);`)
	if got := find(t, read(t, "cmd/engramd/reanchor_witness_cli.go"), `numValidators\s*=\s*(\d+)`); got != want {
		t.Errorf("the boundary-witness tool expects %s validators, the circuit %s", got, want)
	}
	slots := find(t, read(t, "deploy/terraform/aws/main.tf"), `(?s)variable "validator_slots" \{.*?default\s*=\s*\[(.*?)\]`)
	if got := strconv.Itoa(len(regexp.MustCompile(`"r\d"`).FindAllString(slots, -1))); got != want {
		t.Errorf("the Terraform default places %s validators, the circuit has %s", got, want)
	}
}

func TestCapacityAgreesWithTheCircuit(t *testing.T) {
	want := find(t, read(t, "circuit/reanchoring/src/main.nr"), `global N_MAX: u32 = (\d+);`)
	if strconv.Itoa(prover.NMax) != want {
		t.Errorf("internal/prover.NMax is %d, the circuit has %s", prover.NMax, want)
	}
	if got := find(t, read(t, "scripts/reanchoring_prover/prove_and_submit.sh"), `(?m)^N_MAX=(\d+)`); got != want {
		t.Errorf("prove_and_submit.sh has N_MAX=%s, the circuit %s", got, want)
	}
}

func TestToolVersionsAgreeWithTheAssetLock(t *testing.T) {
	lock := read(t, "x/sovereignty/keeper/zk_assets/ASSETS.lock")
	bb := find(t, lock, `(?m)^#\s+bb\s+(\S+)`)
	nargo := find(t, lock, `(?m)^#\s+nargo\s+(\S+)`)
	for _, file := range []string{"Dockerfile", "docker/reanchoring-prover/Dockerfile"} {
		if got := find(t, read(t, file), `ARG BB_VERSION=(\S+)`); got != bb {
			t.Errorf("%s installs bb %s, the asset lock says %s", file, got, bb)
		}
	}
	if got := find(t, read(t, "docker/reanchoring-prover/Dockerfile"), `ARG NARGO_VERSION=(\S+)`); got != nargo {
		t.Errorf("the prover image installs nargo %s, the asset lock says %s", got, nargo)
	}
}
