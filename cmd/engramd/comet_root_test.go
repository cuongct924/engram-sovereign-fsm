package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCometBlockHashSource_DecodesBlockID(t *testing.T) {
	hash := strings.Repeat("ab", 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/block", r.URL.Path)
		require.Equal(t, "41", r.URL.Query().Get("height"))
		_, _ = w.Write([]byte(`{"result":{"block_id":{"hash":"` + hash + `"}}}`))
	}))
	t.Cleanup(srv.Close)

	root, err := cometBlockHashSource(srv.URL)(context.Background(), 41)
	require.NoError(t, err)
	require.Len(t, root, 32)
}

func TestCometBlockHashSource_ErrorsOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	_, err := cometBlockHashSource(srv.URL)(context.Background(), 41)
	require.Error(t, err)
}

func TestCometBlockDataSource_FramesTxs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "7", r.URL.Query().Get("height"))
		// "AQID" is base64 for {1,2,3}; "BA==" is {4}.
		_, _ = w.Write([]byte(`{"result":{"block":{"data":{"txs":["AQID","BA=="]}}}}`))
	}))
	t.Cleanup(srv.Close)

	data, err := cometBlockDataSource(srv.URL)(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, []byte{0, 0, 0, 3, 1, 2, 3, 0, 0, 0, 1, 4}, data)
}

func TestCometBlockDataSource_EmptyBlockHasNoData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":{"block":{"data":{"txs":[]}}}}`))
	}))
	t.Cleanup(srv.Close)

	data, err := cometBlockDataSource(srv.URL)(context.Background(), 7)
	require.NoError(t, err)
	require.Empty(t, data)
}
