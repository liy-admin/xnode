package panel

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cedar2025/xboard-node/internal/config"
)

func newMachineTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return NewClient(config.PanelConfig{URL: ts.URL, Token: "machine-token", MachineID: 4})
}

func TestEnsureMachineNode_Created(t *testing.T) {
	client := newMachineTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/server/machine/auto-node" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["token"] != "machine-token" || body["machine_id"] != float64(4) {
			t.Errorf("missing machine auth in body: %v", body)
		}
		if _, hasNode := body["node_id"]; hasNode {
			t.Errorf("machine-level request must not carry node_id: %v", body)
		}
		if body["host"] != "203.0.113.7" {
			t.Errorf("host not forwarded: %v", body["host"])
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"created":true,"node_ids":[12],"message":"Node created"}`))
	})

	res, err := client.EnsureMachineNode("203.0.113.7")
	if err != nil {
		t.Fatalf("EnsureMachineNode: %v", err)
	}
	if !res.Created || len(res.NodeIDs) != 1 || res.NodeIDs[0] != 12 {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestEnsureMachineNode_OmitsEmptyHost(t *testing.T) {
	client := newMachineTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["host"]; ok {
			t.Errorf("empty host should be omitted so the panel uses the source IP: %v", body)
		}
		w.Write([]byte(`{"created":false,"node_ids":[],"message":"Auto creation is disabled in plugin settings"}`))
	})

	res, err := client.EnsureMachineNode("")
	if err != nil {
		t.Fatalf("EnsureMachineNode: %v", err)
	}
	if res.Created || !strings.Contains(res.Message, "disabled") {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestEnsureMachineNode_Unsupported(t *testing.T) {
	client := newMachineTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	if _, err := client.EnsureMachineNode(""); !errors.Is(err, ErrAutoNodeUnsupported) {
		t.Errorf("expected ErrAutoNodeUnsupported, got %v", err)
	}
}

func TestEnsureMachineNode_ErrorMessage(t *testing.T) {
	client := newMachineTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"Plugin setting group_ids is empty"}`))
	})

	_, err := client.EnsureMachineNode("")
	if err == nil || !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "group_ids") {
		t.Errorf("expected 422 error carrying the panel message, got %v", err)
	}
}
