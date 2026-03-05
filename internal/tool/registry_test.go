package tool

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/toolname"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}
}

func TestRegistry_Register_And_Has(t *testing.T) {
	r := NewRegistry()

	// Has returns false before registration
	if r.Has("test_tool") {
		t.Fatal("Has returned true for unregistered tool")
	}

	// Register a tool
	r.Register("test_tool", ToolEntry{
		Definition: func() provider.Tool {
			return provider.Tool{Name: "test_tool"}
		},
	})

	// Has returns true after registration
	if !r.Has("test_tool") {
		t.Fatal("Has returned false for registered tool")
	}

	// Has returns false for a different name
	if r.Has("other_tool") {
		t.Fatal("Has returned true for unregistered tool")
	}

	// Register same name again — silent replacement
	r.Register("test_tool", ToolEntry{
		Definition: func() provider.Tool {
			return provider.Tool{Name: "test_tool", Description: "replaced"}
		},
	})
	tools, err := r.Resolve([]string{"test_tool"})
	if err != nil {
		t.Fatalf("Resolve after replacement returned error: %v", err)
	}
	if tools[0].Description != "replaced" {
		t.Errorf("expected replaced description, got %q", tools[0].Description)
	}
}

func TestRegistry_Resolve_KnownTools(t *testing.T) {
	r := NewRegistry()
	r.Register("tool_a", ToolEntry{
		Definition: func() provider.Tool {
			return provider.Tool{Name: "tool_a", Description: "Tool A"}
		},
	})
	r.Register("tool_b", ToolEntry{
		Definition: func() provider.Tool {
			return provider.Tool{Name: "tool_b", Description: "Tool B"}
		},
	})

	tools, err := r.Resolve([]string{"tool_a", "tool_b"})
	if err != nil {
		t.Fatalf("Resolve returned unexpected error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "tool_a" {
		t.Errorf("expected tools[0].Name = %q, got %q", "tool_a", tools[0].Name)
	}
	if tools[1].Name != "tool_b" {
		t.Errorf("expected tools[1].Name = %q, got %q", "tool_b", tools[1].Name)
	}
}

func TestRegistry_Resolve_UnknownTool(t *testing.T) {
	r := NewRegistry()
	r.Register("tool_a", ToolEntry{
		Definition: func() provider.Tool {
			return provider.Tool{Name: "tool_a"}
		},
	})

	_, err := r.Resolve([]string{"tool_a", "nonexistent"})
	if err == nil {
		t.Fatal("Resolve should return error for unknown tool")
	}
	if !strings.Contains(err.Error(), `unknown tool "nonexistent"`) {
		t.Errorf("error should contain unknown tool message, got: %v", err)
	}
}

func TestRegistry_Resolve_Empty(t *testing.T) {
	r := NewRegistry()

	// Empty slice
	tools, err := r.Resolve([]string{})
	if err != nil {
		t.Fatalf("Resolve([]string{}) returned error: %v", err)
	}
	if tools == nil {
		t.Fatal("Resolve([]string{}) returned nil slice, expected empty non-nil")
	}
	if len(tools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(tools))
	}

	// Nil input
	tools, err = r.Resolve(nil)
	if err != nil {
		t.Fatalf("Resolve(nil) returned error: %v", err)
	}
	if tools == nil {
		t.Fatal("Resolve(nil) returned nil slice, expected empty non-nil")
	}
	if len(tools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(tools))
	}
}

func TestRegistry_Resolve_NilDefinition(t *testing.T) {
	r := NewRegistry()
	r.Register("bad_tool", ToolEntry{
		Definition: nil,
		Execute: func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult {
			return provider.ToolResult{}
		},
	})

	_, err := r.Resolve([]string{"bad_tool"})
	if err == nil {
		t.Fatal("Resolve should return error for nil definition")
	}
	if !strings.Contains(err.Error(), `has no definition`) {
		t.Errorf("error should contain 'has no definition', got: %v", err)
	}
}

func TestRegistry_Dispatch_KnownTool(t *testing.T) {
	r := NewRegistry()
	r.Register("echo_tool", ToolEntry{
		Definition: func() provider.Tool {
			return provider.Tool{Name: "echo_tool"}
		},
		Execute: func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult {
			return provider.ToolResult{
				CallID:  call.ID,
				Content: "echoed: " + call.Arguments["input"],
			}
		},
	})

	result, err := r.Dispatch(context.Background(), provider.ToolCall{
		ID:        "call-1",
		Name:      "echo_tool",
		Arguments: map[string]string{"input": "hello"},
	}, ExecContext{})

	if err != nil {
		t.Fatalf("Dispatch returned unexpected error: %v", err)
	}
	if result.CallID != "call-1" {
		t.Errorf("expected CallID %q, got %q", "call-1", result.CallID)
	}
	if result.Content != "echoed: hello" {
		t.Errorf("expected Content %q, got %q", "echoed: hello", result.Content)
	}
	if result.IsError {
		t.Error("expected IsError to be false")
	}
}

func TestRegistry_Dispatch_UnknownTool(t *testing.T) {
	r := NewRegistry()

	_, err := r.Dispatch(context.Background(), provider.ToolCall{
		ID:   "call-1",
		Name: "nonexistent",
	}, ExecContext{})

	if err == nil {
		t.Fatal("Dispatch should return error for unknown tool")
	}
	if !strings.Contains(err.Error(), `unknown tool "nonexistent"`) {
		t.Errorf("error should contain unknown tool message, got: %v", err)
	}
}

func TestRegistry_Dispatch_NilExecutor(t *testing.T) {
	r := NewRegistry()
	r.Register("nil_tool", ToolEntry{
		Definition: func() provider.Tool {
			return provider.Tool{Name: "nil_tool"}
		},
		Execute: nil,
	})

	_, err := r.Dispatch(context.Background(), provider.ToolCall{
		ID:   "call-1",
		Name: "nil_tool",
	}, ExecContext{})

	if err == nil {
		t.Fatal("Dispatch should return error for nil executor")
	}
	if !strings.Contains(err.Error(), `has no executor`) {
		t.Errorf("error should contain 'has no executor', got: %v", err)
	}
}

func TestRegistry_Dispatch_PassesExecContext(t *testing.T) {
	var capturedEC ExecContext

	r := NewRegistry()
	r.Register("capture_tool", ToolEntry{
		Definition: func() provider.Tool {
			return provider.Tool{Name: "capture_tool"}
		},
		Execute: func(ctx context.Context, call provider.ToolCall, ec ExecContext) provider.ToolResult {
			capturedEC = ec
			return provider.ToolResult{CallID: call.ID, Content: "ok"}
		},
	})

	stderrBuf := &bytes.Buffer{}
	ec := ExecContext{
		Workdir: "/tmp/test-workdir",
		Stderr:  stderrBuf,
		Verbose: true,
	}

	_, err := r.Dispatch(context.Background(), provider.ToolCall{
		ID:   "call-1",
		Name: "capture_tool",
	}, ec)

	if err != nil {
		t.Fatalf("Dispatch returned unexpected error: %v", err)
	}
	if capturedEC.Workdir != "/tmp/test-workdir" {
		t.Errorf("expected Workdir %q, got %q", "/tmp/test-workdir", capturedEC.Workdir)
	}
	if capturedEC.Stderr != stderrBuf {
		t.Error("expected Stderr to be the provided buffer")
	}
	if capturedEC.Verbose != true {
		t.Error("expected Verbose to be true")
	}
}

func TestRegisterAll_RegistersListDirectory(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	if !r.Has(toolname.ListDirectory) {
		t.Fatalf("Has(%q) returned false after RegisterAll", toolname.ListDirectory)
	}
}

func TestRegisterAll_ResolvesListDirectory(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	tools, err := r.Resolve([]string{toolname.ListDirectory})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != toolname.ListDirectory {
		t.Errorf("tool Name: got %q, want %q", tools[0].Name, toolname.ListDirectory)
	}
	if _, ok := tools[0].Parameters["path"]; !ok {
		t.Error("expected tool to have a 'path' parameter")
	}
}

func TestRegisterAll_RegistersReadFile(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	if !r.Has(toolname.ReadFile) {
		t.Fatalf("Has(%q) returned false after RegisterAll", toolname.ReadFile)
	}
}

func TestRegisterAll_ResolvesReadFile(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	tools, err := r.Resolve([]string{toolname.ReadFile})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != toolname.ReadFile {
		t.Errorf("tool Name: got %q, want %q", tools[0].Name, toolname.ReadFile)
	}
	for _, param := range []string{"path", "offset", "limit"} {
		if _, ok := tools[0].Parameters[param]; !ok {
			t.Errorf("expected tool to have a %q parameter", param)
		}
	}
}

func TestRegisterAll_RegistersWriteFile(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	if !r.Has(toolname.WriteFile) {
		t.Fatalf("Has(%q) returned false after RegisterAll", toolname.WriteFile)
	}
}

func TestRegisterAll_ResolvesWriteFile(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	tools, err := r.Resolve([]string{toolname.WriteFile})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != toolname.WriteFile {
		t.Errorf("tool Name: got %q, want %q", tools[0].Name, toolname.WriteFile)
	}
	for _, param := range []string{"path", "content"} {
		if _, ok := tools[0].Parameters[param]; !ok {
			t.Errorf("expected tool to have a %q parameter", param)
		}
	}
}

func TestRegisterAll_RegistersEditFile(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	if !r.Has(toolname.EditFile) {
		t.Fatalf("Has(%q) returned false after RegisterAll", toolname.EditFile)
	}
}

func TestRegisterAll_ResolvesEditFile(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	tools, err := r.Resolve([]string{toolname.EditFile})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != toolname.EditFile {
		t.Errorf("tool Name: got %q, want %q", tools[0].Name, toolname.EditFile)
	}
	for _, param := range []string{"path", "old_string", "new_string", "replace_all"} {
		if _, ok := tools[0].Parameters[param]; !ok {
			t.Errorf("expected tool to have a %q parameter", param)
		}
	}
}

func TestRegisterAll_RegistersRunCommand(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	if !r.Has(toolname.RunCommand) {
		t.Fatalf("Has(%q) returned false after RegisterAll", toolname.RunCommand)
	}
}

func TestRegisterAll_ResolvesRunCommand(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	tools, err := r.Resolve([]string{toolname.RunCommand})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != toolname.RunCommand {
		t.Errorf("tool Name: got %q, want %q", tools[0].Name, toolname.RunCommand)
	}
	if _, ok := tools[0].Parameters["command"]; !ok {
		t.Error("expected tool to have a 'command' parameter")
	}
}

func TestRegisterAll_RegistersURLFetch(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	if !r.Has(toolname.URLFetch) {
		t.Fatalf("Has(%q) returned false after RegisterAll", toolname.URLFetch)
	}
}

func TestRegisterAll_ResolvesURLFetch(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)

	tools, err := r.Resolve([]string{toolname.URLFetch})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != toolname.URLFetch {
		t.Errorf("tool Name: got %q, want %q", tools[0].Name, toolname.URLFetch)
	}
	if _, ok := tools[0].Parameters["url"]; !ok {
		t.Error("expected tool to have a 'url' parameter")
	}
}

func TestRegisterAll_Idempotent(t *testing.T) {
	r := NewRegistry()
	RegisterAll(r)
	RegisterAll(r) // second call should not panic

	if !r.Has(toolname.ListDirectory) {
		t.Fatalf("Has(%q) returned false after double RegisterAll", toolname.ListDirectory)
	}
}
