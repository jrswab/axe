package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

const (
	// defaultOllamaBaseURL is the default Ollama API base URL.
	defaultOllamaBaseURL = "http://localhost:11434"
)

// OllamaOption is a functional option for configuring the Ollama provider.
type OllamaOption func(*Ollama)

// WithOllamaBaseURL sets a custom base URL for the Ollama provider.
func WithOllamaBaseURL(url string) OllamaOption {
	return func(o *Ollama) {
		o.baseURL = url
	}
}

// Ollama implements the Provider interface for the Ollama Chat API.
type Ollama struct {
	baseURL string
	client  *http.Client
}

// NewOllama creates a new Ollama provider. Ollama does not require authentication.
func NewOllama(opts ...OllamaOption) (*Ollama, error) {
	o := &Ollama{
		baseURL: defaultOllamaBaseURL,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	for _, opt := range opts {
		opt(o)
	}

	return o, nil
}

// ollamaOptions holds the optional parameters for the Ollama request.
type ollamaOptions struct {
	Temperature *float64 `json:"temperature,omitempty"`
	NumPredict  *int     `json:"num_predict,omitempty"`
}

// ollamaRequest is the JSON body sent to the Ollama Chat API.
type ollamaRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Stream   bool           `json:"stream"`
	Options  *ollamaOptions `json:"options,omitempty"`
}

// ollamaResponse represents the JSON response from the Ollama Chat API.
type ollamaResponse struct {
	Model   string `json:"model"`
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	DoneReason    string `json:"done_reason"`
	PromptEvalCnt int    `json:"prompt_eval_count"`
	EvalCnt       int    `json:"eval_count"`
}

// ollamaErrorResponse represents an Ollama API error response.
type ollamaErrorResponse struct {
	Error string `json:"error"`
}

// Send makes a completion request to the Ollama Chat API.
func (o *Ollama) Send(ctx context.Context, req *Request) (*Response, error) {
	var messages []Message
	if req.System != "" {
		messages = append(messages, Message{Role: "system", Content: req.System})
	}
	messages = append(messages, req.Messages...)

	body := ollamaRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   false,
	}

	// Build options only if needed
	var opts ollamaOptions
	hasOpts := false
	if req.Temperature != 0 {
		temp := req.Temperature
		opts.Temperature = &temp
		hasOpts = true
	}
	if req.MaxTokens != 0 {
		mt := req.MaxTokens
		opts.NumPredict = &mt
		hasOpts = true
	}
	if hasOpts {
		body.Options = &opts
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &ProviderError{
				Category: ErrCategoryTimeout,
				Message:  ctx.Err().Error(),
				Err:      ctx.Err(),
			}
		}
		// Check for connection refused
		if isConnectionRefused(err) {
			return nil, &ProviderError{
				Category: ErrCategoryServer,
				Message:  fmt.Sprintf("connection refused: is Ollama running? (expected at %s)", o.baseURL),
				Err:      err,
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

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, o.handleErrorResponse(httpResp.StatusCode, respBody)
	}

	var apiResp ollamaResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  fmt.Sprintf("failed to parse response: %s", err),
			Err:      err,
		}
	}

	if apiResp.Message.Content == "" {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  "response contains no content",
		}
	}

	return &Response{
		Content:      apiResp.Message.Content,
		Model:        apiResp.Model,
		InputTokens:  apiResp.PromptEvalCnt,
		OutputTokens: apiResp.EvalCnt,
		StopReason:   apiResp.DoneReason,
	}, nil
}

func (o *Ollama) handleErrorResponse(status int, body []byte) *ProviderError {
	message := http.StatusText(status)
	var errResp ollamaErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		message = errResp.Error
	}

	return &ProviderError{
		Category: o.mapStatusToCategory(status),
		Status:   status,
		Message:  message,
	}
}

func (o *Ollama) mapStatusToCategory(status int) ErrorCategory {
	switch status {
	case 400:
		return ErrCategoryBadRequest
	case 404:
		return ErrCategoryBadRequest
	case 500:
		return ErrCategoryServer
	default:
		return ErrCategoryServer
	}
}

// isConnectionRefused checks if the error is a connection refused error.
func isConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return strings.Contains(opErr.Err.Error(), "connection refused")
	}
	return strings.Contains(err.Error(), "connection refused")
}
