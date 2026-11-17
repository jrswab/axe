package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewLiteLLM(t *testing.T) {
	type input struct {
		apiKey  string
		baseURL string
	}
	type want struct {
		baseURL     string
		errCategory ErrorCategory
		errContains string
	}

	tests := []struct {
		name  string
		input input
		want  want
	}{
		{
			name:  "default base URL",
			input: input{apiKey: "sk-virtual"},
			want:  want{baseURL: defaultLiteLLMBaseURL},
		},
		{
			name:  "proxy root gets /v1",
			input: input{apiKey: "sk-virtual", baseURL: "http://litellm.internal:4000"},
			want:  want{baseURL: "http://litellm.internal:4000/v1"},
		},
		{
			name:  "trailing slash and /v1 kept once",
			input: input{apiKey: "sk-virtual", baseURL: "https://gateway.example.com/v1/"},
			want:  want{baseURL: "https://gateway.example.com/v1"},
		},
		{
			name:  "missing API key",
			input: input{baseURL: "http://localhost:4000"},
			want: want{
				errCategory: ErrCategoryAuth,
				errContains: "LITELLM_API_KEY",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewLiteLLM(tt.input.apiKey, tt.input.baseURL)
			if tt.want.errCategory != "" {
				var providerErr *ProviderError
				if !errors.As(err, &providerErr) {
					t.Fatalf("expected ProviderError, got %T (%v)", err, err)
				}
				if providerErr.Category != tt.want.errCategory {
					t.Errorf("error category = %q, want %q", providerErr.Category, tt.want.errCategory)
				}
				if !strings.Contains(providerErr.Message, tt.want.errContains) {
					t.Errorf("error message = %q, want it to contain %q", providerErr.Message, tt.want.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.baseURL != tt.want.baseURL {
				t.Errorf("baseURL = %q, want %q", p.baseURL, tt.want.baseURL)
			}
			if _, ok := interface{}(p).(StreamProvider); !ok {
				t.Error("expected LiteLLM to support streaming")
			}
		})
	}
}

func TestLiteLLM_Send_ReportsProxyCost(t *testing.T) {
	tests := []struct {
		name       string
		costHeader string
		wantCost   float64
	}{
		{name: "cost header", costHeader: "0.000412", wantCost: 0.000412},
		{name: "missing header", costHeader: "", wantCost: 0},
		{name: "unparsable header", costHeader: "n/a", wantCost: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotAuth, gotModel string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotAuth = r.Header.Get("Authorization")
				var body map[string]interface{}
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &body)
				gotModel, _ = body["model"].(string)
				if tt.costHeader != "" {
					w.Header().Set(liteLLMCostHeader, tt.costHeader)
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"model":   "claude-sonnet",
					"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
					"usage":   map[string]interface{}{"prompt_tokens": 12, "completion_tokens": 3},
				})
			}))
			defer server.Close()

			p, err := NewLiteLLM("sk-virtual", server.URL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			resp, err := p.Send(context.Background(), &Request{
				Model:    "claude-sonnet",
				Messages: []Message{{Role: "user", Content: "Hi"}},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPath != "/v1/chat/completions" {
				t.Errorf("path = %q, want /v1/chat/completions", gotPath)
			}
			if gotAuth != "Bearer sk-virtual" {
				t.Errorf("Authorization = %q, want Bearer sk-virtual", gotAuth)
			}
			if gotModel != "claude-sonnet" {
				t.Errorf("model = %q, want the proxy alias", gotModel)
			}
			if resp.Cost != tt.wantCost {
				t.Errorf("Cost = %v, want %v", resp.Cost, tt.wantCost)
			}
			if resp.InputTokens != 12 || resp.OutputTokens != 3 {
				t.Errorf("tokens = %d/%d, want 12/3", resp.InputTokens, resp.OutputTokens)
			}
		})
	}
}

func TestLiteLLM_SendStream_ReportsProxyCostOnDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set(liteLLMCostHeader, "0.0021")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":1}}\n\n" +
			"data: [DONE]\n\n"))
	}))
	defer server.Close()

	p, _ := NewLiteLLM("sk-virtual", server.URL)
	stream, err := p.SendStream(context.Background(), &Request{
		Model:    "claude-sonnet",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var text string
	var done *StreamEvent
	for {
		ev, err := stream.Next()
		if err != nil {
			break
		}
		if ev.Type == StreamEventText {
			text += ev.Text
		}
		if ev.Type == StreamEventDone {
			evCopy := ev
			done = &evCopy
		}
	}
	if text != "hi" {
		t.Errorf("text = %q, want hi", text)
	}
	if done == nil {
		t.Fatal("expected a done event")
	}
	if done.Cost != 0.0021 || done.InputTokens != 4 || done.StopReason != "stop" {
		t.Errorf("done = cost %v tokens %d stop %q, want 0.0021/4/stop", done.Cost, done.InputTokens, done.StopReason)
	}
}

func TestOpenAI_IgnoresCostHeaderUnlessConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(liteLLMCostHeader, "1.5")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}, "finish_reason": "stop"}},
		})
	}))
	defer server.Close()

	o, _ := NewOpenAI("test-key", WithOpenAIBaseURL(server.URL))
	resp, err := o.Send(context.Background(), &Request{Model: "gpt-4o", Messages: []Message{{Role: "user", Content: "Hi"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Cost != 0 {
		t.Errorf("Cost = %v, want 0 for plain OpenAI", resp.Cost)
	}
}

// drainStream collects every event until EOF.
func drainStream(t *testing.T, stream *EventStream) []StreamEvent {
	t.Helper()
	var events []StreamEvent
	for {
		ev, err := stream.Next()
		if err != nil {
			if err != io.EOF {
				t.Fatalf("unexpected stream error: %v", err)
			}
			return events
		}
		events = append(events, ev)
	}
}

func TestOpenAIStream_WholeToolCallsInOneChunk(t *testing.T) {
	// Gemini via LiteLLM sends id, name and complete arguments in one delta,
	// and can put several calls in the same chunk.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"tool_calls":[` +
			`{"index":0,"id":"call_a","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"a.csv\"}"}},` +
			`{"index":1,"id":"call_b","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"b.csv\"}"}}]},` +
			`"finish_reason":"tool_calls"}]}` + "\n\ndata: [DONE]\n\n"))
	}))
	defer server.Close()

	p, _ := NewLiteLLM("sk-virtual", server.URL)
	stream, err := p.SendStream(context.Background(), &Request{Model: "gemini-2.5-flash", Messages: []Message{{Role: "user", Content: "Hi"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = stream.Close() }()

	var got []string
	for _, ev := range drainStream(t, stream) {
		switch ev.Type {
		case StreamEventToolStart:
			got = append(got, "start:"+ev.ToolCallID+":"+ev.ToolName)
		case StreamEventToolDelta:
			got = append(got, "delta:"+ev.ToolCallID+":"+ev.ToolInput)
		case StreamEventToolEnd:
			got = append(got, "end:"+ev.ToolCallID)
		case StreamEventDone:
			got = append(got, "done:"+ev.StopReason)
		}
	}
	want := []string{
		"start:call_a:read_file", `delta:call_a:{"path":"a.csv"}`,
		"start:call_b:read_file", `delta:call_b:{"path":"b.csv"}`,
		"end:call_a", "end:call_b", "done:tool_calls",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("events =\n%v\nwant\n%v", got, want)
	}
}

func TestOpenAIStream_UsageOnAChunkWithChoices(t *testing.T) {
	// LiteLLM puts the usage block, with its computed cost, on a chunk that
	// still carries an empty choice.
	body := "data: {\"choices\":[{\"delta\":{\"content\":\"425\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{}}],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":50,\"prompt_tokens_details\":{\"cached_tokens\":4},\"cost\":0.0001286}}\n\n" +
		"data: [DONE]\n\n"

	tests := []struct {
		name     string
		newProv  func(url string) StreamProvider
		wantCost float64
	}{
		{
			name:     "litellm reads cost from usage",
			newProv:  func(url string) StreamProvider { p, _ := NewLiteLLM("sk-virtual", url); return p },
			wantCost: 0.0001286,
		},
		{
			name:     "plain openai ignores usage cost",
			newProv:  func(url string) StreamProvider { p, _ := NewOpenAI("test-key", WithOpenAIBaseURL(url)); return p },
			wantCost: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			stream, err := tt.newProv(server.URL).SendStream(context.Background(), &Request{Model: "m", Messages: []Message{{Role: "user", Content: "Hi"}}})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer func() { _ = stream.Close() }()

			var done *StreamEvent
			for _, ev := range drainStream(t, stream) {
				if ev.Type == StreamEventDone {
					evCopy := ev
					done = &evCopy
				}
			}
			if done == nil {
				t.Fatal("expected a done event")
			}
			if done.InputTokens != 12 || done.OutputTokens != 50 || done.CacheReadTokens != 4 {
				t.Errorf("tokens = %d/%d/%d, want 12/50/4", done.InputTokens, done.OutputTokens, done.CacheReadTokens)
			}
			if done.StopReason != "stop" {
				t.Errorf("stop = %q, want stop", done.StopReason)
			}
			if done.Cost != tt.wantCost {
				t.Errorf("cost = %v, want %v", done.Cost, tt.wantCost)
			}
		})
	}
}
