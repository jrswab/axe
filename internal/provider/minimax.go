package provider

import (
	"fmt"
	"net/http"
)

const (
	// defaultMiniMaxBaseURL is the default MiniMax API base URL.
	defaultMiniMaxBaseURL = "https://api.minimax.io/anthropic"
)

// NewMiniMax creates a new MiniMax provider. Returns an error if apiKey is empty.
func NewMiniMax(apiKey string, opts ...AnthropicOption) (*Anthropic, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	a := &Anthropic{
		apiKey:  apiKey,
		baseURL: defaultMiniMaxBaseURL,
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
