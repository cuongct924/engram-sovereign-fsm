package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
)

// cometRPCTimeout bounds each local CometBFT RPC read: a slow read skips this
// block's checkpoint submission or ends a backfill batch early (the gap grows)
// instead of stalling block production.
const cometRPCTimeout = 800 * time.Millisecond

// cometBlock is the part of CometBFT's /block response the checkpoint root and
// the DA backfill need.
type cometBlock struct {
	Result struct {
		BlockID struct {
			Hash string `json:"hash"`
		} `json:"block_id"`
		Block struct {
			Data struct {
				Txs []string `json:"txs"`
			} `json:"data"`
		} `json:"block"`
	} `json:"result"`
}

// fetchCometBlock reads the committed block at height from this node's own
// CometBFT RPC. Its results only go to Bitcoin checkpoints and DA blobs, never
// into committed state, so a node's local read cannot diverge AppHash.
func fetchCometBlock(ctx context.Context, client *http.Client, rpcURL string, height uint64) (*cometBlock, error) {
	ctx, cancel := context.WithTimeout(ctx, cometRPCTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/block?height=%d", rpcURL, height), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("comet /block: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("comet /block: status %d", resp.StatusCode)
	}

	var out cometBlock
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("comet /block: decode: %w", err)
	}
	return &out, nil
}

// cometBlockHashSource returns an anchor.RootSource: the block hash of a
// committed height, committed to Bitcoin as the checkpoint root.
func cometBlockHashSource(rpcURL string) anchor.RootSource {
	client := &http.Client{}
	return func(ctx context.Context, height uint64) ([]byte, error) {
		block, err := fetchCometBlock(ctx, client, rpcURL, height)
		if err != nil {
			return nil, err
		}
		root, err := hex.DecodeString(block.Result.BlockID.Hash)
		if err != nil {
			return nil, fmt.Errorf("comet /block: hash: %w", err)
		}
		return root, nil
	}
}

// cometBlockDataSource returns a da.BlockSource: the committed block's
// transactions, each framed as len(4) || raw bytes, published to DA by the
// backfill queue.
func cometBlockDataSource(rpcURL string) da.BlockSource {
	client := &http.Client{}
	return func(ctx context.Context, height uint64) ([]byte, error) {
		block, err := fetchCometBlock(ctx, client, rpcURL, height)
		if err != nil {
			return nil, err
		}
		var data []byte
		for _, b64 := range block.Result.Block.Data.Txs {
			tx, err := base64.StdEncoding.DecodeString(b64)
			if err != nil {
				return nil, fmt.Errorf("comet /block: tx: %w", err)
			}
			data = binary.BigEndian.AppendUint32(data, uint32(len(tx)))
			data = append(data, tx...)
		}
		return data, nil
	}
}
