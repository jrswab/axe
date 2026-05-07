package runner

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/provider"
)

func TestOptionsMessagesField(t *testing.T) {
	opts := Options{
		AgentName: "test",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	}
	if len(opts.Messages) != 1 || opts.Messages[0].Content != "hello" {
		t.Errorf("Messages = %v", opts.Messages)
	}
}

func TestResultMessagesField(t *testing.T) {
	result := Result{
		Content: "ok",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
	}
	if len(result.Messages) != 1 || result.Messages[0].Role != "user" {
		t.Errorf("Messages = %v", result.Messages)
	}
}

func TestDryRunInfoMessageCount(t *testing.T) {
	info := DryRunInfo{
		MessageCount: 3,
	}
	if info.MessageCount != 3 {
		t.Errorf("MessageCount = %d, want 3", info.MessageCount)
	}
}

func TestMessageTypeCanBeConstructed(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "hello",
	}
	if msg.Role != "user" {
		t.Errorf("Role = %q, want %q", msg.Role, "user")
	}
	if msg.Content != "hello" {
		t.Errorf("Content = %q, want %q", msg.Content, "hello")
	}
}

func TestToProviderMessages(t *testing.T) {
	msgs := []Message{
		{
			Role:    "user",
			Content: "hello",
		},
		{
			Role:    "assistant",
			Content: "hi there",
			ToolCalls: []ToolCall{
				{ID: "call1", Name: "list_directory", Arguments: map[string]string{"path": "."}},
			},
		},
		{
			Role: "tool",
			ToolResults: []ToolResult{
				{CallID: "call1", Content: "file1\nfile2", IsError: false},
			},
		},
	}

	got := toProviderMessages(msgs)
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}

	if got[0].Role != "user" || got[0].Content != "hello" {
		t.Errorf("msg[0] = %+v, want user/hello", got[0])
	}

	if got[1].Role != "assistant" || got[1].Content != "hi there" || len(got[1].ToolCalls) != 1 {
		t.Errorf("msg[1] = %+v, want assistant/hi there with 1 tool call", got[1])
	}
	if got[1].ToolCalls[0].ID != "call1" || got[1].ToolCalls[0].Name != "list_directory" {
		t.Errorf("tool call = %+v", got[1].ToolCalls[0])
	}
	if !reflect.DeepEqual(got[1].ToolCalls[0].Arguments, map[string]string{"path": "."}) {
		t.Errorf("tool call args = %v", got[1].ToolCalls[0].Arguments)
	}

	if got[2].Role != "tool" || len(got[2].ToolResults) != 1 {
		t.Errorf("msg[2] = %+v, want tool with 1 result", got[2])
	}
	if got[2].ToolResults[0].CallID != "call1" || got[2].ToolResults[0].Content != "file1\nfile2" {
		t.Errorf("tool result = %+v", got[2].ToolResults[0])
	}
}

func TestFromProviderMessage(t *testing.T) {
	pm := provider.Message{
		Role:    "assistant",
		Content: "result",
		ToolCalls: []provider.ToolCall{
			{ID: "tc1", Name: "read_file", Arguments: map[string]string{"path": "/tmp/a"}},
		},
	}

	got := fromProviderMessage(pm)
	if got.Role != "assistant" || got.Content != "result" {
		t.Errorf("got = %+v", got)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].ID != "tc1" {
		t.Errorf("tool calls = %+v", got.ToolCalls)
	}
}

func TestRoundTripConversion(t *testing.T) {
	original := []Message{
		{Role: "user", Content: "step 1"},
		{
			Role:    "assistant",
			Content: "step 2",
			ToolCalls: []ToolCall{
				{ID: "a", Name: "tool", Arguments: map[string]string{"k": "v"}},
			},
		},
		{
			Role: "tool",
			ToolResults: []ToolResult{
				{CallID: "a", Content: "ok", IsError: true},
			},
		},
	}

	provMsgs := toProviderMessages(original)
	// Check the first user message survives
	if len(provMsgs) != 3 {
		t.Fatalf("provider messages length = %d, want 3", len(provMsgs))
	}
	// Verify the ToolCalls round trip properly
	if !reflect.DeepEqual(provMsgs[1].ToolCalls[0].Arguments, map[string]string{"k": "v"}) {
		t.Errorf("args = %v", provMsgs[1].ToolCalls[0].Arguments)
	}
}

func TestValidateMessages_Valid(t *testing.T) {
	cases := []struct {
		name string
		msgs []Message
	}{
		{"empty", []Message{}},
		{"single user", []Message{{Role: "user", Content: "hello"}}},
		{"user + assistant", []Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		}},
		{"with tool call chain", []Message{
			{Role: "user", Content: "list files"},
			{Role: "assistant", Content: "", ToolCalls: []ToolCall{{ID: "a", Name: "list_directory", Arguments: map[string]string{"path": "."}}}},
			{Role: "tool", ToolResults: []ToolResult{{CallID: "a", Content: "ok"}}},
			{Role: "assistant", Content: "done"},
		}},
		{"tool with empty content and no toolcalls", []Message{
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "a", Name: "n"}}},
			{Role: "tool", Content: "", ToolResults: []ToolResult{{CallID: "a", Content: "x"}}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateMessages(tc.msgs); err != nil {
				t.Fatalf("validateMessages() = %v, want nil", err)
			}
		})
	}
}

func TestValidateMessages_InvalidRole(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "hello"},
		{Role: "system", Content: "bad"},
	}
	err := validateMessages(msgs)
	if err == nil {
		t.Fatal("expected error for invalid role")
	}
	if !IsConfigError(err) {
		t.Fatalf("expected ConfigError, got %T", err)
	}
	if !contains(err.Error(), `"system"`) {
		t.Errorf("error should mention invalid role, got: %v", err)
	}
	if !contains(err.Error(), "index 1") {
		t.Errorf("error should mention index 1, got: %v", err)
	}
}

func TestValidateMessages_AssistantWithToolResults(t *testing.T) {
	msgs := []Message{
		{Role: "assistant", Content: "hi", ToolResults: []ToolResult{{CallID: "a", Content: "x"}}},
	}
	err := validateMessages(msgs)
	if err == nil {
		t.Fatal("expected error for assistant with tool results")
	}
	if !IsConfigError(err) {
		t.Fatalf("expected ConfigError, got %T", err)
	}
}

func TestValidateMessages_ToolWithContent(t *testing.T) {
	msgs := []Message{
		{Role: "tool", Content: "bad", ToolResults: []ToolResult{{CallID: "a", Content: "x"}}},
	}
	err := validateMessages(msgs)
	if err == nil {
		t.Fatal("expected error for tool with content")
	}
	if !IsConfigError(err) {
		t.Fatalf("expected ConfigError, got %T", err)
	}
}

func TestValidateMessages_ToolWithToolCalls(t *testing.T) {
	msgs := []Message{
		{Role: "tool", ToolCalls: []ToolCall{{ID: "a", Name: "n"}}, ToolResults: []ToolResult{{CallID: "a", Content: "x"}}},
	}
	err := validateMessages(msgs)
	if err == nil {
		t.Fatal("expected error for tool with tool calls")
	}
	if !IsConfigError(err) {
		t.Fatalf("expected ConfigError, got %T", err)
	}
}

func TestValidateMessages_UserWithToolCalls(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "hello", ToolCalls: []ToolCall{{ID: "a", Name: "n"}}},
	}
	err := validateMessages(msgs)
	if err == nil {
		t.Fatal("expected error for user with tool calls")
	}
	if !IsConfigError(err) {
		t.Fatalf("expected ConfigError, got %T", err)
	}
}

func TestValidateMessages_UserWithToolResults(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "hello", ToolResults: []ToolResult{{CallID: "a", Content: "x"}}},
	}
	err := validateMessages(msgs)
	if err == nil {
		t.Fatal("expected error for user with tool results")
	}
	if !IsConfigError(err) {
		t.Fatalf("expected ConfigError, got %T", err)
	}
}

func TestValidateMessages_NoPrecedingAssistant(t *testing.T) {
	msgs := []Message{
		{Role: "tool", ToolResults: []ToolResult{{CallID: "a", Content: "x"}}},
	}
	err := validateMessages(msgs)
	if err == nil {
		t.Fatal("expected error for tool message with no preceding assistant")
	}
	if !IsConfigError(err) {
		t.Fatalf("expected ConfigError, got %T", err)
	}
	if !contains(err.Error(), "no preceding assistant message with tool calls") {
		t.Errorf("error should mention no preceding assistant, got: %v", err)
	}
}

func TestValidateMessages_DanglingCallID(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "list"},
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "a", Name: "n"}}},
		{Role: "tool", ToolResults: []ToolResult{{CallID: "b", Content: "x"}}},
	}
	err := validateMessages(msgs)
	if err == nil {
		t.Fatal("expected error for dangling call ID")
	}
	if !IsConfigError(err) {
		t.Fatalf("expected ConfigError, got %T", err)
	}
	if !contains(err.Error(), "b") {
		t.Errorf("error should mention unmatched call ID b, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
