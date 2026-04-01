package provider

import (
	"io"
	"testing"
)

func TestEventStream_NextReturnsEventsInOrderThenEOF(t *testing.T) {
	t.Parallel()

	events := []StreamEvent{
		{Type: StreamEventText, Text: "hello"},
		{Type: StreamEventText, Text: " world"},
		{Type: StreamEventDone, InputTokens: 10, OutputTokens: 5, StopReason: "end_turn"},
	}

	idx := 0
	pr, pw := io.Pipe()
	go func() { _ = pw.Close() }()

	stream := NewEventStream(pr, func() (StreamEvent, error) {
		if idx >= len(events) {
			return StreamEvent{}, io.EOF
		}
		e := events[idx]
		idx++
		return e, nil
	})
	defer func() { _ = stream.Close() }()

	for i, want := range events {
		got, err := stream.Next()
		if err != nil {
			t.Fatalf("event %d: unexpected error: %v", i, err)
		}
		if got.Type != want.Type {
			t.Errorf("event %d: got type %q, want %q", i, got.Type, want.Type)
		}
		if got.Text != want.Text {
			t.Errorf("event %d: got text %q, want %q", i, got.Text, want.Text)
		}
		if got.InputTokens != want.InputTokens {
			t.Errorf("event %d: got input_tokens %d, want %d", i, got.InputTokens, want.InputTokens)
		}
		if got.OutputTokens != want.OutputTokens {
			t.Errorf("event %d: got output_tokens %d, want %d", i, got.OutputTokens, want.OutputTokens)
		}
		if got.StopReason != want.StopReason {
			t.Errorf("event %d: got stop_reason %q, want %q", i, got.StopReason, want.StopReason)
		}
	}

	_, err := stream.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF after all events, got %v", err)
	}
}

func TestEventStream_CloseIsIdempotent(t *testing.T) {
	t.Parallel()

	pr, pw := io.Pipe()
	go func() { _ = pw.Close() }()

	stream := NewEventStream(pr, func() (StreamEvent, error) {
		return StreamEvent{}, io.EOF
	})

	if err := stream.Close(); err != nil {
		t.Fatalf("first Close: unexpected error: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second Close: unexpected error: %v", err)
	}
}

func TestEventStream_NextAfterCloseReturnsEOF(t *testing.T) {
	t.Parallel()

	pr, pw := io.Pipe()
	go func() { _ = pw.Close() }()

	called := false
	stream := NewEventStream(pr, func() (StreamEvent, error) {
		called = true
		return StreamEvent{Type: StreamEventText, Text: "should not see"}, nil
	})

	if err := stream.Close(); err != nil {
		t.Fatalf("Close: unexpected error: %v", err)
	}

	_, err := stream.Next()
	if err != io.EOF {
		t.Fatalf("expected io.EOF after Close, got %v", err)
	}
	if called {
		t.Error("nextFunc should not be called after Close")
	}
}

func TestEventStream_ToolCallEvents(t *testing.T) {
	t.Parallel()

	events := []StreamEvent{
		{Type: StreamEventToolStart, ToolCallID: "tc_1", ToolName: "read_file"},
		{Type: StreamEventToolDelta, ToolCallID: "tc_1", ToolInput: `{"path":"`},
		{Type: StreamEventToolDelta, ToolCallID: "tc_1", ToolInput: `foo.txt"}`},
		{Type: StreamEventToolEnd, ToolCallID: "tc_1"},
		{Type: StreamEventDone, InputTokens: 20, OutputTokens: 10, StopReason: "tool_use"},
	}

	idx := 0
	pr, pw := io.Pipe()
	go func() { _ = pw.Close() }()

	stream := NewEventStream(pr, func() (StreamEvent, error) {
		if idx >= len(events) {
			return StreamEvent{}, io.EOF
		}
		e := events[idx]
		idx++
		return e, nil
	})
	defer func() { _ = stream.Close() }()

	for i, want := range events {
		got, err := stream.Next()
		if err != nil {
			t.Fatalf("event %d: unexpected error: %v", i, err)
		}
		if got.Type != want.Type {
			t.Errorf("event %d: type got %q, want %q", i, got.Type, want.Type)
		}
		if got.ToolCallID != want.ToolCallID {
			t.Errorf("event %d: tool_call_id got %q, want %q", i, got.ToolCallID, want.ToolCallID)
		}
		if got.ToolName != want.ToolName {
			t.Errorf("event %d: tool_name got %q, want %q", i, got.ToolName, want.ToolName)
		}
		if got.ToolInput != want.ToolInput {
			t.Errorf("event %d: tool_input got %q, want %q", i, got.ToolInput, want.ToolInput)
		}
	}
}

func TestStreamEventConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		constant string
		want     string
	}{
		{StreamEventText, "text"},
		{StreamEventToolStart, "tool_start"},
		{StreamEventToolDelta, "tool_delta"},
		{StreamEventToolEnd, "tool_end"},
		{StreamEventDone, "done"},
	}

	for _, tt := range tests {
		if tt.constant != tt.want {
			t.Errorf("constant value %q != %q", tt.constant, tt.want)
		}
	}
}
