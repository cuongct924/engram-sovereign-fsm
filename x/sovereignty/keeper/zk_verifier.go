package keeper

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"fmt"
	"io"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/vmihailenco/msgpack/v5"
)

// zkVerifierWasmGz is barretenberg.wasm (a wasm32-wasi UltraHonk/BN254
// verifier, built from https://github.com/cuongct924/engram-barretenberg-wasm
// -- a minimal, patched subset of AztecProtocol/aztec-packages, pinned to
// commit 87aa5283d5b0cab6632ed49fdd25f6548d2ba91f / tag
// v5.0.0-nightly.20260522, matching the exact `bb` release this repo's own
// Dockerfile installs (ARG BB_VERSION=5.0.0-nightly.20260522) -- prover and
// verifier must share one toolchain build, see that repo's README for why),
// gzip-compressed (83MB -> 5.6MB; WASM bytecode compresses very well) to
// stay well under GitHub's 100MB file limit.
//
//go:embed zk_assets/barretenberg.wasm.gz
var zkVerifierWasmGz []byte

// bn254G1FirstElement is the CIRCUIT-INDEPENDENT first point of the BN254
// trusted-setup G1 SRS, uncompressed (bb::srs::factories::
// BN254_G1_FIRST_ELEMENT_COMPRESSED's affine (1,2), matching
// ~/.bb-crs/bn254_g1.dat's first 64 bytes on any real bb install). Real
// UltraHonk verification only ever needs num_points=1
// (VerifierCommitmentKey::get_verifier_crs() == get_crs(1)) -- the full
// multi-hundred-MB proving SRS is never touched here.
var bn254G1FirstElement = []byte{
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02,
}

// bn254G2Element is bb::srs::factories::BN254_G2_ELEMENT_BYTES, the
// canonical Aztec trusted-setup [x]_2 -- a compile-time constant inside
// barretenberg.wasm itself (src/barretenberg/srs/factories/bn254_crs_data.hpp),
// not read from any file. Kept here identically so SrsInitSrs (which
// requires a caller-supplied g2_point regardless) has something to pass.
var bn254G2Element = []byte{
	0x01, 0x18, 0xc4, 0xd5, 0xb8, 0x37, 0xbc, 0xc2, 0xbc, 0x89, 0xb5, 0xb3, 0x98, 0xb5, 0x97, 0x4e, 0x9f, 0x59, 0x44, 0x07, 0x3b, 0x32, 0x07, 0x8b, 0x7e, 0x23, 0x1f, 0xec, 0x93, 0x88, 0x83, 0xb0, 0x26, 0x0e, 0x01, 0xb2, 0x51, 0xf6, 0xf1, 0xc7, 0xe7, 0xff, 0x4e, 0x58, 0x07, 0x91, 0xde, 0xe8, 0xea, 0x51, 0xd8, 0x7a, 0x35, 0x8e, 0x03, 0x8b, 0x4e, 0xfe, 0x30, 0xfa, 0xc0, 0x93, 0x83, 0xc1, 0x22, 0xfe, 0xbd, 0xa3, 0xc0, 0xc0, 0x63, 0x2a, 0x56, 0x47, 0x5b, 0x42, 0x14, 0xe5, 0x61, 0x5e, 0x11, 0xe6, 0xdd, 0x3f, 0x96, 0xe6, 0xce, 0xa2, 0x85, 0x4a, 0x87, 0xd4, 0xda, 0xcc, 0x5e, 0x55, 0x04, 0xfc, 0x63, 0x69, 0xf7, 0x11, 0x0f, 0xe3, 0xd2, 0x51, 0x56, 0xc1, 0xbb, 0x9a, 0x72, 0x85, 0x9c, 0xf2, 0xa0, 0x46, 0x41, 0xf9, 0x9b, 0xa4, 0xee, 0x41, 0x3c, 0x80, 0xda, 0x6a, 0x5f, 0xe4,
}

// wasmMemoryMinPages/wasmMemoryMaxPages are barretenberg.wasm's own memory
// import limits (--stack-first,-z,stack-size=16777216,--max-memory=4294967296
// at link time) -- must match exactly what the module was linked to expect,
// re-derived via wazero's own CompiledModule.ImportedMemories() if this
// build is ever regenerated with different limits.
const (
	wasmMemoryMinPages = 272
	wasmMemoryMaxPages = 65536
)

type zkVerifierRuntime struct {
	runtime        wazero.Runtime
	mainMod        wazero.CompiledModule
	envMod         wazero.CompiledModule
	gofuncsBuilder wazero.HostModuleBuilder
}

var (
	zkRuntimeOnce sync.Once
	zkRuntime     *zkVerifierRuntime
	zkRuntimeErr  error
)

func getZKVerifierRuntime(ctx context.Context) (*zkVerifierRuntime, error) {
	zkRuntimeOnce.Do(func() {
		zkRuntime, zkRuntimeErr = newZKVerifierRuntime(ctx)
	})
	return zkRuntime, zkRuntimeErr
}

func newZKVerifierRuntime(ctx context.Context) (*zkVerifierRuntime, error) {
	gz, err := gzip.NewReader(bytes.NewReader(zkVerifierWasmGz))
	if err != nil {
		return nil, fmt.Errorf("open embedded wasm gzip: %w", err)
	}
	wasmBytes, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("decompress embedded wasm: %w", err)
	}

	rt := wazero.NewRuntime(ctx)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("instantiate wasi_snapshot_preview1: %w", err)
	}

	mainMod, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("compile barretenberg.wasm: %w", err)
	}

	envModBytes := buildZKVerifierEnvModule(wasmMemoryMinPages, wasmMemoryMaxPages)
	envMod, err := rt.CompileModule(ctx, envModBytes)
	if err != nil {
		rt.Close(ctx)
		return nil, fmt.Errorf("compile env shim module: %w", err)
	}

	gofuncsBuilder := rt.NewHostModuleBuilder("gofuncs").
		NewFunctionBuilder().
		WithFunc(func(_ context.Context, m api.Module, msgPtr uint32) {
			// throw_or_abort_impl: real invariant violations inside
			// barretenberg (e.g. a malformed VK) surface here as a wasm
			// trap right after this returns -- log for diagnosis, the trap
			// itself is what VerifyZKProof's caller ultimately observes.
			fmt.Println("engramd: zk verifier wasm throw_or_abort:", readWasmCString(m, msgPtr))
		}).
		Export("throw_or_abort_impl").
		NewFunctionBuilder().
		WithFunc(func(_ context.Context, _ api.Module, _ uint32) {
			// logstr: barretenberg's internal info()/vinfo() logging --
			// deliberately dropped, not printed, so a validator's own
			// verify-path logging stays as quiet as the previous `bb`
			// CLI subprocess's stdout already was (never captured).
		}).
		Export("logstr")

	return &zkVerifierRuntime{
		runtime:        rt,
		mainMod:        mainMod,
		envMod:         envMod,
		gofuncsBuilder: gofuncsBuilder,
	}, nil
}

// buildZKVerifierEnvModule hand-assembles a minimal WASM binary module that
// imports the "gofuncs"-hosted throw_or_abort_impl/logstr functions,
// defines the linear memory barretenberg.wasm's own "env" import expects,
// and re-exports both the memory and the two functions under module name
// "env" -- WASM natively allows a module to re-export what it imports.
//
// This exists because wazero's HostModuleBuilder can only export functions,
// never memory: since barretenberg.wasm imports BOTH memory and functions
// from the same source module name ("env"), a real Go-backed host module
// can't supply the memory half. Splitting the Go functions into a
// separately-named host module ("gofuncs") and gluing the two together
// via this synthetic module lets one WASM instantiation of "env" satisfy
// both halves of that single import namespace.
func buildZKVerifierEnvModule(minPages, maxPages uint32) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}) // magic + version

	// Type section: type 0 = (i32) -> ()
	typeSec := []byte{0x01, 0x60, 0x01, 0x7f, 0x00} // count=1, func, params=[i32], results=[]
	writeWasmSection(&buf, 1, typeSec)

	// Import section: "gofuncs"."throw_or_abort_impl" func 0, "gofuncs"."logstr" func 0
	var importSec bytes.Buffer
	writeUleb128(&importSec, 2) // 2 imports
	writeWasmName(&importSec, "gofuncs")
	writeWasmName(&importSec, "throw_or_abort_impl")
	importSec.WriteByte(0x00) // import kind: func
	writeUleb128(&importSec, 0)
	writeWasmName(&importSec, "gofuncs")
	writeWasmName(&importSec, "logstr")
	importSec.WriteByte(0x00)
	writeUleb128(&importSec, 0)
	writeWasmSection(&buf, 2, importSec.Bytes())

	// Memory section: one memory, min/max pages
	var memSec bytes.Buffer
	writeUleb128(&memSec, 1) // 1 memory
	memSec.WriteByte(0x01)   // limits flag: has max
	writeUleb128(&memSec, uint64(minPages))
	writeUleb128(&memSec, uint64(maxPages))
	writeWasmSection(&buf, 5, memSec.Bytes())

	// Export section: memory 0 as "memory", func 0/1 re-exported by their
	// original names (imported funcs occupy index space 0..N-1 first).
	var exportSec bytes.Buffer
	writeUleb128(&exportSec, 3) // 3 exports
	writeWasmName(&exportSec, "memory")
	exportSec.WriteByte(0x02) // export kind: memory
	writeUleb128(&exportSec, 0)
	writeWasmName(&exportSec, "throw_or_abort_impl")
	exportSec.WriteByte(0x00) // export kind: func
	writeUleb128(&exportSec, 0)
	writeWasmName(&exportSec, "logstr")
	exportSec.WriteByte(0x00)
	writeUleb128(&exportSec, 1)
	writeWasmSection(&buf, 7, exportSec.Bytes())

	return buf.Bytes()
}

func writeWasmSection(buf *bytes.Buffer, id byte, content []byte) {
	buf.WriteByte(id)
	writeUleb128(buf, uint64(len(content)))
	buf.Write(content)
}

func writeWasmName(buf *bytes.Buffer, s string) {
	writeUleb128(buf, uint64(len(s)))
	buf.WriteString(s)
}

func writeUleb128(buf *bytes.Buffer, v uint64) {
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf.WriteByte(b)
		if v == 0 {
			return
		}
	}
}

func readWasmCString(m api.Module, ptr uint32) string {
	var out []byte
	for i := uint32(0); i < 1<<20; i++ {
		b, ok := m.Memory().ReadByte(ptr + i)
		if !ok || b == 0 {
			break
		}
		out = append(out, b)
	}
	return string(out)
}

// circuitVerifyRequest mirrors bb::bbapi::CircuitVerify's SERIALIZATION_FIELDS
// exactly (verification_key, public_inputs, proof, settings) -- field NAMES
// are the wire keys as-declared (snake_case), not camelCase; see
// https://github.com/cuongct924/engram-barretenberg-wasm's README for how
// this was confirmed against the real C++ NVP/msgpack macro.
type circuitVerifyRequest struct {
	VerificationKey []byte              `msgpack:"verification_key"`
	PublicInputs    [][]byte            `msgpack:"public_inputs"`
	Proof           [][]byte            `msgpack:"proof"`
	Settings        proofSystemSettings `msgpack:"settings"`
}

type proofSystemSettings struct {
	IpaAccumulation           bool   `msgpack:"ipa_accumulation"`
	OracleHashType            string `msgpack:"oracle_hash_type"`
	DisableZK                 bool   `msgpack:"disable_zk"`
	OptimizedSolidityVerifier bool   `msgpack:"optimized_solidity_verifier"`
}

type circuitVerifyResponse struct {
	Verified bool `msgpack:"verified"`
}

type srsInitSrsRequest struct {
	PointsBuf []byte `msgpack:"points_buf"`
	NumPoints uint32 `msgpack:"num_points"`
	G2Point   []byte `msgpack:"g2_point"`
}

// chunk32 splits raw concatenated 32-byte field elements -- the same
// convention circuit/reanchoring's proof/public_inputs files, and
// Barretenberg's own many_from_buffer_exact<uint256_t>, already use. No
// length prefix, no byte-order conversion: msg.ZkProof/msg.PublicInputs are
// already in exactly this format.
func chunk32(data []byte) [][]byte {
	out := make([][]byte, 0, len(data)/32)
	for i := 0; i+32 <= len(data); i += 32 {
		out = append(out, data[i:i+32])
	}
	return out
}

// verifyZKProofWasm runs vk/proof/publicInputs through the embedded
// wasm32-wasi UltraHonk verifier via wazero -- a deterministic, pure-Go
// replacement for shelling out to the `bb` CLI. Every validator running the
// same engramd binary runs the exact same verifier bytecode, removing the
// external-binary/PATH/OS dependency a pinned checksum could only detect
// after the fact, never eliminate.
func verifyZKProofWasm(vk, proof, publicInputs []byte) (bool, error) {
	ctx := context.Background()
	zkrt, err := getZKVerifierRuntime(ctx)
	if err != nil {
		return false, fmt.Errorf("zk verifier runtime: %w", err)
	}

	// Each call gets its own fresh gofuncs/env/main instances -- module names
	// must be unique within a runtime at any given time, and reusing a
	// single main-module instance across calls would leak barretenberg's
	// internal C++ static state (the SRS factory, CRS init flags) between
	// verifications. Closed in dependency order (main, then env, then
	// gofuncs) via LIFO defers below.
	gofuncsInst, err := zkrt.gofuncsBuilder.Instantiate(ctx)
	if err != nil {
		return false, fmt.Errorf("instantiate gofuncs host module: %w", err)
	}
	defer gofuncsInst.Close(ctx)

	envInst, err := zkrt.runtime.InstantiateModule(ctx, zkrt.envMod, wazero.NewModuleConfig().WithName("env"))
	if err != nil {
		return false, fmt.Errorf("instantiate env shim module: %w", err)
	}
	defer envInst.Close(ctx)

	mod, err := zkrt.runtime.InstantiateModule(ctx, zkrt.mainMod, wazero.NewModuleConfig().WithStartFunctions())
	if err != nil {
		return false, fmt.Errorf("instantiate barretenberg.wasm: %w", err)
	}
	defer mod.Close(ctx)

	// WASI reactor pattern: barretenberg.wasm exports "_initialize" (runs
	// C++ global/static constructors, e.g. Poseidon2's round-constant
	// tables), not "_start" -- WithStartFunctions() above only auto-runs
	// "_start", so this must be called explicitly before any other export.
	if initFn := mod.ExportedFunction("_initialize"); initFn != nil {
		if _, err := initFn.Call(ctx); err != nil {
			return false, fmt.Errorf("_initialize: %w", err)
		}
	}

	bbmalloc := mod.ExportedFunction("bbmalloc")
	bbfree := mod.ExportedFunction("bbfree")
	bbapiFn := mod.ExportedFunction("bbapi")
	if bbmalloc == nil || bbfree == nil || bbapiFn == nil {
		return false, fmt.Errorf("barretenberg.wasm missing expected exports (bbmalloc/bbfree/bbapi)")
	}

	// SrsInitSrs must run once per instance before CircuitVerify's pairing
	// check, or get_bn254_crs_factory() traps with "you need to initialize
	// the global CRS".
	srsCmd, err := encodeBBAPICommand("SrsInitSrs", srsInitSrsRequest{
		PointsBuf: bn254G1FirstElement,
		NumPoints: 1,
		G2Point:   bn254G2Element,
	})
	if err != nil {
		return false, fmt.Errorf("encode SrsInitSrs: %w", err)
	}
	if _, _, err := callBBAPI(ctx, mod, bbmalloc, bbfree, bbapiFn, srsCmd); err != nil {
		return false, fmt.Errorf("SrsInitSrs: %w", err)
	}

	verifyCmd, err := encodeBBAPICommand("CircuitVerify", circuitVerifyRequest{
		VerificationKey: vk,
		PublicInputs:    chunk32(publicInputs),
		Proof:           chunk32(proof),
		Settings: proofSystemSettings{
			IpaAccumulation:           false,
			OracleHashType:            "poseidon2",
			DisableZK:                 false,
			OptimizedSolidityVerifier: false,
		},
	})
	if err != nil {
		return false, fmt.Errorf("encode CircuitVerify: %w", err)
	}
	respBytes, respType, err := callBBAPI(ctx, mod, bbmalloc, bbfree, bbapiFn, verifyCmd)
	if err != nil {
		return false, fmt.Errorf("CircuitVerify: %w", err)
	}
	if respType == "ErrorResponse" {
		return false, fmt.Errorf("CircuitVerify returned ErrorResponse")
	}

	dec := msgpack.NewDecoder(bytes.NewReader(respBytes))
	if _, err := dec.DecodeArrayLen(); err != nil {
		return false, fmt.Errorf("decode response array: %w", err)
	}
	if err := dec.Skip(); err != nil { // already-read type name string
		return false, fmt.Errorf("skip response type name: %w", err)
	}
	var vresp circuitVerifyResponse
	if err := dec.Decode(&vresp); err != nil {
		return false, fmt.Errorf("decode CircuitVerifyResponse: %w", err)
	}
	return vresp.Verified, nil
}

// encodeBBAPICommand builds bbapi's msgpack wire format for Command:
// bbapi(Command&&) takes ONE argument, so msgpack_cbind_impl unpacks into
// std::tuple<Command> -- an outer 1-element array wrapping the Command
// itself, which is in turn NamedUnion's own [type_name, payload] 2-array.
func encodeBBAPICommand(name string, payload interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := msgpack.NewEncoder(&buf)
	if err := enc.EncodeArrayLen(1); err != nil {
		return nil, err
	}
	if err := enc.EncodeArrayLen(2); err != nil {
		return nil, err
	}
	if err := enc.EncodeString(name); err != nil {
		return nil, err
	}
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// callBBAPI runs one bbapi(Command) round-trip: writes cmdBuf into the
// module's linear memory via bbmalloc, calls bbapi(input_ptr, input_len,
// &output_ptr, &output_len), copies the response out, and frees every
// allocation it made (matching c_bind.hpp's "caller must free the memory"
// doc). Returns the raw CommandResponse bytes and the response's own
// NamedUnion type name (e.g. "CircuitVerifyResponse" or "ErrorResponse").
func callBBAPI(ctx context.Context, mod api.Module, bbmalloc, bbfree, bbapiFn api.Function, cmdBuf []byte) ([]byte, string, error) {
	res, err := bbmalloc.Call(ctx, uint64(len(cmdBuf)))
	if err != nil {
		return nil, "", fmt.Errorf("bbmalloc(input): %w", err)
	}
	inputPtr := uint32(res[0])
	if !mod.Memory().Write(inputPtr, cmdBuf) {
		return nil, "", fmt.Errorf("write command bytes to wasm memory")
	}

	// output_out / output_len_out: 4-byte wasm32 pointer/size_t slots,
	// zeroed (no scratch buffer offered -- forces bbapi to allocate its
	// own, whose pointer/length it writes back into these two slots).
	res, err = bbmalloc.Call(ctx, 4)
	if err != nil {
		return nil, "", fmt.Errorf("bbmalloc(output_out): %w", err)
	}
	outputOutPtr := uint32(res[0])
	res, err = bbmalloc.Call(ctx, 4)
	if err != nil {
		return nil, "", fmt.Errorf("bbmalloc(output_len_out): %w", err)
	}
	outputLenOutPtr := uint32(res[0])
	mod.Memory().WriteUint32Le(outputOutPtr, 0)
	mod.Memory().WriteUint32Le(outputLenOutPtr, 0)

	defer func() {
		_, _ = bbfree.Call(ctx, uint64(inputPtr))
		_, _ = bbfree.Call(ctx, uint64(outputOutPtr))
		_, _ = bbfree.Call(ctx, uint64(outputLenOutPtr))
	}()

	if _, err := bbapiFn.Call(ctx, uint64(inputPtr), uint64(len(cmdBuf)), uint64(outputOutPtr), uint64(outputLenOutPtr)); err != nil {
		return nil, "", fmt.Errorf("bbapi call: %w", err)
	}

	resultPtr, _ := mod.Memory().ReadUint32Le(outputOutPtr)
	resultLen, _ := mod.Memory().ReadUint32Le(outputLenOutPtr)
	respBytes, ok := mod.Memory().Read(resultPtr, resultLen)
	if !ok {
		return nil, "", fmt.Errorf("read response bytes from wasm memory")
	}
	respBytes = append([]byte(nil), respBytes...) // copy out before freeing
	defer func() { _, _ = bbfree.Call(ctx, uint64(resultPtr)) }()

	dec := msgpack.NewDecoder(bytes.NewReader(respBytes))
	if _, err := dec.DecodeArrayLen(); err != nil {
		return nil, "", fmt.Errorf("decode response array len: %w", err)
	}
	typeName, err := dec.DecodeString()
	if err != nil {
		return nil, "", fmt.Errorf("decode response type name: %w", err)
	}

	return respBytes, typeName, nil
}
