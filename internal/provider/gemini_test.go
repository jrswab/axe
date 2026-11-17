package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrswab/axe/internal/testutil"
)

func TestNewGemini_EmptyAPIKey(t *testing.T) {
	_, err := NewGemini("")
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "API key is required") {
		t.Errorf("expected 'API key is required', got %q", err.Error())
	}
}

func TestNewGemini_ValidAPIKey(t *testing.T) {
	g, err := NewGemini("test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g == nil {
		t.Fatal("expected non-nil Gemini provider")
	}
}

func TestGemini_Send_Success(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiResponse("Hello from Gemini"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		System:   "You are helpful.",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello from Gemini" {
		t.Errorf("expected 'Hello from Gemini', got %q", resp.Content)
	}
	if resp.Model != "gemini-2.0-flash" {
		t.Errorf("expected model 'gemini-2.0-flash', got %q", resp.Model)
	}
	if resp.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 5 {
		t.Errorf("expected 5 output tokens, got %d", resp.OutputTokens)
	}
	if resp.StopReason != "STOP" {
		t.Errorf("expected stop reason 'STOP', got %q", resp.StopReason)
	}
}

func TestGemini_Send_RequestFormat(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiResponse("ok"),
	})

	g, err := NewGemini("my-api-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.Requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(mock.Requests))
	}

	req := mock.Requests[0]

	// Check method
	if req.Method != "POST" {
		t.Errorf("expected POST, got %s", req.Method)
	}

	// Check URL path
	expectedPath := "/v1beta/models/gemini-2.0-flash:generateContent"
	if req.Path != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, req.Path)
	}

	// Check headers
	if got := req.Headers.Get("x-goog-api-key"); got != "my-api-key" {
		t.Errorf("expected x-goog-api-key 'my-api-key', got %q", got)
	}
	if got := req.Headers.Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", got)
	}

	// Check body structure
	var bodyMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(req.Body), &bodyMap); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	if _, ok := bodyMap["contents"]; !ok {
		t.Error("expected 'contents' field in request body")
	}

	// Model should NOT be in the body
	if _, ok := bodyMap["model"]; ok {
		t.Error("model should not be in request body (it's in the URL path)")
	}
}

func TestGemini_Send_SystemInstruction(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiResponse("ok"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		System:   "Be concise.",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var bodyMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(mock.Requests[0].Body), &bodyMap); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	sysRaw, ok := bodyMap["system_instruction"]
	if !ok {
		t.Fatal("expected 'system_instruction' field in request body")
	}

	var sysInstr struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}
	if err := json.Unmarshal(sysRaw, &sysInstr); err != nil {
		t.Fatalf("failed to parse system_instruction: %v", err)
	}

	if len(sysInstr.Parts) != 1 || sysInstr.Parts[0].Text != "Be concise." {
		t.Errorf("expected system_instruction text 'Be concise.', got %+v", sysInstr)
	}
}

func TestGemini_Send_OmitsEmptySystem(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiResponse("ok"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		System:   "",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var bodyMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(mock.Requests[0].Body), &bodyMap); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	if _, ok := bodyMap["system_instruction"]; ok {
		t.Error("expected 'system_instruction' to be absent when System is empty")
	}
}

func TestGemini_Send_OmitsGenerationConfig(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiResponse("ok"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:       "gemini-2.0-flash",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0,
		MaxTokens:   0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var bodyMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(mock.Requests[0].Body), &bodyMap); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	if _, ok := bodyMap["generationConfig"]; ok {
		t.Error("expected 'generationConfig' to be absent when both Temperature and MaxTokens are 0")
	}
}

func TestGemini_Send_OmitsZeroTemperature(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiResponse("ok"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:       "gemini-2.0-flash",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0,
		MaxTokens:   100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var bodyMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(mock.Requests[0].Body), &bodyMap); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	genCfgRaw, ok := bodyMap["generationConfig"]
	if !ok {
		t.Fatal("expected 'generationConfig' to be present when MaxTokens is non-zero")
	}

	var genCfg map[string]json.RawMessage
	if err := json.Unmarshal(genCfgRaw, &genCfg); err != nil {
		t.Fatalf("failed to parse generationConfig: %v", err)
	}

	if _, ok := genCfg["temperature"]; ok {
		t.Error("expected 'temperature' to be absent when Temperature is 0")
	}
	if _, ok := genCfg["maxOutputTokens"]; !ok {
		t.Error("expected 'maxOutputTokens' to be present")
	}
}

func TestGemini_IgnoresReasoningEffortAndExplicitZeroTemperature(t *testing.T) {
	cases := []struct {
		name   string
		stream bool
	}{
		{"Send", false},
		{"SendStream", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				gotBody, err = io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("failed to read request body: %v", err)
				}
				response := `{"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2}}`
				if tc.stream {
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = fmt.Fprint(w, geminiSSEChunk(response))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(response))
			}))
			defer server.Close()

			g, err := NewGemini("test-key", WithGeminiBaseURL(server.URL))
			if err != nil {
				t.Fatalf("unexpected constructor error: %v", err)
			}
			req := &Request{
				Model:           "gemini-2.0-flash",
				Messages:        []Message{{Role: "user", Content: "Hi"}},
				ReasoningEffort: "low",
				TemperatureSet:  true,
				Temperature:     0,
				MaxTokens:       100,
			}
			if tc.stream {
				stream, err := g.SendStream(context.Background(), req)
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
			} else if _, err := g.Send(context.Background(), req); err != nil {
				t.Fatalf("unexpected Send error: %v", err)
			}

			var body map[string]interface{}
			if err := json.Unmarshal(gotBody, &body); err != nil {
				t.Fatalf("failed to parse request body: %v", err)
			}
			genCfg, ok := body["generationConfig"].(map[string]interface{})
			if !ok || genCfg["maxOutputTokens"] != float64(100) {
				t.Errorf("generationConfig = %v, want maxOutputTokens 100", body["generationConfig"])
			}
			for _, key := range []string{"reasoning_effort", "temperature"} {
				if strings.Contains(string(gotBody), `"`+key+`"`) {
					t.Errorf("body contains %q: %s", key, gotBody)
				}
			}
		})
	}
}

func TestGemini_Send_IncludesMaxTokens(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiResponse("ok"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:     "gemini-2.0-flash",
		Messages:  []Message{{Role: "user", Content: "Hi"}},
		MaxTokens: 256,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var bodyMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(mock.Requests[0].Body), &bodyMap); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	genCfgRaw, ok := bodyMap["generationConfig"]
	if !ok {
		t.Fatal("expected 'generationConfig' to be present")
	}

	var genCfg struct {
		MaxOutputTokens int `json:"maxOutputTokens"`
	}
	if err := json.Unmarshal(genCfgRaw, &genCfg); err != nil {
		t.Fatalf("failed to parse generationConfig: %v", err)
	}

	if genCfg.MaxOutputTokens != 256 {
		t.Errorf("expected maxOutputTokens 256, got %d", genCfg.MaxOutputTokens)
	}
}

func TestGemini_Send_IncludesTemperature(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiResponse("ok"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:       "gemini-2.0-flash",
		Messages:    []Message{{Role: "user", Content: "Hi"}},
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var bodyMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(mock.Requests[0].Body), &bodyMap); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	genCfgRaw, ok := bodyMap["generationConfig"]
	if !ok {
		t.Fatal("expected 'generationConfig' to be present")
	}

	var genCfg struct {
		Temperature float64 `json:"temperature"`
	}
	if err := json.Unmarshal(genCfgRaw, &genCfg); err != nil {
		t.Fatalf("failed to parse generationConfig: %v", err)
	}

	if genCfg.Temperature != 0.7 {
		t.Errorf("expected temperature 0.7, got %f", genCfg.Temperature)
	}
}

func TestGemini_Send_RoleMapping(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiResponse("ok"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model: "gemini-2.0-flash",
		Messages: []Message{
			{Role: "user", Content: "Hi"},
			{Role: "assistant", Content: "Hello!"},
			{Role: "user", Content: "How are you?"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body struct {
		Contents []struct {
			Role string `json:"role"`
		} `json:"contents"`
	}
	if err := json.Unmarshal([]byte(mock.Requests[0].Body), &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	if len(body.Contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(body.Contents))
	}
	if body.Contents[0].Role != "user" {
		t.Errorf("expected first message role 'user', got %q", body.Contents[0].Role)
	}
	if body.Contents[1].Role != "model" {
		t.Errorf("expected second message role 'model', got %q", body.Contents[1].Role)
	}
	if body.Contents[2].Role != "user" {
		t.Errorf("expected third message role 'user', got %q", body.Contents[2].Role)
	}
}

func TestGemini_Send_ToolCallResponse(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiToolCallResponse("thinking...", []testutil.MockToolCall{
			{Name: "get_weather", Input: map[string]string{"city": "NYC"}},
			{Name: "get_time", Input: map[string]string{"tz": "EST"}},
		}),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "What's the weather?"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "thinking..." {
		t.Errorf("expected content 'thinking...', got %q", resp.Content)
	}

	if len(resp.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(resp.ToolCalls))
	}

	if resp.ToolCalls[0].ID != "gemini_0" {
		t.Errorf("expected first tool call ID 'gemini_0', got %q", resp.ToolCalls[0].ID)
	}
	if resp.ToolCalls[0].Name != "get_weather" {
		t.Errorf("expected first tool call name 'get_weather', got %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Arguments["city"] != "NYC" {
		t.Errorf("expected city 'NYC', got %q", resp.ToolCalls[0].Arguments["city"])
	}

	if resp.ToolCalls[1].ID != "gemini_1" {
		t.Errorf("expected second tool call ID 'gemini_1', got %q", resp.ToolCalls[1].ID)
	}
	if resp.ToolCalls[1].Name != "get_time" {
		t.Errorf("expected second tool call name 'get_time', got %q", resp.ToolCalls[1].Name)
	}
}

func TestGemini_Send_ToolDefinitions(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiResponse("ok"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Tools: []Tool{
			{
				Name:        "get_weather",
				Description: "Get weather for a city",
				Parameters: map[string]ToolParameter{
					"city": {Type: "string", Description: "City name", Required: true},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body struct {
		Tools []struct {
			FunctionDeclarations []struct {
				Name        string                 `json:"name"`
				Description string                 `json:"description"`
				Parameters  map[string]interface{} `json:"parameters"`
			} `json:"functionDeclarations"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(mock.Requests[0].Body), &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	if len(body.Tools) != 1 {
		t.Fatalf("expected 1 tool def, got %d", len(body.Tools))
	}
	if len(body.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("expected 1 function declaration, got %d", len(body.Tools[0].FunctionDeclarations))
	}

	decl := body.Tools[0].FunctionDeclarations[0]
	if decl.Name != "get_weather" {
		t.Errorf("expected name 'get_weather', got %q", decl.Name)
	}
	if decl.Description != "Get weather for a city" {
		t.Errorf("expected description 'Get weather for a city', got %q", decl.Description)
	}
}

func TestGemini_Send_ToolResults(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiResponse("The weather is sunny."),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model: "gemini-2.0-flash",
		Messages: []Message{
			{Role: "user", Content: "What's the weather?"},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ToolCall{
					{ID: "call_1", Name: "get_weather", Arguments: map[string]string{"city": "NYC"}},
				},
			},
			{
				Role: "tool",
				ToolResults: []ToolResult{
					{CallID: "call_1", Content: "Sunny, 72°F"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text         string `json:"text,omitempty"`
				FunctionCall *struct {
					Name string                 `json:"name"`
					Args map[string]interface{} `json:"args"`
				} `json:"functionCall,omitempty"`
				FunctionResponse *struct {
					Name     string `json:"name"`
					Response struct {
						Result string `json:"result"`
					} `json:"response"`
				} `json:"functionResponse,omitempty"`
			} `json:"parts"`
		} `json:"contents"`
	}
	if err := json.Unmarshal([]byte(mock.Requests[0].Body), &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}

	if len(body.Contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(body.Contents))
	}

	// Third message should be tool result as user with functionResponse
	toolResultMsg := body.Contents[2]
	if toolResultMsg.Role != "user" {
		t.Errorf("expected tool result role 'user', got %q", toolResultMsg.Role)
	}
	if len(toolResultMsg.Parts) != 1 {
		t.Fatalf("expected 1 part in tool result, got %d", len(toolResultMsg.Parts))
	}
	if toolResultMsg.Parts[0].FunctionResponse == nil {
		t.Fatal("expected functionResponse part")
	}
	if toolResultMsg.Parts[0].FunctionResponse.Name != "get_weather" {
		t.Errorf("expected function response name 'get_weather', got %q", toolResultMsg.Parts[0].FunctionResponse.Name)
	}
	if toolResultMsg.Parts[0].FunctionResponse.Response.Result != "Sunny, 72°F" {
		t.Errorf("expected result 'Sunny, 72°F', got %q", toolResultMsg.Parts[0].FunctionResponse.Response.Result)
	}
}

func TestGemini_Send_MixedContentAndToolCalls(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiToolCallResponse("Let me check", []testutil.MockToolCall{
			{Name: "search", Input: map[string]string{"q": "test"}},
		}),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Search for test"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Let me check" {
		t.Errorf("expected content 'Let me check', got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "search" {
		t.Errorf("expected tool call name 'search', got %q", resp.ToolCalls[0].Name)
	}
}

func TestGemini_Send_AuthError(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiErrorResponse(401, "Invalid API key"),
	})

	g, err := NewGemini("bad-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryAuth {
		t.Errorf("expected ErrCategoryAuth, got %q", provErr.Category)
	}
}

func TestGemini_Send_ForbiddenError(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiErrorResponse(403, "Forbidden"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryAuth {
		t.Errorf("expected ErrCategoryAuth, got %q", provErr.Category)
	}
}

func TestGemini_Send_NotFoundError(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiErrorResponse(404, "Model not found"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-nonexistent",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryBadRequest {
		t.Errorf("expected ErrCategoryBadRequest, got %q", provErr.Category)
	}
}

func TestGemini_Send_RateLimitError(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiErrorResponse(429, "Rate limit exceeded"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryRateLimit {
		t.Errorf("expected ErrCategoryRateLimit, got %q", provErr.Category)
	}
}

func TestGemini_Send_ServerError(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiErrorResponse(500, "Internal server error"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %q", provErr.Category)
	}
}

func TestGemini_Send_Timeout(t *testing.T) {
	// Use a raw httptest server with a long delay to trigger timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	g, err := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = g.Send(ctx, &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for timeout")
	}

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if provErr.Category != ErrCategoryTimeout {
		t.Errorf("expected ErrCategoryTimeout, got %q", provErr.Category)
	}
}

func TestGemini_Send_EmptyCandidates(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		{StatusCode: 200, Body: `{"candidates":[],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":0,"totalTokenCount":10}}`},
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error for empty candidates")
	}

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %q", provErr.Category)
	}
	if !strings.Contains(provErr.Message, "response contains no candidates") {
		t.Errorf("expected message about no candidates, got %q", provErr.Message)
	}
}

func TestGemini_Send_ErrorResponseParsing(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		testutil.GeminiErrorResponse(400, "Invalid request: missing model"),
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if !strings.Contains(provErr.Message, "Invalid request: missing model") {
		t.Errorf("expected error message from JSON, got %q", provErr.Message)
	}
}

func TestGemini_Send_UnparseableErrorBody(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		{StatusCode: 500, Body: "not json at all"},
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	// Should fall back to HTTP status text
	if provErr.Message != "Internal Server Error" {
		t.Errorf("expected 'Internal Server Error', got %q", provErr.Message)
	}
}

// --- Streaming Tests ---

func geminiSSEChunk(jsonData string) string {
	return "data: " + jsonData + "\n\n"
}

func TestGemini_SendStream_RequestFormat(t *testing.T) {
	var capturedPath string
	var capturedAPIKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path + "?" + r.URL.RawQuery
		capturedAPIKey = r.Header.Get("x-goog-api-key")

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2}}`))
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	stream, err := g.SendStream(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

	expectedPath := "/v1beta/models/gemini-2.0-flash:streamGenerateContent?alt=sse"
	if capturedPath != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, capturedPath)
	}
	if capturedAPIKey != "test-key" {
		t.Errorf("expected API key 'test-key', got %q", capturedAPIKey)
	}
}

func TestGemini_SendStream_TextDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]}}]}`))
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"text":" "}]}}]}`))
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"text":"world"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":20}}`))
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	stream, err := g.SendStream(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
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
			t.Fatalf("unexpected stream error: %v", err)
		}
		events = append(events, ev)
	}

	// 3 text + 1 done
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	if events[0].Type != StreamEventText || events[0].Text != "Hello" {
		t.Errorf("event 0: expected text 'Hello', got type=%s text=%q", events[0].Type, events[0].Text)
	}
	if events[1].Type != StreamEventText || events[1].Text != " " {
		t.Errorf("event 1: expected text ' ', got type=%s text=%q", events[1].Type, events[1].Text)
	}
	if events[2].Type != StreamEventText || events[2].Text != "world" {
		t.Errorf("event 2: expected text 'world', got type=%s text=%q", events[2].Type, events[2].Text)
	}
	if events[3].Type != StreamEventDone {
		t.Errorf("event 3: expected done, got type=%s", events[3].Type)
	}
	if events[3].StopReason != "STOP" {
		t.Errorf("expected stop reason 'STOP', got %q", events[3].StopReason)
	}
	if events[3].InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", events[3].InputTokens)
	}
	if events[3].OutputTokens != 20 {
		t.Errorf("expected 20 output tokens, got %d", events[3].OutputTokens)
	}
}

func TestGemini_SendStream_ToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"city":"NYC"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3}}`))
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	stream, err := g.SendStream(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Weather?"}},
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
			t.Fatalf("unexpected stream error: %v", err)
		}
		events = append(events, ev)
	}

	// tool_start + tool_end + done = 3
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d: %+v", len(events), events)
	}

	if events[0].Type != StreamEventToolStart || events[0].ToolName != "get_weather" {
		t.Errorf("event 0: expected tool_start get_weather, got type=%s name=%s", events[0].Type, events[0].ToolName)
	}
	if events[0].ToolCallID != "gemini_0" {
		t.Errorf("event 0: expected ID 'gemini_0', got %q", events[0].ToolCallID)
	}
	if events[1].Type != StreamEventToolEnd || events[1].ToolCallID != "gemini_0" {
		t.Errorf("event 1: expected tool_end gemini_0, got type=%s id=%s", events[1].Type, events[1].ToolCallID)
	}
	if events[2].Type != StreamEventDone {
		t.Errorf("event 2: expected done, got type=%s", events[2].Type)
	}
}

func TestLargeIntegerArgs_Gemini_NativeToolInput(t *testing.T) {
	// Arrange a native JSON args object at the HTTP boundary, not string-valued tool inputs.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"example","args":{"million":1000000,"precise":9007199254740993,"enabled":true,"disabled":false,"nothing":null,"object":{"a":1000000},"array":[1,true,null,{"a":2}]}}}]},"finishReason":"STOP"}]}`))
	}))
	defer server.Close()
	g, err := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewGemini: %v", err)
	}

	// Act through the real provider and consume its public event stream to EOF.
	stream, err := g.SendStream(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Read values"}},
	})
	if err != nil {
		t.Fatalf("SendStream: %v", err)
	}
	defer func() { _ = stream.Close() }()
	var inputs []string
	for {
		ev, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream.Next: %v", err)
		}
		if ev.Type == StreamEventToolStart {
			if ev.ToolName != "example" {
				t.Errorf("ToolName = %q, want example", ev.ToolName)
			}
			inputs = append(inputs, ev.ToolInput)
		}
	}

	// Assert raw tokens so neither float64 decoding nor string coercion can hide type/digit loss.
	if len(inputs) != 1 {
		t.Fatalf("got %d tool inputs, want 1", len(inputs))
	}
	var args map[string]json.RawMessage
	if err := json.Unmarshal([]byte(inputs[0]), &args); err != nil {
		t.Fatalf("ToolInput = %q, want JSON object: %v", inputs[0], err)
	}
	wants := []struct {
		key, token string
	}{
		{key: "million", token: `1000000`},
		{key: "precise", token: `9007199254740993`},
		{key: "enabled", token: `true`},
		{key: "disabled", token: `false`},
		{key: "nothing", token: `null`},
		{key: "object", token: `{"a":1000000}`},
		{key: "array", token: `[1,true,null,{"a":2}]`},
	}
	if len(args) != len(wants) {
		t.Errorf("ToolInput has %d arguments, want %d: %s", len(args), len(wants), inputs[0])
	}
	for _, want := range wants {
		if got := string(args[want.key]); got != want.token {
			t.Errorf("ToolInput[%s] = %s, want native JSON token %s (not a quoted/flattened string)", want.key, got, want.token)
		}
	}
}

func TestLargeIntegerArgs_Gemini_StreamErrors(t *testing.T) {
	bodyFor := func(args string) string {
		return `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"example","args":` + args + `}}]}}],"usageMetadata":{"promptTokenCount":10}}`
	}
	base := bodyFor(`{"n":"text"}`)
	overflow := "1" + strings.Repeat("0", 399)
	tests := []struct {
		name, body, input string
		wantErr           bool
	}{
		{name: "malformed_response", body: `{"candidates":`, wantErr: true},
		{name: "malformed_args", body: bodyFor(`{"n":`), wantErr: true},
		{name: "trailing_value", body: base + `{}`, wantErr: true},
		{name: "trailing_junk", body: base + `junk`, wantErr: true},
		{name: "overflow", body: bodyFor(`{"n":1e400}`), wantErr: true},
		{name: "400_digits", body: bodyFor(`{"n":` + overflow + `}`), wantErr: true},
		{name: "nested_overflow", body: bodyFor(`{"n":[{"bad":1e400}]}`), wantErr: true},
		{name: "nested_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `}}`), wantErr: true},
		{name: "overwritten_scalar_overflow", body: bodyFor(`{"n":1e400,"n":1000000}`), wantErr: true},
		{name: "overwritten_subtree_overflow", body: bodyFor(`{"n":{"bad":1e400},"n":1000000}`), wantErr: true},
		{name: "overwritten_scalar_400_digits", body: bodyFor(`{"n":` + overflow + `,"n":1000000}`), wantErr: true},
		{name: "overwritten_subtree_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `},"n":1000000}`), wantErr: true},
		{name: "scalar_duplicate", body: bodyFor(`{"n":999999,"n":1000000}`), input: `{"n":1000000}`},
		{name: "subtree_duplicate", body: bodyFor(`{"n":{"ok":1},"n":1000000}`), input: `{"n":1000000}`},
		{name: "empty_args", body: bodyFor(`{}`), input: `{}`},
		{name: "null_args", body: bodyFor(`null`), input: `null`},
		{name: "array_args", body: bodyFor(`[]`), wantErr: true},
		{name: "number_args", body: bodyFor(`0`), wantErr: true},
		{name: "negative_zero_args", body: bodyFor(`-0`), wantErr: true},
		{name: "decimal_zero_args", body: bodyFor(`-0.0`), wantErr: true},
		{name: "string_args", body: bodyFor(`"text"`), wantErr: true},
		{name: "bool_args", body: bodyFor(`true`), wantErr: true},
		{name: "usage_overflow", body: `{"usageMetadata":{"promptTokenCount":1e400},` + base[1:], wantErr: true},
		{name: "usage_400_digits", body: `{"usageMetadata":{"promptTokenCount":` + overflow + `},` + base[1:], wantErr: true},
		{name: "usage_duplicate_overflow", body: `{"usageMetadata":{"promptTokenCount":1e400,"promptTokenCount":7},` + base[1:], wantErr: true},
		{name: "unknown_scalar_overflow", body: `{"unknown":1e400,` + base[1:], input: `{"n":"text"}`},
		{name: "unknown_nested_overflow", body: `{"unknown":{"bad":1e400},` + base[1:], input: `{"n":"text"}`},
		{name: "unknown_400_digits", body: `{"unknown":` + overflow + `,` + base[1:], input: `{"n":"text"}`},
		{name: "unknown_part_overflow", body: `{"candidates":[{"content":{"parts":[{"unknown":{"bad":1e400},"functionCall":{"name":"example","args":{"n":"text"}}}]}}]}`, input: `{"n":"text"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, geminiSSEChunk(tt.body))
			}))
			defer server.Close()
			g, err := NewGemini("test-key", WithGeminiBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			stream, err := g.SendStream(context.Background(), &Request{Model: "gemini-2.0-flash", Messages: []Message{{Role: "user", Content: "Read values"}}})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = stream.Close() }()
			event, err := stream.Next()
			if tt.wantErr {
				var pe *ProviderError
				if !errors.As(err, &pe) || pe.Category != ErrCategoryServer || !strings.HasPrefix(pe.Message, "failed to parse streaming chunk: ") {
					t.Fatalf("Next = %+v, %v; want parse server ProviderError", event, err)
				}
				return
			}
			if err != nil || event.Type != StreamEventToolStart || event.ToolName != "example" || event.ToolInput != tt.input {
				t.Fatalf("Next = %+v, %v; want tool start input %s", event, err, tt.input)
			}
		})
	}
}

func TestGemini_SendStream_MixedTextAndToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"text":"Let me check"},{"functionCall":{"name":"search","args":{"q":"test"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3}}`))
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	stream, err := g.SendStream(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Search"}},
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
			t.Fatalf("unexpected stream error: %v", err)
		}
		events = append(events, ev)
	}

	// text + tool_start + tool_end + done = 4
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	if events[0].Type != StreamEventText || events[0].Text != "Let me check" {
		t.Errorf("event 0: expected text, got type=%s", events[0].Type)
	}
	if events[1].Type != StreamEventToolStart {
		t.Errorf("event 1: expected tool_start, got type=%s", events[1].Type)
	}
	if events[2].Type != StreamEventToolEnd {
		t.Errorf("event 2: expected tool_end, got type=%s", events[2].Type)
	}
	if events[3].Type != StreamEventDone {
		t.Errorf("event 3: expected done, got type=%s", events[3].Type)
	}
}

func TestGemini_SendStream_EmptyTextPart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"text":""}]}}]}`))
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2}}`))
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	stream, err := g.SendStream(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
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
			t.Fatalf("unexpected stream error: %v", err)
		}
		events = append(events, ev)
	}

	// 1 text + 1 done (empty text skipped)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != StreamEventText || events[0].Text != "Hello" {
		t.Errorf("event 0: expected text 'Hello', got type=%s text=%q", events[0].Type, events[0].Text)
	}
}

func TestGemini_SendStream_NoCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[]}`))
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2}}`))
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	stream, err := g.SendStream(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
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
			t.Fatalf("unexpected stream error: %v", err)
		}
		events = append(events, ev)
	}

	// 1 text + 1 done (empty candidates skipped)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestGemini_SendStream_UsageInMiddleChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]}}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":1}}`))
		_, _ = fmt.Fprint(w, geminiSSEChunk(`{"candidates":[{"content":{"role":"model","parts":[{"text":" world"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":10}}`))
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	stream, err := g.SendStream(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
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
			t.Fatalf("unexpected stream error: %v", err)
		}
		events = append(events, ev)
	}

	// 2 text + 1 done
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	done := events[2]
	if done.Type != StreamEventDone {
		t.Fatalf("expected done event, got %s", done.Type)
	}
	if done.OutputTokens != 10 {
		t.Errorf("expected latest output tokens 10, got %d", done.OutputTokens)
	}
}

func TestGemini_SendStream_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"code":401,"message":"Invalid API key","status":"UNAUTHENTICATED"}}`))
	}))
	defer server.Close()

	g, _ := NewGemini("bad-key", WithGeminiBaseURL(server.URL))
	_, err := g.SendStream(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})

	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if provErr.Category != ErrCategoryAuth {
		t.Errorf("expected ErrCategoryAuth, got %s", provErr.Category)
	}
}

func TestGemini_SendStream_MalformedSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: not valid json\n\n")
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	stream, err := g.SendStream(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error from SendStream: %v", err)
	}
	defer func() { _ = stream.Close() }()

	_, err = stream.Next()
	var provErr *ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("expected ProviderError, got %T: %v", err, err)
	}
	if provErr.Category != ErrCategoryServer {
		t.Errorf("expected ErrCategoryServer, got %s", provErr.Category)
	}
}

func TestGemini_SendStream_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()

	g, _ := NewGemini("test-key", WithGeminiBaseURL(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	stream, err := g.SendStream(ctx, &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		var provErr *ProviderError
		if !errors.As(err, &provErr) {
			t.Fatalf("expected ProviderError, got %T", err)
		}
		if provErr.Category != ErrCategoryTimeout {
			t.Errorf("expected ErrCategoryTimeout, got %s", provErr.Category)
		}
		return
	}
	defer func() { _ = stream.Close() }()

	for {
		_, err := stream.Next()
		if err == nil {
			continue
		}
		if err == io.EOF {
			t.Fatal("expected timeout error, got EOF")
		}
		var provErr *ProviderError
		if !errors.As(err, &provErr) {
			t.Fatalf("expected ProviderError, got %T: %v", err, err)
		}
		if provErr.Category != ErrCategoryTimeout {
			t.Errorf("expected ErrCategoryTimeout, got %s", provErr.Category)
		}
		break
	}
}

func TestLargeIntegerArgs_Gemini_Buffered(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "million", raw: `1000000`, want: "1000000"},
		{name: "negative_million", raw: `-1000000`, want: "-1000000"},
		{name: "non_round_integer", raw: `1234567`, want: "1234567"},
		{name: "negative_non_round_integer", raw: `-1234567`, want: "-1234567"},
		{name: "precise", raw: `9007199254740993`, want: "9007199254740993"},
		{name: "negative_precise", raw: `-9007199254740993`, want: "-9007199254740993"},
		{name: "exponent", raw: `1e6`, want: "1000000"},
		{name: "negative_exponent", raw: `-1e6`, want: "-1000000"},
		{name: "decimal", raw: `1000000.0`, want: "1000000"},
		{name: "negative_decimal", raw: `-1000000.0`, want: "-1000000"},
		{name: "precise_decimal", raw: `9007199254740993.0`, want: "9007199254740993"},
		{name: "negative_precise_decimal", raw: `-9007199254740993.0`, want: "-9007199254740993"},
		{name: "precise_exponent", raw: `90071992547409930e-1`, want: "9007199254740993"},
		{name: "negative_precise_exponent", raw: `-90071992547409930e-1`, want: "-9007199254740993"},
		{name: "beyond_int64", raw: `99999999999999999999999`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64", raw: `-99999999999999999999999`, want: "-99999999999999999999999"},
		{name: "beyond_int64_decimal", raw: `99999999999999999999999.0`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64_decimal", raw: `-99999999999999999999999.0`, want: "-99999999999999999999999"},
		{name: "beyond_int64_exponent", raw: `999999999999999999999990e-1`, want: "99999999999999999999999"},
		{name: "negative_beyond_int64_exponent", raw: `-999999999999999999999990e-1`, want: "-99999999999999999999999"},
		// 10^308 has 309 digits, is finite, and is inside the approved cap.
		{name: "eligible_309_digits", raw: `1e308`, want: "100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"},
		{name: "negative_eligible_309_digits", raw: `-1e308`, want: "-100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"},
		{name: "small", raw: `999999`, want: "999999"},
		{name: "negative_small", raw: `-999999`, want: "-999999"},
		{name: "small_decimal", raw: `999999.0`, want: "999999"},
		{name: "negative_small_decimal", raw: `-999999.0`, want: "-999999"},
		{name: "zero", raw: `0`, want: "0"},
		{name: "decimal_zero", raw: `0.0`, want: "0"},
		{name: "negative_zero", raw: `-0`, want: "-0"},
		{name: "negative_decimal_zero", raw: `-0.0`, want: "-0"},
		{name: "fraction", raw: `1.5`, want: "1.5"},
		{name: "negative_fraction", raw: `-1.5`, want: "-1.5"},
		{name: "rounded_threshold", raw: `999999.99999999999`, want: "1e+06"},
		{name: "negative_rounded_threshold", raw: `-999999.99999999999`, want: "-1e+06"},
		{name: "rounded_fraction", raw: `1000000.00000000001`, want: "1e+06"},
		{name: "negative_rounded_fraction", raw: `-1000000.00000000001`, want: "-1e+06"},
		{name: "huge_negative_exponent", raw: `1e-1000000000`, want: "0"},
		{name: "negative_huge_negative_exponent", raw: `-1e-1000000000`, want: "-0"},
		{name: "zero_huge_exponent", raw: `0e1000000000`, want: "0"},
		{name: "negative_zero_huge_exponent", raw: `-0e1000000000`, want: "-0"},
		// Approved exceptional tokens A/B retain legacy text without a token-length limit.
		{name: "exception_A_parse_equality_guard", raw: "1" + strings.Repeat("0", 100006) + "e-100000", want: "0"},
		{name: "exception_B_digit_cap_guard", raw: "0." + strings.Repeat("0", 9999) + "1e100000", want: "1"},
		{name: "text", raw: `"text"`, want: "text"},
		{name: "string_integer", raw: `"1000000"`, want: "1000000"},
		{name: "string_exponent", raw: `"1e6"`, want: "1e6"},
		{name: "string_decimal", raw: `"1000000.0"`, want: "1000000.0"},
		{name: "string_precise", raw: `"9007199254740993"`, want: "9007199254740993"},
		{name: "boolean_true", raw: `true`, want: "true"},
		{name: "boolean_false", raw: `false`, want: "false"},
		{name: "null", raw: `null`, want: "<nil>"},
		{name: "object_exponent", raw: `{"a":1e6}`, want: "map[a:1e+06]"},
		{name: "object_integer", raw: `{"a":1000000}`, want: "map[a:1e+06]"},
		{name: "array", raw: `[1.50]`, want: "[1.5]"},
		{name: "recursive_collection", raw: `{"a":[1000000,1.50]}`, want: "map[a:[1e+06 1.5]]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Gemini args is a native object, not a JSON-string arguments field.
			// Preserve numeric tokens on the wire; expectations are independent literals.
			body := `{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"example","args":{"n":` + tt.raw + `}}}]},"finishReason":"STOP"}]}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1beta/models/gemini-2.0-flash:generateContent" {
					t.Errorf("request = %s %s, want POST /v1beta/models/gemini-2.0-flash:generateContent", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			g, err := NewGemini("test-key", WithGeminiBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewGemini: %v", err)
			}

			resp, err := g.Send(context.Background(), &Request{
				Model:    "gemini-2.0-flash",
				Messages: []Message{{Role: "user", Content: "Read values"}},
			})

			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if resp == nil {
				t.Fatal("Send returned nil response without error")
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

func TestLargeIntegerArgs_Gemini_BufferedErrors(t *testing.T) {
	bodyFor := func(args string) string {
		return `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"example","args":` + args + `}}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}`
	}
	base := bodyFor(`{"n":"text"}`)
	overflow := "1" + strings.Repeat("0", 399)
	tests := []struct {
		name, body string
		want       map[string]string
		wantTokens int
		wantErr    bool
	}{
		{name: "malformed_response", body: `{"candidates":`, wantErr: true},
		{name: "malformed_args", body: bodyFor(`{"n":`), wantErr: true},
		{name: "trailing_value", body: base + `{}`, wantErr: true},
		{name: "trailing_junk", body: base + `junk`, wantErr: true},
		{name: "args_overflow", body: bodyFor(`{"n":1e400}`), wantErr: true},
		{name: "args_400_digits", body: bodyFor(`{"n":` + overflow + `}`), wantErr: true},
		{name: "nested_overflow", body: bodyFor(`{"n":[{"bad":1e400}]}`), wantErr: true},
		{name: "nested_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `}}`), wantErr: true},
		{name: "overwritten_scalar_overflow", body: bodyFor(`{"n":1e400,"n":1000000}`), wantErr: true},
		{name: "overwritten_subtree_overflow", body: bodyFor(`{"n":{"bad":1e400},"n":1000000}`), wantErr: true},
		{name: "overwritten_scalar_400_digits", body: bodyFor(`{"n":` + overflow + `,"n":1000000}`), wantErr: true},
		{name: "overwritten_subtree_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `},"n":1000000}`), wantErr: true},
		{name: "successful_scalar_duplicate", body: bodyFor(`{"n":999999,"n":1000000}`), want: map[string]string{"n": "1000000"}, wantTokens: 10},
		{name: "successful_subtree_duplicate", body: bodyFor(`{"n":{"ok":1},"n":1000000}`), want: map[string]string{"n": "1000000"}, wantTokens: 10},
		{name: "empty_args", body: bodyFor(`{}`), want: map[string]string{}, wantTokens: 10},
		{name: "null_args", body: bodyFor(`null`), want: map[string]string{}, wantTokens: 10},
		{name: "array_args", body: bodyFor(`[]`), wantErr: true},
		{name: "number_args", body: bodyFor(`0`), wantErr: true},
		{name: "string_args", body: bodyFor(`"text"`), wantErr: true},
		{name: "bool_args", body: bodyFor(`true`), wantErr: true},
		{name: "usage_exponent_overflow", body: `{"usageMetadata":{"promptTokenCount":1e400},` + base[1:], wantErr: true},
		{name: "usage_400_digits", body: `{"usageMetadata":{"candidatesTokenCount":` + overflow + `},` + base[1:], wantErr: true},
		{name: "usage_total_overflow", body: `{"usageMetadata":{"totalTokenCount":1e400},` + base[1:], wantErr: true},
		{name: "usage_duplicate_overflow", body: `{"usageMetadata":{"promptTokenCount":1e400,"promptTokenCount":7},` + base[1:], wantErr: true},
		{name: "usage_replaced_overflow", body: `{"usageMetadata":{"promptTokenCount":1e400},` + base[1:], wantErr: true},
		{name: "usage_duplicate_success", body: `{"usageMetadata":{"promptTokenCount":2,"promptTokenCount":7},"candidates":[{"content":{"parts":[{"functionCall":{"name":"example","args":{}}}]}}]}`, want: map[string]string{}, wantTokens: 7},
		{name: "usage_subtree_success", body: `{"usageMetadata":{"promptTokenCount":2},"usageMetadata":{"promptTokenCount":7},"candidates":[{"content":{"parts":[{"functionCall":{"name":"example","args":{}}}]}}]}`, want: map[string]string{}, wantTokens: 7},
		{name: "unknown_scalar_overflow", body: `{"unknown":1e400,` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "unknown_nested_overflow", body: `{"unknown":{"bad":1e400},` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "unknown_400_digits", body: `{"unknown":` + overflow + `,` + base[1:], want: map[string]string{"n": "text"}, wantTokens: 10},
		{name: "unknown_part_overflow", body: `{"candidates":[{"content":{"parts":[{"unknown":{"bad":1e400},"functionCall":{"name":"example","args":{"n":"text"}}}]}}]}`, want: map[string]string{"n": "text"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()
			g, err := NewGemini("test-key", WithGeminiBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := g.Send(context.Background(), &Request{Model: "gemini-2.0-flash", Messages: []Message{{Role: "user", Content: "Read values"}}})
			if tt.wantErr {
				var pe *ProviderError
				if resp != nil || !errors.As(err, &pe) || pe.Category != ErrCategoryServer || !strings.HasPrefix(pe.Message, "failed to parse response: ") {
					t.Fatalf("Send = %+v, %v; want nil response and parse server ProviderError", resp, err)
				}
				return
			}
			if err != nil || resp == nil {
				t.Fatalf("Send = %+v, %v", resp, err)
			}
			if len(resp.ToolCalls) != 1 || resp.InputTokens != tt.wantTokens {
				t.Fatalf("response = %+v, want one call and %d input tokens", resp, tt.wantTokens)
			}
			got := resp.ToolCalls[0].Arguments
			if len(got) != len(tt.want) {
				t.Errorf("arguments = %v, want %v", got, tt.want)
			}
			for key, want := range tt.want {
				if value, ok := got[key]; !ok || value != want {
					t.Errorf("arguments[%s] = %q (present=%t), want %q", key, value, ok, want)
				}
			}
			for _, value := range got {
				if strings.Contains(value, "+Inf") {
					t.Errorf("arguments contain +Inf: %v", got)
				}
			}
		})
	}
}

func TestGemini_Send_ZeroTokenCounts(t *testing.T) {
	mock := testutil.NewMockLLMServer(t, []testutil.MockLLMResponse{
		{StatusCode: 200, Body: `{"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}]}`},
	})

	g, err := NewGemini("test-key", WithGeminiBaseURL(mock.URL()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := g.Send(context.Background(), &Request{
		Model:    "gemini-2.0-flash",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.InputTokens != 0 {
		t.Errorf("expected 0 input tokens, got %d", resp.InputTokens)
	}
	if resp.OutputTokens != 0 {
		t.Errorf("expected 0 output tokens, got %d", resp.OutputTokens)
	}
	if resp.Content != "hello" {
		t.Errorf("expected content 'hello', got %q", resp.Content)
	}
}
