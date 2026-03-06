package mcpclient

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jrswab/axe/internal/provider"
)

func TestRouter_RegisterAndDispatch(t *testing.T) {
	ts := newMCPStreamableServer(t)
	client := mustConnectTestClient(t, ts.URL)
	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	r := NewRouter()
	filtered, err := r.Register(client, tools, map[string]bool{})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if len(filtered) != len(tools) {
		t.Fatalf("filtered len = %d, want %d", len(filtered), len(tools))
	}

	result, err := r.Dispatch(context.Background(), provider.ToolCall{ID: "1", Name: "success"})
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}
	if result.Content != "ok" {
		t.Fatalf("content = %q, want ok", result.Content)
	}
}

func TestRouter_Has_UnknownTool(t *testing.T) {
	r := NewRouter()
	if r.Has("missing") {
		t.Fatal("expected Has to return false")
	}
}

func TestRouter_Dispatch_UnknownTool(t *testing.T) {
	r := NewRouter()
	_, err := r.Dispatch(context.Background(), provider.ToolCall{ID: "1", Name: "missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unknown MCP tool`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRouter_Register_SkipsBuiltins(t *testing.T) {
	ts := newMCPStreamableServer(t)
	client := mustConnectTestClient(t, ts.URL)
	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	r := NewRouter()
	filtered, err := r.Register(client, tools, map[string]bool{"success": true})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if len(filtered) != len(tools)-1 {
		t.Fatalf("filtered len = %d, want %d", len(filtered), len(tools)-1)
	}
	if r.Has("success") {
		t.Fatal("expected built-in tool to be skipped")
	}
}

func TestRouter_Register_MCPCollision(t *testing.T) {
	ts1 := newMCPStreamableServer(t)
	client1, err := Connect(context.Background(), agentTestConfig("one", ts1.URL))
	if err != nil {
		t.Fatalf("Connect client1 failed: %v", err)
	}
	t.Cleanup(func() { _ = client1.Close() })
	tools1, err := client1.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools client1 failed: %v", err)
	}

	ts2 := newMCPStreamableServer(t)
	client2, err := Connect(context.Background(), agentTestConfig("two", ts2.URL))
	if err != nil {
		t.Fatalf("Connect client2 failed: %v", err)
	}
	t.Cleanup(func() { _ = client2.Close() })
	tools2, err := client2.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools client2 failed: %v", err)
	}

	r := NewRouter()
	if _, err := r.Register(client1, tools1, map[string]bool{}); err != nil {
		t.Fatalf("Register client1 failed: %v", err)
	}
	_, err = r.Register(client2, tools2, map[string]bool{})
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), `collision`) || !strings.Contains(err.Error(), `"one"`) || !strings.Contains(err.Error(), `"two"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRouter_Close_ClosesAllClients(t *testing.T) {
	r := NewRouter()
	var closed int32
	clientA := &Client{name: "a", closeFn: func() error { atomic.AddInt32(&closed, 1); return nil }}
	clientB := &Client{name: "b", closeFn: func() error { atomic.AddInt32(&closed, 1); return nil }}
	r.tools["a"] = clientA
	r.tools["b"] = clientB
	r.clients = append(r.clients, clientA, clientB)

	if err := r.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if got := atomic.LoadInt32(&closed); got != 2 {
		t.Fatalf("close count = %d, want 2", got)
	}
}

func TestRouter_Close_DeduplicatesClients(t *testing.T) {
	r := NewRouter()
	var closed int32
	shared := &Client{name: "shared", closeFn: func() error { atomic.AddInt32(&closed, 1); return nil }}
	r.tools["a"] = shared
	r.tools["b"] = shared
	r.clients = append(r.clients, shared, shared)

	if err := r.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if got := atomic.LoadInt32(&closed); got != 1 {
		t.Fatalf("close count = %d, want 1", got)
	}
}

func TestRouter_Close_ClosesZeroToolClients(t *testing.T) {
	r := NewRouter()
	var closed int32
	zeroToolClient := &Client{name: "zero", closeFn: func() error {
		atomic.AddInt32(&closed, 1)
		return nil
	}}

	// Register with all tools filtered out (all are builtins).
	_, err := r.Register(zeroToolClient, []provider.Tool{{Name: "builtin_a"}}, map[string]bool{"builtin_a": true})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if got := atomic.LoadInt32(&closed); got != 1 {
		t.Fatalf("close count = %d, want 1 (zero-tool client not closed)", got)
	}
}

func TestRouter_Close_ReturnsFirstError(t *testing.T) {
	r := NewRouter()
	errBoom := errors.New("boom")
	clientA := &Client{name: "a", closeFn: func() error { return errBoom }}
	clientB := &Client{name: "b", closeFn: func() error { return nil }}
	r.tools["a"] = clientA
	r.tools["b"] = clientB
	r.clients = append(r.clients, clientA, clientB)

	err := r.Close()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errBoom) {
		t.Fatalf("got %v, want %v", err, errBoom)
	}
}
