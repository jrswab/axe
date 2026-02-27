package provider

import (
	"context"
	"fmt"
)

// ErrorCategory classifies provider errors for exit code mapping.
type ErrorCategory string

const (
	// ErrCategoryAuth indicates authentication failure (missing/invalid API key).
	ErrCategoryAuth ErrorCategory = "auth"
	// ErrCategoryRateLimit indicates the provider rate limited the request.
	ErrCategoryRateLimit ErrorCategory = "rate_limit"
	// ErrCategoryTimeout indicates the request timed out.
	ErrCategoryTimeout ErrorCategory = "timeout"
	// ErrCategoryOverloaded indicates the provider is overloaded (529).
	ErrCategoryOverloaded ErrorCategory = "overloaded"
	// ErrCategoryBadRequest indicates a malformed request (invalid model, etc.).
	ErrCategoryBadRequest ErrorCategory = "bad_request"
	// ErrCategoryServer indicates a provider server error (5xx).
	ErrCategoryServer ErrorCategory = "server"
)

// Message represents a single message in the conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request represents an LLM completion request.
type Request struct {
	Model       string
	System      string
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

// Response represents an LLM completion response.
type Response struct {
	Content      string
	Model        string
	InputTokens  int
	OutputTokens int
	StopReason   string
}

// Provider defines the interface for LLM providers.
type Provider interface {
	Send(ctx context.Context, req *Request) (*Response, error)
}

// ProviderError wraps provider-specific errors with categorization.
type ProviderError struct {
	Category ErrorCategory
	Status   int
	Message  string
	Err      error
}

// Error returns a formatted error message: "<category>: <message>".
func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s", e.Category, e.Message)
}

// Unwrap returns the wrapped error, supporting errors.Is and errors.As.
func (e *ProviderError) Unwrap() error {
	return e.Err
}
