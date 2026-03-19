package testutil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockLLMResponse represents a canned response for the mock LLM server.
type MockLLMResponse struct {
	StatusCode int
	Body       string
}

// MockLLMRequest captures an HTTP request received by the mock server.
type MockLLMRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    string
}

// MockLLMServer is a reusable httptest-based mock LLM server that serves
// a queue of canned responses and records all incoming requests.
type MockLLMServer struct {
	Server    *httptest.Server
	Requests  []MockLLMRequest
	mu        sync.Mutex
	responses []MockLLMResponse
	callIndex int
}

// MockToolCall describes a tool invocation for use in response helpers.
type MockToolCall struct {
	ID    string
	Name  string
	Input map[string]string
}

// NewMockLLMServer creates a mock HTTP server that serves responses from a queue.
// Each incoming request pops the next response. Requests beyond the queue size
// cause a test fatal. The server is automatically closed when the test ends.
func NewMockLLMServer(t *testing.T, responses []MockLLMResponse) *MockLLMServer {
	t.Helper()

	m := &MockLLMServer{
		responses: responses,
	}

	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("mock server: failed to read request body: %v", err)
		}

		m.mu.Lock()
		m.Requests = append(m.Requests, MockLLMRequest{
			Method:  r.Method,
			Path:    r.URL.Path,
			Headers: r.Header.Clone(),
			Body:    string(body),
		})

		idx := m.callIndex
		m.callIndex++

		if idx >= len(m.responses) {
			m.mu.Unlock()
			t.Fatalf("mock server received unexpected request #%d (only %d responses queued)", idx+1, len(m.responses))
			return
		}

		resp := m.responses[idx]
		m.mu.Unlock()

		// Handle slow response sentinel
		if resp.StatusCode == -1 {
			delay, err := time.ParseDuration(resp.Body)
			if err != nil {
				t.Fatalf("mock server: invalid slow response duration %q: %v", resp.Body, err)
				return
			}

			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `{"id":"msg_mock","type":"message","role":"assistant","content":[{"type":"text","text":""}],"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","usage":{"input_tokens":0,"output_tokens":0}}`)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = fmt.Fprint(w, resp.Body)
	}))

	t.Cleanup(m.Server.Close)

	return m
}

// URL returns the base URL of the mock server.
func (m *MockLLMServer) URL() string {
	return m.Server.URL
}

// RequestCount returns the number of requests served so far. Thread-safe.
func (m *MockLLMServer) RequestCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callIndex
}

// AnthropicResponse returns a MockLLMResponse with a valid Anthropic messages API
// response containing the given text.
func AnthropicResponse(text string) MockLLMResponse {
	body := fmt.Sprintf(`{"id":"msg_mock","type":"message","role":"assistant","content":[{"type":"text","text":%s}],"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`, jsonString(text))
	return MockLLMResponse{StatusCode: 200, Body: body}
}

// AnthropicResponseWithTokens returns a MockLLMResponse with custom token counts.
func AnthropicResponseWithTokens(text string, inputTokens, outputTokens int) MockLLMResponse {
	body := fmt.Sprintf(`{"id":"msg_mock","type":"message","role":"assistant","content":[{"type":"text","text":%s}],"model":"claude-sonnet-4-20250514","stop_reason":"end_turn","usage":{"input_tokens":%d,"output_tokens":%d}}`, jsonString(text), inputTokens, outputTokens)
	return MockLLMResponse{StatusCode: 200, Body: body}
}

// AnthropicToolUseResponse returns a MockLLMResponse with an Anthropic response
// containing both a text block and tool_use blocks.
func AnthropicToolUseResponse(text string, toolCalls []MockToolCall) MockLLMResponse {
	var blocks []string
	blocks = append(blocks, fmt.Sprintf(`{"type":"text","text":%s}`, jsonString(text)))

	for _, tc := range toolCalls {
		inputJSON, _ := json.Marshal(tc.Input)
		blocks = append(blocks, fmt.Sprintf(`{"type":"tool_use","id":%s,"name":%s,"input":%s}`, jsonString(tc.ID), jsonString(tc.Name), string(inputJSON)))
	}

	body := fmt.Sprintf(`{"id":"msg_mock","type":"message","role":"assistant","content":[%s],"model":"claude-sonnet-4-20250514","stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":20}}`, strings.Join(blocks, ","))
	return MockLLMResponse{StatusCode: 200, Body: body}
}

// AnthropicToolUseResponseWithTokens returns a MockLLMResponse with tool calls and custom token counts.
func AnthropicToolUseResponseWithTokens(text string, toolCalls []MockToolCall, inputTokens, outputTokens int) MockLLMResponse {
	var blocks []string
	blocks = append(blocks, fmt.Sprintf(`{"type":"text","text":%s}`, jsonString(text)))

	for _, tc := range toolCalls {
		inputJSON, _ := json.Marshal(tc.Input)
		blocks = append(blocks, fmt.Sprintf(`{"type":"tool_use","id":%s,"name":%s,"input":%s}`, jsonString(tc.ID), jsonString(tc.Name), string(inputJSON)))
	}

	body := fmt.Sprintf(`{"id":"msg_mock","type":"message","role":"assistant","content":[%s],"model":"claude-sonnet-4-20250514","stop_reason":"tool_use","usage":{"input_tokens":%d,"output_tokens":%d}}`, strings.Join(blocks, ","), inputTokens, outputTokens)
	return MockLLMResponse{StatusCode: 200, Body: body}
}

// AnthropicErrorResponse returns a MockLLMResponse with an Anthropic error shape.
func AnthropicErrorResponse(statusCode int, errType, message string) MockLLMResponse {
	body := fmt.Sprintf(`{"type":"error","error":{"type":%s,"message":%s}}`, jsonString(errType), jsonString(message))
	return MockLLMResponse{StatusCode: statusCode, Body: body}
}

// OpenAIResponse returns a MockLLMResponse with a valid OpenAI chat completions response.
func OpenAIResponse(text string) MockLLMResponse {
	body := fmt.Sprintf(`{"model":"gpt-4o","choices":[{"message":{"content":%s},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`, jsonString(text))
	return MockLLMResponse{StatusCode: 200, Body: body}
}

// OpenAIToolCallResponse returns a MockLLMResponse with an OpenAI response containing tool calls.
func OpenAIToolCallResponse(text string, toolCalls []MockToolCall) MockLLMResponse {
	var tcs []string
	for _, tc := range toolCalls {
		argsJSON, _ := json.Marshal(tc.Input)
		tcs = append(tcs, fmt.Sprintf(`{"id":%s,"type":"function","function":{"name":%s,"arguments":%s}}`, jsonString(tc.ID), jsonString(tc.Name), jsonString(string(argsJSON))))
	}

	var contentField string
	if text == "" {
		contentField = "null"
	} else {
		contentField = jsonString(text)
	}

	body := fmt.Sprintf(`{"model":"gpt-4o","choices":[{"message":{"content":%s,"tool_calls":[%s]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":20}}`, contentField, strings.Join(tcs, ","))
	return MockLLMResponse{StatusCode: 200, Body: body}
}

// OpenAIErrorResponse returns a MockLLMResponse with an OpenAI error shape.
func OpenAIErrorResponse(statusCode int, errType, message string) MockLLMResponse {
	body := fmt.Sprintf(`{"error":{"message":%s,"type":%s,"code":%s}}`, jsonString(message), jsonString(errType), jsonString(errType))
	return MockLLMResponse{StatusCode: statusCode, Body: body}
}

// SlowResponse returns a MockLLMResponse sentinel that causes the mock server
// to sleep for the given duration before responding.
func SlowResponse(delay time.Duration) MockLLMResponse {
	return MockLLMResponse{StatusCode: -1, Body: delay.String()}
}

// GeminiResponse returns a MockLLMResponse with a valid Gemini generateContent response.
func GeminiResponse(text string) MockLLMResponse {
	body := fmt.Sprintf(`{"candidates":[{"content":{"role":"model","parts":[{"text":%s}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15}}`, jsonString(text))
	return MockLLMResponse{StatusCode: 200, Body: body}
}

// GeminiToolCallResponse returns a MockLLMResponse with a Gemini response containing text and functionCall parts.
func GeminiToolCallResponse(text string, toolCalls []MockToolCall) MockLLMResponse {
	var parts []string
	if text != "" {
		parts = append(parts, fmt.Sprintf(`{"text":%s}`, jsonString(text)))
	}
	for _, tc := range toolCalls {
		argsJSON, _ := json.Marshal(tc.Input)
		parts = append(parts, fmt.Sprintf(`{"functionCall":{"name":%s,"args":%s}}`, jsonString(tc.Name), string(argsJSON)))
	}
	body := fmt.Sprintf(`{"candidates":[{"content":{"role":"model","parts":[%s]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":20,"totalTokenCount":30}}`, strings.Join(parts, ","))
	return MockLLMResponse{StatusCode: 200, Body: body}
}

// GeminiErrorResponse returns a MockLLMResponse with a Gemini error shape.
func GeminiErrorResponse(statusCode int, message string) MockLLMResponse {
	body := fmt.Sprintf(`{"error":{"code":%d,"message":%s,"status":"ERROR"}}`, statusCode, jsonString(message))
	return MockLLMResponse{StatusCode: statusCode, Body: body}
}

// jsonString returns a JSON-encoded string value (with quotes and escaping).
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
