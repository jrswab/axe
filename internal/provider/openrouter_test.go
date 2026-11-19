package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestOpenRouter_StableToolSchemaConversation protects R3/R5 and checklist T3
// (docs/plans/053_corrected_caching_001_spec.md). The literal Chat shapes follow
// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create.md
// retrieved 2026-11-18 as recorded in the spec. Lexicographic required names are
// Axe's stability policy, not a provider requirement. This compatible-adapter
// regression proves no OpenRouter automatic-caching contract, acceptance or hit.
func TestOpenRouter_StableToolSchemaConversation(t *testing.T) {
	const repetitions = 16
	const toolsWire = `[
		{"type":"function","function":{"name":"z_lookup","description":"Look up a record","parameters":{"type":"object","properties":{
			"account":{"type":"string","description":"Account identifier"},
			"key":{"type":"string","description":"Record key"},
			"limit":{"type":"integer","description":"Maximum records"},
			"region":{"type":"string","description":"Lookup region"},
			"verbose":{"type":"boolean","description":"Include details"}
		},"required":["account","key","region"]}}},
		{"type":"function","function":{"name":"a_report","description":"Report a record","parameters":{"type":"object","properties":{
			"format":{"type":"string","description":"Report format"},
			"ratio":{"type":"number","description":"Sampling ratio"},
			"record":{"type":"string","description":"Record to report"},
			"target":{"type":"string","description":"Report destination"},
			"title":{"type":"string","description":"Optional title"}
		},"required":["format","record","target"]}}}
	]`
	decode := func(t *testing.T, raw []byte) any {
		t.Helper()
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatalf("decode literal/request: %v; body %s", err, raw)
		}
		return value
	}
	wantTools := decode(t, []byte(toolsWire))
	turns := []struct {
		name     string
		messages []Message
		wire     string
	}{
		{
			name:     "initial-user",
			messages: []Message{{Role: "user", Content: "Task one; context one."}},
			wire:     `[{"role":"system","content":"Static instructions only."},{"role":"user","content":"Task one; context one."}]`,
		},
		{
			name: "assistant-content-and-result",
			messages: []Message{
				{Role: "user", Content: "Task one; context one."},
				{Role: "assistant", Content: "Looking up.", ToolCalls: []ToolCall{
					{ID: "lookup-1", Name: "z_lookup", Arguments: map[string]string{"account": "acct-1", "key": "key-1", "region": "west"}},
				}},
				{Role: "tool", ToolResults: []ToolResult{{CallID: "lookup-1", Content: "record one"}}},
				{Role: "user", Content: "Report it; context two."},
			},
			wire: `[{"role":"system","content":"Static instructions only."},{"role":"user","content":"Task one; context one."},
				{"role":"assistant","content":"Looking up.","tool_calls":[{"id":"lookup-1","type":"function","function":{"name":"z_lookup","arguments":"{\"account\":\"acct-1\",\"key\":\"key-1\",\"region\":\"west\"}"}}]},
				{"role":"tool","tool_call_id":"lookup-1","content":"record one"},{"role":"user","content":"Report it; context two."}]`,
		},
		{
			name: "changed-history-null-assistant-and-multiple-results",
			messages: []Message{
				{Role: "user", Content: "Task three; context three."},
				{Role: "assistant", Content: "Earlier answer."},
				{Role: "user", Content: "Now report and look up."},
				{Role: "assistant", ToolCalls: []ToolCall{
					{ID: "report-3", Name: "a_report", Arguments: map[string]string{"format": "csv", "record": "record three", "target": "archive"}},
					{ID: "lookup-3", Name: "z_lookup", Arguments: map[string]string{"account": "acct-3", "key": "key-3", "region": "east"}},
				}},
				{Role: "tool", ToolResults: []ToolResult{
					{CallID: "report-3", Content: "report stored"},
					{CallID: "lookup-3", Content: "lookup failed", IsError: true},
				}},
				{Role: "user", Content: "Summarize both."},
			},
			wire: `[{"role":"system","content":"Static instructions only."},{"role":"user","content":"Task three; context three."},
				{"role":"assistant","content":"Earlier answer."},{"role":"user","content":"Now report and look up."},
				{"role":"assistant","content":null,"tool_calls":[
					{"id":"report-3","type":"function","function":{"name":"a_report","arguments":"{\"format\":\"csv\",\"record\":\"record three\",\"target\":\"archive\"}"}},
					{"id":"lookup-3","type":"function","function":{"name":"z_lookup","arguments":"{\"account\":\"acct-3\",\"key\":\"key-3\",\"region\":\"east\"}"}}]},
				{"role":"tool","tool_call_id":"report-3","content":"report stored"},{"role":"tool","tool_call_id":"lookup-3","content":"lookup failed"},
				{"role":"user","content":"Summarize both."}]`,
		},
	}
	var firstTools json.RawMessage
	for _, streaming := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			for construction := range repetitions {
				req := &Request{Model: "openai/gpt-5", System: "Static instructions only.", Tools: stableToolSchemaFixture(reverse)}
				for _, cacheEnabled := range []bool{true, false} {
					req.CacheConfig = cacheEnabled
					for _, turn := range turns {
						name := fmt.Sprintf("stream=%t/reverse=%t/construction=%02d/cache=%t/%s", streaming, reverse, construction, cacheEnabled, turn.name)
						t.Run(name, func(t *testing.T) {
							requests := make(chan []byte, 1)
							ready := make(chan struct{}, 1)
							server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
								body, err := io.ReadAll(r.Body)
								if err != nil {
									t.Errorf("read local request: %v", err)
									w.WriteHeader(http.StatusBadRequest)
									return
								}
								var envelope struct {
									Stream bool `json:"stream"`
								}
								if err := json.Unmarshal(body, &envelope); err != nil {
									t.Errorf("decode local request: %v", err)
									w.WriteHeader(http.StatusBadRequest)
									return
								}
								select {
								case requests <- body:
									ready <- struct{}{}
								default:
									t.Error("request capture already full")
									return
								}
								if envelope.Stream {
									w.Header().Set("Content-Type", "text/event-stream")
									_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
									return
								}
								w.Header().Set("Content-Type", "application/json")
								_, _ = io.WriteString(w, `{"model":"openai/gpt-5","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`)
							}))
							defer server.Close()
							o, err := NewOpenRouter("local-test-key", WithOpenRouterBaseURL(server.URL))
							if err != nil {
								t.Fatal(err)
							}
							ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
							defer cancel()
							req.Messages = turn.messages
							before, err := json.Marshal(req)
							if err != nil {
								t.Fatal(err)
							}

							if streaming {
								stream, err := o.SendStream(ctx, req)
								if err != nil {
									t.Fatalf("SendStream: %v", err)
								}
								var text string
								for {
									var event StreamEvent
									event, err = stream.Next()
									if err != nil {
										break
									}
									if event.Type == StreamEventText {
										text += event.Text
									}
								}
								closeErr := stream.Close()
								if err != io.EOF || closeErr != nil {
									t.Fatalf("drain/close stream: %v / %v", err, closeErr)
								}
								if text != "ok" {
									t.Errorf("stream text = %q, want ok", text)
								}
							} else {
								response, err := o.Send(ctx, req)
								if err != nil {
									t.Fatalf("Send: %v", err)
								}
								if response.Content != "ok" {
									t.Errorf("response content = %q, want ok", response.Content)
								}
							}

							after, err := json.Marshal(req)
							if err != nil {
								t.Fatal(err)
							}
							if !bytes.Equal(before, after) {
								t.Errorf("mutated caller tools/history: before %s; after %s", before, after)
							}
							var got map[string]json.RawMessage
							select {
							case <-ready:
							case <-ctx.Done():
								t.Fatalf("request capture: %v", ctx.Err())
							}
							if err := json.Unmarshal(<-requests, &got); err != nil {
								t.Fatal(err)
							}
							var schemas []struct {
								Function struct {
									Name       string `json:"name"`
									Parameters struct {
										Required []string `json:"required"`
									} `json:"parameters"`
								} `json:"function"`
							}
							if err := json.Unmarshal(got["tools"], &schemas); err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(decode(t, got["tools"]), wantTools) {
								t.Errorf("Chat schemas differ from literal: actual names/required=%+v; want z_lookup=[account key region], a_report=[format record target], exactly once/no optional, with literal types/descriptions and caller order", schemas)
							}
							if firstTools == nil {
								firstTools = append(json.RawMessage(nil), got["tools"]...)
							} else if !bytes.Equal(firstTools, got["tools"]) {
								t.Errorf("serialized tools changed across transports/cache choices/turns/constructions/insertion order: actual names/required=%+v", schemas)
							}
							if !reflect.DeepEqual(decode(t, got["messages"]), decode(t, []byte(turn.wire))) {
								t.Errorf("conversation changed: got %s; want %s", got["messages"], turn.wire)
							}
							delete(got, "tools")
							delete(got, "messages")
							metadata, err := json.Marshal(got)
							if err != nil {
								t.Fatal(err)
							}
							wantMetadata := `{"model":"openai/gpt-5"}`
							if streaming {
								wantMetadata = `{"model":"openai/gpt-5","stream":true,"stream_options":{"include_usage":true}}`
							}
							if !reflect.DeepEqual(decode(t, metadata), decode(t, []byte(wantMetadata))) {
								t.Errorf("unexpected adapter metadata/cache controls: got %s; want %s", metadata, wantMetadata)
							}
						})
					}
				}
			}
		}
	}
	t.Logf("exercised 2 transports x 2 insertion orders x %d constructions x 2 cache choices x %d dynamic rows = %d local requests; streams drained and closed", repetitions, len(turns), 2*2*repetitions*2*len(turns))
}

// --- Constructor tests ---

func TestNewOpenRouter(t *testing.T) {
	cases := []struct {
		name           string
		apiKey         string
		opts           []OpenRouterOption
		wantErr        bool
		wantErrMsg     string
		wantBaseURL    string
		wantReferer    string
		wantTitle      string
		wantCategories string
	}{
		{
			name:       "empty API key errors",
			apiKey:     "",
			wantErr:    true,
			wantErrMsg: "API key is required",
		},
		{
			name:        "valid API key",
			apiKey:      "test-key",
			wantBaseURL: defaultOpenRouterBaseURL,
		},
		{
			name:        "custom base URL",
			apiKey:      "test-key",
			opts:        []OpenRouterOption{WithOpenRouterBaseURL("https://custom.example.com/v1")},
			wantBaseURL: "https://custom.example.com/v1",
		},
		{
			name:           "all options set",
			apiKey:         "test-key",
			opts:           []OpenRouterOption{WithReferer("https://example.com"), WithTitle("My App"), WithCategories("cli,agent")},
			wantBaseURL:    defaultOpenRouterBaseURL,
			wantReferer:    "https://example.com",
			wantTitle:      "My App",
			wantCategories: "cli,agent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := NewOpenRouter(tc.apiKey, tc.opts...)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tc.wantErrMsg) {
					t.Errorf("expected error containing %q, got %q", tc.wantErrMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if o == nil {
				t.Fatal("expected non-nil OpenRouter")
			}
			if tc.wantBaseURL != "" && o.baseURL != tc.wantBaseURL {
				t.Errorf("baseURL = %q, want %q", o.baseURL, tc.wantBaseURL)
			}
			if o.referer != tc.wantReferer {
				t.Errorf("referer = %q, want %q", o.referer, tc.wantReferer)
			}
			if o.title != tc.wantTitle {
				t.Errorf("title = %q, want %q", o.title, tc.wantTitle)
			}
			if o.categories != tc.wantCategories {
				t.Errorf("categories = %q, want %q", o.categories, tc.wantCategories)
			}
		})
	}
}

// --- Non-streaming Send tests ---

func TestOpenRouter_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-OpenRouter-Cache-Status", "HIT")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"model": "anthropic/claude-sonnet-4",
			"choices": []map[string]interface{}{
				{
					"message":              map[string]string{"content": "Hello from OpenRouter"},
					"finish_reason":        "stop",
					"native_finish_reason": "end_turn",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"cost":              0.00014,
			},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "anthropic/claude-sonnet-4",
		System:   "You are helpful.",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello from OpenRouter" {
		t.Errorf("expected 'Hello from OpenRouter', got %q", resp.Content)
	}
	if resp.Model != "anthropic/claude-sonnet-4" {
		t.Errorf("expected model 'anthropic/claude-sonnet-4', got %q", resp.Model)
	}
	if resp.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("expected 5 output tokens, got %d", resp.OutputTokens)
	}
	if resp.Cost != 0.00014 {
		t.Errorf("expected cost 0.00014, got %f", resp.Cost)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("expected native stop reason 'end_turn', got %q", resp.StopReason)
	}
	if resp.CacheStatus != "HIT" {
		t.Errorf("expected cache status 'HIT', got %q", resp.CacheStatus)
	}
}

func TestOpenRouter_Send_RequestFormat(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotCT, gotModel string
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		gotModel, _ = gotBody["model"].(string)

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "OK"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "openai/gpt-5",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %q", gotMethod)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("expected path /chat/completions, got %q", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Errorf("expected Bearer auth, got %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("expected application/json, got %q", gotCT)
	}
	if gotModel != "openai/gpt-5" {
		t.Errorf("expected model 'openai/gpt-5', got %q", gotModel)
	}
}

func TestOpenRouter_ReasoningEffortIgnored(t *testing.T) {
	methods := []struct {
		name     string
		isStream bool
		wantBody string
	}{
		{
			name:     "Send",
			wantBody: `{"model":"openai/gpt-5","messages":[{"role":"user","content":"Hi"}]}`,
		},
		{
			name:     "SendStream",
			isStream: true,
			wantBody: `{"model":"openai/gpt-5","messages":[{"role":"user","content":"Hi"}],"stream":true,"stream_options":{"include_usage":true}}`,
		},
	}
	tests := []struct {
		name           string
		effort         string
		temperatureSet bool
	}{
		{name: "empty"},
		{name: "low", effort: "low"},
		{name: "explicit zero", temperatureSet: true},
	}

	for _, method := range methods {
		t.Run(method.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					var gotBody []byte
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						gotBody, _ = io.ReadAll(r.Body)
						if method.isStream {
							w.Header().Set("Content-Type", "text/event-stream")
							_, _ = w.Write([]byte("data: [DONE]\n\n"))
							return
						}
						_ = json.NewEncoder(w).Encode(map[string]interface{}{
							"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
							"usage":   map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
						})
					}))
					defer server.Close()

					o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
					req := &Request{
						Model:           "openai/gpt-5",
						Messages:        []Message{{Role: "user", Content: "Hi"}},
						ReasoningEffort: tt.effort,
						TemperatureSet:  tt.temperatureSet,
					}
					if method.isStream {
						stream, err := o.SendStream(context.Background(), req)
						if err != nil {
							t.Fatalf("unexpected error: %v", err)
						}
						defer func() { _ = stream.Close() }()
					} else if _, err := o.Send(context.Background(), req); err != nil {
						t.Fatalf("unexpected error: %v", err)
					}

					var body map[string]interface{}
					if err := json.Unmarshal(gotBody, &body); err != nil {
						t.Fatalf("failed to parse request body: %v", err)
					}
					if _, ok := body["reasoning_effort"]; ok {
						t.Error("OpenRouter request body should not contain reasoning_effort")
					}
					if _, ok := body["reasoning"]; ok {
						t.Error("OpenRouter request body should not contain a nested reasoning object")
					}
					if _, ok := body["temperature"]; ok {
						t.Error("OpenRouter request body should omit zero temperature even when explicitly set")
					}
					if string(gotBody) != method.wantBody {
						t.Errorf("request body = %s, want %s", gotBody, method.wantBody)
					}
				})
			}
		})
	}
}

func TestOpenRouter_Send_AttributionHeaders(t *testing.T) {
	var gotReferer, gotTitle, gotCategories string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-OpenRouter-Title")
		gotCategories = r.Header.Get("X-OpenRouter-Categories")

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "OK"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key",
		WithOpenRouterBaseURL(server.URL),
		WithReferer("https://github.com/jrswab/axe"),
		WithTitle("Axe CLI"),
		WithCategories("cli-agent"),
	)
	_, err := o.Send(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotReferer != "https://github.com/jrswab/axe" {
		t.Errorf("referer = %q, want %q", gotReferer, "https://github.com/jrswab/axe")
	}
	if gotTitle != "Axe CLI" {
		t.Errorf("title = %q, want %q", gotTitle, "Axe CLI")
	}
	if gotCategories != "cli-agent" {
		t.Errorf("categories = %q, want %q", gotCategories, "cli-agent")
	}
}

func TestOpenRouter_Send_NoAttributionHeadersByDefault(t *testing.T) {
	var gotReferer, gotTitle, gotCategories string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-OpenRouter-Title")
		gotCategories = r.Header.Get("X-OpenRouter-Categories")

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "OK"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	_, err := o.Send(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotReferer != "" {
		t.Errorf("expected no referer, got %q", gotReferer)
	}
	if gotTitle != "" {
		t.Errorf("expected no title, got %q", gotTitle)
	}
	if gotCategories != "" {
		t.Errorf("expected no categories, got %q", gotCategories)
	}
}

func TestOpenRouter_Send_GracefulDegradation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "OK"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				// cost omitted intentionally
			},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Cost != 0 {
		t.Errorf("expected cost 0 when absent, got %f", resp.Cost)
	}
	if resp.CacheStatus != "" {
		t.Errorf("expected empty cache status when header absent, got %q", resp.CacheStatus)
	}
}

func TestOpenRouter_Send_ErrorResponses(t *testing.T) {
	tests := []struct {
		status  int
		wantCat ErrorCategory
	}{
		{400, ErrCategoryBadRequest},
		{401, ErrCategoryAuth},
		{403, ErrCategoryAuth},
		{404, ErrCategoryBadRequest},
		{429, ErrCategoryRateLimit},
		{500, ErrCategoryServer},
		{502, ErrCategoryServer},
		{503, ErrCategoryServer},
	}

	for _, tc := range tests {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{"message": "something went wrong"},
				})
			}))
			defer server.Close()

			o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
			_, err := o.Send(context.Background(), &Request{
				Model:    "test-model",
				Messages: []Message{{Role: "user", Content: "Hello"}},
			})
			if err == nil {
				t.Fatal("expected error")
			}
			provErr, ok := err.(*ProviderError)
			if !ok {
				t.Fatalf("expected *ProviderError, got %T", err)
			}
			if provErr.Category != tc.wantCat {
				t.Errorf("category = %q, want %q", provErr.Category, tc.wantCat)
			}
		})
	}
}

// --- Streaming tests ---

func TestOpenRouter_SendStream_TextDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-OpenRouter-Cache-Status", "MISS")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}

		events := []string{
			`data: {"choices":[{"delta":{"content":"Hello"},"finish_reason":null}],"model":"openai/gpt-5"}`,
			`data: {"choices":[{"delta":{"content":" world"},"finish_reason":null}],"model":"openai/gpt-5"}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
			`data: {"choices":[],"usage":{"prompt_tokens":3,"completion_tokens":2,"cost":0.00001}}`,
			`data: [DONE]`,
		}
		for _, ev := range events {
			_, _ = w.Write([]byte(ev + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "openai/gpt-5",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var texts []string
	var doneEvent *StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		if ev.Type == StreamEventText {
			texts = append(texts, ev.Text)
		}
		if ev.Type == StreamEventDone {
			doneEvent = &ev
		}
	}

	if len(texts) != 2 || texts[0] != "Hello" || texts[1] != " world" {
		t.Errorf("texts = %v, want [Hello,  world]", texts)
	}
	if doneEvent == nil {
		t.Fatal("expected StreamEventDone")
	}
	if doneEvent.InputTokens != 3 {
		t.Errorf("input tokens = %d, want 3", doneEvent.InputTokens)
	}
	if doneEvent.OutputTokens != 2 {
		t.Errorf("output tokens = %d, want 2", doneEvent.OutputTokens)
	}
	if doneEvent.Cost != 0.00001 {
		t.Errorf("cost = %f, want 0.00001", doneEvent.Cost)
	}
	if doneEvent.CacheStatus != "MISS" {
		t.Errorf("cache status = %q, want MISS", doneEvent.CacheStatus)
	}
}

func TestOpenRouter_SendStream_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"message": "invalid key"},
		})
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	_, err := o.SendStream(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	provErr, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected *ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryAuth {
		t.Errorf("category = %q, want %q", provErr.Category, ErrCategoryAuth)
	}
}

func TestOpenRouter_SendStream_ToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}

		events := []string{
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"run_command"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"command\":\"echo hi"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"}"}}]}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
			`data: {"usage":{"prompt_tokens":10,"completion_tokens":5}}`,
			`data: [DONE]`,
		}
		for _, ev := range events {
			_, _ = w.Write([]byte(ev + "\n\n"))
			flusher.Flush()
		}
	}))
	defer server.Close()

	o, _ := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
	stream, err := o.SendStream(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Run echo hi"}},
		Tools: []Tool{{
			Name:        "run_command",
			Description: "Run a shell command",
			Parameters: map[string]ToolParameter{
				"command": {Type: "string", Description: "command to run", Required: true},
			},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var toolStarts int
	var toolEnds int
	var doneEvent *StreamEvent
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		switch ev.Type {
		case StreamEventToolStart:
			toolStarts++
		case StreamEventToolEnd:
			toolEnds++
		case StreamEventDone:
			doneEvent = &ev
		}
	}

	if toolStarts != 1 {
		t.Errorf("tool starts = %d, want 1", toolStarts)
	}
	if toolEnds != 1 {
		t.Errorf("tool ends = %d, want 1", toolEnds)
	}
	if doneEvent == nil {
		t.Fatal("expected StreamEventDone")
	}
}

func TestLargeIntegerArgs_OpenRouter_Buffered(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "threshold integer", raw: `1000000`, want: "1000000"},
		{name: "integer beyond float64 precision", raw: `9007199254740993`, want: "9007199254740993"},
		{name: "integral exponent", raw: `1e6`, want: "1000000"},
		{name: "integral decimal", raw: `1000000.0`, want: "1000000"},
		{name: "negative integral exponent", raw: `-1e6`, want: "-1000000"},
		{name: "exact decimal beyond float64 precision", raw: `9007199254740993.0`, want: "9007199254740993"},
		{name: "exact exponent beyond float64 precision", raw: `90071992547409930e-1`, want: "9007199254740993"},
		{name: "negative threshold integer", raw: `-1000000`, want: "-1000000"},
		{name: "integer beyond int64", raw: `99999999999999999999999`, want: "99999999999999999999999"},
		{name: "integral decimal beyond int64", raw: `99999999999999999999999.0`, want: "99999999999999999999999"},
		{name: "integral exponent beyond int64", raw: `999999999999999999999990e-1`, want: "99999999999999999999999"},
		{name: "below threshold", raw: `999999`, want: "999999"},
		{name: "negative below threshold", raw: `-999999`, want: "-999999"},
		{name: "below threshold decimal", raw: `999999.0`, want: "999999"},
		{name: "zero", raw: `0`, want: "0"},
		{name: "negative zero", raw: `-0`, want: "-0"},
		{name: "negative decimal zero", raw: `-0.0`, want: "-0"},
		{name: "fraction", raw: `1.5`, want: "1.5"},
		{name: "fraction rounded to threshold", raw: `999999.99999999999`, want: "1e+06"},
		{name: "fraction rounded above threshold", raw: `1000000.00000000001`, want: "1e+06"},
		{name: "text", raw: `"text"`, want: "text"},
		{name: "quoted exponent", raw: `"1e6"`, want: "1e6"},
		{name: "quoted decimal", raw: `"1000000.0"`, want: "1000000.0"},
		{name: "quoted large integer", raw: `"9007199254740993"`, want: "9007199254740993"},
		{name: "boolean true", raw: `true`, want: "true"},
		{name: "boolean false", raw: `false`, want: "false"},
		{name: "null", raw: `null`, want: "<nil>"},
		{name: "nested exponent", raw: `{"a":1e6}`, want: "map[a:1e+06]"},
		{name: "nested integer", raw: `{"a":1000000}`, want: "map[a:1e+06]"},
		{name: "nested array", raw: `[1.50]`, want: "[1.5]"},
		{name: "recursive collection", raw: `{"a":[1000000,1.50]}`, want: "map[a:[1e+06 1.5]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Quote only the outer arguments string; keep the inner numeric
			// tokens raw so the fixture cannot round them before Send does.
			arguments, err := json.Marshal(`{"n":` + tt.raw + `}`)
			if err != nil {
				t.Fatalf("encode arguments string: %v", err)
			}
			body := `{"model":"openai/gpt-5","choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"example","arguments":` + string(arguments) + `}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			o, err := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewOpenRouter: %v", err)
			}

			resp, err := o.Send(context.Background(), &Request{
				Model:    "openai/gpt-5",
				Messages: []Message{{Role: "user", Content: "Read values"}},
			})

			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if len(resp.ToolCalls) != 1 {
				t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
			}
			if got, ok := resp.ToolCalls[0].Arguments["n"]; !ok || got != tt.want {
				t.Errorf("Arguments[n] for raw %s = %q (present=%t), want %q", tt.raw, got, ok, tt.want)
			}
		})
	}
}

func TestLargeIntegerArgs_OpenRouter_DecodeErrors(t *testing.T) {
	const envelope = `{"model":"openai/gpt-5","choices":[{"message":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"example","arguments":%s}}]},"finish_reason":"tool_calls"}]}`
	bodyFor := func(raw string) string { return openAIRawArgumentBody(t, envelope, raw) }
	base := bodyFor(`{"n":1000000}`)
	overflow := strings.Repeat("9", 400)
	type testCase struct {
		name, body string
		wantErr    bool
		want       map[string]string
		wantCost   float64
	}
	tests := []testCase{
		{name: "outer malformed", body: `{"choices":`, wantErr: true},
		{name: "outer trailing value", body: base + `{}`, wantErr: true},
		{name: "outer trailing junk", body: base + `junk`, wantErr: true},
		{name: "cost overflow", body: `{"usage":{"cost":1e400},` + base[1:], wantErr: true},
		{name: "cost 400 digits", body: `{"usage":{"cost":` + overflow + `},` + base[1:], wantErr: true},
		{name: "integer overflow", body: `{"usage":{"prompt_tokens":1e400},` + base[1:], wantErr: true},
		{name: "integer 400 digits", body: `{"usage":{"completion_tokens":` + overflow + `},` + base[1:], wantErr: true},
		{name: "overwritten cost", body: `{"usage":{"cost":1e400,"cost":0.1},` + base[1:], wantErr: true},
		{name: "overwritten usage", body: `{"usage":{"cost":1e400},"usage":{"cost":0.1},` + base[1:], wantErr: true},
		{name: "overwritten integer", body: `{"usage":{"prompt_tokens":1e400,"prompt_tokens":1},` + base[1:], wantErr: true},
		{name: "outer type mismatch", body: `{"usage":{"cost":true},` + base[1:], wantErr: true},
		{name: "unknown scalar overflow", body: `{"unknown":1e400,` + base[1:], want: map[string]string{"n": "1000000"}},
		{name: "unknown subtree overflow", body: `{"unknown":{"bad":1e400},` + base[1:], want: map[string]string{"n": "1000000"}},
		{name: "unknown 400 digits", body: `{"unknown":` + overflow + `,` + base[1:], want: map[string]string{"n": "1000000"}},
		{name: "successful typed duplicate", body: `{"usage":{"cost":0.1,"cost":0.2},` + base[1:], want: map[string]string{"n": "1000000"}, wantCost: 0.2},
		{name: "successful typed subtree duplicate", body: `{"usage":{"cost":0.1},"usage":{"cost":0.2},` + base[1:], want: map[string]string{"n": "1000000"}, wantCost: 0.2},
	}
	for _, tt := range []struct {
		name, raw string
		want      map[string]string
	}{
		{name: "inner malformed", raw: `{"n":`},
		{name: "inner trailing value", raw: `{"n":1} {}`},
		{name: "inner trailing junk", raw: `{"n":1} junk`},
		{name: "inner overflow", raw: `{"n":1e400}`},
		{name: "inner 400 digits", raw: `{"n":` + overflow + `}`},
		{name: "inner nested overflow", raw: `{"n":[{"bad":1e400}]}`},
		{name: "inner nested 400 digits", raw: `{"n":{"bad":` + overflow + `}}`},
		{name: "inner overwritten scalar", raw: `{"n":1e400,"n":1000000}`},
		{name: "inner overwritten subtree", raw: `{"n":{"bad":1e400},"n":1000000}`},
		{name: "inner duplicate scalar", raw: `{"n":999999,"n":1000000}`, want: map[string]string{"n": "1000000"}},
		{name: "inner duplicate subtree", raw: `{"n":{"ok":1},"n":1000000}`, want: map[string]string{"n": "1000000"}},
		{name: "inner map replacement", raw: `{"n":{"old":1},"n":{"new":2}}`, want: map[string]string{"n": "map[new:2]"}},
		{name: "inner empty", raw: `{}`},
		{name: "inner null", raw: `null`},
		{name: "inner array", raw: `[]`},
		{name: "inner scalar", raw: `0`},
		{name: "inner negative zero", raw: `-0`},
		{name: "inner decimal zero", raw: `-0.0`},
		{name: "inner string", raw: `"text"`},
		{name: "inner boolean", raw: `true`},
	} {
		tests = append(tests, testCase{name: tt.name, body: bodyFor(tt.raw), want: tt.want})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			o, err := NewOpenRouter("test-key", WithOpenRouterBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := o.Send(context.Background(), &Request{Model: "openai/gpt-5"})
			if tt.wantErr {
				providerErr, ok := err.(*ProviderError)
				if !ok || providerErr.Category != ErrCategoryServer {
					t.Fatalf("Send error = %v, want server ProviderError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Send error = %v, want successful response", err)
			}
			if len(resp.ToolCalls) != 1 {
				t.Fatalf("ToolCalls = %#v, want one call", resp.ToolCalls)
			}
			want := tt.want
			if want == nil {
				want = map[string]string{}
			}
			if resp.Cost != tt.wantCost {
				t.Errorf("Cost = %v, want %v", resp.Cost, tt.wantCost)
			}
			got := resp.ToolCalls[0].Arguments
			if len(got) != len(want) {
				t.Errorf("Arguments = %#v, want %#v (no +Inf)", got, want)
			}
			for key, value := range want {
				if gotValue, ok := got[key]; !ok || gotValue != value {
					t.Errorf("Arguments[%s] = %q (present=%t), want %q", key, gotValue, ok, value)
				}
			}
		})
	}
}
