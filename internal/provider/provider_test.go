package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestProviderError_ErrorInterface(t *testing.T) {
	pe := &ProviderError{
		Category: ErrCategoryAuth,
		Status:   401,
		Message:  "invalid api key",
		Err:      nil,
	}

	// Must implement error interface
	var _ error = pe

	// Error() must return "<category>: <message>"
	got := pe.Error()
	want := "auth: invalid api key"
	if got != want {
		t.Errorf("ProviderError.Error() = %q, want %q", got, want)
	}
}

func TestProviderError_Unwrap(t *testing.T) {
	inner := errors.New("original cause")
	pe := &ProviderError{
		Category: ErrCategoryServer,
		Status:   500,
		Message:  "server error",
		Err:      inner,
	}

	// Wrap it further
	wrapped := fmt.Errorf("wrapper: %w", pe)

	// errors.As must extract ProviderError
	var extracted *ProviderError
	if !errors.As(wrapped, &extracted) {
		t.Fatal("errors.As failed to extract ProviderError from wrapped error")
	}
	if extracted.Category != ErrCategoryServer {
		t.Errorf("extracted Category = %q, want %q", extracted.Category, ErrCategoryServer)
	}

	// errors.Is must work through Unwrap chain
	if !errors.Is(pe, inner) {
		t.Error("errors.Is(pe, inner) = false, want true")
	}
}

// TestProviderInterface_Compile verifies the Provider interface compiles correctly.
// This is a compile-time check, not a runtime test.
func TestProviderInterface_Compile(t *testing.T) {
	// Verify the interface has the expected method signature
	var _ Provider = (*mockProvider)(nil)
}

// mockProvider is a minimal implementation for compile-time interface check.
type mockProvider struct{}

func (m *mockProvider) Send(ctx context.Context, req *Request) (*Response, error) {
	return nil, nil
}

// --- Phase 1a: New struct zero-value tests ---

func TestTool_ZeroValue(t *testing.T) {
	var tool Tool
	if tool.Name != "" {
		t.Errorf("Tool.Name = %q, want empty", tool.Name)
	}
	if tool.Description != "" {
		t.Errorf("Tool.Description = %q, want empty", tool.Description)
	}
	if tool.Parameters != nil {
		t.Errorf("Tool.Parameters = %v, want nil", tool.Parameters)
	}
}

func TestToolCall_ZeroValue(t *testing.T) {
	var tc ToolCall
	if tc.ID != "" {
		t.Errorf("ToolCall.ID = %q, want empty", tc.ID)
	}
	if tc.Name != "" {
		t.Errorf("ToolCall.Name = %q, want empty", tc.Name)
	}
	if tc.Arguments != nil {
		t.Errorf("ToolCall.Arguments = %v, want nil", tc.Arguments)
	}
}

func TestToolResult_ZeroValue(t *testing.T) {
	var tr ToolResult
	if tr.CallID != "" {
		t.Errorf("ToolResult.CallID = %q, want empty", tr.CallID)
	}
	if tr.Content != "" {
		t.Errorf("ToolResult.Content = %q, want empty", tr.Content)
	}
	if tr.IsError {
		t.Error("ToolResult.IsError = true, want false")
	}
}

// --- Phase 1b: Extended struct tests ---

func TestRequest_NilTools_BackwardsCompatible(t *testing.T) {
	req := &Request{
		Model:    "anthropic/claude-sonnet-4-20250514",
		System:   "test",
		Messages: []Message{{Role: "user", Content: "hello"}},
		Tools:    nil,
	}
	if req.Tools != nil {
		t.Error("Request.Tools should be nil when not set")
	}
	// Verify all existing fields still accessible
	if req.Model != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("Request.Model = %q, want %q", req.Model, "anthropic/claude-sonnet-4-20250514")
	}
}

func TestResponse_NilToolCalls_BackwardsCompatible(t *testing.T) {
	resp := &Response{
		Content:      "hello",
		Model:        "claude-sonnet-4-20250514",
		InputTokens:  10,
		OutputTokens: 5,
		StopReason:   "end_turn",
	}
	if len(resp.ToolCalls) != 0 {
		t.Errorf("len(Response.ToolCalls) = %d, want 0", len(resp.ToolCalls))
	}
	// Verify all existing fields still accessible
	if resp.Content != "hello" {
		t.Errorf("Response.Content = %q, want %q", resp.Content, "hello")
	}
}

func TestMessage_ToolCallsField(t *testing.T) {
	msg := Message{
		Role:    "assistant",
		Content: "I'll call a tool",
		ToolCalls: []ToolCall{
			{ID: "tc_1", Name: "call_agent", Arguments: map[string]string{"agent": "helper", "task": "do stuff"}},
		},
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("len(Message.ToolCalls) = %d, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "tc_1" {
		t.Errorf("ToolCalls[0].ID = %q, want %q", msg.ToolCalls[0].ID, "tc_1")
	}
	if msg.ToolCalls[0].Name != "call_agent" {
		t.Errorf("ToolCalls[0].Name = %q, want %q", msg.ToolCalls[0].Name, "call_agent")
	}
	if msg.ToolCalls[0].Arguments["agent"] != "helper" {
		t.Errorf("ToolCalls[0].Arguments[agent] = %q, want %q", msg.ToolCalls[0].Arguments["agent"], "helper")
	}
}

func TestMessage_ToolResultsField(t *testing.T) {
	msg := Message{
		Role: "tool",
		ToolResults: []ToolResult{
			{CallID: "tc_1", Content: "result text", IsError: false},
			{CallID: "tc_2", Content: "error occurred", IsError: true},
		},
	}
	if len(msg.ToolResults) != 2 {
		t.Fatalf("len(Message.ToolResults) = %d, want 2", len(msg.ToolResults))
	}
	if msg.ToolResults[0].CallID != "tc_1" {
		t.Errorf("ToolResults[0].CallID = %q, want %q", msg.ToolResults[0].CallID, "tc_1")
	}
	if msg.ToolResults[0].Content != "result text" {
		t.Errorf("ToolResults[0].Content = %q, want %q", msg.ToolResults[0].Content, "result text")
	}
	if msg.ToolResults[1].IsError != true {
		t.Error("ToolResults[1].IsError = false, want true")
	}
}
