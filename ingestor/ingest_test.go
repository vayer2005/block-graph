package ingestor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlanQueue(t *testing.T) {
	got, err := PlanQueue(900_000, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{899_998, 899_999, 900_000}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestRunWorkerPool(t *testing.T) {
	// Minimal blockchain.info-like stub: /latestblock, /block-height, /rawblock
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/latestblock":
			_, _ = w.Write([]byte(`{"height":100,"hash":"abcd"}`))
		case strings.HasPrefix(r.URL.Path, "/api/block-height/"):
			_, _ = w.Write([]byte(`{"blocks":[{"hash":"abcd","height":100}]}`))
		case strings.HasPrefix(r.URL.Path, "/api/rawblock/"):
			blk := Block{Hash: "abcd", Height: 100, Tx: []json.RawMessage{json.RawMessage(`{}`)}}
			_ = json.NewEncoder(w).Encode(blk)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient()
	c.BaseURL = srv.URL + "/api"

	res, err := Run(context.Background(), c, Config{BlockCount: 10, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("errors: %+v", res.Errors)
	}
	fb, ok := res.ByHeight[100]
	if !ok || len(fb.Block.Tx) != 1 {
		t.Fatalf("unexpected result: %+v", fb)
	}
}
