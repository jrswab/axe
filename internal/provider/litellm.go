package provider

import "strings"

const (
	defaultLiteLLMBaseURL = "http://localhost:4000/v1"

	// liteLLMCostHeader carries the proxy's computed cost (USD) for the request.
	liteLLMCostHeader = "x-litellm-response-cost"
)

// LiteLLM is an OpenAI-compatible provider for a LiteLLM Proxy (AI gateway),
// which fronts 100+ providers (Anthropic, Bedrock, Vertex AI, Azure, ...)
// behind one endpoint and virtual key. Model names are the proxy's aliases,
// e.g. "litellm/claude-sonnet" in agent TOML.
type LiteLLM struct {
	*OpenAI
}

// NewLiteLLM creates a LiteLLM provider. baseURL defaults to a local proxy;
// the /v1 suffix is added when omitted so both URL forms work.
func NewLiteLLM(apiKey string, baseURL string) (*LiteLLM, error) {
	if apiKey == "" {
		return nil, &ProviderError{
			Category: ErrCategoryAuth,
			Message:  "LiteLLM API key is required; set LITELLM_API_KEY to the proxy's virtual or master key",
		}
	}

	openAI, err := NewOpenAI(
		apiKey,
		WithOpenAIBaseURL(normalizeLiteLLMBaseURL(baseURL)),
		withOpenAICostHeader(liteLLMCostHeader),
	)
	if err != nil {
		return nil, err
	}

	return &LiteLLM{OpenAI: openAI}, nil
}

func normalizeLiteLLMBaseURL(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return defaultLiteLLMBaseURL
	}
	if strings.HasSuffix(trimmed, "/v1") {
		return trimmed
	}
	return trimmed + "/v1"
}
