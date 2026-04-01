package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func writeSSE(w http.ResponseWriter, event, data string) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// --- Task 2.4: Request construction ---

func TestAnthropic_SendStream_RequestFormat(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":10}}}`)
		writeSSE(w, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`)
		writeSSE(w, "content_block_stop", `{"type":"content_block_stop","index":0}`)
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("my-api-key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:       "claude-sonnet-4-20250514",
		System:      "Be helpful",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0.7,
		MaxTokens:   1024,
		Tools: []Tool{{
			Name:        "read_file",
			Description: "Read a file",
			Parameters:  map[string]ToolParameter{"path": {Type: "string", Description: "File path", Required: true}},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Drain the stream
	for {
		_, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected stream error: %v", err)
		}
	}

	if gotBody["stream"] != true {
		t.Errorf("stream = %v, want true", gotBody["stream"])
	}
	if gotBody["model"] != "claude-sonnet-4-20250514" {
		t.Errorf("model = %v, want claude-sonnet-4-20250514", gotBody["model"])
	}
	if gotBody["system"] != "Be helpful" {
		t.Errorf("system = %v, want Be helpful", gotBody["system"])
	}
	if gotBody["max_tokens"] != float64(1024) {
		t.Errorf("max_tokens = %v, want 1024", gotBody["max_tokens"])
	}
	if gotBody["temperature"] != float64(0.7) {
		t.Errorf("temperature = %v, want 0.7", gotBody["temperature"])
	}

	msgs, ok := gotBody["messages"].([]interface{})
	if !ok || len(msgs) != 1 {
		t.Fatalf("messages = %v, want 1-element array", gotBody["messages"])
	}

	tools, ok := gotBody["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %v, want 1-element array", gotBody["tools"])
	}
}

func TestAnthropic_SendStream_DefaultMaxTokens(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`)
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 0,
		Messages:  []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	if gotBody["max_tokens"] != float64(defaultMaxTokens) {
		t.Errorf("max_tokens = %v, want %d", gotBody["max_tokens"], defaultMaxTokens)
	}
}

func TestAnthropic_SendStream_OmitsZeroTemperature(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`)
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:       "claude-sonnet-4-20250514",
		Temperature: 0,
		Messages:    []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	if _, exists := gotBody["temperature"]; exists {
		t.Errorf("body contains 'temperature' key when Temperature is 0")
	}
}

// --- Task 2.5: Error responses (pre-stream) ---

func TestAnthropic_SendStream_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"type": "error",
			"error": map[string]string{
				"type":    "authentication_error",
				"message": "invalid x-api-key",
			},
		})
	}))
	defer server.Close()

	a, _ := NewAnthropic("bad-key", WithBaseURL(server.URL))
	_, err := a.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Category != ErrCategoryAuth {
		t.Errorf("Category = %q, want %q", pe.Category, ErrCategoryAuth)
	}
}

func TestAnthropic_SendStream_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := a.SendStream(ctx, &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Category != ErrCategoryTimeout {
		t.Errorf("Category = %q, want %q", pe.Category, ErrCategoryTimeout)
	}
}

// --- Task 2.6: Text streaming ---

func TestAnthropic_SendStream_TextDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":25}}}`)
		writeSSE(w, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`)
		writeSSE(w, "content_block_stop", `{"type":"content_block_stop","index":0}`)
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":15}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	if events[0].Type != StreamEventText || events[0].Text != "Hello" {
		t.Errorf("events[0] = %+v, want Text Hello", events[0])
	}
	if events[1].Type != StreamEventText || events[1].Text != " world" {
		t.Errorf("events[1] = %+v, want Text ' world'", events[1])
	}
	if events[2].Type != StreamEventDone {
		t.Errorf("events[2].Type = %q, want %q", events[2].Type, StreamEventDone)
	}
	if events[2].InputTokens != 25 {
		t.Errorf("InputTokens = %d, want 25", events[2].InputTokens)
	}
	if events[2].OutputTokens != 15 {
		t.Errorf("OutputTokens = %d, want 15", events[2].OutputTokens)
	}
	if events[2].StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", events[2].StopReason, "end_turn")
	}
}

// --- Task 2.7: Tool call streaming ---

func TestAnthropic_SendStream_ToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":10}}}`)
		writeSSE(w, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_abc","name":"read_file"}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"pat"}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"h\": \"x\"}"}}`)
		writeSSE(w, "content_block_stop", `{"type":"content_block_stop","index":0}`)
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":8}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 5 {
		t.Fatalf("got %d events, want 5 (start, 2 deltas, end, done)", len(events))
	}

	if events[0].Type != StreamEventToolStart || events[0].ToolCallID != "toolu_abc" || events[0].ToolName != "read_file" {
		t.Errorf("events[0] = %+v, want ToolStart toolu_abc read_file", events[0])
	}
	if events[1].Type != StreamEventToolDelta || events[1].ToolCallID != "toolu_abc" || events[1].ToolInput != `{"pat` {
		t.Errorf("events[1] = %+v, want ToolDelta toolu_abc {\"pat", events[1])
	}
	if events[2].Type != StreamEventToolDelta || events[2].ToolCallID != "toolu_abc" || events[2].ToolInput != `h": "x"}` {
		t.Errorf("events[2] = %+v, want ToolDelta toolu_abc h\": \"x\"}", events[2])
	}
	if events[3].Type != StreamEventToolEnd || events[3].ToolCallID != "toolu_abc" {
		t.Errorf("events[3] = %+v, want ToolEnd toolu_abc", events[3])
	}
	if events[4].Type != StreamEventDone || events[4].StopReason != "tool_use" {
		t.Errorf("events[4] = %+v, want Done tool_use", events[4])
	}
}

// --- Task 2.8: Mixed text + tool blocks ---

func TestAnthropic_SendStream_TextAndToolBlocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":5}}}`)
		writeSSE(w, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Let me read that."}}`)
		writeSSE(w, "content_block_stop", `{"type":"content_block_stop","index":0}`)
		writeSSE(w, "content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_xyz","name":"read_file"}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"path\":\"a.go\"}"}}`)
		writeSSE(w, "content_block_stop", `{"type":"content_block_stop","index":1}`)
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":20}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, ev)
	}

	// text delta, tool start, tool delta, tool end, done
	if len(events) != 5 {
		t.Fatalf("got %d events, want 5", len(events))
	}

	if events[0].Type != StreamEventText || events[0].Text != "Let me read that." {
		t.Errorf("events[0] = %+v, want Text", events[0])
	}
	if events[1].Type != StreamEventToolStart || events[1].ToolName != "read_file" {
		t.Errorf("events[1] = %+v, want ToolStart", events[1])
	}
	if events[2].Type != StreamEventToolDelta {
		t.Errorf("events[2] = %+v, want ToolDelta", events[2])
	}
	if events[3].Type != StreamEventToolEnd || events[3].ToolCallID != "toolu_xyz" {
		t.Errorf("events[3] = %+v, want ToolEnd toolu_xyz", events[3])
	}
	if events[4].Type != StreamEventDone {
		t.Errorf("events[4] = %+v, want Done", events[4])
	}
}

// --- Task 2.9: Edge cases ---

func TestAnthropic_SendStream_PingIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`)
		writeSSE(w, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		writeSSE(w, "ping", `{"type":"ping"}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"A"}}`)
		writeSSE(w, "ping", `{"type":"ping"}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"B"}}`)
		writeSSE(w, "content_block_stop", `{"type":"content_block_stop","index":0}`)
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, ev)
	}

	// Only text A, text B, done — no ping events
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3 (no ping events)", len(events))
	}
	if events[0].Text != "A" || events[1].Text != "B" {
		t.Errorf("events = %+v, want A, B texts", events)
	}
}

func TestAnthropic_SendStream_MidStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`)
		writeSSE(w, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		writeSSE(w, "error", `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	// Consume until error
	for {
		_, err := stream.Next()
		if err != nil {
			var pe *ProviderError
			if !errors.As(err, &pe) {
				t.Fatalf("expected ProviderError, got %T: %v", err, err)
			}
			if pe.Category != ErrCategoryOverloaded {
				t.Errorf("Category = %q, want %q", pe.Category, ErrCategoryOverloaded)
			}
			break
		}
	}
}

func TestAnthropic_SendStream_EmptyTextDelta(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`)
		writeSSE(w, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":""}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"real"}}`)
		writeSSE(w, "content_block_stop", `{"type":"content_block_stop","index":0}`)
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, ev)
	}

	// Empty text delta should be skipped; only "real" + done
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Text != "real" {
		t.Errorf("events[0].Text = %q, want %q", events[0].Text, "real")
	}
}

func TestAnthropic_SendStream_EmptyPartialJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`)
		writeSSE(w, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"test"}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":""}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}`)
		writeSSE(w, "content_block_stop", `{"type":"content_block_stop","index":0}`)
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":1}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, ev)
	}

	// tool_start, tool_delta (only non-empty "{}"), tool_end, done
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4", len(events))
	}
	if events[1].Type != StreamEventToolDelta || events[1].ToolInput != "{}" {
		t.Errorf("events[1] = %+v, want ToolDelta with input {}", events[1])
	}
}

func TestAnthropic_SendStream_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "event: message_start\ndata: {invalid json\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	_, err = stream.Next()
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if pe.Category != ErrCategoryServer {
		t.Errorf("Category = %q, want %q", pe.Category, ErrCategoryServer)
	}
}

func TestAnthropic_SendStream_UnknownBlockIndex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`)
		// Delta for index 99 which was never started
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":99,"delta":{"type":"text_delta","text":"ghost"}}`)
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		events = append(events, ev)
	}

	// Only done event; the unknown index delta should be skipped
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 (only done)", len(events))
	}
	if events[0].Type != StreamEventDone {
		t.Errorf("events[0].Type = %q, want %q", events[0].Type, StreamEventDone)
	}
}

func TestAnthropic_SendStream_MessageDeltaWithoutStart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// No message_start, jump straight to message_delta
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet-4-20250514",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	ev, err := stream.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Type != StreamEventDone {
		t.Errorf("Type = %q, want %q", ev.Type, StreamEventDone)
	}
	if ev.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", ev.InputTokens)
	}
	if ev.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", ev.OutputTokens)
	}
}
