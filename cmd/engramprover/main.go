// Command engramprover keeps the re-anchoring proofs flowing while the chain is in SOVEREIGN: it starts a segment every STAGGER_S seconds,
// has up to PROVERS proofs built at once (on this host, or on the worker hosts of WORKERS) and submits them in order. See internal/prover.
//
// It is configured through the environment, like the shell scripts it replaces:
//
//	NODE_URL                  the validator whose RPC it polls (default http://127.0.0.1:26657)
//	ENGRAM_CHAIN_ID           the chain id of the boundary witness (default engram-dev-1)
//	ENGRAMD_BIN               engramd, for the node queries and transactions (default engramd)
//	REPO_ROOT                 where circuit/ and x/ are (default /app)
//	PROVERS                   proofs in flight (default: the number of WORKERS, else 1)
//	L_EST_S, STAGGER_S        expected seconds per proof (default 75); segments start STAGGER_S apart (default L_EST_S / PROVERS)
//	POLL_INTERVAL_S           seconds between looks at the chain (default 2)
//	MIN_BATCH                 headers that must wait before a segment starts (default 1)
//	MAX_CHECKS                polls before it stops (default: run until stopped)
//	WORKERS                   space-separated user@host: hand each proof to a worker (one per worker at a time)
//	WORKER_KEY, WORKER_IMAGE  the SSH key and the prover image on the workers
//	BB_THREADS                threads of bb when proving here (default: all cores)
//	WITNESS_PUBLISH_MIN_INTERVAL_S   seconds between audit-trail publications to Celestia (default 120)
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cuongct220020/engram-sovereign-fsm/internal/prover"
)

func envStr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s=%q is not an integer\n", k, v)
			os.Exit(2)
		}
		return n
	}
	return def
}

func main() { os.Exit(run()) }

func run() int {
	root := envStr("REPO_ROOT", "/app")
	workers := strings.Fields(os.Getenv("WORKERS"))
	provers := envInt("PROVERS", 0)
	if provers == 0 {
		provers = max(1, len(workers))
	}
	estimate := envInt("L_EST_S", 75)
	stagger := envInt("STAGGER_S", (estimate+provers-1)/provers)

	chain := &prover.ExecChain{Engramd: envStr("ENGRAMD_BIN", "engramd"), NodeURL: envStr("NODE_URL", "http://127.0.0.1:26657"), ChainID: envStr("ENGRAM_CHAIN_ID", "engram-dev-1")}
	linker := &prover.ExecLinker{Nargo: "nargo", CircuitDir: filepath.Join(root, "circuit", "reanchoring_witness")}
	var pr prover.Prover
	if len(workers) > 0 {
		image := os.Getenv("WORKER_IMAGE")
		if image == "" {
			fmt.Fprintln(os.Stderr, "WORKERS is set but WORKER_IMAGE is not")
			return 2
		}
		known, err := os.CreateTemp("", "known_hosts-*")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		known.Close()
		defer os.Remove(known.Name())
		pr = prover.NewSSHPool(workers, envStr("WORKER_KEY", "/run/worker_key"), image, known.Name())
	} else {
		pr = &prover.LocalProver{
			Nargo: "nargo", BB: "bb", CircuitDir: filepath.Join(root, "circuit", "reanchoring"),
			VK: filepath.Join(root, "x", "sovereignty", "keeper", "zk_assets", "vk"), BBThreads: envInt("BB_THREADS", 0),
		}
	}

	p := &prover.Pipeline{
		Cfg: prover.Config{
			Provers:                   provers,
			Stagger:                   time.Duration(stagger) * time.Second,
			Poll:                      time.Duration(envInt("POLL_INTERVAL_S", 2)) * time.Second,
			MinBatch:                  envInt("MIN_BATCH", 1),
			MaxChecks:                 envInt("MAX_CHECKS", 0),
			BoundaryRetries:           envInt("BOUNDARY_RETRIES", 0),
			ControlDir:                os.Getenv("PROVER_CONTROL_DIR"),
			WitnessPublishMinInterval: time.Duration(envInt("WITNESS_PUBLISH_MIN_INTERVAL_S", 120)) * time.Second,
			Logf: func(f string, a ...any) {
				fmt.Printf("[%s] "+f+"\n", append([]any{time.Now().UTC().Format("15:04:05")}, a...)...)
			},
			ProofLog: func(line string) { fmt.Println(line) },
		},
		Chain: chain, Linker: linker, Prover: pr,
	}
	fmt.Printf("[engramprover] %d proofs in flight, a segment every %ds, %d worker(s), node %s\n", provers, stagger, len(workers), chain.NodeURL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	st := p.Run(ctx)
	fmt.Printf("[engramprover] stopped: %d proof(s) accepted, %d segment(s) failed\n", st.Submitted, st.Failed)
	if st.Submitted == 0 {
		return 1
	}
	return 0
}
