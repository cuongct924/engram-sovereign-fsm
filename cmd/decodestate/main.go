package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/cosmos/gogoproto/proto"

	sovtypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

func main() {
	port := os.Args[1]
	u := fmt.Sprintf("http://127.0.0.1:%s/abci_query", port)
	q := url.Values{}
	q.Set("path", `"/engram.sovereignty.v1.Query/State"`)
	q.Set("data", "0x")
	resp, err := http.Get(u + "?" + q.Encode())
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var rpcResp struct {
		Result struct {
			Response struct {
				Value string `json:"value"`
			} `json:"response"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		panic(err)
	}
	raw, err := base64.StdEncoding.DecodeString(rpcResp.Result.Response.Value)
	if err != nil {
		panic(err)
	}
	var out sovtypes.QueryStateResponse
	if err := proto.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	fmt.Printf("fsm_state=%s safe_blocks=%d suspicious_duration=%d reanchoring_proof_valid=%v\n",
		out.FsmState, out.SafeBlocks, out.SuspiciousDuration, out.ReanchoringProofValid)
	if out.Metrics != nil {
		m := out.Metrics
		fmt.Printf("BtcGap=%d DaGap=%d IsDasFailed=%v IsAttestationFailed=%v\n", m.BtcGap, m.DaGap, m.IsDasFailed, m.IsAttestationFailed)
		fmt.Printf("SubnetDiversity=%d ActiveAnchors=%d CleanPeers=%d PeerChurnRate=%d AvgPeerTenure=%d PeerLatency=%d\n",
			m.SubnetDiversity, m.ActiveAnchors, m.CleanPeers, m.PeerChurnRate, m.AvgPeerTenure, m.PeerLatency)
	} else {
		fmt.Println("Metrics is nil")
	}
}
