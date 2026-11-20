package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// Contract provenance (retrieved 2026-11-18 UTC, spec 053 R3/R4/R5):
// https://platform.claude.com/docs/en/build-with-claude/prompt-caching
// https://developers.openai.com/api/docs/guides/prompt-caching
// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create.md
// https://developers.openai.com/api/reference/resources/chat/subresources/completions/streaming-events.md
// The literal wire fixtures below protect the selected automatic-Chat/read-only
// boundary, not live eligibility, hits, savings, or first-party cache support at
// compatible gateways. Adapter-specific errors and OpenRouter's unmapped cache
// counts are preservation contracts, not a universal provider error protocol.
func TestCacheCompatibility_WireUsageErrors(t *testing.T) {
	for _, adapter := range cacheCompatibilityAdapters() {
		t.Run(adapter.name, func(t *testing.T) {
			if adapter.cost {
				t.Run("optional-proxy-cost", func(t *testing.T) {
					cacheCompatibilityCost(t, adapter)
				})
			}
			for _, streaming := range []bool{false, true} {
				t.Run(fmt.Sprintf("stream=%t", streaming), func(t *testing.T) {
					for _, enabled := range []bool{false, true} {
						for _, usage := range []string{"reported", "zero", "absent"} {
							t.Run(fmt.Sprintf("success/cache=%t/usage=%s", enabled, usage), func(t *testing.T) {
								cacheCompatibilitySuccess(t, adapter, streaming, enabled, usage)
							})
						}
					}
					for _, status := range []int{400, 403, 429, 529} {
						t.Run(fmt.Sprintf("HTTP=%d", status), func(t *testing.T) {
							cacheCompatibilityHTTPError(t, adapter, streaming, status)
						})
					}
					t.Run("ambiguous-lost-response", func(t *testing.T) {
						cacheCompatibilityLostResponse(t, adapter, streaming)
					})
				})
			}
			for _, tool := range []bool{false, true} {
				for _, failure := range []string{"provider-or-parser-error", "truncated", "transport-error"} {
					t.Run(fmt.Sprintf("partial/tool=%t/%s", tool, failure), func(t *testing.T) {
						cacheCompatibilityPartial(t, adapter, tool, failure)
					})
				}
			}
			t.Run("configured-nonstream-transient-retry", func(t *testing.T) {
				cacheCompatibilityRetry(t, adapter)
			})
		})
	}
}

type cacheCompatibilityAdapter struct {
	name     string
	messages bool
	router   bool
	cost     bool
	path     string
	new      func(string) (StreamProvider, error)
}

func cacheCompatibilityAdapters() []cacheCompatibilityAdapter {
	return []cacheCompatibilityAdapter{
		{name: "OpenAI", path: "/chat/completions", new: func(url string) (StreamProvider, error) { return NewOpenAI("local-key", WithOpenAIBaseURL(url)) }},
		{name: "Anthropic", messages: true, path: "/v1/messages", new: func(url string) (StreamProvider, error) { return NewAnthropic("local-key", WithBaseURL(url)) }},
		{name: "MiniMax", messages: true, path: "/v1/messages", new: func(url string) (StreamProvider, error) { return NewMiniMax("local-key", WithBaseURL(url)) }},
		{name: "AtlasCloud", path: "/chat/completions", new: func(url string) (StreamProvider, error) { return NewAtlasCloud("local-key", url) }},
		{name: "CheaperInference", path: "/chat/completions", new: func(url string) (StreamProvider, error) { return NewCheaperInference("local-key", url) }},
		{name: "EdenAI", path: "/chat/completions", new: func(url string) (StreamProvider, error) { return NewEdenAI("local-key", url) }},
		{name: "LiteLLM", cost: true, path: "/v1/chat/completions", new: func(url string) (StreamProvider, error) { return NewLiteLLM("local-key", url) }},
		{name: "Runcrate", path: "/chat/completions", new: func(url string) (StreamProvider, error) { return NewRuncrate("local-key", url) }},
		{name: "StandardCompute", path: "/chat/completions", new: func(url string) (StreamProvider, error) { return NewStandardCompute("local-key", url) }},
		{name: "OpenRouter", router: true, path: "/chat/completions", new: func(url string) (StreamProvider, error) {
			return NewOpenRouter("local-key", WithOpenRouterBaseURL(url))
		}},
	}
}

func cacheCompatibilityRequest(enabled bool) *Request {
	return &Request{
		Model: "local-model", System: "static instructions", MaxTokens: 64, CacheConfig: enabled,
		Messages: []Message{{Role: "user", Content: "dynamic question"}},
		Tools: []Tool{{Name: "lookup", Description: "Look up a value", Parameters: map[string]ToolParameter{
			"query": {Type: "string", Description: "Lookup key", Required: true},
		}}},
	}
}

// Captures are per-case, synchronized and nonblocking even if replay regresses.
// No handler depends on a test goroutine receiving from a channel.
type cacheCompatibilityCapture struct {
	mu     sync.Mutex
	bodies []string
}

func (c *cacheCompatibilityCapture) record(t *testing.T, a cacheCompatibilityAdapter, r *http.Request) int {
	t.Helper()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		t.Errorf("read local request: %v", err)
	}
	if r.Method != http.MethodPost || r.URL.Path != a.path {
		t.Errorf("wire endpoint = %s %s, want POST %s", r.Method, r.URL.Path, a.path)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bodies = append(c.bodies, string(body))
	return len(c.bodies)
}

func (c *cacheCompatibilityCapture) check(t *testing.T, a cacheCompatibilityAdapter, count int, streaming, enabled bool) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.bodies) != count {
		t.Errorf("requests = %d, want %d (no cache-off replay)", len(c.bodies), count)
	}
	for i, body := range c.bodies {
		var wire map[string]interface{}
		if err := json.Unmarshal([]byte(body), &wire); err != nil {
			t.Fatalf("request %d JSON: %v", i, err)
		}
		if !a.messages {
			// Literal Chat request oracle, independent of production wire structs.
			want := `{"model":"local-model","messages":[{"role":"system","content":"static instructions"},{"role":"user","content":"dynamic question"}],"max_completion_tokens":64,"tools":[{"type":"function","function":{"name":"lookup","description":"Look up a value","parameters":{"type":"object","properties":{"query":{"type":"string","description":"Lookup key"}},"required":["query"]}}}]}`
			var expected map[string]interface{}
			if err := json.Unmarshal([]byte(want), &expected); err != nil {
				t.Fatal(err)
			}
			if streaming {
				expected["stream"] = true
				expected["stream_options"] = map[string]interface{}{"include_usage": true}
			}
			if !reflect.DeepEqual(wire, expected) {
				t.Errorf("automatic Chat request %d = %s, want %+v", i, body, expected)
			}
		} else {
			// Observing the original hint on every attempt distinguishes generic
			// retries from cache-disable fallback. Static-prefix detail is T2.
			var expected interface{} = "static instructions"
			if enabled {
				expected = []interface{}{map[string]interface{}{"type": "text", "text": "static instructions", "cache_control": map[string]interface{}{"type": "ephemeral"}}}
			}
			if !reflect.DeepEqual(wire["system"], expected) {
				t.Errorf("Messages request %d system = %#v, want %#v", i, wire["system"], expected)
			}
		}
		if i > 0 && body != c.bodies[0] {
			t.Errorf("retry request %d changed wire body", i)
		}
	}
}

func cacheCompatibilityContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func cacheCompatibilityProvider(t *testing.T, a cacheCompatibilityAdapter, url string) StreamProvider {
	t.Helper()
	p, err := a.new(url)
	if err != nil {
		t.Fatalf("construct %s: %v", a.name, err)
	}
	return p
}

func cacheCompatibilityUsage(a cacheCompatibilityAdapter, variant string) string {
	if variant == "absent" {
		return ""
	}
	if variant == "zero" {
		if a.messages {
			return `"usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}`
		}
		return `"usage":{"prompt_tokens":0,"completion_tokens":0,"prompt_tokens_details":{"cached_tokens":0,"cache_write_tokens":0}}`
	}
	if a.messages {
		return `"usage":{"input_tokens":11,"output_tokens":7,"cache_read_input_tokens":5,"cache_creation_input_tokens":3}`
	}
	return `"usage":{"prompt_tokens":11,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":5,"cache_write_tokens":3}}`
}

func cacheCompatibilityNormal(a cacheCompatibilityAdapter, usage string) string {
	var body string
	if a.messages {
		body = `{"model":"local-model","content":[{"type":"text","text":"hello"},{"type":"tool_use","id":"call_1","name":"lookup","input":{"query":"value"}}],"stop_reason":"tool_use"`
	} else {
		body = `{"model":"local-model","choices":[{"message":{"content":"hello","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"query\":\"value\"}"}}]},"finish_reason":"tool_calls"}]`
	}
	if usage != "" {
		body += "," + usage
	}
	return body + "}"
}

func cacheCompatibilityPrefix(a cacheCompatibilityAdapter, tool bool) string {
	if a.messages {
		if tool {
			return "data: " + `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_1","name":"lookup"}}` + "\n\ndata: " + `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"value\"}"}}` + "\n\n"
		}
		return "data: " + `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}` + "\n\ndata: " + `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"hello"}}` + "\n\n"
	}
	if tool {
		return "data: " + `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":""}}]}}]}` + "\n\ndata: " + `{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"query\":\"value\"}"}}]}}]}` + "\n\n"
	}
	return "data: " + `{"choices":[{"delta":{"content":"hello"}}]}` + "\n\n"
}

func cacheCompatibilitySSE(a cacheCompatibilityAdapter, variant string) string {
	usage := cacheCompatibilityUsage(a, variant)
	if a.messages {
		start := `{"type":"message_start","message":{}}`
		if usage != "" {
			start = `{"type":"message_start","message":{` + usage + `}}`
		}
		end := `{"type":"message_delta","delta":{"stop_reason":"tool_use"}`
		if usage != "" {
			end += "," + usage
		}
		return "data: " + start + "\n\ndata: " +
			`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\ndata: " +
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}` + "\n\ndata: " +
			`{"type":"content_block_stop","index":0}` + "\n\ndata: " +
			`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call_1","name":"lookup"}}` + "\n\ndata: " +
			`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"value\"}"}}` + "\n\ndata: " +
			`{"type":"content_block_stop","index":1}` + "\n\ndata: " + end + "}\n\ndata: {\"type\":\"message_stop\"}\n\n"
	}
	body := cacheCompatibilityPrefix(a, false) + cacheCompatibilityPrefix(a, true) + "data: " + `{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n"
	if usage != "" {
		body += `data: {"choices":[],` + usage + "}\n\n"
	}
	return body + "data: [DONE]\n\n"
}

func cacheCompatibilitySuccess(t *testing.T, a cacheCompatibilityAdapter, streaming, enabled bool, variant string) {
	t.Helper()
	capture := &cacheCompatibilityCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.record(t, a, r)
		if streaming {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, cacheCompatibilitySSE(a, variant))
		} else {
			_, _ = io.WriteString(w, cacheCompatibilityNormal(a, cacheCompatibilityUsage(a, variant)))
		}
	}))
	defer server.Close()
	p := cacheCompatibilityProvider(t, a, server.URL)
	input, output, read, write := 0, 0, 0, 0
	if variant == "reported" {
		input, output = 11, 7
		if !a.router {
			read = 5
		}
		if a.messages {
			write = 3
		}
	}
	stop := "tool_calls"
	if a.messages {
		stop = "tool_use"
	}
	if streaming {
		stream, err := p.SendStream(cacheCompatibilityContext(t), cacheCompatibilityRequest(enabled))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = stream.Close() }()
		want := []StreamEvent{
			{Type: StreamEventText, Text: "hello"},
			{Type: StreamEventToolStart, ToolCallID: "call_1", ToolName: "lookup"},
			{Type: StreamEventToolDelta, ToolCallID: "call_1", ToolInput: `{"query":"value"}`},
			{Type: StreamEventToolEnd, ToolCallID: "call_1"},
			{Type: StreamEventDone, StopReason: stop, InputTokens: input, OutputTokens: output, CacheReadTokens: read, CacheWriteTokens: write},
		}
		if got := drainStream(t, stream); !reflect.DeepEqual(got, want) {
			t.Errorf("events = %+v, want %+v", got, want)
		}
	} else {
		got, err := p.Send(cacheCompatibilityContext(t), cacheCompatibilityRequest(enabled))
		if err != nil {
			t.Fatal(err)
		}
		want := &Response{Content: "hello", Model: "local-model", InputTokens: input, OutputTokens: output, CacheReadTokens: read, CacheWriteTokens: write, StopReason: stop,
			ToolCalls: []ToolCall{{ID: "call_1", Name: "lookup", Arguments: map[string]string{"query": "value"}}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("response = %+v, want %+v", got, want)
		}
	}
	capture.check(t, a, 1, streaming, enabled)
}

func cacheCompatibilityError(t *testing.T, err error, category ErrorCategory, status int, message string) {
	t.Helper()
	var got *ProviderError
	if !errors.As(err, &got) {
		t.Fatalf("error = %v (%T), want ProviderError", err, err)
	}
	if got.Category != category || got.Status != status || got.Message != message || err.Error() != string(category)+": "+message {
		t.Errorf("error = %+v / %q, want category=%s status=%d message=%q", got, err.Error(), category, status, message)
	}
}

func cacheCompatibilityHTTPError(t *testing.T, a cacheCompatibilityAdapter, streaming bool, status int) {
	t.Helper()
	capture := &cacheCompatibilityCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.record(t, a, r)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"error":{"type":"invalid_request_error","message":"cache prefix or unrelated request rejected"}}`)
	}))
	defer server.Close()
	p := NewRetry(cacheCompatibilityProvider(t, a, server.URL), RetryConfig{MaxRetries: 1, Backoff: "fixed", InitialDelayMs: 1, MaxDelayMs: 1})
	category := ErrCategoryServer
	switch status {
	case 400:
		category = ErrCategoryBadRequest
	case 403:
		if !a.messages {
			category = ErrCategoryAuth
		}
	case 429:
		category = ErrCategoryRateLimit
	case 529:
		if a.messages {
			category = ErrCategoryOverloaded
		}
	}
	var err error
	if streaming {
		var stream *EventStream
		stream, err = p.SendStream(cacheCompatibilityContext(t), cacheCompatibilityRequest(true))
		if stream != nil {
			_ = stream.Close()
			t.Error("pre-stream error returned a stream")
		}
	} else {
		var response *Response
		response, err = p.Send(cacheCompatibilityContext(t), cacheCompatibilityRequest(true))
		if response != nil {
			t.Error("HTTP failure returned a response")
		}
	}
	cacheCompatibilityError(t, err, category, status, "cache prefix or unrelated request rejected")
	count := 1
	if !streaming && (category == ErrCategoryServer || category == ErrCategoryOverloaded || category == ErrCategoryRateLimit) {
		count = 2 // configured generic transient retry, with the same cache choice
	}
	if p.Attempts() != count-1 {
		t.Errorf("configured retries = %d, want %d", p.Attempts(), count-1)
	}
	capture.check(t, a, count, streaming, true)
}

func cacheCompatibilityLostResponse(t *testing.T, a cacheCompatibilityAdapter, streaming bool) {
	t.Helper()
	capture := &cacheCompatibilityCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.record(t, a, r) // accepted POST; remote outcome is now ambiguous
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack local connection: %v", err)
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()
	// Generic nonstream retries are deliberately disabled here. A stream never
	// retries even when configured; neither mode gets an extra cache-off replay.
	retries := 0
	if streaming {
		retries = 1
	}
	p := NewRetry(cacheCompatibilityProvider(t, a, server.URL), RetryConfig{MaxRetries: retries})
	var err error
	if streaming {
		_, err = p.SendStream(cacheCompatibilityContext(t), cacheCompatibilityRequest(true))
	} else {
		_, err = p.Send(cacheCompatibilityContext(t), cacheCompatibilityRequest(true))
	}
	cacheCompatibilityError(t, err, ErrCategoryServer, 0, `Post "`+server.URL+a.path+`": EOF`)
	if !errors.Is(err, io.EOF) {
		t.Errorf("lost-response cause = %v, want wrapped EOF", err)
	}
	capture.check(t, a, 1, streaming, true)
}

func cacheCompatibilityPartial(t *testing.T, a cacheCompatibilityAdapter, tool bool, failure string) {
	t.Helper()
	capture := &cacheCompatibilityCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.record(t, a, r)
		w.Header().Set("Content-Type", "text/event-stream")
		if failure == "transport-error" {
			w.Header().Set("Content-Length", "1000000") // incomplete HTTP body, not a clean SSE EOF
		}
		// Even reported input counts in message_start are not a fabricated final
		// usage event when message_delta never arrives.
		if a.messages {
			_, _ = io.WriteString(w, "data: "+`{"type":"message_start","message":{"usage":{"input_tokens":11,"cache_read_input_tokens":5,"cache_creation_input_tokens":3}}}`+"\n\n")
		}
		_, _ = io.WriteString(w, cacheCompatibilityPrefix(a, tool))
		if failure == "provider-or-parser-error" {
			if a.messages {
				_, _ = io.WriteString(w, "data: "+`{"type":"error","error":{"type":"overloaded_error","message":"original partial stream failure"}}`+"\n\n")
			} else {
				// Chat has no agreed universal in-band cache-error discriminator.
				_, _ = io.WriteString(w, "data: not-json\n\n")
			}
		}
	}))
	defer server.Close()
	p := NewRetry(cacheCompatibilityProvider(t, a, server.URL), RetryConfig{MaxRetries: 2})
	stream, err := p.SendStream(cacheCompatibilityContext(t), cacheCompatibilityRequest(true))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stream.Close() }()
	want := []StreamEvent{{Type: StreamEventText, Text: "hello"}}
	if tool {
		want = []StreamEvent{{Type: StreamEventToolStart, ToolCallID: "call_1", ToolName: "lookup"}, {Type: StreamEventToolDelta, ToolCallID: "call_1", ToolInput: `{"query":"value"}`}}
	}
	for i, expected := range want {
		got, nextErr := stream.Next()
		if nextErr != nil || got != expected {
			t.Fatalf("partial event %d = %+v / %v, want %+v", i, got, nextErr, expected)
		}
	}
	got, err := stream.Next()
	if got != (StreamEvent{}) {
		t.Errorf("interruption invented final event/usage: %+v", got)
	}
	switch failure {
	case "truncated":
		if err != io.EOF {
			t.Errorf("clean truncated SSE error = %v, want EOF without final usage", err)
		}
	case "transport-error":
		cacheCompatibilityError(t, err, ErrCategoryServer, 0, "stream read error: unexpected EOF")
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("transport error = %v, want wrapped unexpected EOF", err)
		}
	default:
		if a.messages {
			cacheCompatibilityError(t, err, ErrCategoryOverloaded, 0, "original partial stream failure")
		} else {
			cacheCompatibilityError(t, err, ErrCategoryServer, 0, "failed to parse streaming chunk: invalid character 'o' in literal null (expecting 'u')")
		}
	}
	if p.Attempts() != 0 {
		t.Errorf("partial stream retried %d times", p.Attempts())
	}
	if err := stream.Close(); err != nil {
		t.Errorf("close interrupted stream: %v", err)
	}
	if _, err := stream.Next(); err != io.EOF {
		t.Errorf("Next after Close = %v, want EOF", err)
	}
	capture.check(t, a, 1, true, true)
}

func cacheCompatibilityRetry(t *testing.T, a cacheCompatibilityAdapter) {
	t.Helper()
	capture := &cacheCompatibilityCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if capture.record(t, a, r) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"message":"ordinary transient throttling"}}`)
			return
		}
		_, _ = io.WriteString(w, cacheCompatibilityNormal(a, ""))
	}))
	defer server.Close()
	p := NewRetry(cacheCompatibilityProvider(t, a, server.URL), RetryConfig{MaxRetries: 1, Backoff: "fixed", InitialDelayMs: 1, MaxDelayMs: 1})
	response, err := p.Send(cacheCompatibilityContext(t), cacheCompatibilityRequest(true))
	if err != nil || response == nil || response.Content != "hello" {
		t.Fatalf("configured transient recovery = %+v / %v", response, err)
	}
	if p.Attempts() != 1 {
		t.Errorf("retries = %d, want 1", p.Attempts())
	}
	capture.check(t, a, 2, false, true)
}

// LiteLLM's independently optional cost metadata is deliberately separate from
// cache counts. Existing named regression tests cover malformed cost and usage
// on a choice chunk; these cases keep both normal and stream absence explicit.
func cacheCompatibilityCost(t *testing.T, a cacheCompatibilityAdapter) {
	t.Helper()
	for _, streaming := range []bool{false, true} {
		for _, header := range []string{"", "0.0021"} {
			t.Run(fmt.Sprintf("stream=%t/header=%q", streaming, header), func(t *testing.T) {
				capture := &cacheCompatibilityCapture{}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					capture.record(t, a, r)
					if header != "" {
						w.Header().Set("x-litellm-response-cost", header)
					}
					if streaming {
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = io.WriteString(w, cacheCompatibilitySSE(a, "absent"))
					} else {
						_, _ = io.WriteString(w, cacheCompatibilityNormal(a, ""))
					}
				}))
				defer server.Close()
				p := cacheCompatibilityProvider(t, a, server.URL)
				var cost float64
				if streaming {
					stream, err := p.SendStream(cacheCompatibilityContext(t), cacheCompatibilityRequest(true))
					if err != nil {
						t.Fatal(err)
					}
					defer func() { _ = stream.Close() }()
					done := 0
					for _, event := range drainStream(t, stream) {
						if event.Type == StreamEventDone {
							cost = event.Cost
							done++
						}
					}
					if done != 1 {
						t.Errorf("done events = %d, want 1", done)
					}
				} else {
					response, err := p.Send(cacheCompatibilityContext(t), cacheCompatibilityRequest(true))
					if err != nil {
						t.Fatal(err)
					}
					cost = response.Cost
				}
				want := 0.0
				if strings.TrimSpace(header) != "" {
					want = 0.0021
				}
				if cost != want {
					t.Errorf("optional proxy cost = %v, want %v", cost, want)
				}
				capture.check(t, a, 1, streaming, true)
			})
		}
	}
}
