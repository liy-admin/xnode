package machine

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/cedar2025/xboard-node/internal/config"
)

// fakePanel serves machine/nodes and machine/auto-node. nodes starts empty and
// becomes [42] once auto-node "creates" it.
type fakePanel struct {
	autoNodeStatus int // 0 = create the node, otherwise reply with this status
	created        atomic.Bool
	nodeCalls      atomic.Int32
	autoCalls      atomic.Int32
}

func (f *fakePanel) handler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v2/server/machine/nodes":
		f.nodeCalls.Add(1)
		if f.created.Load() {
			w.Write([]byte(`{"nodes":[{"id":42,"type":"vless","name":"auto"}],"base_config":{"push_interval":60,"pull_interval":60}}`))
			return
		}
		w.Write([]byte(`{"nodes":[],"base_config":{"push_interval":60,"pull_interval":60}}`))
	case "/api/v2/server/machine/auto-node":
		f.autoCalls.Add(1)
		if f.autoNodeStatus != 0 {
			w.WriteHeader(f.autoNodeStatus)
			w.Write([]byte(`{"message":"nope"}`))
			return
		}
		f.created.Store(true)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"created":true,"node_ids":[42],"message":"Node created"}`))
	default:
		http.NotFound(w, r)
	}
}

func newTestOrchestrator(t *testing.T, f *fakePanel, autoNode *bool) *Orchestrator {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(ts.Close)
	return New(&config.Config{
		Panel:   config.PanelConfig{URL: ts.URL},
		Machine: &config.MachineConfig{MachineID: 4, Token: "machine-token", AutoNode: autoNode},
	})
}

func TestDiscoverNodes_AutoCreatesWhenEmpty(t *testing.T) {
	f := &fakePanel{}
	o := newTestOrchestrator(t, f, nil)

	resp, err := o.discoverNodes()
	if err != nil {
		t.Fatalf("discoverNodes: %v", err)
	}
	if len(resp.Nodes) != 1 || resp.Nodes[0].ID != 42 {
		t.Fatalf("expected the auto-created node after refetch, got %+v", resp.Nodes)
	}
	if f.autoCalls.Load() != 1 || f.nodeCalls.Load() != 2 {
		t.Errorf("expected 1 auto-node call and 2 node fetches, got %d / %d", f.autoCalls.Load(), f.nodeCalls.Load())
	}

	// Once the machine has nodes, auto-node must not be called again.
	if _, err := o.discoverNodes(); err != nil {
		t.Fatal(err)
	}
	if f.autoCalls.Load() != 1 {
		t.Errorf("auto-node called again although the machine has nodes")
	}
}

func TestDiscoverNodes_StopsWhenPanelUnsupported(t *testing.T) {
	f := &fakePanel{autoNodeStatus: http.StatusNotFound}
	o := newTestOrchestrator(t, f, nil)

	for i := 0; i < 3; i++ {
		resp, err := o.discoverNodes()
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Nodes) != 0 {
			t.Fatalf("expected no nodes, got %+v", resp.Nodes)
		}
	}
	if f.autoCalls.Load() != 1 {
		t.Errorf("expected a single probe on a panel without the plugin, got %d calls", f.autoCalls.Load())
	}
}

func TestDiscoverNodes_RetriesOnOtherErrors(t *testing.T) {
	f := &fakePanel{autoNodeStatus: http.StatusUnprocessableEntity}
	o := newTestOrchestrator(t, f, nil)

	for i := 0; i < 2; i++ {
		if _, err := o.discoverNodes(); err != nil {
			t.Fatal(err)
		}
	}
	if f.autoCalls.Load() != 2 {
		t.Errorf("a misconfigured plugin should be retried on each rediscovery, got %d calls", f.autoCalls.Load())
	}
}

func TestDiscoverNodes_DisabledInConfig(t *testing.T) {
	f := &fakePanel{}
	off := false
	o := newTestOrchestrator(t, f, &off)

	resp, err := o.discoverNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Nodes) != 0 || f.autoCalls.Load() != 0 {
		t.Errorf("auto_node: false must never call the panel (nodes=%d, calls=%d)", len(resp.Nodes), f.autoCalls.Load())
	}
}
