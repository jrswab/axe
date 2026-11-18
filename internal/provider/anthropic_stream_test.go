package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func writeSSE(w http.ResponseWriter, event, data string) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func TestAnthropic_StreamStaticPrefixWire(t *testing.T) {
	// R2/R5, docs/plans/053_corrected_caching_001_spec.md: the approved
	// https://platform.claude.com/docs/en/build-with-claude/prompt-caching
	// contract orders the prefix tools -> system -> messages. Literal wire
	// expectations below do not use production serialization types or helpers.
	const plainTools = `[
		{"name":"z_lookup","description":"Look up a record","input_schema":{"type":"object","properties":{"key":{"type":"string","description":"Record key"},"verbose":{"type":"boolean","description":"Include details"}},"required":["key"]}},
		{"name":"a_report","description":"Report a record","input_schema":{"type":"object","properties":{"record":{"type":"string","description":"Record to report"}},"required":["record"]}}
	]`
	const cachedTools = `[
		{"name":"z_lookup","description":"Look up a record","input_schema":{"type":"object","properties":{"key":{"type":"string","description":"Record key"},"verbose":{"type":"boolean","description":"Include details"}},"required":["key"]}},
		{"name":"a_report","description":"Report a record","input_schema":{"type":"object","properties":{"record":{"type":"string","description":"Record to report"}},"required":["record"]},"cache_control":{"type":"ephemeral"}}
	]`
	turns := []struct {
		name     string
		messages []Message
		want     string
	}{
		{
			name: "first",
			messages: []Message{
				{Role: "user", Content: "History: first record"},
				{Role: "assistant", Content: "First history acknowledged"},
				{Role: "assistant", Content: "Looking up first", ToolCalls: []ToolCall{{ID: "call_first", Name: "z_lookup", Arguments: map[string]string{"key": "first"}}}},
				{Role: "tool", ToolResults: []ToolResult{{CallID: "call_first", Content: "First lookup result"}}},
				{Role: "user", Content: "Task: report first\nContext: first private context"},
			},
			want: `[
				{"role":"user","content":"History: first record"},
				{"role":"assistant","content":"First history acknowledged"},
				{"role":"assistant","content":[{"type":"text","text":"Looking up first"},{"type":"tool_use","id":"call_first","name":"z_lookup","input":{"key":"first"}}]},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_first","content":"First lookup result","is_error":false}]},
				{"role":"user","content":"Task: report first\nContext: first private context"}
			]`,
		},
		{
			name: "second",
			messages: []Message{
				{Role: "user", Content: "History: second record"},
				{Role: "assistant", Content: "Second history acknowledged"},
				{Role: "assistant", Content: "Reporting second", ToolCalls: []ToolCall{{ID: "call_second", Name: "a_report", Arguments: map[string]string{"record": "second"}}}},
				{Role: "tool", ToolResults: []ToolResult{{CallID: "call_second", Content: "Second report failed", IsError: true}}},
				{Role: "user", Content: "Task: retry second\nContext: second private context"},
			},
			want: `[
				{"role":"user","content":"History: second record"},
				{"role":"assistant","content":"Second history acknowledged"},
				{"role":"assistant","content":[{"type":"text","text":"Reporting second"},{"type":"tool_use","id":"call_second","name":"a_report","input":{"record":"second"}}]},
				{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_second","content":"Second report failed","is_error":true}]},
				{"role":"user","content":"Task: retry second\nContext: second private context"}
			]`,
		},
	}
	decode := func(t *testing.T, raw []byte) interface{} {
		t.Helper()
		var value interface{}
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("invalid JSON %s: %v", raw, err)
		}
		return value
	}
	// Root logs stay visible in the non-verbose RED run for passing rows too.
	report := func(row, mode string, passed bool) {
		t.Logf("%s/%s: pass=%t (two drained/closed streams, literal wire + static stability + caller nonmutation)", row, mode, passed)
	}
	for _, row := range []struct {
		name   string
		system string
		tools  bool
	}{
		{name: "system_and_tools", system: "Static instructions only.", tools: true},
		{name: "system_only", system: "Static instructions only."},
		{name: "tools_only", tools: true},
		{name: "neither"},
	} {
		t.Run(row.name, func(t *testing.T) {
			// Nonalphabetical caller order; one required property per tool keeps
			// multi-required schema ordering (T3) outside this breakpoint slice.
			req := &Request{Model: "claude-sonnet-4-20250514", MaxTokens: 128, System: row.system}
			if row.tools {
				req.Tools = []Tool{
					{Name: "z_lookup", Description: "Look up a record", Parameters: map[string]ToolParameter{
						"key":     {Type: "string", Description: "Record key", Required: true},
						"verbose": {Type: "boolean", Description: "Include details"},
					}},
					{Name: "a_report", Description: "Report a record", Parameters: map[string]ToolParameter{
						"record": {Type: "string", Description: "Record to report", Required: true},
					}},
				}
			}
			requests := make(chan []byte, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read local request: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				requests <- body
				w.Header().Set("Content-Type", "text/event-stream")
				writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`)
				writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`)
				writeSSE(w, "message_stop", `{"type":"message_stop"}`)
			}))
			defer server.Close()
			a, err := NewAnthropic("local-test-key", WithBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			// Reuse request, tools and parameter maps enabled -> disabled to
			// expose any mutation or hint leakage instead of using fresh inputs.
			for _, mode := range []struct {
				name    string
				enabled bool
			}{{name: "enabled", enabled: true}, {name: "disabled"}} {
				passed := t.Run(mode.name, func(t *testing.T) {
					req.CacheConfig = mode.enabled
					var firstSystem, firstTools json.RawMessage
					for i, turn := range turns {
						t.Run(turn.name, func(t *testing.T) {
							req.Messages = turn.messages
							before, err := json.Marshal(req)
							if err != nil {
								t.Fatal(err)
							}

							stream, err := a.SendStream(context.Background(), req)
							if err != nil {
								t.Fatalf("SendStream: %v", err)
							}
							defer func() {
								if err := stream.Close(); err != nil {
									t.Errorf("close local stream: %v", err)
								}
							}()
							for {
								_, err := stream.Next()
								if err == io.EOF {
									break
								}
								if err != nil {
									t.Fatalf("drain local stream: %v", err)
								}
							}

							after, err := json.Marshal(req)
							if err != nil {
								t.Fatal(err)
							}
							if !bytes.Equal(before, after) {
								t.Errorf("SendStream mutated caller request/tools/schema/history:\nbefore %s\nafter  %s", before, after)
							}
							var got map[string]json.RawMessage
							if err := json.Unmarshal(<-requests, &got); err != nil {
								t.Fatal(err)
							}
							if i == 0 {
								firstSystem, firstTools = got["system"], got["tools"]
							} else if !bytes.Equal(firstSystem, got["system"]) || !bytes.Equal(firstTools, got["tools"]) {
								t.Error("static system/tool prefix changed when only dynamic conversation changed")
							}
							wantSystem, wantTools := "", ""
							if row.system != "" {
								wantSystem = `"Static instructions only."`
								if mode.enabled {
									wantSystem = `[{"type":"text","text":"Static instructions only.","cache_control":{"type":"ephemeral"}}]`
								}
							}
							if row.tools {
								wantTools = plainTools
								if mode.enabled && row.system == "" {
									wantTools = cachedTools
								}
							}
							for _, field := range []struct{ name, want string }{
								{"system", wantSystem}, {"tools", wantTools}, {"messages", turn.want},
							} {
								raw, present := got[field.name]
								if field.want == "" {
									if present {
										t.Errorf("%s must be omitted, got %s", field.name, raw)
									}
								} else if !present || !reflect.DeepEqual(decode(t, raw), decode(t, []byte(field.want))) {
									t.Errorf("%s wire mismatch (only the last caller-ordered tool gets a hint for enabled tools-only):\ngot  %s\nwant %s", field.name, raw, field.want)
								}
								delete(got, field.name)
							}
							// Exact metadata and literal field comparisons reject all
							// synthetic hints, including hints hidden in messages/schema.
							metadata, err := json.Marshal(got)
							if err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(decode(t, metadata), decode(t, []byte(`{"model":"claude-sonnet-4-20250514","max_tokens":128,"stream":true}`))) {
								t.Errorf("unexpected request metadata: %s", metadata)
							}
						})
					}
				})
				report(row.name, mode.name, passed)
			}
		})
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

func TestAnthropic_SendStream_IgnoresReasoningEffortAndExplicitZeroTemperature(t *testing.T) {
	cases := []struct {
		name        string
		newProvider func(string, ...AnthropicOption) (*Anthropic, error)
		model       string
	}{
		{"anthropic", NewAnthropic, "claude-sonnet-4-20250514"},
		{"minimax", NewMiniMax, "MiniMax-M2.5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Errorf("failed to parse request body: %v", err)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`)
				writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`)
				writeSSE(w, "message_stop", `{"type":"message_stop"}`)
			}))
			defer server.Close()

			a, err := tc.newProvider("test-key", WithBaseURL(server.URL))
			if err != nil {
				t.Fatalf("unexpected constructor error: %v", err)
			}
			stream, err := a.SendStream(context.Background(), &Request{
				Model:           tc.model,
				Messages:        []Message{{Role: "user", Content: "Hi"}},
				ReasoningEffort: "low",
				TemperatureSet:  true,
				Temperature:     0,
			})
			if err != nil {
				t.Fatalf("unexpected SendStream error: %v", err)
			}
			defer func() { _ = stream.Close() }()
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
			if gotBody["model"] != tc.model {
				t.Errorf("model = %v, want %q", gotBody["model"], tc.model)
			}
			for _, key := range []string{"reasoning_effort", "temperature"} {
				if _, exists := gotBody[key]; exists {
					t.Errorf("body contains %q: %v", key, gotBody[key])
				}
			}
		})
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

func TestAnthropic_SendStream_CacheUsageParsing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, "message_start", `{"type":"message_start","message":{"usage":{"input_tokens":100,"cache_creation_input_tokens":80,"cache_read_input_tokens":10}}}`)
		writeSSE(w, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		writeSSE(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello from cached prompt"}}`)
		writeSSE(w, "content_block_stop", `{"type":"content_block_stop","index":0}`)
		writeSSE(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`)
		writeSSE(w, "message_stop", `{"type":"message_stop"}`)
	}))
	defer server.Close()

	a, _ := NewAnthropic("key", WithBaseURL(server.URL))
	stream, err := a.SendStream(context.Background(), &Request{
		Model:       "claude-sonnet-4-20250514",
		CacheConfig: true,
		Messages:    []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var gotDone StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev.Type == StreamEventDone {
			gotDone = ev
		}
	}

	if gotDone.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", gotDone.InputTokens)
	}
	if gotDone.OutputTokens != 5 {
		t.Errorf("OutputTokens = %d, want 5", gotDone.OutputTokens)
	}
	if gotDone.CacheReadTokens != 10 {
		t.Errorf("CacheReadTokens = %d, want 10", gotDone.CacheReadTokens)
	}
	if gotDone.CacheWriteTokens != 80 {
		t.Errorf("CacheWriteTokens = %d, want 80", gotDone.CacheWriteTokens)
	}
}

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
