package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/internal/testutil"
)

// nopCloser is a no-op io.Closer for test EventStreams.
type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// makeStream creates an EventStream from a slice of events.
// After all events are consumed, it returns io.EOF.
func makeStream(events []provider.StreamEvent) *provider.EventStream {
	i := 0
	return provider.NewEventStream(nopCloser{}, func() (provider.StreamEvent, error) {
		if i >= len(events) {
			return provider.StreamEvent{}, io.EOF
		}
		ev := events[i]
		i++
		return ev, nil
	})
}

// errStream creates an EventStream that emits the given events,
// then returns the specified error.
func errStream(events []provider.StreamEvent, err error) *provider.EventStream {
	i := 0
	return provider.NewEventStream(nopCloser{}, func() (provider.StreamEvent, error) {
		if i >= len(events) {
			return provider.StreamEvent{}, err
		}
		ev := events[i]
		i++
		return ev, nil
	})
}

func TestDrainEventStream_TextOnly(t *testing.T) {
	stream := makeStream([]provider.StreamEvent{
		{Type: provider.StreamEventText, Text: "Hello "},
		{Type: provider.StreamEventText, Text: "world"},
		{Type: provider.StreamEventDone, InputTokens: 10, OutputTokens: 5, StopReason: "end_turn"},
	})

	resp, err := drainEventStream(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello world" {
		t.Errorf("Content = %q, want %q", resp.Content, "Hello world")
	}
	if resp.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", resp.OutputTokens)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", resp.StopReason, "end_turn")
	}
}

func TestDrainEventStream_ToolCalls(t *testing.T) {
	stream := makeStream([]provider.StreamEvent{
		{Type: provider.StreamEventToolStart, ToolCallID: "call_1", ToolName: "read_file"},
		{Type: provider.StreamEventToolDelta, ToolCallID: "call_1", ToolInput: `{"path":`},
		{Type: provider.StreamEventToolDelta, ToolCallID: "call_1", ToolInput: `"foo.txt"}`},
		{Type: provider.StreamEventToolEnd, ToolCallID: "call_1"},
		{Type: provider.StreamEventToolStart, ToolCallID: "call_2", ToolName: "list_directory"},
		{Type: provider.StreamEventToolDelta, ToolCallID: "call_2", ToolInput: `{"path":"."}`},
		{Type: provider.StreamEventToolEnd, ToolCallID: "call_2"},
		{Type: provider.StreamEventDone, StopReason: "tool_calls"},
	})

	resp, err := drainEventStream(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("got %d tool calls, want 2", len(resp.ToolCalls))
	}

	tc1 := resp.ToolCalls[0]
	if tc1.ID != "call_1" || tc1.Name != "read_file" {
		t.Errorf("tool call 0: ID=%q Name=%q", tc1.ID, tc1.Name)
	}
	if tc1.Arguments["path"] != "foo.txt" {
		t.Errorf("tool call 0 args = %v, want path=foo.txt", tc1.Arguments)
	}

	tc2 := resp.ToolCalls[1]
	if tc2.ID != "call_2" || tc2.Name != "list_directory" {
		t.Errorf("tool call 1: ID=%q Name=%q", tc2.ID, tc2.Name)
	}
	if tc2.Arguments["path"] != "." {
		t.Errorf("tool call 1 args = %v, want path=.", tc2.Arguments)
	}
}

func TestDrainEventStream_ToolCallsFromStartInput(t *testing.T) {
	stream := makeStream([]provider.StreamEvent{
		{Type: provider.StreamEventToolStart, ToolCallID: "ollama_0", ToolName: "read_file", ToolInput: `{"path":"main.go"}`},
		{Type: provider.StreamEventToolEnd, ToolCallID: "ollama_0"},
		{Type: provider.StreamEventDone, StopReason: "tool_calls"},
	})

	resp, err := drainEventStream(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(resp.ToolCalls))
	}

	tc := resp.ToolCalls[0]
	if tc.ID != "ollama_0" || tc.Name != "read_file" {
		t.Errorf("tool call: ID=%q Name=%q", tc.ID, tc.Name)
	}
	if tc.Arguments["path"] != "main.go" {
		t.Errorf("tool call args = %v, want path=main.go", tc.Arguments)
	}
}

func TestDrainEventStream_TextAndToolCalls(t *testing.T) {
	stream := makeStream([]provider.StreamEvent{
		{Type: provider.StreamEventText, Text: "Let me help."},
		{Type: provider.StreamEventToolStart, ToolCallID: "call_1", ToolName: "read_file"},
		{Type: provider.StreamEventToolDelta, ToolCallID: "call_1", ToolInput: `{"path":"a.go"}`},
		{Type: provider.StreamEventToolEnd, ToolCallID: "call_1"},
		{Type: provider.StreamEventDone, StopReason: "tool_calls", InputTokens: 20, OutputTokens: 15},
	})

	resp, err := drainEventStream(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Let me help." {
		t.Errorf("Content = %q, want %q", resp.Content, "Let me help.")
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(resp.ToolCalls))
	}
}

func TestDrainEventStream_IncrementalWrite(t *testing.T) {
	stream := makeStream([]provider.StreamEvent{
		{Type: provider.StreamEventText, Text: "one"},
		{Type: provider.StreamEventText, Text: "two"},
		{Type: provider.StreamEventText, Text: "three"},
		{Type: provider.StreamEventDone, StopReason: "end_turn"},
	})

	var buf bytes.Buffer
	resp, err := drainEventStream(stream, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "onetwothree" {
		t.Errorf("writer got %q, want %q", buf.String(), "onetwothree")
	}
	if resp.Content != "onetwothree" {
		t.Errorf("Content = %q, want %q", resp.Content, "onetwothree")
	}
}

func TestDrainEventStream_NilWriter(t *testing.T) {
	stream := makeStream([]provider.StreamEvent{
		{Type: provider.StreamEventText, Text: "buffered"},
		{Type: provider.StreamEventDone, StopReason: "end_turn"},
	})

	resp, err := drainEventStream(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "buffered" {
		t.Errorf("Content = %q, want %q", resp.Content, "buffered")
	}
}

func TestDrainEventStream_MidStreamError(t *testing.T) {
	testErr := errors.New("connection reset")
	stream := errStream([]provider.StreamEvent{
		{Type: provider.StreamEventText, Text: "partial"},
		{Type: provider.StreamEventText, Text: " data"},
	}, testErr)

	_, err := drainEventStream(stream, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, testErr) {
		t.Errorf("got error %v, want %v", err, testErr)
	}
}

func TestDrainEventStream_EmptyStream(t *testing.T) {
	stream := makeStream([]provider.StreamEvent{
		{Type: provider.StreamEventDone, StopReason: "end_turn", InputTokens: 3, OutputTokens: 1},
	})

	resp, err := drainEventStream(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "" {
		t.Errorf("Content = %q, want empty", resp.Content)
	}
	if resp.ToolCalls != nil {
		t.Errorf("ToolCalls = %v, want nil", resp.ToolCalls)
	}
	if resp.InputTokens != 3 {
		t.Errorf("InputTokens = %d, want 3", resp.InputTokens)
	}
}

func TestDrainEventStream_EOFWithoutDone(t *testing.T) {
	stream := makeStream([]provider.StreamEvent{
		{Type: provider.StreamEventText, Text: "hello"},
	})

	resp, err := drainEventStream(stream, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello" {
		t.Errorf("Content = %q, want %q", resp.Content, "hello")
	}
	if resp.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", resp.InputTokens)
	}
	if resp.OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0", resp.OutputTokens)
	}
	if resp.StopReason != "" {
		t.Errorf("StopReason = %q, want empty", resp.StopReason)
	}
}

// --- Integration Tests ---

func TestIntegration_Streaming_SingleShot_OpenAI(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIStreamResponse("Hello streamed", 10, 5),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "stream-single", `name = "stream-single"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "stream-single", "--stream"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nstderr: %s", err, errBuf.String())
	}

	output := buf.String()
	if output != "Hello streamed" {
		t.Errorf("expected stdout %q, got %q", "Hello streamed", output)
	}
}

func TestIntegration_Streaming_JSON_Buffers(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIStreamResponse("buffered text", 10, 5),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "stream-json", `name = "stream-json"
model = "openai/gpt-4o"
`)

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "stream-json", "--stream", "--json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nstderr: %s", err, errBuf.String())
	}

	output := buf.String()
	var envelope map[string]interface{}
	if jsonErr := json.Unmarshal([]byte(strings.TrimSpace(output)), &envelope); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", jsonErr, output)
	}

	content, _ := envelope["content"].(string)
	if content != "buffered text" {
		t.Errorf("JSON content = %q, want %q", content, "buffered text")
	}
}

func TestIntegration_Streaming_SingleShot_Anthropic(t *testing.T) {
	resetRunCmd(t)

	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.AnthropicStreamResponse("Fallback works", 10, 5),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	writeAgentConfig(t, configDir, "stream-anthropic", `name = "stream-anthropic"
model = "anthropic/claude-sonnet-4-20250514"
`)

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("AXE_ANTHROPIC_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "stream-anthropic", "--stream"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nstderr: %s", err, errBuf.String())
	}

	output := buf.String()
	if output != "Fallback works" {
		t.Errorf("expected stdout %q, got %q", "Fallback works", output)
	}
}

func TestIntegration_Streaming_WithTools(t *testing.T) {
	resetRunCmd(t)

	// First response: tool call via streaming
	// Second response: final text (non-streaming is fine since loop checks each turn)
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.OpenAIStreamToolCallResponse("", []testutil.MockToolCall{
			{ID: "call_1", Name: "list_directory", Input: map[string]string{"path": "."}},
		}, 10, 15),
		testutil.OpenAIStreamResponse("Done listing", 5, 3),
	})

	configDir, _ := testutil.SetupXDGDirs(t)
	workdir := t.TempDir()
	// Create a file so list_directory has something to find
	if err := os.WriteFile(workdir+"/test.txt", []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	writeAgentConfig(t, configDir, "stream-tools", fmt.Sprintf(`name = "stream-tools"
model = "openai/gpt-4o"
tools = ["list_directory"]
workdir = %q
`, workdir))

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("AXE_OPENAI_BASE_URL", mock.URL())

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(errBuf)
	rootCmd.SetArgs([]string{"run", "stream-tools", "--stream"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error, got: %v\nstderr: %s", err, errBuf.String())
	}

	output := buf.String()
	if !strings.Contains(output, "Done listing") {
		t.Errorf("expected stdout to contain %q, got %q", "Done listing", output)
	}

	if mock.RequestCount() != 2 {
		t.Errorf("expected 2 requests (tool call + final), got %d", mock.RequestCount())
	}
}
