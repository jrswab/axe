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

func TestLargeIntegerArgs_Bedrock_Buffered(t *testing.T) {
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
		// Approved exceptional tokens A/B keep legacy text, without length rejection.
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
			// Preserve native numeric tokens at the HTTP boundary, not string-only fixtures.
			body := `{"output":{"message":{"role":"assistant","content":[{"toolUse":{"toolUseId":"call_numeric","name":"example","input":{"n":` + tt.raw + `}}}]}},"stopReason":"tool_use"}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/model/test-model/converse" {
					t.Errorf("request = %s %s, want POST /model/test-model/converse", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			b, err := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
			if err != nil {
				t.Fatalf("NewBedrock: %v", err)
			}

			resp, err := b.Send(context.Background(), &Request{
				Model:    "test-model",
				Messages: []Message{{Role: "user", Content: "Read values"}},
				Tools:    []Tool{{Name: "example", Description: "Read values", Parameters: map[string]ToolParameter{}}},
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

func TestLargeIntegerArgs_Bedrock_DecodeErrors(t *testing.T) {
	bodyFor := func(args string) string {
		return `{"output":{"message":{"role":"assistant","content":[{"toolUse":{"toolUseId":"call_1","name":"example","input":` + args + `}}]}},"stopReason":"tool_use"}`
	}
	base := bodyFor(`{"n":"text"}`)
	overflow := "1" + strings.Repeat("0", 399)
	tests := []struct {
		name, body string
		want       map[string]string
		wantErr    bool
	}{
		{name: "malformed_response", body: `{"output":`, wantErr: true},
		{name: "malformed_arguments", body: bodyFor(`{"n":`), wantErr: true},
		{name: "trailing_value", body: base + `{}`, wantErr: true},
		{name: "trailing_junk", body: base + `junk`, wantErr: true},
		{name: "scalar_overflow", body: bodyFor(`{"n":1e400}`), wantErr: true},
		{name: "scalar_400_digits", body: bodyFor(`{"n":` + overflow + `}`), wantErr: true},
		{name: "nested_overflow", body: bodyFor(`{"n":[{"bad":1e400}]}`), wantErr: true},
		{name: "nested_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `}}`), wantErr: true},
		{name: "overwritten_scalar_overflow", body: bodyFor(`{"n":1e400,"n":1000000}`), wantErr: true},
		{name: "replaced_subtree_overflow", body: bodyFor(`{"n":{"bad":1e400},"n":1000000}`), wantErr: true},
		{name: "overwritten_scalar_400_digits", body: bodyFor(`{"n":` + overflow + `,"n":1000000}`), wantErr: true},
		{name: "replaced_subtree_400_digits", body: bodyFor(`{"n":{"bad":` + overflow + `},"n":1000000}`), wantErr: true},
		{name: "successful_scalar_duplicate", body: bodyFor(`{"n":999999,"n":1000000}`), want: map[string]string{"n": "1000000"}},
		{name: "successful_subtree_duplicate", body: bodyFor(`{"n":{"ok":1},"n":1000000}`), want: map[string]string{"n": "1000000"}},
		{name: "empty_map", body: bodyFor(`{}`), want: map[string]string{}},
		{name: "null_map", body: bodyFor(`null`), want: map[string]string{}},
		{name: "array_map", body: bodyFor(`[]`), wantErr: true},
		{name: "number_map", body: bodyFor(`0`), wantErr: true},
		{name: "negative_zero_map", body: bodyFor(`-0`), wantErr: true},
		{name: "decimal_zero_map", body: bodyFor(`-0.0`), wantErr: true},
		{name: "string_map", body: bodyFor(`"text"`), wantErr: true},
		{name: "bool_map", body: bodyFor(`true`), wantErr: true},
		{name: "known_usage_overflow", body: `{"usage":{"inputTokens":1e400},` + base[1:], wantErr: true},
		{name: "known_usage_400_digits", body: `{"usage":{"inputTokens":` + overflow + `},` + base[1:], wantErr: true},
		{name: "overwritten_usage_overflow", body: `{"usage":{"inputTokens":1e400,"inputTokens":10},` + base[1:], wantErr: true},
		{name: "unknown_scalar_overflow", body: `{"unknown":1e400,` + base[1:], want: map[string]string{"n": "text"}},
		{name: "unknown_nested_overflow", body: `{"unknown":{"bad":1e400},` + base[1:], want: map[string]string{"n": "text"}},
		{name: "unknown_400_digits", body: `{"unknown":` + overflow + `,` + base[1:], want: map[string]string{"n": "text"}},
		{name: "unknown_nested_400_digits", body: `{"unknown":{"bad":` + overflow + `},` + base[1:], want: map[string]string{"n": "text"}},
		{name: "unknown_message_overflow", body: `{"output":{"message":{"unknown":[1e400],"content":[{"toolUse":{"name":"example","input":{"n":"text"}}}]}}}`, want: map[string]string{"n": "text"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			b, err := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := b.Send(context.Background(), &Request{Model: "test-model", Messages: []Message{{Role: "user", Content: "Read values"}}})
			if tt.wantErr {
				var pe *ProviderError
				if resp != nil || !errors.As(err, &pe) || pe.Category != ErrCategoryServer || !strings.HasPrefix(pe.Message, "failed to parse response: ") {
					t.Fatalf("Send = %+v, %v; want nil response and parse server ProviderError", resp, err)
				}
				return
			}
			if err != nil || resp == nil || len(resp.ToolCalls) != 1 {
				t.Fatalf("Send = %+v, %v; want one call", resp, err)
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

func TestBedrock_Send_IgnoresReasoningEffortAndExplicitZeroTemperature(t *testing.T) {
	cases := []struct {
		name      string
		maxTokens int
	}{
		{"without inference config", 0},
		{"with inference config", 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Errorf("failed to parse request body: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(bedrockResponse{
					Output:     bedrockOutput{Message: &bedrockMessage{Role: "assistant", Content: []bedrockBlock{{Text: "ok"}}}},
					StopReason: "end_turn",
				})
			}))
			defer server.Close()

			b, err := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
			if err != nil {
				t.Fatalf("unexpected constructor error: %v", err)
			}
			resp, err := b.Send(context.Background(), &Request{
				Model:           "test-model",
				Messages:        []Message{{Role: "user", Content: "Hi"}},
				ReasoningEffort: "low",
				TemperatureSet:  true,
				Temperature:     0,
				MaxTokens:       tc.maxTokens,
			})
			if err != nil {
				t.Fatalf("unexpected Send error: %v", err)
			}
			if resp.Content != "ok" {
				t.Errorf("Content = %q, want ok", resp.Content)
			}
			cfg, hasConfig := gotBody["inferenceConfig"].(map[string]interface{})
			if hasConfig != (tc.maxTokens > 0) {
				t.Errorf("inferenceConfig present = %v, want %v", hasConfig, tc.maxTokens > 0)
			}
			if tc.maxTokens > 0 && cfg["maxTokens"] != float64(tc.maxTokens) {
				t.Errorf("maxTokens = %v, want %d", cfg["maxTokens"], tc.maxTokens)
			}
			for _, key := range []string{"reasoning_effort", "temperature"} {
				if _, exists := gotBody[key]; exists {
					t.Errorf("body contains %q: %v", key, gotBody[key])
				}
				if _, exists := cfg[key]; exists {
					t.Errorf("inferenceConfig contains %q: %v", key, cfg[key])
				}
			}
		})
	}
}

func TestBedrock_Send_Cache(t *testing.T) {
	cases := []struct {
		name                 string
		setupRequest         func() *Request
		serverResponse       bedrockResponse
		wantSystemCachePoint bool
		wantToolCachePoint   bool
		wantCacheReadTokens  int
		wantCacheWriteTokens int
	}{
		{
			name: "system block gets cache point",
			setupRequest: func() *Request {
				return &Request{
					Model:       "test-model",
					System:      "You are helpful.",
					CacheConfig: true,
					Messages:    []Message{{Role: "user", Content: "hi"}},
				}
			},
			serverResponse: bedrockResponse{
				Output:     bedrockOutput{Message: &bedrockMessage{Role: "assistant", Content: []bedrockBlock{{Text: "ok"}}}},
				StopReason: "end_turn",
			},
			wantSystemCachePoint: true,
		},
		{
			name: "tool config gets cache point when tools present",
			setupRequest: func() *Request {
				return &Request{
					Model:       "test-model",
					CacheConfig: true,
					Messages:    []Message{{Role: "user", Content: "hi"}},
					Tools: []Tool{{
						Name:        "read_file",
						Description: "Read a file",
						Parameters:  map[string]ToolParameter{"path": {Type: "string", Description: "File path", Required: true}},
					}},
				}
			},
			serverResponse: bedrockResponse{
				Output:     bedrockOutput{Message: &bedrockMessage{Role: "assistant", Content: []bedrockBlock{{Text: "ok"}}}},
				StopReason: "end_turn",
			},
			wantToolCachePoint: true,
		},
		{
			name: "cache usage parsing",
			setupRequest: func() *Request {
				return &Request{
					Model:    "test-model",
					Messages: []Message{{Role: "user", Content: "hi"}},
				}
			},
			serverResponse: bedrockResponse{
				Output: bedrockOutput{Message: &bedrockMessage{
					Role:    "assistant",
					Content: []bedrockBlock{{Text: "Hello with cache"}},
				}},
				StopReason: "end_turn",
				Usage: bedrockUsage{
					InputTokens:           100,
					OutputTokens:          5,
					CacheReadInputTokens:  10,
					CacheWriteInputTokens: 80,
				},
			},
			wantCacheReadTokens:  10,
			wantCacheWriteTokens: 80,
		},
		{
			name: "cache usage omitted",
			setupRequest: func() *Request {
				return &Request{
					Model:    "test-model",
					Messages: []Message{{Role: "user", Content: "hi"}},
				}
			},
			serverResponse: bedrockResponse{
				Output: bedrockOutput{Message: &bedrockMessage{
					Role:    "assistant",
					Content: []bedrockBlock{{Text: "Hello"}},
				}},
				StopReason: "end_turn",
				Usage:      bedrockUsage{InputTokens: 10, OutputTokens: 5},
			},
			wantCacheReadTokens:  0,
			wantCacheWriteTokens: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var receivedReq bedrockRequest
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&receivedReq)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(tc.serverResponse)
			}))
			defer server.Close()

			b, _ := NewBedrock("us-east-1", WithBedrockBaseURL(server.URL), withBedrockCreds(testCreds()))
			resp, err := b.Send(context.Background(), tc.setupRequest())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantSystemCachePoint {
				if len(receivedReq.System) != 1 {
					t.Fatalf("expected 1 system block, got %d", len(receivedReq.System))
				}
				if receivedReq.System[0].CachePoint == nil {
					t.Fatal("expected cachePoint on system block")
				}
				if receivedReq.System[0].CachePoint.Type != "default" {
					t.Errorf("cachePoint.type = %q, want %q", receivedReq.System[0].CachePoint.Type, "default")
				}
			}
			if tc.wantToolCachePoint {
				if receivedReq.ToolConfig == nil {
					t.Fatal("expected ToolConfig when tools present")
				}
				if receivedReq.ToolConfig.CachePoint == nil {
					t.Fatal("expected cachePoint on ToolConfig")
				}
				if receivedReq.ToolConfig.CachePoint.Type != "default" {
					t.Errorf("toolConfig cachePoint.type = %q, want %q", receivedReq.ToolConfig.CachePoint.Type, "default")
				}
			}
			if resp.CacheReadTokens != tc.wantCacheReadTokens {
				t.Errorf("CacheReadTokens = %d, want %d", resp.CacheReadTokens, tc.wantCacheReadTokens)
			}
			if resp.CacheWriteTokens != tc.wantCacheWriteTokens {
				t.Errorf("CacheWriteTokens = %d, want %d", resp.CacheWriteTokens, tc.wantCacheWriteTokens)
			}
		})
	}
}
