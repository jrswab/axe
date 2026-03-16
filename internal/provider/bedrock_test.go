package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testCreds() *awsCredentials {
	return &awsCredentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET"}
}

func TestNewBedrock_EmptyRegion(t *testing.T) {
	_, err := NewBedrock("", withBedrockCreds(testCreds()))
	if err == nil {
		t.Fatal("expected error for empty region")
	}
}

func TestNewBedrock_ValidRegion(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "SECRET")
	b, err := NewBedrock("us-east-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.region != "us-east-1" {
		t.Errorf("expected region us-east-1, got %s", b.region)
	}
}

func TestNewBedrock_WithBedrockRegion(t *testing.T) {
	b, err := NewBedrock("us-west-2", WithBedrockRegion("eu-west-1"), withBedrockCreds(testCreds()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.region != "eu-west-1" {
		t.Errorf("expected region eu-west-1, got %s", b.region)
	}
}

func TestBedrock_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify SigV4 Authorization header present
		if auth := r.Header.Get("Authorization"); !strings.HasPrefix(auth, "AWS4-HMAC-SHA256") {
			t.Errorf("missing or invalid Authorization header: %s", auth)
		}
		if r.Header.Get("x-amz-date") == "" {
			t.Error("missing x-amz-date header")
		}

		// Verify request path
		if !strings.Contains(r.URL.Path, "/model/test-model/converse") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(bedrockResponse{
			Output: bedrockOutput{Message: &bedrockMessage{
				Role:    "assistant",
				Content: []bedrockBlock{{Text: "Hello from Bedrock"}},
			}},
			StopReason: "end_turn",
			Usage:      bedrockUsage{InputTokens: 10, OutputTokens: 5},
		})
	}))
	defer server.Close()

	b, err := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := b.Send(context.Background(), &Request{
		Model:    "test-model",
		System:   "You are helpful.",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Hello from Bedrock" {
		t.Errorf("expected 'Hello from Bedrock', got %q", resp.Content)
	}
	if resp.InputTokens != 10 || resp.OutputTokens != 5 {
		t.Errorf("unexpected tokens: in=%d out=%d", resp.InputTokens, resp.OutputTokens)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("expected stop_reason end_turn, got %s", resp.StopReason)
	}
}

func TestBedrock_Send_ToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify tool config was sent
		var req bedrockRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.ToolConfig == nil || len(req.ToolConfig.Tools) == 0 {
			t.Error("expected tool config in request")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(bedrockResponse{
			Output: bedrockOutput{Message: &bedrockMessage{
				Role: "assistant",
				Content: []bedrockBlock{{ToolUse: &bedrockToolUse{
					ToolUseID: "call_123",
					Name:      "read_file",
					Input:     map[string]interface{}{"path": "test.go"},
				}}},
			}},
			StopReason: "tool_use",
			Usage:      bedrockUsage{InputTokens: 20, OutputTokens: 10},
		})
	}))
	defer server.Close()

	b, err := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := b.Send(context.Background(), &Request{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "Read test.go"}},
		Tools: []Tool{{
			Name:        "read_file",
			Description: "Read a file",
			Parameters:  map[string]ToolParameter{"path": {Type: "string", Description: "File path", Required: true}},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "call_123" || tc.Name != "read_file" || tc.Arguments["path"] != "test.go" {
		t.Errorf("unexpected tool call: %+v", tc)
	}
}

func TestBedrock_Send_ToolResult(t *testing.T) {
	var receivedReq bedrockRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(bedrockResponse{
			Output:     bedrockOutput{Message: &bedrockMessage{Role: "assistant", Content: []bedrockBlock{{Text: "Done"}}}},
			StopReason: "end_turn",
			Usage:      bedrockUsage{InputTokens: 5, OutputTokens: 2},
		})
	}))
	defer server.Close()

	b, err := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = b.Send(context.Background(), &Request{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "Read test.go"},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "call_1", Name: "read_file", Arguments: map[string]string{"path": "test.go"}}}},
			{Role: "user", ToolResults: []ToolResult{{CallID: "call_1", Content: "file contents", IsError: false}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tool result was sent correctly
	if len(receivedReq.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(receivedReq.Messages))
	}
	lastMsg := receivedReq.Messages[2]
	if len(lastMsg.Content) == 0 || lastMsg.Content[0].ToolResult == nil {
		t.Fatal("expected tool result in last message")
	}
	if lastMsg.Content[0].ToolResult.Status != "success" {
		t.Errorf("expected status success, got %s", lastMsg.Content[0].ToolResult.Status)
	}
}

func TestBedrock_Send_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_ = json.NewEncoder(w).Encode(bedrockErrorResponse{Message: "Access denied", Type: "AccessDeniedException"})
	}))
	defer server.Close()

	b, _ := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
	_, err := b.Send(context.Background(), &Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatal("expected error")
	}
	var pe *ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("expected ProviderError, got %T", err)
	}
	if pe.Category != ErrCategoryAuth {
		t.Errorf("expected ErrCategoryAuth, got %s", pe.Category)
	}
}

func TestBedrock_Send_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		_ = json.NewEncoder(w).Encode(bedrockErrorResponse{Message: "Too many requests"})
	}))
	defer server.Close()

	b, _ := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
	_, err := b.Send(context.Background(), &Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	var pe *ProviderError
	if !errors.As(err, &pe) || pe.Category != ErrCategoryRateLimit {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestBedrock_Send_BadRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(bedrockErrorResponse{Message: "Invalid model"})
	}))
	defer server.Close()

	b, _ := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
	_, err := b.Send(context.Background(), &Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	var pe *ProviderError
	if !errors.As(err, &pe) || pe.Category != ErrCategoryBadRequest {
		t.Errorf("expected bad request error, got %v", err)
	}
}

func TestBedrock_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_ = json.NewEncoder(w).Encode(bedrockErrorResponse{Message: "Internal error"})
	}))
	defer server.Close()

	b, _ := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
	_, err := b.Send(context.Background(), &Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	var pe *ProviderError
	if !errors.As(err, &pe) || pe.Category != ErrCategoryServer {
		t.Errorf("expected server error, got %v", err)
	}
}

func TestBedrock_Send_Temperature(t *testing.T) {
	var receivedReq bedrockRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&receivedReq)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(bedrockResponse{
			Output:     bedrockOutput{Message: &bedrockMessage{Role: "assistant", Content: []bedrockBlock{{Text: "ok"}}}},
			StopReason: "end_turn",
		})
	}))
	defer server.Close()

	b, _ := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
	_, _ = b.Send(context.Background(), &Request{
		Model: "m", Messages: []Message{{Role: "user", Content: "hi"}},
		Temperature: 0.7, MaxTokens: 100,
	})

	if receivedReq.InferenceConfig == nil {
		t.Fatal("expected inference config")
	}
	if receivedReq.InferenceConfig.Temperature == nil || *receivedReq.InferenceConfig.Temperature != 0.7 {
		t.Error("expected temperature 0.7")
	}
	if receivedReq.InferenceConfig.MaxTokens == nil || *receivedReq.InferenceConfig.MaxTokens != 100 {
		t.Error("expected max_tokens 100")
	}
}
