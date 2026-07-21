package provider

import (
	"fmt"
	"net/http"
)

const (
	// defaultEdenAIBaseURL is the default Eden AI API base URL. Eden AI is an
	// EU-based, OpenAI-compatible gateway, so the base already includes the /v3
	// version segment, mirroring how the OpenAI default includes /v1.
	defaultEdenAIBaseURL = "https://api.edenai.run/v3"
)

// NewEdenAI creates a new Eden AI provider. Eden AI exposes an OpenAI-compatible
// Chat Completions API, so it reuses the OpenAI provider implementation with a
// different base URL (the same thin-wrapper approach MiniMax uses over the
// Anthropic provider). Returns an error if apiKey is empty.
func NewEdenAI(apiKey string, opts ...OpenAIOption) (*OpenAI, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	o := &OpenAI{
		apiKey:  apiKey,
		baseURL: defaultEdenAIBaseURL,
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
