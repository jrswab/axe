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
	// defaultOpenAIBaseURL is the default OpenAI API base URL.
	defaultOpenAIBaseURL = "https://api.openai.com"
)

// OpenAIOption is a functional option for configuring the OpenAI provider.
type OpenAIOption func(*OpenAI)

// WithOpenAIBaseURL sets a custom base URL for the OpenAI provider.
func WithOpenAIBaseURL(url string) OpenAIOption {
	return func(o *OpenAI) {
		o.baseURL = url
	}
}

// OpenAI implements the Provider interface for the OpenAI Chat Completions API.
type OpenAI struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewOpenAI creates a new OpenAI provider. Returns an error if apiKey is empty.
func NewOpenAI(apiKey string, opts ...OpenAIOption) (*OpenAI, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	o := &OpenAI{
		apiKey:  apiKey,
		baseURL: defaultOpenAIBaseURL,
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

// openaiRequest is the JSON body sent to the OpenAI Chat Completions API.
type openaiRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
}

// openaiResponse represents the JSON response from the OpenAI Chat Completions API.
type openaiResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// openaiErrorResponse represents an OpenAI API error response.
type openaiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Send makes a completion request to the OpenAI Chat Completions API.
func (o *OpenAI) Send(ctx context.Context, req *Request) (*Response, error) {
	var messages []Message
	if req.System != "" {
		messages = append(messages, Message{Role: "system", Content: req.System})
	}
	messages = append(messages, req.Messages...)

	body := openaiRequest{
		Model:    req.Model,
		Messages: messages,
	}

	if req.Temperature != 0 {
		temp := req.Temperature
		body.Temperature = &temp
	}

	if req.MaxTokens != 0 {
		mt := req.MaxTokens
		body.MaxTokens = &mt
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
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

	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  fmt.Sprintf("failed to parse response: %s", err),
			Err:      err,
		}
	}

	if len(apiResp.Choices) == 0 {
		return nil, &ProviderError{
			Category: ErrCategoryServer,
			Message:  "response contains no choices",
		}
	}

	return &Response{
		Content:      apiResp.Choices[0].Message.Content,
		Model:        apiResp.Model,
		InputTokens:  apiResp.Usage.PromptTokens,
		OutputTokens: apiResp.Usage.CompletionTokens,
		StopReason:   apiResp.Choices[0].FinishReason,
	}, nil
}

func (o *OpenAI) handleErrorResponse(status int, body []byte) *ProviderError {
	message := http.StatusText(status)
	var errResp openaiErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
	}

	return &ProviderError{
		Category: o.mapStatusToCategory(status),
		Status:   status,
		Message:  message,
	}
}

func (o *OpenAI) mapStatusToCategory(status int) ErrorCategory {
	switch status {
	case 401:
		return ErrCategoryAuth
	case 403:
		return ErrCategoryAuth
	case 400:
		return ErrCategoryBadRequest
	case 404:
		return ErrCategoryBadRequest
	case 429:
		return ErrCategoryRateLimit
	case 500, 502, 503:
		return ErrCategoryServer
	default:
		return ErrCategoryServer
	}
}
