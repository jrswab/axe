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
