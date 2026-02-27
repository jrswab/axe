package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	// defaultBaseURL is the default Anthropic API base URL.
	defaultBaseURL = "https://api.anthropic.com"

	// defaultMaxTokens is the default max_tokens value when none is specified.
	// The Anthropic API requires max_tokens to be present and greater than zero.
	defaultMaxTokens = 4096

	// anthropicVersion is the required API version header value.
	anthropicVersion = "2023-06-01"
)

// AnthropicOption is a functional option for configuring the Anthropic provider.
type AnthropicOption func(*Anthropic)

// WithBaseURL sets a custom base URL (used for testing with httptest).
func WithBaseURL(url string) AnthropicOption {
	return func(a *Anthropic) {
		a.baseURL = url
	}
}

// Anthropic implements the Provider interface for the Anthropic Messages API.
type Anthropic struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewAnthropic creates a new Anthropic provider. Returns an error if apiKey is empty.
func NewAnthropic(apiKey string, opts ...AnthropicOption) (*Anthropic, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	a := &Anthropic{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	for _, opt := range opts {
		opt(a)
	}

	return a, nil
}

// anthropicRequest is the JSON body sent to the Anthropic Messages API.
type anthropicRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Messages    []Message `json:"messages"`
	System      string    `json:"system,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
}

// anthropicResponse represents the JSON response from the Anthropic Messages API.
type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// anthropicErrorResponse represents an Anthropic API error response.
type anthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Send makes a completion request to the Anthropic Messages API.
func (a *Anthropic) Send(ctx context.Context, req *Request) (*Response, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}

	body := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  req.Messages,
		System:    req.System,
	}

	if req.Temperature != 0 {
		temp := req.Temperature
		body.Temperature = &temp
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("content-type", "application/json")

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		// Check for context cancellation/deadline exceeded
		if ctx.Err() != nil {
			return nil, &ProviderError{
				Category: ErrCategoryTimeout,
				Message:  ctx.Err().Error(),
				Err:      ctx.Err(),
			}
		}
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  err.Error(),
			Err:      err,
		}
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Handle non-2xx responses
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, a.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	// Parse success response
	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  fmt.Sprintf("failed to parse response: %s", err),
			Err:      err,
		}
	}

	// Check for empty content array
	if len(apiResp.Content) == 0 {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  "response contains no content",
		}
	}

	return &Response{
		Content:      apiResp.Content[0].Text,
		Model:        apiResp.Model,
		InputTokens:  apiResp.Usage.InputTokens,
		OutputTokens: apiResp.Usage.OutputTokens,
		StopReason:   apiResp.StopReason,
	}, nil
}

// handleErrorResponse maps HTTP error responses to ProviderError.
func (a *Anthropic) handleErrorResponse(status int, body []byte) *ProviderError {
	// Attempt to parse Anthropic error response
	message := http.StatusText(status)
	var errResp anthropicErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
	}

	category := a.mapStatusToCategory(status)

	return &ProviderError{
		Category: category,
		Status:   status,
		Message:  message,
	}
}

// mapStatusToCategory maps HTTP status codes to error categories.
func (a *Anthropic) mapStatusToCategory(status int) ErrorCategory {
	switch status {
	case 401:
		return ErrCategoryAuth
	case 400:
		return ErrCategoryBadRequest
	case 429:
		return ErrCategoryRateLimit
	case 529:
		return ErrCategoryOverloaded
	case 500, 502, 503:
		return ErrCategoryServer
	default:
		return ErrCategoryServer
	}
}
